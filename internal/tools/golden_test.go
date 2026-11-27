package tools

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/registry"
)

// The golden files are the TypeScript scripts' stdout, stderr, exit code,
// requests and written files on the same recordings and test registry
// (testdata/ts/gen.mjs regenerates them). TestScriptsMatchTypeScript runs
// the Go commands on the same inputs and compares everything.

type recEntry struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	ReqBody  *string           `json:"reqBody"`
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
	BodyFile string            `json:"bodyFile"`
	Error    string            `json:"error"`
}

type toolCase struct {
	Name          string            `json:"name"`
	Script        string            `json:"script"`
	Args          []string          `json:"args"`
	Recording     string            `json:"recording"`
	Override      []recEntry        `json:"override"`
	Registry      string            `json:"registry"`
	BaseRegistry  string            `json:"baseRegistry"`
	Env           map[string]string `json:"env"`
	SemanticFiles bool              `json:"semanticFiles"`
}

type golden struct {
	Stdout   string            `json:"stdout"`
	Stderr   string            `json:"stderr"`
	Code     int               `json:"code"`
	Requests []string          `json:"requests"`
	Files    map[string]string `json:"files"`
}

// replay answers requests from a recording: the first entry with the same
// method, URL and body; anything else is a 404, as in the TypeScript driver.
type replay struct {
	entries []recEntry
	mu      sync.Mutex
	log     []string
}

func newReplay(t *testing.T, file string, override []recEntry) *replay {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata/recordings", file))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct{ Responses []recEntry }
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	r := &replay{entries: append(slices.Clone(override), rec.Responses...)}
	for i, e := range r.entries {
		if e.BodyFile == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join("testdata", e.BodyFile))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(e.BodyFile, ".gz") {
			zr, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			if b, err = io.ReadAll(zr); err != nil {
				t.Fatal(err)
			}
		}
		r.entries[i].Body = string(b)
	}
	return r
}

func (r *replay) RoundTrip(req *http.Request) (*http.Response, error) {
	var body *string
	line := req.Method + " " + req.URL.String()
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		s := string(b)
		body = &s
		line += " " + s
	}
	r.mu.Lock()
	r.log = append(r.log, line)
	r.mu.Unlock()
	for _, e := range r.entries {
		if e.Method != req.Method || e.URL != req.URL.String() {
			continue
		}
		if (e.ReqBody == nil) != (body == nil) || e.ReqBody != nil && *e.ReqBody != *body {
			continue
		}
		if e.Error != "" {
			return nil, errors.New("connection refused (recorded failure)")
		}
		status := e.Status
		if status == 0 {
			status = 200
		}
		h := http.Header{}
		for k, v := range e.Headers {
			h.Set(k, v)
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(e.Body)), Request: req}, nil
	}
	return &http.Response{StatusCode: 404, Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader("Not Found")), Request: req}, nil
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(b)
	}
	return out
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

var fixedNow = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

// runTool runs one command the way cmd_tools.go does and returns its exit
// code.
func runTool(ctx context.Context, env Env, script string, args []string) (int, error) {
	switch script {
	case "gap-report":
		return 0, GapReport(ctx, env, args)
	case "discover-accelerators":
		return DiscoverAccelerators(ctx, env, args)
	case "recover-tokens":
		return 0, RecoverTokens(ctx, env, args)
	case "validate-tokens":
		return ValidateTokens(ctx, env, args)
	}
	return 2, errors.New("unknown script " + script)
}

// goWording maps the next-step hints the scripts printed (pnpm commands that
// no longer exist) to the Go commands.
var goWording = strings.NewReplacer(
	"Next: pnpm validate-companies && pnpm validate-tokens -- ../../data/companies/tech-c.yaml",
	"Next: openintern validate-companies && openintern validate-tokens data/companies/tech-c.yaml",
	"Then: pnpm sync-companies && ingest (when ready).",
	"Then: openintern sync-companies && ingest (when ready).",
	"# verify with pnpm validate-tokens",
	"# verify with openintern validate-tokens",
)

func TestScriptsMatchTypeScript(t *testing.T) {
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []toolCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var want golden
			gb, err := os.ReadFile(filepath.Join("testdata/golden", tc.Name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(gb, &want); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			companies := filepath.Join(root, "data", "companies")
			reg := tc.Registry
			if reg == "" {
				reg = "registry"
			}
			if tc.BaseRegistry != "" {
				if _, err := exec.LookPath("git"); err != nil {
					t.Skip("git not installed")
				}
				copyDir(t, filepath.Join("testdata", tc.BaseRegistry, "companies"), companies)
				gitRun(t, root, "init", "-q")
				gitRun(t, root, "add", "data")
				gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base")
				gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
				if err := os.RemoveAll(companies); err != nil {
					t.Fatal(err)
				}
			}
			copyDir(t, filepath.Join("testdata", reg, "companies"), companies)
			before := readDir(t, companies)
			work := filepath.Join(root, "packages", "ingest")
			if err := os.MkdirAll(work, 0o755); err != nil {
				t.Fatal(err)
			}

			rp := newReplay(t, tc.Recording, tc.Override)
			var stdout, stderr bytes.Buffer
			env := Env{
				Stdout: &stdout, Stderr: &stderr,
				DataDir: filepath.Join(root, "data"), WorkDir: work,
				Now:    func() time.Time { return fixedNow },
				Getenv: func(k string) string { return tc.Env[k] },
			}
			ctx := fetch.WithRoundTripper(context.Background(), rp)
			code, err := runTool(ctx, env, tc.Script, tc.Args)
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if code != want.Code {
				t.Errorf("exit code %d, TypeScript %d", code, want.Code)
			}
			diffText(t, "stdout", stdout.String(), goWording.Replace(want.Stdout))
			diffText(t, "stderr", stderr.String(), want.Stderr)
			got := slices.Clone(rp.log)
			slices.Sort(got)
			if want.Requests == nil {
				want.Requests = []string{}
			}
			if got == nil {
				got = []string{}
			}
			diffText(t, "requests", strings.Join(got, "\n"), strings.Join(want.Requests, "\n"))

			after := readDir(t, companies)
			changed := map[string]string{}
			for f, text := range after {
				if before[f] != text {
					changed[f] = text
				}
			}
			if len(changed) != len(want.Files) {
				t.Errorf("changed files %v, TypeScript %v", keys(changed), keys(want.Files))
			}
			for f, wantText := range want.Files {
				if tc.SemanticFiles {
					sameCompanies(t, f, changed[f], wantText)
					continue
				}
				diffText(t, "file "+f, changed[f], wantText)
			}
		})
	}
}

// sameCompanies compares two company files by their parsed entries (the
// recover-tokens divergence: Go edits lines in place, the script rewrote the
// whole file).
func sameCompanies(t *testing.T, name, got, want string) {
	t.Helper()
	g, _, err := registry.ParseCompaniesFile(name, []byte(got))
	if err != nil {
		t.Fatalf("%s (Go): %v", name, err)
	}
	w, _, err := registry.ParseCompaniesFile(name, []byte(want))
	if err != nil {
		t.Fatalf("%s (TypeScript): %v", name, err)
	}
	strip := func(list []registry.CompanyYAML) []registry.CompanyYAML {
		for i := range list {
			list[i].Line, list[i].File = 0, ""
		}
		return list
	}
	gj, _ := json.Marshal(strip(g))
	wj, _ := json.Marshal(strip(w))
	if !bytes.Equal(gj, wj) {
		t.Errorf("%s companies differ:\nGo:         %s\nTypeScript: %s", name, gj, wj)
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func diffText(t *testing.T, what, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gl) || i < len(wl); i++ {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			t.Errorf("%s differs at line %d:\n  Go:         %q\n  TypeScript: %q", what, i+1, g, w)
			return
		}
	}
	t.Errorf("%s differs", what)
}
