package tools

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/internal/fetch"
)

type statusRT int

func (s statusRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: int(s),
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    r,
	}, nil
}

func TestCheckApplyReportsDead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	body := `{"jobs":[
		{"id":"a","title":"Intern","apply_url":"https://dead.example/a","company":{"slug":"acme"}},
		{"id":"b","title":"Intern","apply_url":"https://dead.example/a","company":{"slug":"acme"}}
	]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := fetch.WithRoundTripper(context.Background(), statusRT(404))
	code, err := CheckApply(ctx, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("exit %d, want 1 for dead link", code)
	}
}

func TestCheckApplyOk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	body := `{"jobs":[{"id":"a","title":"Intern","apply_url":"https://ok.example/a","company":{"slug":"acme"}}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := fetch.WithRoundTripper(context.Background(), statusRT(200))
	code, err := CheckApply(ctx, []string{path})
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestCheckApplyRedirectAndRefusal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	body := `{"jobs":[
		{"id":"a","title":"Intern","apply_url":"https://gone.example/careers/jobs/12345","company":{"slug":"acme"}},
		{"id":"b","title":"Intern","apply_url":"https://shy.example/jobs/67890","company":{"slug":"acme"}}
	]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		res := &http.Response{StatusCode: 200, Header: make(http.Header), Body: http.NoBody, Request: r}
		switch {
		case r.URL.Host == "shy.example":
			res.StatusCode = 403
		case r.URL.Path == "/careers/jobs/12345":
			res.StatusCode = 302
			res.Header.Set("Location", "/careers")
		}
		return res, nil
	})
	var out, errOut strings.Builder
	code, err := checkApply(fetch.WithRoundTripper(context.Background(), rt), []string{path}, &out, &errOut, 0)
	if err != nil || code != 1 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if !strings.Contains(out.String(), "dead\tacme\ta\t") || !strings.Contains(out.String(), "https://gone.example/careers") {
		t.Errorf("redirect to the careers index is dead:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "inconclusive\tacme\tb\t") {
		t.Errorf("403 is inconclusive:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "2 URLs, 1 dead, 1 inconclusive, 0 alive") {
		t.Errorf("totals: %s", errOut.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
