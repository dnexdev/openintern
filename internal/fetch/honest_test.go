package fetch

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The Go form of scripts/check-honest-requests.mjs: every request to a job
// board goes through this package, so nothing else builds an HTTP client or
// request, names an identity header, or looks like a browser. Test files are
// skipped, as the TypeScript gate skipped *.test.ts.
var (
	ownClient   = regexp.MustCompile(`\bhttp\.(?:Client\b|NewRequest\b|NewRequestWithContext\b|Get\b|Post\b|Head\b|PostForm\b|DefaultClient\b)`)
	identity    = regexp.MustCompile(`(?i)"(?:user-agent|referer|origin|cookie)"\s*:|\.(?:Set|Add)\(\s*"(?:user-agent|referer|origin|cookie)"`)
	browserUA   = regexp.MustCompile(`Mozilla/5\.0|AppleWebKit|Chrome/\d`)
	browserAuto = regexp.MustCompile(`(?i)\b(?:playwright|puppeteer|selenium|chromium)\b`)
	legacyCall  = regexp.MustCompile(`\bLegacyPortalPostJSON\b`)
)

// clientAllowed lists packages that may build their own client because they
// never talk to a job board. internal/board reads OpenIntern's own published
// dump, health snapshot and kill switch (see docs/go-rewrite/status/sources.md,
// Requests).
var clientAllowed = []string{"internal/board/"}

// legacyAllowed is the one file that may call the PENDING OWNER DECISION
// ByteDance request, and it must keep that marker.
const legacyAllowed = "internal/ats/bytedance.go"

func TestHonestRequests(t *testing.T) {
	root := moduleRoot(t)
	var problems []string
	for _, top := range []string{"internal", "cmd", "api"} {
		dir := filepath.Join(root, top)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "third_party" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			text, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			problems = append(problems, problemsIn(filepath.ToSlash(rel), string(text))...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(problems) > 0 {
		t.Fatalf("honest-requests check failed:\n%s", strings.Join(problems, "\n"))
	}
}

func TestHonestScannerFindings(t *testing.T) {
	cases := []struct {
		name, file, src string
		want            bool
	}{
		{"own request", "internal/ats/x.go", "_ = http.NewRequest(\"GET\", u, nil)", true},
		{"own client", "internal/ats/x.go", "c := &http.Client{}", true},
		{"fetch may", "internal/fetch/fetch.go", "c := &http.Client{}", false},
		{"board may read its dump", "internal/board/source.go", "c := &http.Client{}", false},
		{"board may not forge", "internal/board/source.go", `h := map[string]string{"Referer": "x"}`, true},
		{"referer key", "internal/ats/x.go", `h := map[string]string{"Referer": "https://example"}`, true},
		{"origin key", "internal/ats/x.go", `h := map[string]string{"origin": o}`, true},
		{"cookie set", "internal/ats/x.go", `req.Header.Set("Cookie", c)`, true},
		{"browser ua", "internal/ats/x.go", `ua := "Mozilla/5.0 (X11)"`, true},
		{"automation", "internal/tools/x.go", `// uses chromium`, false},
		{"automation code", "internal/tools/x.go", `run("chromium")`, true},
		{"comment", "internal/ats/x.go", "// http.NewRequest is not called", false},
		{"block comment", "internal/ats/x.go", "/* http.NewRequest */ x := 1", false},
		{"origin word", "internal/ats/x.go", `origin := phenomOrigin(token)`, false},
		{"json key named origin", "internal/ats/x.go", `get(v, "origin")`, false},
		{"legacy elsewhere", "internal/ats/phenom.go", "fetch.LegacyPortalPostJSON(ctx, u, o, b, h)", true},
		{"legacy allowed", legacyAllowed, "// PENDING OWNER DECISION\nfetch.LegacyPortalPostJSON(ctx, u, o, b, h)", false},
		{"legacy without marker", legacyAllowed, "fetch.LegacyPortalPostJSON(ctx, u, o, b, h)", true},
	}
	for _, tc := range cases {
		got := len(problemsIn(tc.file, "package p\n\n"+tc.src+"\n")) > 0
		if got != tc.want {
			t.Errorf("%s: finding = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func problemsIn(name, src string) []string {
	if strings.HasPrefix(name, "internal/fetch/") {
		return nil
	}
	allowClient := false
	for _, p := range clientAllowed {
		if strings.HasPrefix(name, p) {
			allowClient = true
		}
	}
	var out []string
	add := func(line int, why, text string) {
		out = append(out, name+":"+strconv.Itoa(line)+": "+why+": "+strings.TrimSpace(text))
	}
	inBlock := false
	for i, line := range strings.Split(src, "\n") {
		code := stripComments(line, &inBlock)
		if strings.TrimSpace(code) == "" {
			continue
		}
		switch {
		case !allowClient && ownClient.MatchString(code):
			add(i+1, "builds its own http.Client or http.Request (use internal/fetch)", line)
		case identity.MatchString(code):
			add(i+1, "sets an identity header", line)
		case browserUA.MatchString(code):
			add(i+1, "browser user-agent", line)
		case browserAuto.MatchString(code):
			add(i+1, "browser automation", line)
		case legacyCall.MatchString(code) && name != legacyAllowed:
			add(i+1, "the legacy ByteDance request is allowed only in "+legacyAllowed, line)
		}
	}
	if name == legacyAllowed && legacyCall.MatchString(src) && !strings.Contains(src, "PENDING OWNER DECISION") {
		add(0, "forged headers without the PENDING OWNER DECISION marker", "")
	}
	return out
}

// stripComments drops // and /* */ comments from one line, carrying block
// state across lines. Comment markers inside string literals are rare enough
// in this tree to ignore.
func stripComments(line string, inBlock *bool) string {
	code := line
	if *inBlock {
		end := strings.Index(code, "*/")
		if end < 0 {
			return ""
		}
		*inBlock = false
		code = code[end+2:]
	}
	for {
		start := strings.Index(code, "/*")
		if start < 0 {
			break
		}
		rest := code[start+2:]
		end := strings.Index(rest, "*/")
		if end < 0 {
			*inBlock = true
			code = code[:start]
			break
		}
		code = code[:start] + " " + rest[end+2:]
	}
	if c := strings.Index(code, "//"); c >= 0 {
		code = code[:c]
	}
	return code
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
