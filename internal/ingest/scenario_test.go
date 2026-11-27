package ingest

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state"
)

// The parity scenario (testdata/gen-scenario.mts) runs the TypeScript
// runIngestFiles day after day with a fake fetcher and fake ids
// (testdata/ingest-ts.mts, driven by testdata/run-ts.mjs) and records the
// sha256 of every state file, the summary JSON, the digest and the step
// summary after each day (testdata/expected.json). Go runs the same days
// from the same start and must produce the same bytes.
//
//	empty       11 days from an empty state: registry sync, 2,178 postings on
//	            347 boards, refused/network/plain/manual-expired failures and
//	            a 4,900-character error, empty boards up to retirement,
//	            dropped postings, OPENINTERN_ONLY_SLUGS, enforce mode,
//	            skipped registry sync, auto-disable and recheck, outage gaps,
//	            the last-seen sweep, run retention and term staleness
//	state-real  2 days from internal/dump/testdata/state-real (existing jobs
//	            re-sighted, orphaned companies, odd timestamp spellings)
//
// Set INGEST_DEBUG_DIR to keep each day's Go output for diffing.

type scenarioDay struct {
	Now      string
	Env      map[string]string
	SkipSync bool
	Errors   map[string]struct {
		Kind    string
		Message string
		Status  int
	}
	Empty []string
	Drop  []string
}

type boardJob struct {
	ExternalID  string   `json:"externalId"`
	Title       string   `json:"title"`
	Locations   []string `json:"locations"`
	ApplyURL    string   `json:"applyUrl"`
	Excerpt     *string  `json:"excerpt"`
	PostedAt    *string  `json:"postedAt"`
	Description string   `json:"description"`
}

func readGz(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fakeFetcher(boards map[string][]boardJob, day scenarioDay) Fetcher {
	empty, drop := map[string]bool{}, map[string]bool{}
	for _, k := range day.Empty {
		empty[k] = true
	}
	for _, e := range day.Drop {
		drop[e] = true
	}
	return func(ctx context.Context, ats, token string) ([]model.NormalizedJob, error) {
		key := ats + "\t" + token
		if e, ok := day.Errors[key]; ok {
			if e.Kind == "" {
				return nil, errors.New(e.Message)
			}
			return nil, &fetch.Failure{Kind: fetch.FailureKind(e.Kind), Message: e.Message, Status: e.Status}
		}
		if empty[key] {
			return nil, nil
		}
		var out []model.NormalizedJob
		for _, j := range boards[key] {
			if drop[j.ExternalID] {
				continue
			}
			out = append(out, model.NormalizedJob{ExternalID: j.ExternalID, Title: j.Title, Locations: j.Locations,
				ApplyURL: j.ApplyURL, Excerpt: j.Excerpt, PostedAt: j.PostedAt, Description: j.Description})
		}
		return out, nil
	}
}

func shaHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestScenarioMatchesTypeScript(t *testing.T) {
	var boards map[string][]boardJob
	if err := json.Unmarshal(readGz(t, "testdata/boards.json.gz"), &boards); err != nil {
		t.Fatal(err)
	}
	var scenarios map[string][]scenarioDay
	var expected map[string][]map[string]string
	for path, dst := range map[string]any{"testdata/scenarios.json": &scenarios, "testdata/expected.json": &expected} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, dst); err != nil {
			t.Fatal(err)
		}
	}
	debug := os.Getenv("INGEST_DEBUG_DIR")
	for _, name := range []string{"empty", "state-real"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			if err := os.MkdirAll(dir, 0o777); err != nil {
				t.Fatal(err)
			}
			if name == "state-real" {
				for _, f := range []string{state.CompaniesFile, state.JobsFile, state.LastSeenFile, state.RunsFile} {
					if err := os.WriteFile(filepath.Join(dir, f), readGz(t, filepath.Join("../dump/testdata/state-real", f+".gz")), 0o666); err != nil {
						t.Fatal(err)
					}
				}
			}
			for d, day := range scenarios[name] {
				st, err := state.Load(dir)
				if err != nil {
					t.Fatal(err)
				}
				now, err := time.Parse(time.RFC3339Nano, day.Now)
				if err != nil {
					t.Fatal(err)
				}
				n := 0x10000000 + d*0x00100000
				if name != "empty" {
					n += 0x40000000
				}
				opts := Options{
					Fetch:             fakeFetcher(boards, day),
					Now:               now,
					NewID:             func() string { n++; return fmt.Sprintf("%x-0000-4000-8000-000000000000", n-1) },
					SkipRegistrySync:  day.SkipSync,
					Only:              OnlySlugs(day.Env["OPENINTERN_ONLY_SLUGS"]),
					EnforceApplyHosts: day.Env["OPENINTERN_APPLY_HOSTS"] == "enforce",
				}
				sum, err := Run(context.Background(), st, opts)
				if err != nil {
					t.Fatalf("day %d: %v", d, err)
				}
				if err := state.Write(dir, st); err != nil {
					t.Fatalf("day %d: %v", d, err)
				}
				got := map[string][]byte{
					"summary.json": sum.JSON(),
					"digest.md":    []byte(Digest(sum)),
					"step.md":      []byte(StepSummary(sum)),
				}
				for _, f := range []string{state.CompaniesFile, state.JobsFile, state.LastSeenFile, state.RunsFile} {
					got[f], _ = os.ReadFile(filepath.Join(dir, f))
				}
				if debug != "" {
					out := filepath.Join(debug, name, fmt.Sprint(d))
					os.MkdirAll(out, 0o777)
					for f, b := range got {
						os.WriteFile(filepath.Join(out, f), b, 0o666)
					}
				}
				bad := false
				for f, b := range got {
					if shaHex(b) != expected[name][d][f] {
						t.Errorf("day %d (%s): %s differs from the TypeScript", d, day.Now, f)
						bad = true
					}
				}
				if bad {
					t.FailNow()
				}
			}
		})
	}
}

func TestOnlySlugs(t *testing.T) {
	if OnlySlugs("  ") != nil {
		t.Error("blank means everyone")
	}
	got := OnlySlugs(" a , b,,c ")
	if len(got) != 3 || !got["a"] || !got["b"] || !got["c"] {
		t.Errorf("OnlySlugs = %v", got)
	}
}
