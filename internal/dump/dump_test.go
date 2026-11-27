package dump

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// Golden files under testdata come from the TypeScript writeDumpsFromState
// (testdata/dump-ts.mts) at the clock below:
//
//	state-real   gen-state.mts over the cached public dump (1,434 jobs, 183
//	             companies, 402 runs: mixed timestamp spellings, delisted,
//	             stale, role-less, inactive and off-allowlist jobs, quotes,
//	             commas and newlines in CSV cells, null run companies,
//	             proprietary boards, zero-match and funnel errors), gzipped
//	ts-warn      TS MANIFEST.json for state-real, OPENINTERN_APPLY_HOSTS unset
//	ts-enforce   the same with OPENINTERN_APPLY_HOSTS=enforce
//	ts/<name>    TS output for internal/state/testdata/<name> (odd shapes:
//	             unknown and missing keys, lone surrogates, U+2028/9)
var goldenNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func optsFor(mode registry.ApplyHostMode) Options {
	reg := registry.Default()
	return Options{ApplyURLAllowed: func(slug, url string) bool {
		if mode == registry.ApplyHostsWarn {
			return true // warn mode publishes everything; it only logs
		}
		return reg.ApplyURLAllowed(slug, url, mode)
	}}
}

func loadGzState(t *testing.T, dir string) *state.State {
	t.Helper()
	tmp := t.TempDir()
	for _, name := range []string{state.CompaniesFile, state.JobsFile, state.LastSeenFile, state.RunsFile} {
		f, err := os.Open(filepath.Join(dir, name+".gz"))
		if err != nil {
			t.Fatal(err)
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(zr)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), b, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	st, err := state.Load(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

type manifestEntry struct {
	sha  string
	size int
}

func readManifest(t *testing.T, path string) (map[string]manifestEntry, []string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsjson.Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	files, _ := v.(jsjson.Object).Get("files")
	out := map[string]manifestEntry{}
	var order []string
	for _, m := range files.(jsjson.Object) {
		sha, _ := m.Value.(jsjson.Object).Get("sha256")
		n, _ := m.Value.(jsjson.Object).Get("bytes")
		out[m.Key] = manifestEntry{sha.(string), int(n.(float64))}
		order = append(order, m.Key)
	}
	return out, order
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// checkAgainstManifest compares every file with the TypeScript manifest;
// .gz files must decompress to the file they compress.
func checkAgainstManifest(t *testing.T, out, manifestPath string) {
	t.Helper()
	want, order := readManifest(t, manifestPath)
	got, gotOrder := readManifest(t, filepath.Join(out, "MANIFEST.json"))
	if len(order) != len(gotOrder) {
		t.Fatalf("manifest files %v, want %v", gotOrder, order)
	}
	for i, name := range order {
		if gotOrder[i] != name {
			t.Errorf("manifest order %v, want %v", gotOrder, order)
		}
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if g := got[name]; g.sha != sum(b) || g.size != len(b) {
			t.Errorf("%s: manifest entry does not describe the file", name)
		}
		if filepath.Ext(name) == ".gz" {
			plain, err := os.ReadFile(filepath.Join(out, name[:len(name)-3]))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gunzip(t, b), plain) {
				t.Errorf("%s does not decompress to %s", name, name[:len(name)-3])
			}
			continue
		}
		if want[name].sha != sum(b) {
			t.Errorf("%s differs from the TypeScript output", name)
		}
	}
	// The rest of MANIFEST.json (generatedAt, counts, byAts) must match too:
	// compare with the file hashes blanked.
	blank := func(path string) []byte {
		b, _ := os.ReadFile(path)
		v, _ := jsjson.Parse(string(b))
		o := v.(jsjson.Object)
		for i := range o {
			if o[i].Key == "files" {
				o[i].Value = nil
			}
		}
		return jsjson.MarshalIndent(o, "  ")
	}
	if g, w := blank(filepath.Join(out, "MANIFEST.json")), blank(manifestPath); !bytes.Equal(g, w) {
		t.Errorf("MANIFEST.json differs outside files:\n got %s\nwant %s", g, w)
	}
}

func TestRealStateMatchesTypeScript(t *testing.T) {
	st := loadGzState(t, "testdata/state-real")
	for _, tc := range []struct {
		mode     registry.ApplyHostMode
		manifest string
		count    int
	}{
		{registry.ApplyHostsWarn, "testdata/ts-warn/MANIFEST.json", 1336},
		{registry.ApplyHostsEnforce, "testdata/ts-enforce/MANIFEST.json", 1281},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			out := t.TempDir()
			res, err := FromState(st, out, goldenNow, optsFor(tc.mode))
			if err != nil {
				t.Fatal(err)
			}
			if res.Count != tc.count {
				t.Errorf("count %d, want %d", res.Count, tc.count)
			}
			checkAgainstManifest(t, out, tc.manifest)
		})
	}
}

func TestOddStatesMatchTypeScript(t *testing.T) {
	for _, name := range []string{"constructed", "raw-in", "export-out"} {
		t.Run(name, func(t *testing.T) {
			st, err := state.Load(filepath.Join("../state/testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			out := t.TempDir()
			if _, err := FromState(st, out, goldenNow, optsFor(registry.ApplyHostsWarn)); err != nil {
				t.Fatal(err)
			}
			for _, f := range []string{"jobs.json", "jobs.csv", "companies.json", "health.json", "denylist.json", "LICENSE-DATA.txt"} {
				got, _ := os.ReadFile(filepath.Join(out, f))
				want, err := os.ReadFile(filepath.Join("testdata/ts", name, f))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s differs\n got %q\nwant %q", f, got, want)
				}
			}
		})
	}
}

func TestCSVEscape(t *testing.T) {
	for in, want := range map[string]string{
		"plain":       "plain",
		"a,b":         `"a,b"`,
		`say "hi"`:    `"say ""hi"""`,
		"two\nlines":  "\"two\nlines\"",
		"carriage\rr": "carriage\rr", // only quote, comma and \n trigger quoting
	} {
		if got := csvEscape(in); got != want {
			t.Errorf("csvEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestManifestListsStrayFiles(t *testing.T) {
	// The TypeScript hashed whatever was in the directory; so does Go.
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "stray.txt"), []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := FromState(state.New(), out, goldenNow, optsFor(registry.ApplyHostsWarn)); err != nil {
		t.Fatal(err)
	}
	got, order := readManifest(t, filepath.Join(out, "MANIFEST.json"))
	if _, ok := got["stray.txt"]; !ok {
		t.Errorf("stray file missing from manifest: %v", order)
	}
	b, _ := os.ReadFile(filepath.Join(out, "jobs.json"))
	if want := "{\n  \"generatedAt\": \"2026-11-14T12:00:00.000Z\",\n  \"count\": 0,\n  \"jobs\": []\n}"; string(b) != want {
		t.Errorf("empty jobs.json = %q", b)
	}
}

func TestInvalidDateFails(t *testing.T) {
	st, err := state.Load("../state/testdata/constructed")
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range st.Jobs {
		j.FirstSeenAt = "not a date"
	}
	if _, err := PublicRows(st, goldenNow, optsFor(registry.ApplyHostsWarn)); err == nil {
		t.Error("an unparseable first_seen_at must fail the dump, as toISOString threw")
	}
}
