package tools

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
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
