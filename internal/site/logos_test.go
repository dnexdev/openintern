package site

import (
	"bytes"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/jobslug"
)

func TestInitials(t *testing.T) {
	cases := map[string]string{
		"Stripe":                 "ST",
		"Jane Street":            "JS",
		"The Boeing Company":     "BC",
		"The Campbell's Company": "CC",
		"Lawrence Livermore National Laboratory (LLNL)": "LL",
		"pony.ai":      "PA",
		"1Password":    "1P",
		"d.e. shaw":    "DE",
		"Ünited Ärzte": "ÜÄ",
		"":             "?",
		"   ":          "?",
		"&&":           "?",
		"X":            "X",
	}
	for name, want := range cases {
		if got := initials(name); got != want {
			t.Errorf("initials(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestMark(t *testing.T) {
	m := mark("stripe", "Stripe")
	if !regexp.MustCompile(`^/logos/stripe\.png\?v=[0-9a-f]{10}$`).MatchString(m.Src) {
		t.Errorf("stripe logo src = %q", m.Src)
	}
	if m := mark("no-such-company", "No Such Co"); m.Src != "" || m.Initials != "NS" {
		t.Errorf("fallback mark = %+v", m)
	}
}

var (
	logoImg     = regexp.MustCompile(`<img class="mark" src="(/logos/[a-z0-9-]+\.png\?v=[0-9a-f]+)" alt="" width="24" height="24"`)
	initialsTag = regexp.MustCompile(`<span class="mark mark-initials" aria-hidden="true">[^<]+</span>`)
	anyImg      = regexp.MustCompile(`<img\b[^>]*>`)
)

// Every company row on every page that lists companies carries a mark, and
// every logo URL serves a 64x64 PNG from this origin.
func TestPagesShowCompanyMarks(t *testing.T) {
	jobs := fixtureJobs(t)
	srv := newServer(t, "testdata/jobs.json", fixtureNow, board.Options{})
	list := get(t, srv, "/jobs")
	rows := strings.Count(list.body, `<li class="row"`)
	marks := len(logoImg.FindAllString(list.body, -1)) + len(initialsTag.FindAllString(list.body, -1))
	if rows == 0 || marks != rows {
		t.Fatalf("/jobs: %d rows, %d company marks", rows, marks)
	}
	home := get(t, srv, "/")
	if n := strings.Count(home.body, `<li class="row-compact">`); n == 0 || len(logoImg.FindAllString(home.body, -1))+len(initialsTag.FindAllString(home.body, -1)) != n {
		t.Errorf("home: %d preview rows without one mark each", n)
	}
	detail := get(t, srv, jobslug.JobPath(jobs[0].Company.Slug, jobs[0].Title, jobs[0].ID))
	if !strings.Contains(detail.body, `<p class="record-company"><img class="mark"`) && !strings.Contains(detail.body, `<p class="record-company"><span class="mark mark-initials"`) {
		t.Errorf("job detail has no company mark")
	}
	seen := map[string]bool{}
	for _, body := range []string{list.body, home.body, detail.body} {
		for _, tag := range anyImg.FindAllString(body, -1) {
			if !logoImg.MatchString(tag) {
				t.Errorf("unexpected image tag %s", tag)
			}
		}
		for _, m := range logoImg.FindAllStringSubmatch(body, -1) {
			seen[m[1]] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("no logos on the fixture pages; the fixture companies all have committed logos")
	}
	for src := range seen {
		r := get(t, srv, src)
		if r.status != 200 || r.header.Get("Content-Type") != "image/png" || r.header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: %d %q", src, r.status, r.header.Get("Content-Type"))
			continue
		}
		if r.header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("%s: cache %q", src, r.header.Get("Cache-Control"))
		}
		cfg, err := png.DecodeConfig(bytes.NewReader([]byte(r.body)))
		if err != nil || cfg.Width != 64 || cfg.Height != 64 {
			t.Errorf("%s: %v %dx%d", src, err, cfg.Width, cfg.Height)
		}
	}
}

// An old slug in a dump still finds the merged company's logo.
func TestMarkFollowsCompanyAliases(t *testing.T) {
	h := &handler{store: aliasReader{}}
	if m := h.companyMark("old-tmx", "TMX Group"); !strings.HasPrefix(m.Src, "/logos/tmx-group.png?v=") {
		t.Errorf("alias mark = %+v", m)
	}
}

type aliasReader struct{ reader }

func (aliasReader) CanonicalCompanySlug(slug string) string {
	if slug == "old-tmx" {
		return "tmx-group"
	}
	return slug
}

func TestLogoRoute(t *testing.T) {
	srv := newServer(t, "testdata/jobs.json", fixtureNow, board.Options{})
	if r := get(t, srv, "/logos/stripe.png"); r.status != 200 || r.header.Get("Cache-Control") != "public, max-age=86400" {
		t.Errorf("bare logo URL: %d %q", r.status, r.header.Get("Cache-Control"))
	}
	for _, p := range []string{"/logos/no-such-company.png", "/logos/README.md", "/logos/SOURCES.tsv", "/logos/Stripe.png", "/logos/stripe.svg", "/logos/..%2fembed.go"} {
		if r := get(t, srv, p); r.status != 404 {
			t.Errorf("%s: %d, want 404", p, r.status)
		}
	}
}
