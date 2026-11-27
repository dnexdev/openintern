package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRetryAfterAndHTTP(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	var seen []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.String()]++
		n := hits[r.URL.String()]
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		hdr := map[string]string{"Content-Type": "application/json"}
		body := "{}"
		status := 500
		switch r.URL.Path {
		case "/ok":
			status, body = 200, `{"a":1}`
		case "/forbidden":
			status = 403
		case "/gone":
			status = 404
		case "/flaky":
			if n < 3 {
				status = 503
			} else {
				status, body = 200, `{"ok":true}`
			}
		case "/throttled":
			if n < 2 {
				status = 429
				hdr["Retry-After"] = "0"
			} else {
				status, body = 200, `{"ok":true}`
			}
		case "/always429":
			status = 429
			hdr["Retry-After"] = "0"
		case "/bad-json":
			status, body = 200, "not json"
		}
		for k, v := range hdr {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	ctx := context.Background()
	kindOf := func(err error) string {
		if err == nil {
			return "ok"
		}
		var f *Failure
		if errors.As(err, &f) {
			return string(f.Kind)
		}
		return "other: " + err.Error()
	}

	var okBody struct {
		A int `json:"a"`
	}
	if err := JSON(ctx, srv.URL+"/ok", Options{}, &okBody); err != nil {
		t.Fatal(err)
	}
	if okBody.A != 1 {
		t.Fatalf("body = %#v", okBody)
	}
	mu.Lock()
	ua := seen[len(seen)-1].Get("User-Agent")
	mu.Unlock()
	if ua != UserAgent {
		t.Fatalf("user-agent = %q", ua)
	}

	_, err := Request(ctx, srv.URL+"/forbidden", Options{})
	if got := kindOf(err); got != "refused" {
		t.Fatalf("forbidden kind %s", got)
	}
	mu.Lock()
	if hits["/forbidden"] != 1 {
		t.Fatalf("refusal was retried: %d", hits["/forbidden"])
	}
	mu.Unlock()

	_, err = Request(ctx, srv.URL+"/gone", Options{})
	if got := kindOf(err); got != "not-found" {
		t.Fatalf("gone kind %s", got)
	}

	var flaky struct {
		OK bool `json:"ok"`
	}
	if err := JSON(ctx, srv.URL+"/flaky", Options{}, &flaky); err != nil || !flaky.OK {
		t.Fatalf("flaky: %v %#v", err, flaky)
	}
	var throttled struct {
		OK bool `json:"ok"`
	}
	if err := JSON(ctx, srv.URL+"/throttled", Options{}, &throttled); err != nil || !throttled.OK {
		t.Fatalf("throttled: %v %#v", err, throttled)
	}
	_, err = Request(ctx, srv.URL+"/always429", Options{Attempts: 2})
	if got := kindOf(err); got != "throttled" {
		t.Fatalf("always429 kind %s", got)
	}
	mu.Lock()
	if hits["/always429"] != 2 {
		t.Fatalf("attempts = %d", hits["/always429"])
	}
	mu.Unlock()

	if got := kindOf(JSON(ctx, srv.URL+"/bad-json", Options{}, &okBody)); got != "invalid" {
		t.Fatalf("bad json kind %s", got)
	}
	_, err = Request(ctx, "http://127.0.0.1:1/", Options{Attempts: 1})
	if got := kindOf(err); got != "network" {
		t.Fatalf("network kind %s", got)
	}
	for _, header := range []string{"Referer", "origin", "User-Agent", "Cookie"} {
		_, err := Request(ctx, srv.URL+"/ok", Options{Headers: map[string]string{header: "x"}})
		if err == nil || !strings.Contains(err.Error(), "honest requests only") {
			t.Fatalf("header %s: %v", header, err)
		}
	}

	if ms, ok := RetryAfterMs("5", time.Now()); !ok || ms != 5000 {
		t.Fatalf("retry 5 = %d %v", ms, ok)
	}
	if ms, ok := RetryAfterMs("999999", time.Now()); !ok || ms != 30000 {
		t.Fatalf("retry cap = %d %v", ms, ok)
	}
	when := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
	if _, ok := RetryAfterMs(when, time.Now()); !ok {
		t.Fatal("http date retry-after")
	}
	if _, ok := RetryAfterMs("soon", time.Now()); ok {
		t.Fatal("bad retry-after")
	}
}

func TestFailureAs(t *testing.T) {
	err := fail(KindManualExpired, "too old", 0)
	var f *Failure
	if !errors.As(err, &f) || f.Kind != KindManualExpired || f.Status != 0 {
		t.Fatalf("%v", err)
	}
	if err.Error() != "manual-expired: too old" {
		t.Fatal(err.Error())
	}
}

// Response.URL is where the body came from after redirects, so relative links
// in it resolve against the right page.
func TestResponseURLAfterRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new/page", http.StatusMovedPermanently)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	res, err := Request(context.Background(), srv.URL+"/old", Options{Attempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != srv.URL+"/new/page" {
		t.Errorf("URL = %q, want %q", res.URL, srv.URL+"/new/page")
	}
}
