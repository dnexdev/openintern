package tools

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// gap-report.test.ts, apart from the parse-ats-url cases (ported in
// internal/ats/url_test.go).
func TestExtractCompaniesFromMarkdown(t *testing.T) {
	md := `
| Company | Position | Age |
|---|---|---|
| Palantir | Software Engineer Intern | 8d |
| **Jane Street** | SWE Intern | 3d |
| [Notion](https://notion.so) | Software Engineer Intern | 1d |
| Acme &amp;quot;Labs&amp;quot; | Intern | 2d |

<table>
<tr><td>Company</td><td>Role</td></tr>
<tr><td>Anduril</td><td>2027 Software Engineer Intern</td></tr>
<tr><td><a href="https://www.spacex.com"><strong>SpaceX</strong></a></td><td>Fall 2026 Software Engineering Internship</td></tr>
<tr><td>🔥 Meta</td><td>Software Engineer Intern</td></tr>
<tr><td>Weird</script foo> Corp</td><td>Intern</td></tr>
</table>
`
	names := ExtractCompaniesFromMarkdown(md)
	for _, want := range []string{"Palantir", "Jane Street", "Notion", "Anduril", "SpaceX", "Meta",
		"Acme &quot;Labs&quot;", // double-encoded &amp;quot; only unescapes &amp; once
		"Weird Corp",            // malformed end-tag attributes stripped
	} {
		if !slices.Contains(names, want) {
			t.Errorf("missing %q in %q", want, names)
		}
	}
	if slices.Contains(names, "Company") {
		t.Error("header row kept")
	}
	for _, n := range names {
		if strings.Contains(n, "<") || strings.Contains(n, "🔥") {
			t.Errorf("raw html or emoji prefix in %q", n)
		}
	}

	for _, c := range []struct{ in, want string }{
		{`Foo &amp;quot;Bar&amp;quot; <b>Baz</b>`, `Foo &quot;Bar&quot; Baz`}, // &amp; last (no double-unescape)
		{`Foo &quot;Bar&quot;`, `Foo "Bar"`},
		{"Before</script foo>After", "Before After"}, // attributes after end-tag name
	} {
		if got := htmlx.Strip(c.in); got != c.want {
			t.Errorf("Strip(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := htmlx.StripTags("<p>Hello&nbsp;world</p><br/>&amp; more"); got != "Hello world & more" {
		t.Errorf("StripTags = %q", got)
	}
}

func readVectors(t *testing.T) (v struct {
	People []struct {
		Name, Slug, Norm, RecAts, RecSlug string
		Website                           *string
		Discover, Recover                 []string
	}
	Anchors []struct {
		HTML    string
		Matches [][2]string
	}
	YC []struct {
		Company json.RawMessage
		Tech    bool
	}
}) {
	t.Helper()
	f, err := os.Open("testdata/vectors_node.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestHelpersMatchNode checks slugify, normalizeName, both token guessers,
// the a16z anchor matcher and the YC filter against the scripts' own
// functions (testdata/ts/vectors.mts).
func TestHelpersMatchNode(t *testing.T) {
	v := readVectors(t)
	nonEmpty := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	for _, p := range v.People {
		if got := Slugify(p.Name); got != p.Slug {
			t.Errorf("Slugify(%q) = %q, Node %q", p.Name, got, p.Slug)
		}
		if got := normalizeName(p.Name); got != p.Norm {
			t.Errorf("normalizeName(%q) = %q, Node %q", p.Name, got, p.Norm)
		}
		hint := p.Slug
		if hint == "" {
			hint = "hint"
		}
		if got := nonEmpty(tokenCandidatesFromName(hint, p.Name, p.Website)); !slices.Equal(got, nonEmpty(p.Discover)) {
			t.Errorf("discover tokens(%q, %v) = %q, Node %q", p.Name, p.Website, got, p.Discover)
		}
		if got := nonEmpty(recoverTokenCandidates(p.RecSlug, p.Name, "Current.Token", p.RecAts)); !slices.Equal(got, nonEmpty(p.Recover)) {
			t.Errorf("recover tokens(%q, %s) = %q, Node %q", p.Name, p.RecAts, got, p.Recover)
		}
	}
	for _, a := range v.Anchors {
		got := outboundAnchors(a.HTML)
		if len(got) != len(a.Matches) || (len(got) > 0 && !slices.Equal(got, a.Matches)) {
			t.Errorf("outboundAnchors(%q) = %q, Node %q", a.HTML, got, a.Matches)
		}
	}
	for _, y := range v.YC {
		val, err := jsjson.Parse(string(y.Company))
		if err != nil {
			t.Fatal(err)
		}
		o := val.(jsjson.Object)
		get := func(k string) any { x, _ := o.Get(k); return x }
		got, err := isTechYC(ycCompany{industry: get("industry"), tags: get("tags"), status: get("status")})
		if err != nil || got != y.Tech {
			t.Errorf("isTechYc(%s) = %v (%v), Node %v", y.Company, got, err, y.Tech)
		}
	}
}

func TestJSNumberAndSlice(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		in   string
		want float64
	}{
		{"40", 40}, {"", 0}, {" 7 ", 7}, {"2.5", 2.5}, {"1e2", 100}, {".5", 0.5}, {"5.", 5},
		{"0x10", 16}, {"0b11", 3}, {"0o17", 15}, {"-3", -3}, {"+4", 4}, {"Infinity", math.Inf(1)},
		{"-Infinity", math.Inf(-1)}, {"abc", nan}, {"1_000", nan}, {"inf", nan}, {"0x", nan}, {"1e", nan}, {"0x1g", nan},
	} {
		got := jsNumber(c.in)
		if got != c.want && !(math.IsNaN(got) && math.IsNaN(c.want)) {
			t.Errorf("jsNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	for _, c := range []struct {
		end  float64
		n    int
		want int
	}{{40, 10, 10}, {3, 10, 3}, {2.9, 10, 2}, {-2, 10, 8}, {-20, 10, 0}, {nan, 10, 0}, {math.Inf(1), 10, 10}, {math.Inf(-1), 10, 0}, {0, 10, 0}} {
		if got := sliceEnd(c.end, c.n); got != c.want {
			t.Errorf("sliceEnd(%v, %d) = %d, want %d", c.end, c.n, got, c.want)
		}
	}
	for f, want := range map[float64]string{3: "3", 2.5: "2.5", 1e21: "1e+21", nan: "NaN", math.Inf(1): "Infinity"} {
		if got := jsString(f); got != want {
			t.Errorf("jsString(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestSetCompanyBoard(t *testing.T) {
	src := `# Header comment stays.
companies:
  - name: Alpha
    slug: alpha
    ats: lever
    board_token: alpha # old board
    active: false # paused
  - name: Beta
    slug: beta
    ats: greenhouse
    board_token: beta
    website_url: https://beta.example
  # Beta again, a second entry with the same slug
  - slug: beta
    name: Beta Two
    ats: ashby
    board_token: "beta2"
`
	out, changed, err := setCompanyBoard("x.yaml", []byte(src), "alpha", "ashby", "alpha.io")
	if err != nil || !changed {
		t.Fatalf("alpha: changed=%v err=%v", changed, err)
	}
	want := strings.Replace(src, `    ats: lever
    board_token: alpha # old board
    active: false # paused`, `    ats: ashby
    board_token: alpha.io # old board
    active: true # paused`, 1)
	if string(out) != want {
		t.Errorf("alpha:\n%s\nwant:\n%s", out, want)
	}

	out, changed, err = setCompanyBoard("x.yaml", []byte(src), "beta", "smartrecruiters", "Beta|1")
	if err != nil || !changed {
		t.Fatalf("beta: changed=%v err=%v", changed, err)
	}
	want = strings.Replace(src, `    ats: greenhouse
    board_token: beta
`, `    ats: smartrecruiters
    board_token: "Beta|1"
    active: true
`, 1)
	want = strings.Replace(want, `    ats: ashby
    board_token: "beta2"
`, `    ats: smartrecruiters
    board_token: "Beta|1"
    active: true
`, 1)
	if string(out) != want {
		t.Errorf("beta:\n%s\nwant:\n%s", out, want)
	}

	if _, changed, err := setCompanyBoard("x.yaml", []byte(src), "gamma", "lever", "g"); changed || err != nil {
		t.Errorf("gamma: changed=%v err=%v", changed, err)
	}
	for tok, want := range map[string]string{"plain": "plain", "true": `"true"`, "123": `"123"`, "a: b": `"a: b"`, "x|y": `"x|y"`, "Tok-en": "Tok-en"} {
		if got := yamlScalar(tok); got != want {
			t.Errorf("yamlScalar(%q) = %s, want %s", tok, got, want)
		}
	}
}

func TestIsDumpGateBlock(t *testing.T) {
	for _, c := range []struct {
		ats, err string
		want     bool
	}{
		{"avature", "refused: HTTP 403 for https://x", true},
		{"tesla", "blocked by Akamai", true},
		{"citadel", "set OPENINTERN_CITADEL_DUMP", true},
		{"greenhouse", "HTTP 403 for https://x", false},
		{"icims", "HTTP 404 for https://x", false},
	} {
		if got := isDumpGateBlock(c.ats, c.err); got != c.want {
			t.Errorf("isDumpGateBlock(%q, %q) = %v", c.ats, c.err, got)
		}
	}
}

// Go-only paths: a CI run skips a dump-gated board that refuses the runner,
// and a network failure reads as the honest client's message.
func TestValidateTokensCIAndNetwork(t *testing.T) {
	root := t.TempDir()
	companies := filepath.Join(root, "data", "companies")
	if err := os.MkdirAll(companies, 0o755); err != nil {
		t.Fatal(err)
	}
	file := `companies:
  - name: Gated
    slug: gated
    ats: avature
    board_token: gated
  - name: Down
    slug: down
    ats: lever
    board_token: down
`
	if err := os.WriteFile(filepath.Join(companies, "x.yaml"), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	avatureURL, err := ats.ProbeURL("avature", "gated")
	if err != nil {
		t.Fatal(err)
	}
	leverURL, _ := ats.ProbeURL("lever", "down")
	rp := &replay{entries: []recEntry{
		{Method: "GET", URL: avatureURL, Status: 403},
		{Method: "GET", URL: leverURL, Error: "network"},
	}}
	var stdout, stderr bytes.Buffer
	env := Env{Stdout: &stdout, Stderr: &stderr, DataDir: filepath.Join(root, "data"), WorkDir: root,
		Getenv: func(k string) string {
			if k == "CI" {
				return "true"
			}
			return ""
		}}
	code, err := ValidateTokens(fetch.WithRoundTripper(context.Background(), rp), env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	wantErr := "skip gated (dump-gated proprietary in CI)\n\nToken validation failures:\n  - down: fetch failed for " + leverURL + "\n"
	if stderr.String() != wantErr {
		t.Errorf("stderr %q, want %q", stderr.String(), wantErr)
	}
}

func TestToolsAreHonest(t *testing.T) {
	// Every request goes through internal/fetch: no package here may build
	// its own client or set identifying headers.
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"net/http\"", "User-Agent", "Referer", "Cookie"} {
			if bytes.Contains(b, []byte(bad)) {
				t.Errorf("%s contains %s: requests must go through internal/fetch", f, bad)
			}
		}
	}
}
