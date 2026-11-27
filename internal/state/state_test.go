package state

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/internal/model"
)

// The directories under testdata were written by the TypeScript writeState
// (/tmp/openintern-pipeline/state-harness.ts, see the pipeline status file):
//
//	constructed  records with control characters, U+2028/9, <>&, emoji, lone
//	             surrogates, nulls and empty arrays, written by writeState
//	raw-in/out   hand-made files (unknown and missing keys, a null run
//	             company, odd number spellings, non-canonical term_years) and
//	             what loadState + writeState made of them
//	export-in/out  export-state.sh output and stateFromExport + writeState
func compareDirs(t *testing.T, got map[string][]byte, wantDir string) {
	t.Helper()
	for _, name := range []string{CompaniesFile, JobsFile, LastSeenFile, RunsFile} {
		want, err := os.ReadFile(filepath.Join(wantDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[name], want) {
			t.Errorf("%s/%s differs\n got: %q\nwant: %q", wantDir, name, got[name], want)
		}
	}
}

func TestTypeScriptStateRoundTrips(t *testing.T) {
	st, err := Load("testdata/constructed")
	if err != nil {
		t.Fatal(err)
	}
	compareDirs(t, Encode(st), "testdata/constructed")

	// And through the real writer.
	dir := filepath.Join(t.TempDir(), "state")
	if err := Write(dir, st); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range []string{CompaniesFile, JobsFile, LastSeenFile, RunsFile} {
		files[name], _ = os.ReadFile(filepath.Join(dir, name))
	}
	compareDirs(t, files, "testdata/constructed")
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(dir), ".state-*")); len(left) > 0 {
		t.Errorf("temporary directory left behind: %v", left)
	}
}

func TestLoaderToleranceMatchesTypeScript(t *testing.T) {
	st, err := Load("testdata/raw-in")
	if err != nil {
		t.Fatal(err)
	}
	compareDirs(t, Encode(st), "testdata/raw-out")
	if j := st.Jobs["50000000-0000-4000-8000-000000000005"]; j.LastSeenAt != "2026-10-23T12:00:00+00:00" {
		t.Errorf("short last_seen.tsv row falls back to first_seen_at, got %q", j.LastSeenAt)
	}
}

func TestFromExport(t *testing.T) {
	st, err := FromExport("testdata/export-in")
	if err != nil {
		t.Fatal(err)
	}
	compareDirs(t, Encode(st), "testdata/export-out")
	if _, err := FromExport(t.TempDir()); err == nil || !strings.HasPrefix(err.Error(), "no public.companies.jsonl in ") {
		t.Errorf("empty export: %v", err)
	}
}

// STATE_TS_REAL points at a state the TypeScript wrote from the cached public
// dump (state-harness.ts writes it to <root>/real).
func TestRealStateRoundTrips(t *testing.T) {
	dir := os.Getenv("STATE_TS_REAL")
	if dir == "" {
		t.Skip("STATE_TS_REAL not set")
	}
	st, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	compareDirs(t, Encode(st), dir)
	t.Logf("%d companies, %d jobs byte-identical", len(st.Companies), len(st.Jobs))
}

func TestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, JobsFile), []byte("\n{\"id\":\"a\"}\n\n{bad\n"), 0o644)
	if _, err := Load(dir); err == nil || err.Error() != "jobs.jsonl:2: invalid JSON" {
		t.Errorf("got %v", err)
	}
	st, err := Load(t.TempDir())
	if err != nil || len(st.Companies)+len(st.Jobs)+len(st.Runs) != 0 {
		t.Errorf("missing files are empty: %v", err)
	}
}

// state.test.ts (the parts that need no ingest; the rest is in
// internal/ingest): two active jobs with the same URL prefix in one company
// are refused, and so are the other unsound states.
func TestValidate(t *testing.T) {
	st, err := Load("testdata/constructed")
	if err != nil {
		t.Fatal(err)
	}
	if p := Validate(st); len(p) != 0 {
		t.Fatalf("sound state: %v", p)
	}
	a := st.Jobs["20000000-0000-4000-8000-000000000002"]
	clone := *a
	clone.ID = a.ID[:8] + "-ffff-4fff-8fff-ffffffffffff"
	clone.Fingerprint = "other"
	st.Jobs[clone.ID] = &clone
	if got := strings.Join(Validate(st), ","); !regexp.MustCompile(`share a URL prefix`).MatchString(got) {
		t.Errorf("got %q", got)
	}
	err = Write(t.TempDir(), st)
	if err == nil || !strings.HasPrefix(err.Error(), "refusing to write state:\n  active jobs 20000000-0000-4000-8000-000000000002 and 20000000-ffff-4fff-8fff-ffffffffffff share a URL prefix") {
		t.Errorf("got %v", err)
	}
	clone.IsActive = false
	clone.Fingerprint = a.Fingerprint
	clone.CompanyID = "nope"
	st.Companies["dup"] = &Company{Company: model.Company{ID: "dup", Slug: "alpha"}}
	want := []string{
		"duplicate company slug alpha",
		"job 20000000-ffff-4fff-8fff-ffffffffffff has unknown company nope",
		"duplicate fingerprint fp2",
	}
	if got := Validate(st); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestNewIDAndPrefix(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := NewID()
		if !re.MatchString(id) || seen[id] {
			t.Fatalf("bad or repeated id %q", id)
		}
		seen[id] = true
	}
	if URLPrefix("0a1b2c3d-0000-4000-8000-000000000001") != "0a1b2c3d" || URLPrefix("ab-c") != "abc" {
		t.Error("URLPrefix")
	}
}

func TestInPlaceChangesAreWritten(t *testing.T) {
	st, err := Load("testdata/raw-in")
	if err != nil {
		t.Fatal(err)
	}
	// Beta has no careers_url/website_url keys; a retired job lacking keys
	// gains is_active once it is set, as assigning the property did.
	j := st.Jobs["50000000-0000-4000-8000-000000000005"]
	j.Deactivate("2026-11-03T00:00:00+00:00")
	line := string(encodeJobLine(j))
	if !strings.Contains(line, `"is_active":false`) || strings.Contains(line, `"locations"`) {
		t.Errorf("got %s", line)
	}
}

func encodeJobLine(j *Job) []byte {
	return Encode(&State{Companies: map[string]*Company{}, Jobs: map[string]*Job{j.ID: j}})[JobsFile]
}
