package logos

import (
	"bytes"
	"image/png"
	"io/fs"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/data"
	"github.com/dnexdev/openintern/internal/registry"
)

// The committed logos are what the site serves: each must be a Size×Size PNG
// that shows on the white logo ground, named for a registry slug and listed
// in SOURCES.tsv, and SOURCES.tsv must not list files that are gone.
func TestCommittedLogos(t *testing.T) {
	reg := registry.Default()
	entries, err := fs.ReadDir(data.Logos, "logos")
	if err != nil {
		t.Fatal(err)
	}
	src, err := fs.ReadFile(data.Logos, "logos/SOURCES.tsv")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for i, line := range strings.Split(strings.TrimRight(string(src), "\n"), "\n") {
		f := strings.Split(line, "\t")
		if i == 0 {
			if line != "slug\tsource\tfetched" {
				t.Fatalf("SOURCES.tsv header %q", line)
			}
			continue
		}
		if len(f) != 3 || f[0] == "" || f[1] == "" || len(f[2]) != len("2006-01-02") {
			t.Errorf("SOURCES.tsv line %d: %q", i+1, line)
			continue
		}
		if f[1] != "manual" && !strings.HasPrefix(f[1], "https://") && !strings.HasPrefix(f[1], "http://") {
			t.Errorf("SOURCES.tsv %s: source %q", f[0], f[1])
		}
		sources[f[0]] = f[1]
	}
	files := 0
	for _, e := range entries {
		name := e.Name()
		if name == "README.md" || name == "SOURCES.tsv" {
			continue
		}
		slug, ok := strings.CutSuffix(name, ".png")
		if !ok || e.IsDir() {
			t.Errorf("logos/%s: only <slug>.png, README.md and SOURCES.tsv belong here", name)
			continue
		}
		files++
		if _, ok := reg.Company(slug); !ok {
			t.Errorf("logos/%s: no company %q in data/companies", name, slug)
		}
		if _, ok := sources[slug]; !ok {
			t.Errorf("logos/%s: not in SOURCES.tsv", name)
		}
		b, err := fs.ReadFile(data.Logos, "logos/"+name)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil || cfg.Width != Size || cfg.Height != Size {
			t.Errorf("logos/%s: %v, %dx%d (want a %dx%d PNG)", name, err, cfg.Width, cfg.Height, Size, Size)
		}
		if img, err := png.Decode(bytes.NewReader(b)); err == nil {
			if err := Check(img); err != nil {
				t.Errorf("logos/%s: %v", name, err)
			}
		}
		if len(b) > 32<<10 {
			t.Errorf("logos/%s: %d bytes; rerun openintern logos to re-encode it", name, len(b))
		}
	}
	for slug := range sources {
		if _, err := fs.Stat(data.Logos, "logos/"+slug+".png"); err != nil {
			t.Errorf("SOURCES.tsv lists %s but logos/%s.png is missing", slug, slug)
		}
	}
	if files == 0 {
		t.Fatal("no committed logos")
	}
}
