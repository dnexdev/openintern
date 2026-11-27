package applycheck

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
)

var t0 = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

func TestNextStrikes(t *testing.T) {
	day := 24 * time.Hour
	none := Strikes{}
	one := Strikes{Count: 1, Last: t0}
	two := Strikes{Count: 2, Last: t0}
	three := Strikes{Count: 3, Last: t0}
	cases := []struct {
		name string
		s    Strikes
		v    Verdict
		now  time.Time
		want Strikes
	}{
		{"first dead verdict is one strike", none, Dead, t0, Strikes{1, t0}},
		{"a day later it counts", one, Dead, t0.Add(day), Strikes{2, t0.Add(day)}},
		{"exactly the gap counts", one, Dead, t0.Add(MinStrikeGap), Strikes{2, t0.Add(MinStrikeGap)}},
		{"sooner than the gap does not", one, Dead, t0.Add(MinStrikeGap - time.Minute), one},
		{"same-day rerun does not", two, Dead, t0.Add(time.Hour), two},
		{"third strike", two, Dead, t0.Add(day), Strikes{3, t0.Add(day)}},
		{"retired stays as is", three, Dead, t0.Add(9 * day), three},
		{"alive clears", two, Alive, t0.Add(day), none},
		{"alive clears a retired job", three, Alive, t0.Add(day), none},
		{"inconclusive keeps", two, Inconclusive, t0.Add(day), two},
		{"inconclusive adds nothing", none, Inconclusive, t0, none},
	}
	for _, c := range cases {
		if got := Next(c.s, c.v, c.now); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	if none.Retired() || two.Retired() || !three.Retired() {
		t.Error("retired at StrikesToRetire")
	}
	if StrikesToRetire != 3 || MinStrikeGap != 20*time.Hour {
		t.Error("PINNED: three strikes, 20 hours apart")
	}
}

// One flaky dead answer between working ones never retires a job, and
// neither does any number of dead answers in one day.
func TestFlakyNeverRetires(t *testing.T) {
	s := Strikes{}
	now := t0
	for i := 0; i < 30; i++ {
		v := Alive
		if i%2 == 0 {
			v = Dead
		}
		s = Next(s, v, now)
		if s.Retired() {
			t.Fatalf("retired after alternating answers at %d", i)
		}
		now = now.Add(24 * time.Hour)
	}
	s = Strikes{}
	for i := 0; i < 50; i++ {
		s = Next(s, Dead, t0.Add(time.Duration(i)*15*time.Minute))
	}
	if s.Count != 1 {
		t.Errorf("50 dead answers in one morning: %d strikes, want 1", s.Count)
	}
	// Inconclusive answers in between neither help nor hurt.
	s = Strikes{}
	for i, v := range []Verdict{Dead, Inconclusive, Inconclusive, Dead, Inconclusive, Dead} {
		s = Next(s, v, t0.Add(time.Duration(i)*24*time.Hour))
	}
	if !s.Retired() {
		t.Error("three dead days with inconclusive days between retire")
	}
}

func TestDue(t *testing.T) {
	if !(Strikes{}).Due(t0) {
		t.Error("no strikes: due")
	}
	s := Strikes{Count: 1, Last: t0}
	if s.Due(t0.Add(time.Hour)) || !s.Due(t0.Add(MinStrikeGap)) {
		t.Error("struck: due once the gap has passed")
	}
}

func TestClassifyStatuses(t *testing.T) {
	u := "https://jobs.example.com/acme/12345"
	for status, want := range map[int]Verdict{
		200: Alive, 204: Alive, 404: Dead, 410: Dead,
		0: Inconclusive, 400: Inconclusive, 401: Inconclusive, 403: Inconclusive, 405: Inconclusive,
		406: Inconclusive, 429: Inconclusive, 451: Inconclusive, 500: Inconclusive, 502: Inconclusive,
		503: Inconclusive, 504: Inconclusive, 999: Inconclusive,
	} {
		if got := Classify(u, status, "").Verdict; got != want {
			t.Errorf("HTTP %d: %s, want %s", status, got, want)
		}
	}
}

func TestRedirectVerdict(t *testing.T) {
	cases := []struct {
		name, from, to string
		want           Verdict
	}{
		{"no redirect", "https://jobs.lever.co/acme/ad346610-e4a6-4cd2-a357-11d731b136a2", "https://jobs.lever.co/acme/ad346610-e4a6-4cd2-a357-11d731b136a2", Alive},
		{"trailing slash and www", "https://acme.com/careers/job/8018853", "https://www.acme.com/careers/job/8018853/", Alive},
		{"board moved host, same id", "https://boards.greenhouse.io/acme/jobs/6594261003?gh_jid=6594261003", "https://job-boards.greenhouse.io/acme/jobs/6594261003", Alive},
		{"custom page keeps gh_jid", "https://acme.com/jobs?gh_jid=8292367", "https://acme.com/en/jobs?gh_jid=8292367", Alive},
		{"workday req id kept", "https://x.wd1.myworkdayjobs.com/Ext/job/City/Intern_R108218", "https://x.wd1.myworkdayjobs.com/en-US/Ext/job/City/Intern_R108218", Alive},
		{"greenhouse closed job", "https://job-boards.greenhouse.io/acme/jobs/4706444006", "https://job-boards.greenhouse.io/acme?error=true", Dead},
		{"careers index", "https://acme.com/careers/jobs/5708589004?gh_jid=5708589004", "https://acme.com/careers", Dead},
		{"home page", "https://careers.acme.com/jobs/103678?lang=en-us", "https://careers.acme.com/?lang=en-us", Dead},
		{"search page", "https://jobs.smartrecruiters.com/Acme/744000144233170-intern", "https://jobs.smartrecruiters.com/Acme", Dead},
		{"login page", "https://acme.wd5.myworkdayjobs.com/Ext/job/Intern_JR-011605", "https://acme.wd5.myworkdayjobs.com/Ext/login", Inconclusive},
		{"bot check", "https://acme.com/jobs/12345", "https://acme.com/cdn-cgi/challenge-platform/x", Inconclusive},
		{"sso host", "https://acme.com/jobs/12345", "https://sso.acme.com/start", Inconclusive},
		{"no id to compare", "https://acme.com/careers/intern", "https://acme.com/careers", Inconclusive},
	}
	for _, c := range cases {
		if got, _ := RedirectVerdict(c.from, c.to); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestIDTokens(t *testing.T) {
	cases := map[string][]string{
		"https://jobs.ashbyhq.com/gritt/df9254b5-e323-411a-a36f-57bd24a3de56":          {"df9254b5", "57bd24a3de56"},
		"https://x.wd1.myworkdayjobs.com/T/job/Singapore/Data-Engineer-Intern_R108218": {"108218"},
		"https://acme.com/jobs?gh_jid=8292367":                                         {"8292367"},
		"https://apply.workable.com/j/9F465A2E1A/apply":                                {"9f465a2e1a"},
		"https://acme.com/careers/software-engineer-intern":                            nil,
		"https://acme.com/jobs/123":                                                    nil,
	}
	for raw, want := range cases {
		u, _ := url.Parse(raw)
		if got := IDTokens(u); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
}

// script answers requests by method from a fixed list and records them.
type script struct {
	mu    sync.Mutex
	calls []string
	reply func(r *http.Request) (*http.Response, error)
}

func (s *script) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.calls = append(s.calls, r.Method+" "+r.URL.String())
	s.mu.Unlock()
	return s.reply(r)
}

func resp(r *http.Request, status int, loc string) *http.Response {
	h := http.Header{}
	if loc != "" {
		h.Set("Location", loc)
	}
	return &http.Response{StatusCode: status, Header: h, Body: http.NoBody, Request: r}
}

func TestProbe(t *testing.T) {
	const u = "https://jobs.example.com/acme/12345"
	cases := []struct {
		name  string
		reply func(r *http.Request) (*http.Response, error)
		want  Verdict
		calls int
	}{
		{"head ok", func(r *http.Request) (*http.Response, error) { return resp(r, 200, ""), nil }, Alive, 1},
		{"404 confirmed by GET", func(r *http.Request) (*http.Response, error) { return resp(r, 404, ""), nil }, Dead, 2},
		{"410", func(r *http.Request) (*http.Response, error) { return resp(r, 410, ""), nil }, Dead, 2},
		{"HEAD 404 but GET works", func(r *http.Request) (*http.Response, error) {
			if r.Method == "HEAD" {
				return resp(r, 404, ""), nil
			}
			return resp(r, 200, ""), nil
		}, Alive, 2},
		{"HEAD 404, GET 503", func(r *http.Request) (*http.Response, error) {
			if r.Method == "HEAD" {
				return resp(r, 404, ""), nil
			}
			return resp(r, 503, ""), nil
		}, Inconclusive, 2},
		{"HEAD not allowed, GET 404", func(r *http.Request) (*http.Response, error) {
			if r.Method == "HEAD" {
				return resp(r, 405, ""), nil
			}
			return resp(r, 404, ""), nil
		}, Dead, 2},
		{"403 is a refusal, not a dead link", func(r *http.Request) (*http.Response, error) { return resp(r, 403, ""), nil }, Inconclusive, 1},
		{"401", func(r *http.Request) (*http.Response, error) { return resp(r, 401, ""), nil }, Inconclusive, 1},
		{"429, no retry", func(r *http.Request) (*http.Response, error) { return resp(r, 429, ""), nil }, Inconclusive, 1},
		{"500, no retry", func(r *http.Request) (*http.Response, error) { return resp(r, 500, ""), nil }, Inconclusive, 1},
		{"network error", func(r *http.Request) (*http.Response, error) { return nil, errors.New("connection reset") }, Inconclusive, 1},
		{"redirect to the careers index", func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/acme/12345" {
				return resp(r, 302, "https://jobs.example.com/acme"), nil
			}
			return resp(r, 200, ""), nil
		}, Dead, 2},
		{"redirect that keeps the id", func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == "jobs.example.com" {
				return resp(r, 301, "https://new.example.com/acme/jobs/12345"), nil
			}
			return resp(r, 200, ""), nil
		}, Alive, 2},
	}
	for _, c := range cases {
		s := &script{reply: c.reply}
		got := Probe(fetch.WithRoundTripper(context.Background(), s), u)
		if got.Verdict != c.want {
			t.Errorf("%s: %s (%s), want %s", c.name, got.Verdict, got.Reason, c.want)
		}
		if len(s.calls) != c.calls {
			t.Errorf("%s: %d requests %v, want %d", c.name, len(s.calls), s.calls, c.calls)
		}
	}
}
