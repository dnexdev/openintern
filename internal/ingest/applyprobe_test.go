package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/applycheck"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state"
)

// probeWorld is one company (khan-academy on Greenhouse) whose board and
// apply links the test controls, run day by day through Run.
type probeWorld struct {
	t     *testing.T
	st    *state.State
	board []model.NormalizedJob
	fail  bool
	links map[string]applycheck.Result // by URL; missing means alive
	calls []string
	day   int
	opts  ApplyProbeOptions
}

var probeT0 = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func newProbeWorld(t *testing.T) *probeWorld {
	w := &probeWorld{t: t, st: state.New(), links: map[string]applycheck.Result{}}
	w.opts = ApplyProbeOptions{
		Probe: func(_ context.Context, u string) applycheck.Result {
			w.calls = append(w.calls, u)
			if r, ok := w.links[u]; ok {
				return r
			}
			return applycheck.Result{Verdict: applycheck.Alive, Status: 200}
		},
		// Every job is in turn every day unless a test narrows it.
		RotationDays: 1,
		Sleep:        func(context.Context, time.Duration) error { return nil },
	}
	return w
}

func internJob(id, link string) model.NormalizedJob {
	return model.NormalizedJob{ExternalID: id, Title: "Software Engineer Intern (Summer 2027)",
		ApplyURL: link, Description: "Software engineering internship, React."}
}

// run is one daily ingest, hours after the previous one.
func (w *probeWorld) run(hours int) Summary {
	w.t.Helper()
	w.calls = nil
	fetcher := func(_ context.Context, ats, token string) ([]model.NormalizedJob, error) {
		if w.fail {
			return nil, fetch.NewFailure(fetch.KindHTTP, "HTTP 500", 500)
		}
		if ats == model.Greenhouse && token == "khanacademy" {
			return w.board, nil
		}
		return nil, fetch.NewFailure(fetch.KindNotFound, "HTTP 404", 404)
	}
	w.day += hours
	opts := w.opts
	s, err := Run(context.Background(), w.st, Options{Fetch: fetcher, Now: probeT0.Add(time.Duration(w.day) * time.Hour),
		Only: map[string]bool{"khan-academy": true}, ApplyProbe: &opts})
	if err != nil {
		w.t.Fatal(err)
	}
	return s
}

func (w *probeWorld) job(link string) *state.Job {
	for _, j := range w.st.Jobs {
		if j.ApplyURL == link {
			return j
		}
	}
	w.t.Fatalf("no job with %s", link)
	return nil
}

const (
	linkA = "https://job-boards.greenhouse.io/khanacademy/jobs/1001"
	linkB = "https://job-boards.greenhouse.io/khanacademy/jobs/1002"
)

var dead404 = applycheck.Result{Verdict: applycheck.Dead, Status: 404, Reason: "HTTP 404"}

func TestProbeRetiresAfterThreeDeadDays(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA), internJob("1002", linkB)}
	w.run(0)
	w.links[linkA] = dead404
	for day, want := range []int{1, 2} {
		s := w.run(24)
		if j := w.job(linkA); j.ApplyStrikes != want || !j.IsActive {
			t.Fatalf("day %d: strikes %d active %v, want %d and active", day+1, j.ApplyStrikes, j.IsActive, want)
		}
		if s.ApplyProbe.Retired != 0 {
			t.Fatalf("day %d: retired early", day+1)
		}
	}
	s := w.run(24)
	a := w.job(linkA)
	if a.IsActive || a.ApplyStrikes != 3 || s.ApplyProbe.Retired != 1 {
		t.Fatalf("third dead day: active %v strikes %d retired %d", a.IsActive, a.ApplyStrikes, s.ApplyProbe.Retired)
	}
	if r := s.ApplyProbe.RetiredJobs; len(r) != 1 || r[0].URL != linkA || r[0].Company != "khan-academy" {
		t.Errorf("retired list %+v", r)
	}
	if !w.job(linkB).IsActive || w.job(linkB).ApplyStrikes != 0 {
		t.Error("the other job is untouched")
	}
	// The board still lists it: the next upsert keeps it retired.
	w.links[linkA] = applycheck.Result{Verdict: applycheck.Inconclusive, Status: 503}
	w.run(24)
	if w.job(linkA).IsActive {
		t.Error("a board that still lists a dead link does not bring the job back")
	}
	// The job stays in state, and the record survives a write and a load.
	dir := filepath.Join(t.TempDir(), "state")
	if err := state.Write(dir, w.st); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, state.JobsFile))
	if !strings.Contains(string(b), `"apply_strikes":3,"apply_struck_at":"`) {
		t.Errorf("strikes written to jobs.jsonl:\n%s", b)
	}
	if strings.Count(string(b), "apply_strikes") != 1 {
		t.Error("jobs without strikes carry no strike keys")
	}
	st, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if j := st.Jobs[a.ID]; j.ApplyStrikes != 3 || j.ApplyStruckAt != a.ApplyStruckAt || j.IsActive {
		t.Errorf("reloaded %+v", j.Job)
	}
	if again := state.Encode(st); string(again[state.JobsFile]) != string(b) {
		t.Error("state round-trips byte for byte")
	}
}

func TestProbeOneFlakyAnswerNeverRetires(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	w.run(0)
	answers := []applycheck.Result{dead404, {Verdict: applycheck.Alive, Status: 200}, dead404, dead404,
		{Verdict: applycheck.Alive, Status: 200}, dead404, dead404}
	for i, r := range answers {
		w.links[linkA] = r
		w.run(24)
		if !w.job(linkA).IsActive {
			t.Fatalf("retired on day %d without three dead days in a row", i+1)
		}
	}
	if got := w.job(linkA).ApplyStrikes; got != 2 {
		t.Errorf("strikes %d, want 2", got)
	}
}

func TestProbeInconclusiveNeverCounts(t *testing.T) {
	for _, status := range []int{0, 401, 403, 429, 500, 502, 503, 504} {
		w := newProbeWorld(t)
		w.board = []model.NormalizedJob{internJob("1001", linkA)}
		w.run(0)
		w.links[linkA] = applycheck.Result{Verdict: applycheck.Inconclusive, Status: status}
		for i := 0; i < 6; i++ {
			w.run(24)
		}
		if j := w.job(linkA); !j.IsActive || j.ApplyStrikes != 0 {
			t.Errorf("HTTP %d: active %v strikes %d", status, j.IsActive, j.ApplyStrikes)
		}
	}
	// ...and does not reset strikes already counted.
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	w.run(0)
	w.links[linkA] = dead404
	w.run(24)
	w.run(24)
	w.links[linkA] = applycheck.Result{Verdict: applycheck.Inconclusive, Status: 503}
	w.run(24)
	if w.job(linkA).ApplyStrikes != 2 {
		t.Errorf("strikes %d after an inconclusive day, want 2", w.job(linkA).ApplyStrikes)
	}
	w.links[linkA] = dead404
	w.run(24)
	if w.job(linkA).IsActive {
		t.Error("third dead day after an inconclusive one retires")
	}
}

func TestProbeSameDayRerunsDoNotStrike(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	w.run(0)
	w.links[linkA] = dead404
	for i := 0; i < 6; i++ {
		w.run(3)
	}
	if j := w.job(linkA); j.ApplyStrikes != 1 || !j.IsActive {
		t.Errorf("six runs in 18 hours: strikes %d active %v, want 1 and active", j.ApplyStrikes, j.IsActive)
	}
	// Runs inside the gap do not even send the probe.
	w.run(3)
	if len(w.calls) != 0 {
		t.Errorf("probed %v inside the strike gap", w.calls)
	}
}

func TestProbeLinkChangeOrRecoveryBringsJobBack(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	w.run(0)
	w.links[linkA] = dead404
	w.run(24)
	w.run(24)
	w.run(24)
	if w.job(linkA).IsActive {
		t.Fatal("retired")
	}
	// The link works again: the probe clears the strikes, the next upsert
	// brings the job back.
	delete(w.links, linkA)
	s := w.run(24)
	if s.ApplyProbe.Cleared != 1 || w.job(linkA).ApplyStrikes != 0 {
		t.Fatalf("cleared %d strikes %d", s.ApplyProbe.Cleared, w.job(linkA).ApplyStrikes)
	}
	w.run(24)
	if !w.job(linkA).IsActive {
		t.Error("working link: job back on the board")
	}

	// Retire it again, then the board gives the posting a new link.
	w.links[linkA] = dead404
	w.run(24)
	w.run(24)
	w.run(24)
	j := w.job(linkA)
	if j.IsActive {
		t.Fatal("retired again")
	}
	id := j.ID
	newLink := "https://job-boards.greenhouse.io/khanacademy/jobs/1001?gh_src=new"
	w.board = []model.NormalizedJob{internJob("1001", newLink)}
	w.run(24)
	if j := w.st.Jobs[id]; !j.IsActive || j.ApplyStrikes != 0 || j.ApplyURL != newLink {
		t.Errorf("new link: active %v strikes %d url %s", j.IsActive, j.ApplyStrikes, j.ApplyURL)
	}
}

func TestProbeOnlyJobsSeenThisRun(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	w.run(0)
	w.links[linkA] = dead404
	w.run(24)
	w.fail = true // the board fetch fails: its jobs are not probed
	s := w.run(24)
	if len(w.calls) != 0 || s.ApplyProbe.Probed != 0 {
		t.Errorf("probed %v after a failed fetch", w.calls)
	}
	if w.job(linkA).ApplyStrikes != 1 {
		t.Error("strikes unchanged")
	}
	w.fail = false
	w.board = nil // gone from the board: plain retirement, no probe
	w.run(24)
	if len(w.calls) != 0 {
		t.Errorf("probed a posting the board dropped: %v", w.calls)
	}
}

func TestProbeCapsAndHostBackoff(t *testing.T) {
	w := newProbeWorld(t)
	var board []model.NormalizedJob
	for i := 0; i < 10; i++ {
		board = append(board, internJob(strconv.Itoa(2000+i), "https://a.example/jobs/"+strconv.Itoa(2000+i)))
		board = append(board, internJob(strconv.Itoa(3000+i), "https://b.example/jobs/"+strconv.Itoa(3000+i)))
	}
	for i := 0; i < 3; i++ {
		board = append(board, internJob(strconv.Itoa(4000+i), "https://c.example/jobs/"+strconv.Itoa(4000+i)))
	}
	w.board = board
	w.opts.MaxPerHost = 4
	w.opts.MaxPerRun = 10
	s := w.run(0)
	p := s.ApplyProbe
	if p.Probed != 10 || p.Deferred != 13 {
		t.Errorf("probed %d deferred %d, want 10 and 13", p.Probed, p.Deferred)
	}
	perHost := map[string]int{}
	for _, c := range w.calls {
		perHost[strings.Split(c, "/")[2]]++
	}
	for h, n := range perHost {
		if n > 4 {
			t.Errorf("%s probed %d times, cap 4", h, n)
		}
	}
	if len(w.calls) > 1 && strings.Split(w.calls[0], "/")[2] == strings.Split(w.calls[1], "/")[2] {
		t.Errorf("requests rotate between hosts: %v", w.calls[:2])
	}

	// A host that refuses is left alone for the rest of the run.
	w.opts.MaxPerHost, w.opts.MaxPerRun = 0, 0
	for _, j := range board {
		if strings.Contains(j.ApplyURL, "a.example") {
			w.links[j.ApplyURL] = applycheck.Result{Verdict: applycheck.Inconclusive, Status: 403}
		}
	}
	s = w.run(24)
	n := 0
	for _, c := range w.calls {
		if strings.Contains(c, "a.example") {
			n++
		}
	}
	if n != 1 || len(s.ApplyProbe.HostsBacked) != 1 || s.ApplyProbe.HostsBacked[0] != "a.example" {
		t.Errorf("refusing host probed %d times, backed off %v", n, s.ApplyProbe.HostsBacked)
	}
}

func TestProbeRotationAndPriority(t *testing.T) {
	w := newProbeWorld(t)
	var board []model.NormalizedJob
	for i := 0; i < 70; i++ {
		board = append(board, internJob(strconv.Itoa(5000+i), "https://jobs.example/"+strconv.Itoa(5000+i)))
	}
	w.board = board
	w.opts.RotationDays = 7
	probed := map[string]int{}
	for day := 0; day < 7; day++ {
		w.run(24)
		for _, c := range w.calls {
			probed[c]++
		}
	}
	if len(probed) != 70 {
		t.Errorf("a week covers every job once: %d of 70", len(probed))
	}
	for u, n := range probed {
		if n != 1 {
			t.Errorf("%s probed %d times in a week", u, n)
		}
	}
	// A struck job is rechecked every day, ahead of the rotation.
	w.links[board[0].ApplyURL] = dead404
	for len(w.calls) == 0 || w.job(board[0].ApplyURL).ApplyStrikes == 0 {
		w.run(24)
	}
	w.opts.MaxPerRun = 1
	w.run(24)
	if len(w.calls) != 1 || w.calls[0] != board[0].ApplyURL {
		t.Errorf("struck job first: %v", w.calls)
	}
	w.run(24)
	if w.job(board[0].ApplyURL).IsActive {
		t.Error("confirmed on consecutive days, retired")
	}
}

func TestProbeSkipsManualSourcesAndBadLinks(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{
		internJob("1001", "http://job-boards.greenhouse.io/khanacademy/jobs/1001"),
		internJob("1002", "https://user:pw@job-boards.greenhouse.io/khanacademy/jobs/1002"),
		internJob("1003", "https://job-boards.greenhouse.io:8443/khanacademy/jobs/1003"),
	}
	w.run(0)
	if len(w.calls) != 0 {
		t.Errorf("probed non-https or credentialed links: %v", w.calls)
	}
	// Manual captures (Tesla, Citadel) come from sites that refuse automated
	// requests: their links are never probed.
	w.board = []model.NormalizedJob{internJob("1004", linkA)}
	for _, c := range w.st.Companies {
		c.ATS = model.Tesla
	}
	fetcher := func(context.Context, string, string) ([]model.NormalizedJob, error) { return w.board, nil }
	opts := w.opts
	w.calls = nil
	s, err := Run(context.Background(), w.st, Options{Fetch: fetcher, Now: probeT0.Add(24 * time.Hour),
		Only: map[string]bool{"khan-academy": true}, SkipRegistrySync: true, ApplyProbe: &opts})
	if err != nil {
		t.Fatal(err)
	}
	if s.JobsUpserted != 1 || len(w.calls) != 0 {
		t.Errorf("manual source: upserted %d, probed %v", s.JobsUpserted, w.calls)
	}
}

func TestProbeOffLeavesSummaryAndStateAlone(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	fetcher := func(_ context.Context, ats, token string) ([]model.NormalizedJob, error) { return w.board, nil }
	s, err := Run(context.Background(), w.st, Options{Fetch: fetcher, Now: probeT0, Only: map[string]bool{"khan-academy": true}})
	if err != nil {
		t.Fatal(err)
	}
	if s.ApplyProbe != nil || strings.Contains(string(s.JSON()), "applyProbe") || strings.Contains(StepSummary(s), "Apply links") {
		t.Error("no probe, no probe report")
	}
	// A job already retired for a dead link stays retired with the probe off.
	j := w.job(linkA)
	j.SetApplyStrikes(applycheck.StrikesToRetire, state.PgTimestamp(probeT0.UnixMilli()))
	j.Deactivate(state.PgTimestamp(probeT0.UnixMilli()))
	if _, err := Run(context.Background(), w.st, Options{Fetch: fetcher, Now: probeT0.Add(24 * time.Hour), Only: map[string]bool{"khan-academy": true}}); err != nil {
		t.Fatal(err)
	}
	if w.job(linkA).IsActive {
		t.Error("retired for a dead link, kept retired")
	}
}

func TestProbeSummaryReport(t *testing.T) {
	w := newProbeWorld(t)
	w.board = []model.NormalizedJob{internJob("1001", linkA)}
	w.run(0)
	w.links[linkA] = dead404
	w.run(24)
	w.run(24)
	s := w.run(24)
	js := string(s.JSON())
	for _, want := range []string{`"applyProbe": {`, `"retired": 1`, `"url": "` + linkA + `"`, `"reason": "HTTP 404"`} {
		if !strings.Contains(js, want) {
			t.Errorf("summary JSON lacks %s:\n%s", want, js)
		}
	}
	md := StepSummary(s)
	if !strings.Contains(md, "Apply links: 1 checked, 1 dead") || !strings.Contains(md, "- retired `khan-academy` "+linkA) {
		t.Errorf("step summary:\n%s", md)
	}
}

func TestProbeHoldsCompanyWhoseEveryLinkDies(t *testing.T) {
	for _, alive := range []int{0, 1} {
		w := newProbeWorld(t)
		var board []model.NormalizedJob
		for i := 0; i < 5+alive; i++ {
			link := "https://jobs.example/acme/" + strconv.Itoa(7000+i)
			board = append(board, internJob(strconv.Itoa(7000+i), link))
			if i < 5 {
				w.links[link] = dead404
			}
		}
		w.board = board
		w.run(0)
		var s Summary
		for i := 0; i < 5; i++ {
			s = w.run(24)
		}
		active := 0
		for _, j := range w.st.Jobs {
			if j.IsActive {
				active++
			}
		}
		p := s.ApplyProbe
		if alive == 0 {
			if active != 5 || p.Retired != 0 || len(p.HeldCompanies) != 1 || p.HeldCompanies[0] != "khan-academy" {
				t.Errorf("all dead: active %d retired %d held %v; want 5 kept and the company held", active, p.Retired, p.HeldCompanies)
			}
			if d := Digest(s); !strings.Contains(d, "Every apply link dead") || !strings.Contains(d, "`khan-academy`") {
				t.Errorf("digest asks a person:\n%s", d)
			}
			for _, j := range w.st.Jobs {
				if j.ApplyStrikes != applycheck.StrikesToRetire-1 {
					t.Errorf("held job strikes %d, want %d", j.ApplyStrikes, applycheck.StrikesToRetire-1)
				}
			}
		} else if active != 1 || len(p.HeldCompanies) != 0 {
			t.Errorf("one live link left: active %d held %v; want the 5 dead retired", active, p.HeldCompanies)
		}
		if alive == 0 {
			// A person approves the drop: the next run retires all five.
			w.opts.Approved = func(scope string) bool { return scope == "company:khan-academy" }
			s = w.run(24)
			if s.ApplyProbe.Retired != 5 || len(s.ApplyProbe.HeldCompanies) != 0 {
				t.Errorf("approved: retired %d held %v", s.ApplyProbe.Retired, s.ApplyProbe.HeldCompanies)
			}
		}
	}
}

// With the weekly rotation, first strikes land on different days. A site
// that fails every link must still be held as a whole, not retired one job
// at a time; a single dead posting among working ones still retires.
func TestProbeHoldSurvivesStaggeredRotation(t *testing.T) {
	for _, alive := range []int{0, 1} {
		w := newProbeWorld(t)
		w.opts.RotationDays = 7
		var board []model.NormalizedJob
		for i := 0; i < 5+alive; i++ {
			link := "https://jobs.example/acme/" + strconv.Itoa(8000+i)
			board = append(board, internJob(strconv.Itoa(8000+i), link))
			if i < 5 {
				w.links[link] = dead404
			}
		}
		w.board = board
		w.run(0)
		var held []string
		for day := 0; day < 14; day++ {
			s := w.run(24)
			held = append(held, s.ApplyProbe.HeldCompanies...)
			if alive == 0 && s.ApplyProbe.Retired != 0 {
				t.Fatalf("day %d: retired %v one by one", day+1, s.ApplyProbe.RetiredJobs)
			}
		}
		active := 0
		for _, j := range w.st.Jobs {
			if j.IsActive {
				active++
			}
		}
		switch {
		case alive == 0 && (active != 5 || len(held) == 0):
			t.Errorf("all links dead: active %d, held %v; want all 5 kept and held", active, held)
		case alive == 1 && active != 1:
			t.Errorf("one working link: active %d, want the 5 dead retired", active)
		}
	}

	// A struck job makes its siblings due the same day, out of turn.
	w := newProbeWorld(t)
	w.opts.RotationDays = 7
	var board []model.NormalizedJob
	for i := 0; i < 8; i++ {
		board = append(board, internJob(strconv.Itoa(9000+i), "https://jobs.example/acme/"+strconv.Itoa(9000+i)))
	}
	w.board = board
	w.run(0)
	j := w.job(board[0].ApplyURL)
	j.SetApplyStrikes(1, state.PgTimestamp(probeT0.UnixMilli()))
	w.run(24)
	if len(w.calls) != 8 {
		t.Errorf("suspect company: %d of 8 links probed", len(w.calls))
	}
}
