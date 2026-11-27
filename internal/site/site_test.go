package site

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/jobslug"
	"github.com/dnexdev/openintern/internal/model"
)

// fixtureNow is one day after the fixture dump was generated, so the data is
// fresh and its jobs are public.
var fixtureNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func newServer(t *testing.T, dump string, now time.Time, opts board.Options) *httptest.Server {
	t.Helper()
	opts.DumpSource = dump
	opts.HealthSource = dump + ".missing-health.json"
	opts.KillSwitchSource = dump + ".missing-kill-switch.json"
	opts.Now = func() time.Time { return now }
	store := board.NewStore(opts)
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(Static())))
	register(mux, store, options{now: func() time.Time { return now }})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type response struct {
	status int
	header http.Header
	body   string
}

func get(t *testing.T, srv *httptest.Server, path string) response {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return response{res.StatusCode, res.Header, string(b)}
}

// checkPage asserts the things every rendered page must satisfy.
func checkPage(t *testing.T, path string, r response) {
	t.Helper()
	for _, bad := range []string{"ZgotmplZ", "<no value>", "%!"} {
		if strings.Contains(r.body, bad) {
			i := strings.Index(r.body, bad)
			t.Errorf("%s: output contains %q near %q", path, bad, r.body[max(0, i-80):min(len(r.body), i+40)])
		}
	}
	if !strings.HasPrefix(r.body, "<!doctype html>") || !strings.HasSuffix(strings.TrimSpace(r.body), "</html>") {
		t.Errorf("%s: not a complete document", path)
	}
	for _, want := range []string{`<html lang="en">`, `<main id="main"`, `class="masthead"`, `class="colophon"`, `href="/assets/site.css?v=`, `src="/assets/site.js?v=`, `rel="icon" href="/icon.svg"`} {
		if !strings.Contains(r.body, want) {
			t.Errorf("%s: missing %q", path, want)
		}
	}
	if ct := r.header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("%s: content type %q", path, ct)
	}
}

func title(body string) string {
	m := regexp.MustCompile(`<title>([^<]*)</title>`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}

func TestPagesRender(t *testing.T) {
	srv := newServer(t, "testdata/jobs.json", fixtureNow, board.Options{})
	cases := []struct {
		path, title, canonical, cache string
		status                        int
		contains                      []string
	}{
		{"/", "OpenIntern — free open-source tech internships", "", cachePublic, 200,
			[]string{"Tech internships, open by default", "active roles from", `data-copy="curl-cmd"`, "A few roles on the board now"}},
		{"/jobs", "Jobs · OpenIntern", "https://openintern.dev/jobs", cacheDynamic, 200,
			[]string{`<form class="filter-form" id="filters" action="/jobs" method="get"`, `name="role" value="ml"`, `name="duration" value="12"`, `<option value="prestige">Prestige sort</option>`, `class="rows"`, `id="job-aliases"`, "Report issue"}},
		{"/jobs?role=ml&region=us&term=summer&duration=4&sort=posted&q=intern", "Jobs · OpenIntern", "https://openintern.dev/jobs", cacheDynamic, 200,
			[]string{`value="ml" checked`, `value="us" checked`, `value="summer" checked`, `value="4" checked`, `<option value="posted" selected>`, `value="intern"`, "matching filters", `<a href="/jobs" data-board-link>Clear filters</a>`}},
		{"/jobs?q=zzzz-nothing", "Jobs · OpenIntern", "https://openintern.dev/jobs", cacheDynamic, 200,
			[]string{"No active internships matched."}},
		{"/docs", "API &amp; dumps · OpenIntern", "https://openintern.dev/docs", cachePublic, 200,
			[]string{"/api/v1/status", "duration_months", "denylist.json", "default 27, max 100", "60 requests per minute"}},
		{"/health", "Ingest health · OpenIntern", "", cachePublic, 200,
			[]string{"No pipeline snapshot published yet; counts come from the public dump."}},
		{"/privacy", "Privacy · OpenIntern", "", cachePublic, 200, []string{"localStorage", "DATA.md"}},
		{"/vs/intern-insider", "OpenIntern vs Intern Insider · OpenIntern", "https://openintern.dev/vs/intern-insider", cachePublic, 200,
			[]string{"Recruiter email finder", "Sold as a feature"}},
		{"/no/such/page", "Page not found · OpenIntern", "", cachePublic, 404, []string{`<meta name="robots" content="noindex">`}},
		{"/jobs/not-a-uuid", "Page not found · OpenIntern", "", cachePublic, 404, nil},
		{"/jobs/tmx-group/no-id-here", "Page not found · OpenIntern", "", cachePublic, 404, nil},
	}
	for _, c := range cases {
		r := get(t, srv, c.path)
		if r.status != c.status {
			t.Errorf("%s: status %d, want %d", c.path, r.status, c.status)
		}
		checkPage(t, c.path, r)
		if got := title(r.body); got != c.title {
			t.Errorf("%s: title %q, want %q", c.path, got, c.title)
		}
		canonical := regexp.MustCompile(`<link rel="canonical" href="([^"]*)">`).FindStringSubmatch(r.body)
		switch {
		case c.canonical == "" && canonical != nil:
			t.Errorf("%s: unexpected canonical %q", c.path, canonical[1])
		case c.canonical != "" && (canonical == nil || canonical[1] != c.canonical):
			t.Errorf("%s: canonical %v, want %q", c.path, canonical, c.canonical)
		}
		if got := r.header.Get("Cache-Control"); got != c.cache {
			t.Errorf("%s: cache %q, want %q", c.path, got, c.cache)
		}
		for _, want := range c.contains {
			if !strings.Contains(r.body, want) {
				t.Errorf("%s: missing %q", c.path, want)
			}
		}
		if strings.Contains(r.body, "The daily refresh is late") {
			t.Errorf("%s: stale notice shown for a fresh dump", c.path)
		}
	}
}

func fixtureJobs(t *testing.T) []struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Company struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"company"`
} {
	t.Helper()
	raw, err := os.ReadFile("testdata/jobs.json")
	if err != nil {
		t.Fatal(err)
	}
	var dump struct {
		Jobs []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Company struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"company"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &dump); err != nil {
		t.Fatal(err)
	}
	return dump.Jobs
}

func TestJobDetailAndRedirects(t *testing.T) {
	jobs := fixtureJobs(t)
	srv := newServer(t, "testdata/jobs.json", fixtureNow, board.Options{
		CanonicalCompanySlug: func(slug string) string {
			if slug == "old-tmx" {
				return "tmx-group"
			}
			return slug
		},
		JobAlias: func(id string) string {
			if id == "00000000-0000-4000-8000-000000000000" {
				return jobs[0].ID
			}
			return ""
		},
		JobAliasByPrefix: func(prefix string) string {
			if prefix == "0badc0de" {
				return jobs[0].ID
			}
			return ""
		},
	})
	seen := 0
	for _, job := range jobs {
		path := jobslug.JobPath(job.Company.Slug, job.Title, job.ID)
		r := get(t, srv, path)
		if r.status != 200 {
			// Jobs that are not public at fixtureNow are skipped by the store.
			continue
		}
		seen++
		checkPage(t, path, r)
		wantTitle := strings.NewReplacer("&", "&amp;", "'", "&#39;", `"`, "&#34;", "<", "&lt;", ">", "&gt;").Replace(job.Title + " at " + job.Company.Name + " · OpenIntern")
		if got := title(r.body); got != wantTitle {
			t.Errorf("%s: title %q, want %q", path, got, wantTitle)
		}
		for _, want := range []string{`<link rel="canonical" href="https://openintern.dev` + hrefEscape(path) + `">`, `<meta property="og:type" content="article">`, "Apply on employer site", `data-applied="` + job.ID + `"`, `class="facts"`} {
			if !strings.Contains(r.body, want) {
				t.Errorf("%s: missing %q", path, want)
			}
		}
	}
	if seen < 50 {
		t.Fatalf("only %d fixture jobs rendered", seen)
	}

	first := jobs[0]
	slug := jobslug.JobSlugSegment(first.Title, first.ID)
	if r := get(t, srv, "/jobs/"+first.ID); r.status != http.StatusTemporaryRedirect || r.header.Get("Location") != jobslug.JobPath(first.Company.Slug, first.Title, first.ID) {
		t.Errorf("legacy uuid: %d %q", r.status, r.header.Get("Location"))
	}
	if r := get(t, srv, "/jobs/00000000-0000-4000-8000-000000000000"); r.status != http.StatusPermanentRedirect || r.header.Get("Location") != "/jobs/"+first.ID {
		t.Errorf("aliased uuid: %d %q", r.status, r.header.Get("Location"))
	}
	if r := get(t, srv, "/jobs/11111111-1111-4111-8111-111111111111"); r.status != 404 {
		t.Errorf("unknown uuid: %d", r.status)
	}
	if r := get(t, srv, "/jobs/old-tmx/"+slug); r.status != http.StatusPermanentRedirect || r.header.Get("Location") != "/jobs/tmx-group/"+slug {
		t.Errorf("company alias: %d %q", r.status, r.header.Get("Location"))
	}
	if r := get(t, srv, "/jobs/tmx-group/gone-0badc0de"); r.status != http.StatusPermanentRedirect || r.header.Get("Location") != "/jobs/"+first.ID {
		t.Errorf("job alias by prefix: %d %q", r.status, r.header.Get("Location"))
	}
}

func TestBoardPagination(t *testing.T) {
	srv := newServer(t, "testdata/jobs.json", fixtureNow, board.Options{})
	r := get(t, srv, "/jobs?role=software&page=2")
	if !strings.Contains(r.body, `aria-current="page" aria-label="Page 2"`) {
		t.Errorf("page 2 not marked current")
	}
	if !strings.Contains(r.body, `href="/jobs?role=software&amp;page=1" rel="prev"`) {
		t.Errorf("previous link keeps filters: %s", regexp.MustCompile(`<nav class="pager"[\s\S]*?</nav>`).FindString(r.body))
	}
	rows := strings.Count(r.body, `<li class="row"`)
	if rows == 0 || rows > boardPageSize {
		t.Errorf("rows on page 2: %d", rows)
	}
}

func TestPageWindow(t *testing.T) {
	href := func(n int) string { return "" }
	render := func(links []pageLink) string {
		var b strings.Builder
		for _, l := range links {
			switch {
			case l.Gap:
				b.WriteString("… ")
			case l.Current:
				b.WriteString("[" + itoa(l.N) + "] ")
			default:
				b.WriteString(itoa(l.N) + " ")
			}
		}
		return strings.TrimSpace(b.String())
	}
	for _, c := range []struct {
		current, total int
		want           string
	}{
		{1, 1, ""},
		{1, 3, "[1] 2 3"},
		{1, 49, "[1] 2 3 … 49"},
		{10, 49, "1 … 8 9 [10] 11 12 … 49"},
		{49, 49, "1 … 47 48 [49]"},
		{4, 7, "1 2 3 [4] 5 6 7"},
	} {
		if got := render(pageWindow(c.current, c.total, href)); got != c.want {
			t.Errorf("pageWindow(%d, %d) = %q, want %q", c.current, c.total, got, c.want)
		}
	}
}

func itoa(n int) string { return formatInt(n) }

func TestLabels(t *testing.T) {
	if got := durationLabel([]int{6, 4, 5}); got != "4–6 mo" {
		t.Errorf("contiguous: %q", got)
	}
	if got := durationLabel([]int{6, 3}); got != "3/6 mo" {
		t.Errorf("gapped: %q", got)
	}
	if got := durationLabel([]int{4}); got != "4 mo" {
		t.Errorf("single: %q", got)
	}
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	for iso, want := range map[string]string{
		"2026-11-13T23:59:30.000Z": "30s ago",
		"2026-11-13T23:00:00.000Z": "1h ago",
		"2026-11-13T23:00:01.000Z": "59m ago",
		"2026-11-13T20:00:00.000Z": "4h ago",
		"2026-11-01T00:00:00.000Z": "13d ago",
	} {
		if got := timeAgo(iso, now); got != want {
			t.Errorf("timeAgo(%s) = %q, want %q", iso, got, want)
		}
	}
	if got := formatInt(1234567); got != "1,234,567" {
		t.Errorf("formatInt: %q", got)
	}
	if got := truncateUTF16("a😀b", 2); got != "a" {
		t.Errorf("truncateUTF16 kept half a surrogate pair: %q", got)
	}
	if got := regionLabel("us"); got != "US" {
		t.Errorf("regionLabel(us) = %q", got)
	}
}

func TestStaleNotice(t *testing.T) {
	late := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	srv := newServer(t, "testdata/jobs.json", late, board.Options{
		IsPublic: func(_ model.DumpJob, _ time.Time) bool { return true },
	})
	r := get(t, srv, "/jobs")
	if !strings.Contains(r.body, "The daily refresh is late.") || !strings.Contains(r.body, "<strong>2d ago</strong>") {
		t.Errorf("stale notice missing on /jobs")
	}
}

func TestUnavailableData(t *testing.T) {
	srv := newServer(t, "testdata/does-not-exist.json", fixtureNow, board.Options{})
	if r := get(t, srv, "/jobs"); r.status != 200 || !strings.Contains(r.body, "The listings could not be loaded right now.") {
		t.Errorf("/jobs without data: %d", r.status)
	}
	if r := get(t, srv, "/"); r.status != 200 || strings.Contains(r.body, "active roles from") {
		t.Errorf("home without data: %d", r.status)
	}
	if r := get(t, srv, "/jobs/tmx-group/intern-bfee3da0"); r.status != http.StatusServiceUnavailable || !strings.Contains(r.body, "Listings unavailable") {
		t.Errorf("job detail without data: %d", r.status)
	}
	if r := get(t, srv, "/health"); !strings.Contains(r.body, "Health data unavailable.") {
		t.Errorf("health without data")
	}
}

func TestAssetsAndIcons(t *testing.T) {
	srv := newServer(t, "testdata/jobs.json", fixtureNow, board.Options{})
	for _, name := range []string{"site.css", "site.js", "icon.svg", "og.png", "fonts/AtkinsonHyperlegibleNext-wght.woff2", "fonts/AtkinsonHyperlegibleMono-wght.woff2", "fonts/OFL-AtkinsonHyperlegibleNext.txt", "fonts/OFL-AtkinsonHyperlegibleMono.txt"} {
		if _, err := fs.Stat(Static(), name); err != nil {
			t.Errorf("static %s: %v", name, err)
		}
		if r := get(t, srv, "/assets/"+name); r.status != 200 {
			t.Errorf("/assets/%s: %d", name, r.status)
		}
	}
	if u := assetURL("site.css"); !regexp.MustCompile(`^/assets/site\.css\?v=[0-9a-f]{10}$`).MatchString(u) {
		t.Errorf("assetURL: %q", u)
	}

	for _, p := range []string{"/opengraph-image", "/twitter-image"} {
		r := get(t, srv, p)
		if r.status != 200 || r.header.Get("Content-Type") != "image/png" {
			t.Fatalf("%s: %d %s", p, r.status, r.header.Get("Content-Type"))
		}
		cfg, err := png.DecodeConfig(strings.NewReader(r.body))
		if err != nil || cfg.Width != 1200 || cfg.Height != 630 {
			t.Errorf("%s: %v %dx%d", p, err, cfg.Width, cfg.Height)
		}
	}

	r := get(t, srv, "/icon.svg")
	if r.status != 200 || r.header.Get("Content-Type") != "image/svg+xml" || !strings.Contains(r.body, "<svg") {
		t.Errorf("/icon.svg: %d %s", r.status, r.header.Get("Content-Type"))
	}

	r = get(t, srv, "/favicon.ico")
	body := []byte(r.body)
	if r.status != 200 || len(body) < 22 || binary.LittleEndian.Uint16(body[2:]) != 1 || binary.LittleEndian.Uint16(body[4:]) != 1 {
		t.Fatalf("/favicon.ico: bad header")
	}
	offset := binary.LittleEndian.Uint32(body[18:])
	cfg, err := png.DecodeConfig(bytes.NewReader(body[offset:]))
	if err != nil || cfg.Width != 32 {
		t.Errorf("/favicon.ico image: %v %d", err, cfg.Width)
	}
}

// TestRealDump renders every board page and a sample of job pages over the
// cached public dump, when it is present (see PLAN.md).
func TestRealDump(t *testing.T) {
	const dump = "../../.cache/dump/jobs.json"
	if _, err := os.Stat(dump); err != nil {
		t.Skip("no cached dump")
	}
	// The cached dump predates later company merges in the registry; with
	// identity slugs the test checks rendering, not alias drift (aliases are
	// covered above).
	srv := newServer(t, dump, fixtureNow, board.Options{
		CanonicalCompanySlug: func(slug string) string { return slug },
	})
	first := get(t, srv, "/jobs")
	checkPage(t, "/jobs", first)
	total := regexp.MustCompile(`aria-label="Page (\d+)" data-board-link>\d+</a>\s*<a href="[^"]*" rel="next"`).FindStringSubmatch(first.body)
	pages := 1
	if total != nil {
		pages = atoi(total[1])
	}
	links := map[string]bool{}
	for n := 1; n <= pages; n++ {
		path := "/jobs?page=" + itoa(n)
		r := get(t, srv, path)
		checkPage(t, path, r)
		for _, m := range regexp.MustCompile(`<h2 class="row-title"><a href="([^"]+)"`).FindAllStringSubmatch(r.body, -1) {
			links[m[1]] = true
		}
	}
	if len(links) < 100 {
		t.Fatalf("only %d families across %d pages", len(links), pages)
	}
	i := 0
	for link := range links {
		if i++; i > 150 {
			break
		}
		r := get(t, srv, link)
		if r.status != 200 {
			t.Errorf("%s: %d", link, r.status)
			continue
		}
		checkPage(t, link, r)
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// hrefEscape percent-encodes non-ASCII bytes the way html/template normalizes
// URLs in href attributes.
func hrefEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 0x80 {
			b.WriteString("%" + strings.ToLower(strconv.FormatUint(uint64(c), 16)))
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}
