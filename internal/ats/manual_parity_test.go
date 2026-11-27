package ats

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// testdata/manual holds saved careers pages (two Citadel listing pages with
// an overlapping job, a Tesla state document with every listing shape) and
// what the TypeScript produced from them (testdata/ts/manual-goldens.mts,
// TZ=UTC): the parsed jobs, the capture file manual-capture.ts writes, and
// the jobs loadManualJobs reads back a day later. Go must match all three.
func TestManualCaptureMatchesTypeScript(t *testing.T) {
	saved := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = saved })

	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join("testdata", "manual", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	readJSON := func(name string) any {
		t.Helper()
		v, err := jsjson.Parse(jsjson.DecodeUTF8(read(name)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	compareJobs := func(t *testing.T, got []model.NormalizedJob, want any) {
		t.Helper()
		w := array(want)
		if len(got) != len(w) {
			t.Fatalf("%d jobs, want %d", len(got), len(w))
		}
		for i := range w {
			compareJob(t, i, got[i], w[i])
		}
	}

	// manual-capture.ts: pages in order, first sighting of an id wins.
	var citadel []model.NormalizedJob
	seen := map[string]bool{}
	for _, page := range []string{"citadel-page1.html", "citadel-page2.html"} {
		for _, j := range ParseCitadelListingHTML(jsjson.DecodeUTF8(read(page)), CitadelOrigins["citadel"]) {
			if !seen[j.ExternalID] {
				seen[j.ExternalID] = true
				citadel = append(citadel, j)
			}
		}
	}
	tesla := ParseTeslaCareersState(readJSON("tesla-state.json"))

	parsed := readJSON("parsed.golden.json")
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name   string
		jobs   []model.NormalizedJob
		source string
	}{
		{"citadel", citadel, CitadelListingURL("citadel", "open-opportunities", 1)},
		{"tesla", tesla, TeslaStateURL},
	} {
		t.Run(c.name, func(t *testing.T) {
			compareJobs(t, c.jobs, get(parsed, c.name))

			capture, err := ToManualCapture(c.jobs, c.source, now, "")
			if err != nil {
				t.Fatal(err)
			}
			file := capture.MarshalFile()
			if want := read(c.name + ".capture.golden.json"); string(file) != string(want) {
				t.Fatalf("capture file differs:\n%s\nwant:\n%s", file, want)
			}

			dir := fstest.MapFS{c.name + ".json": {Data: file}}
			loaded, err := LoadManualJobs(c.name, ManualOptions{Dir: dir, Now: now.Add(24 * time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			compareJobs(t, loaded, readJSON(c.name+".jobs.golden.json"))
		})
	}
}
