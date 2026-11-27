package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/board"
)

var parityNow = time.Date(2026, time.November, 25, 2, 0, 0, 0, time.UTC)

func parityHandler(t *testing.T, mutate func(*board.Options)) http.Handler {
	t.Helper()
	options := board.Options{
		DumpSource:   filepath.Join("testdata", "jobs.json"),
		HealthSource: filepath.Join("testdata", "missing-health.json"),
		Now:          func() time.Time { return parityNow },
	}
	if mutate != nil {
		mutate(&options)
	}
	limit := int64(100_000)
	return Handler(Options{
		Board:        options,
		Now:          func() time.Time { return parityNow },
		RateLimit:    &limit,
		RegisterSite: func(*http.ServeMux, *board.Store) {},
	})
}

func request(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.17, 203.0.113.9")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func assertGolden(t *testing.T, name string, response *httptest.ResponseRecorder) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(response.Body.Bytes(), want) {
		t.Fatalf("body differs from %s\nwant: %.1000s\n got: %.1000s", name, want, response.Body.Bytes())
	}
}

func TestAPIGoldens(t *testing.T) {
	handler := parityHandler(t, nil)
	tests := []struct {
		name   string
		path   string
		golden string
	}{
		{"jobs default", "/api/v1/jobs", "jobs-default.golden.json"},
		{"jobs filtered", "/api/v1/jobs?q=engineer&role=software,data&region=us,remote&season=summer&duration_months=3,6&sort=posted&limit=100", "jobs-filtered.golden.json"},
		{"jobs last page", "/api/v1/jobs?page=100001&limit=1", "jobs-last-page.golden.json"},
		{"job detail", "/api/v1/jobs/ecddac83-43da-4433-9cc9-ac1ad4b80153", "job-detail.golden.json"},
		{"companies", "/api/v1/companies", "companies.golden.json"},
		{"health fallback", "/api/v1/health", "health.golden.json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, test.path)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			assertGolden(t, test.golden, response)
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("Content-Type = %q", contentType)
			}
		})
	}
}

func TestMetadataGoldens(t *testing.T) {
	handler := parityHandler(t, nil)
	tests := []struct {
		path        string
		golden      string
		contentType string
	}{
		{"/robots.txt", "robots.golden.txt", "text/plain"},
		{"/sitemap.xml", "sitemap.golden.xml", "application/xml"},
	}
	for _, test := range tests {
		response := request(t, handler, test.path)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", test.path, response.Code)
		}
		assertGolden(t, test.golden, response)
		if got := response.Header().Get("Content-Type"); got != test.contentType {
			t.Fatalf("%s Content-Type = %q", test.path, got)
		}
		if got := response.Header().Get("Cache-Control"); got != "public, max-age=0, must-revalidate" {
			t.Fatalf("%s Cache-Control = %q", test.path, got)
		}
	}
}

func TestAPIHeadersAndErrors(t *testing.T) {
	handler := parityHandler(t, nil)
	response := request(t, handler, "/api/v1/jobs?limit=1")
	if got := response.Header().Get("Cache-Control"); got != apiCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("X-Data-Source"); got != "dump" {
		t.Fatalf("X-Data-Source = %q", got)
	}
	if got := response.Header().Get("X-Dump-Generated-At"); got != "2026-11-13T14:38:58.955Z" {
		t.Fatalf("X-Dump-Generated-At = %q", got)
	}
	if got := response.Header().Get("X-RateLimit-Remaining"); got == "" {
		t.Fatal("missing X-RateLimit-Remaining")
	}

	notFound := request(t, handler, "/api/v1/jobs/not-a-job")
	if notFound.Code != http.StatusNotFound || notFound.Body.String() != `{"error":"Not found"}` {
		t.Fatalf("not found = %d %s", notFound.Code, notFound.Body.String())
	}
}

func TestStatusUsesUnroundedAgeForFreshness(t *testing.T) {
	now := time.Date(2026, time.November, 14, 20, 39, 28, 955_000_000, time.UTC)
	limit := int64(10)
	handler := Handler(Options{
		Board: board.Options{
			DumpSource: filepath.Join("testdata", "jobs.json"),
			Now:        func() time.Time { return now },
		},
		Now:          func() time.Time { return now },
		RateLimit:    &limit,
		RegisterSite: func(*http.ServeMux, *board.Store) {},
	})
	response := request(t, handler, "/api/v1/status")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Status       string   `json:"status"`
		DumpAgeHours *float64 `json:"dump_age_hours"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "stale" || body.DumpAgeHours == nil || *body.DumpAgeHours != 30 {
		t.Fatalf("body = %+v", body)
	}
}

func TestRateLimitAndClientIP(t *testing.T) {
	limit := int64(1)
	handler := Handler(Options{
		Board: board.Options{DumpSource: filepath.Join("testdata", "jobs.json"), Now: func() time.Time { return parityNow }},
		Now:   func() time.Time { return parityNow }, RateLimit: &limit,
		RegisterSite: func(*http.ServeMux, *board.Store) {},
	})
	first := request(t, handler, "/api/v1/jobs?limit=1")
	if first.Code != http.StatusOK || first.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("first = %d, remaining %q", first.Code, first.Header().Get("X-RateLimit-Remaining"))
	}
	second := request(t, handler, "/api/v1/jobs?limit=1")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second = %d", second.Code)
	}
	if second.Header().Get("Retry-After") != "60" || second.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatalf("rate headers = %#v", second.Header())
	}
}

func TestRateLimiterRemovesExpiredBuckets(t *testing.T) {
	now := parityNow
	limiter := newRateLimiter(10, func() time.Time { return now })
	limiter.step("expired")

	now = now.Add(time.Minute)
	limiter.step("current")

	if _, ok := limiter.buckets["expired"]; ok {
		t.Fatal("expired rate-limit bucket was retained")
	}
	if _, ok := limiter.buckets["current"]; !ok {
		t.Fatal("current rate-limit bucket is missing")
	}
}

func TestMaintenanceGate(t *testing.T) {
	on := true
	handler := Handler(Options{
		Board:       board.Options{DumpSource: filepath.Join("testdata", "jobs.json")},
		Maintenance: &on,
		RegisterSite: func(mux *http.ServeMux, _ *board.Store) {
			mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("site")) })
		},
	})
	api := request(t, handler, "/api/v1/jobs")
	if api.Code != http.StatusServiceUnavailable || api.Body.String() != `{"error":"Maintenance","message":"OpenIntern is down for maintenance. Please check back soon."}` {
		t.Fatalf("api = %d %s", api.Code, api.Body.String())
	}
	if api.Header().Get("Cache-Control") != "no-store" || api.Header().Get("Retry-After") != "600" {
		t.Fatalf("headers = %#v", api.Header())
	}
	page := request(t, handler, "/")
	if page.Code != http.StatusServiceUnavailable || !strings.Contains(page.Body.String(), "Down for maintenance") {
		t.Fatalf("page = %d %s", page.Code, page.Body.String())
	}
	status := request(t, handler, "/api/v1/status")
	if status.Body.String() == api.Body.String() {
		t.Fatal("status endpoint was caught by maintenance gate")
	}
}

func TestJobAliasRedirect(t *testing.T) {
	handler := parityHandler(t, func(options *board.Options) {
		options.JobAlias = func(id string) string {
			if id == "old" {
				return "new"
			}
			return ""
		}
	})
	response := request(t, handler, "/api/v1/jobs/old")
	if response.Code != http.StatusPermanentRedirect || response.Header().Get("Location") != "/api/v1/jobs/new" {
		t.Fatalf("redirect = %d %#v", response.Code, response.Header())
	}
	if response.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestJavaScriptJSON(t *testing.T) {
	body, err := javascriptJSON(struct {
		Value string `json:"value"`
	}{Value: "<>&\u2028"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte(`\u003c`)) || bytes.Contains(body, []byte(`\u2028`)) {
		t.Fatalf("unexpected escaping: %s", body)
	}
}
