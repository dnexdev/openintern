package ats

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// testdata/fixtures/<name>.json.gz are board responses: real ones recorded
// once through the TypeScript adapters (list responses trimmed to three jobs
// before the adapter saw them, so detail requests stay few), and syn-* ones
// written by hand for paths real boards rarely take (errors, throttling,
// pagination, fallbacks). <name>.golden.json is the TypeScript adapter's
// output on the same responses (testdata/ts/replay.mts, TZ=UTC), postedAt
// passed through safeDate as plan.ts does. Go must produce the same jobs.

type recorded struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	ReqBody *string           `json:"reqBody"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Error   string            `json:"error"`
}

type fixture struct {
	Name     string     `json:"name"`
	ATS      string     `json:"ats"`
	Token    string     `json:"token"`
	Requests []recorded `json:"requests"`
}

// replay serves a recording: requests are matched on method, URL and body;
// a repeated request gets the next recorded answer.
type replay struct {
	t     *testing.T
	mu    sync.Mutex
	byKey map[string][]recorded
	used  map[string]int
}

func (r *replay) RoundTrip(req *http.Request) (*http.Response, error) {
	if ua := req.Header.Get("User-Agent"); ua != fetch.UserAgent {
		r.t.Errorf("request to %s with User-Agent %q", req.URL, ua)
	}
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	key := req.Method + " " + req.URL.String() + " " + body
	r.mu.Lock()
	q := r.byKey[key]
	n := r.used[key]
	r.used[key] = n + 1
	r.mu.Unlock()
	if len(q) == 0 {
		// The TypeScript adapter made only recorded requests.
		r.t.Errorf("unrecorded request %s", key)
		return nil, errors.New("fetch failed")
	}
	rec := q[min(n, len(q)-1)]
	if rec.Error != "" {
		return nil, errors.New(rec.Error)
	}
	h := http.Header{}
	for k, v := range rec.Headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: rec.Status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(rec.Body)),
		Request:    req,
	}, nil
}

func loadFixture(t *testing.T, path string) fixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var fx fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatal(err)
	}
	return fx
}

func TestFixturesMatchTypeScript(t *testing.T) {
	// The goldens were produced with TZ=UTC.
	saved := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = saved })

	files, _ := filepath.Glob("testdata/fixtures/*.json.gz")
	if len(files) == 0 {
		t.Fatal("no fixtures")
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json.gz")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := loadFixture(t, file)
			rt := &replay{t: t, byKey: map[string][]recorded{}, used: map[string]int{}}
			for _, r := range fx.Requests {
				body := ""
				if r.ReqBody != nil {
					body = *r.ReqBody
				}
				key := r.Method + " " + r.URL + " " + body
				rt.byKey[key] = append(rt.byKey[key], r)
			}
			ctx := withoutPacing(fetch.WithRoundTripper(context.Background(), rt))
			jobs, err := Fetch(ctx, fx.ATS, fx.Token)

			goldenRaw, rerr := os.ReadFile(filepath.Join("testdata/fixtures", name+".golden.json"))
			if rerr != nil {
				t.Fatal(rerr)
			}
			golden, perr := jsjson.Parse(string(goldenRaw))
			if perr != nil {
				t.Fatal(perr)
			}
			if gerr := get(golden, "error"); !nullish(gerr) {
				if err == nil {
					t.Fatalf("TypeScript failed (%s), Go returned %d jobs", text(get(gerr, "message")), len(jobs))
				}
				kind := get(gerr, "kind")
				var f *fetch.Failure
				if !nullish(kind) {
					if !errors.As(err, &f) || string(f.Kind) != text(kind) {
						t.Fatalf("error %v, want kind %s", err, text(kind))
					}
					// The JSON parser's own message differs from V8's (see
					// the status file); every other failure reads the same.
					if !strings.Contains(err.Error(), "unreadable JSON") && err.Error() != text(get(gerr, "message")) {
						t.Fatalf("error %q, want %q", err, text(get(gerr, "message")))
					}
				} else if err.Error() != text(get(gerr, "message")) {
					t.Fatalf("error %q, want %q", err, text(get(gerr, "message")))
				}
				return
			}
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			want := array(get(golden, "jobs"))
			if len(jobs) != len(want) {
				t.Fatalf("%d jobs, want %d", len(jobs), len(want))
			}
			for i, w := range want {
				compareJob(t, i, jobs[i], w)
			}
		})
	}
}

func optString(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func compareJob(t *testing.T, i int, got model.NormalizedJob, want any) {
	t.Helper()
	eq := func(field, g, w string) {
		if g != w {
			t.Errorf("job %d %s = %q, want %q", i, field, g, w)
		}
	}
	eqPtr := func(field string, g, w *string) {
		switch {
		case g == nil && w == nil:
		case g == nil || w == nil:
			t.Errorf("job %d %s = %v, want %v", i, field, deref(g), deref(w))
		default:
			eq(field, *g, *w)
		}
	}
	eq("externalId", got.ExternalID, text(get(want, "externalId")))
	eq("title", got.Title, text(get(want, "title")))
	eq("applyUrl", got.ApplyURL, text(get(want, "applyUrl")))
	eq("description", got.Description, text(get(want, "description")))
	eqPtr("excerpt", got.Excerpt, optString(get(want, "excerpt")))
	eqPtr("postedAt", got.PostedAt, optString(get(want, "postedAt")))
	var wl []string
	for _, l := range array(get(want, "locations")) {
		wl = append(wl, text(l))
	}
	if strings.Join(got.Locations, "\x00") != strings.Join(wl, "\x00") || got.Locations == nil {
		t.Errorf("job %d locations = %q, want %q", i, got.Locations, wl)
	}
}

func deref(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// OPENINTERN_SHADOW_HONEST=1 adds one honest request after the legacy
// ByteDance fetch and records the outcome without changing the jobs.
func TestBytedanceShadowProbe(t *testing.T) {
	t.Setenv("OPENINTERN_SHADOW_HONEST", "1")
	var mu sync.Mutex
	var legacyHeaders, honestHeaders http.Header
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(req.Body)
		mu.Lock()
		defer mu.Unlock()
		body := `{"code":0,"data":{"count":1,"job_post_list":[{"id":"1","title":"T"}]}}`
		if req.Header.Get("Referer") != "" {
			legacyHeaders = req.Header.Clone()
		} else {
			honestHeaders = req.Header.Clone()
			body = `{"code":5,"message":"denied","data":{}}`
		}
		if !strings.Contains(string(b), `"recruitment_id_list":["2"]`) {
			t.Errorf("body %s", b)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	before := len(ShadowResults())
	jobs, err := Fetch(fetch.WithRoundTripper(context.Background(), rt), "tiktok", "")
	if err != nil || len(jobs) != 1 {
		t.Fatalf("%v %v", jobs, err)
	}
	if legacyHeaders.Get("Origin") != "https://lifeattiktok.com" || legacyHeaders.Get("Referer") != "https://lifeattiktok.com/search" || legacyHeaders.Get("Website-Path") != "tiktok" {
		t.Fatalf("legacy headers %v", legacyHeaders)
	}
	if honestHeaders == nil || honestHeaders.Get("Origin") != "" || honestHeaders.Get("User-Agent") != fetch.UserAgent {
		t.Fatalf("honest headers %v", honestHeaders)
	}
	got := ShadowResults()
	if len(got) != before+1 {
		t.Fatalf("%d shadow results", len(got))
	}
	r := got[len(got)-1]
	line, _ := json.Marshal(r)
	if string(line) != `{"source":"tiktok","ok":false,"status":200,"count":0,"legacyCount":1,"error":"code=5 denied"}` {
		t.Fatalf("%s", line)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
