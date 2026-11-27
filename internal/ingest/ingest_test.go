package ingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/dump"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/state"
)

func ptr[T any](v T) *T { return &v }

// Ports packages/ingest/src/ingest.test.ts.

func TestSightingAndRetirement(t *testing.T) {
	// #24: a closed posting on a board where nothing else matches must be retired.
	seen := SightJob(nil, false)
	if seen != proven.JobSightingGone {
		t.Errorf("closed posting is gone from the board: %v", seen)
	}
	if !proven.ShouldRetireJob(proven.BoardRunListed, 0, seen) {
		t.Error("#24: gone posting is retired")
	}
	cases := []struct {
		desc    string
		matched bool
		want    proven.JobSighting
	}{
		{"SWE intern", true, proven.JobSightingMatched},
		{"Senior engineer", false, proven.JobSightingRejected},
		{"  ", false, proven.JobSightingUnverified},
	}
	for _, c := range cases {
		if got := SightJob(&model.NormalizedJob{Description: c.desc}, c.matched); got != c.want {
			t.Errorf("SightJob(%q, %v) = %v, want %v", c.desc, c.matched, got, c.want)
		}
	}
	if proven.ShouldRetireJob(proven.BoardRunListed, 0, proven.JobSightingUnverified) {
		t.Error("missing details keep the job")
	}
	if proven.ShouldRetireJob(proven.BoardRunFailed, 5, proven.JobSightingGone) {
		t.Error("failed fetch retires nothing")
	}
	if proven.ShouldRetireJob(proven.BoardRunEmpty, 0, proven.JobSightingGone) || proven.ShouldRetireJob(proven.BoardRunEmpty, 1, proven.JobSightingGone) {
		t.Error("first and second empty runs wait")
	}
	if !proven.ShouldRetireJob(proven.BoardRunEmpty, 2, proven.JobSightingGone) {
		t.Error("third empty run retires")
	}
}

func TestCountPriorEmptyRuns(t *testing.T) {
	ok := func(fetched string) HistoryRun {
		return HistoryRun{Status: "ok", Error: ptr("funnel fetched=" + fetched + " title=0 tech=0 upserted=0")}
	}
	errRun := HistoryRun{Status: "error", Error: ptr("HTTP 500")}
	cases := []struct {
		name string
		runs []HistoryRun
		want int
	}{
		{"no history", nil, 0},
		{"two empty runs, then a listed one", []HistoryRun{ok("0"), ok("0"), ok("4")}, 2},
		{"failed runs don't break or add to the count", []HistoryRun{ok("0"), errRun, ok("0"), ok("3")}, 2},
		{"latest run listed postings", []HistoryRun{ok("7"), ok("0")}, 0},
		{"zero_match prefix still parses", []HistoryRun{{Status: "ok", Error: ptr("zero_match: no tech internships classified; funnel fetched=0 title=0 tech=0 upserted=0")}}, 1},
		{"an ok run without a funnel ends the count", []HistoryRun{{Status: "ok"}, ok("0")}, 0},
	}
	for _, c := range cases {
		if got := CountPriorEmptyRuns(c.runs); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestAutoDisableState(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	fails := func(n int, newestAgoDays int) []HistoryRun {
		out := make([]HistoryRun, n)
		for i := range out {
			out[i] = HistoryRun{Status: "error", RanAt: now.Add(-time.Duration(newestAgoDays+i) * day)}
		}
		return out
	}
	cases := []struct {
		name   string
		runs   []HistoryRun
		exempt bool
		want   AutoDisable
	}{
		{"below the limit", fails(ConsecutiveFailureLimit-1, 1), false, Enabled},
		{"at the limit", fails(ConsecutiveFailureLimit, 1), false, Disabled},
		{"tier-1 is exempt", fails(ConsecutiveFailureLimit, 1), true, Enabled},
		{"one success re-enables", append([]HistoryRun{{Status: "ok", RanAt: now.Add(-day)}}, fails(ConsecutiveFailureLimit, 2)...), false, Enabled},
		{"weekly recheck", fails(ConsecutiveFailureLimit, DisabledRecheckDays), false, Recheck},
	}
	for _, c := range cases {
		if got := AutoDisableState(c.runs, c.exempt, now); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestShouldSkipLastSeenSweep(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	at := func(t time.Time) *time.Time { return &t }
	cases := []struct {
		name string
		last *time.Time
		want bool
	}{
		{"no previous run skips the sweep", nil, true},
		{"daily runs keep sweeping", at(now.Add(-day)), false},
		{"exactly the gap limit still sweeps", at(now.Add(-OutageGapDays * day)), false},
		{"first run after an outage skips the sweep", at(now.Add(-OutageGapDays*day - time.Millisecond)), true},
		{"Neon quota outage from 2026-10-24 skips the sweep", at(time.Date(2026, 10, 23, 16, 23, 0, 0, time.UTC)), true},
	}
	for _, c := range cases {
		if got := ShouldSkipLastSeenSweep(c.last, now); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestDigestAndStepSummary(t *testing.T) {
	base := Summary{Companies: 1}
	if Digest(base) != "" {
		t.Error("nothing to report")
	}
	d := Digest(Summary{AutoDisabled: []string{"acme"}, SuspectedBlocks: []string{"tiktok"}})
	if !strings.Contains(d, "`acme`") || !strings.Contains(d, "`tiktok`") {
		t.Errorf("digest lists both: %s", d)
	}
	sum := StepSummary(Summary{Shadow: []ShadowResult{{Source: "tiktok", Status: ptr(403), LegacyCount: 324, Error: ptr("refused: HTTP 403")}}})
	if !strings.Contains(sum, "| tiktok | false | 403 |  | 324 |") {
		t.Errorf("shadow table row: %s", sum)
	}
}

func TestFetchAllBoards(t *testing.T) {
	job := func(id string) model.NormalizedJob {
		return model.NormalizedJob{ExternalID: id, Title: id, ApplyURL: "https://x/" + id}
	}
	boards := map[string][]model.NormalizedJob{"a": {job("1"), job("2")}, "b": {job("2"), job("3")}}
	merged, err := FetchAllBoards(context.Background(), "workday", []string{"a", "b"}, func(_ context.Context, _, tok string) ([]model.NormalizedJob, error) {
		return boards[tok], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, j := range merged {
		ids = append(ids, j.ExternalID)
	}
	if strings.Join(ids, ",") != "1,2,3" {
		t.Errorf("boards merge by posting id: %v", ids)
	}
	_, err = FetchAllBoards(context.Background(), "workday", []string{"a", "missing"}, func(_ context.Context, _, tok string) ([]model.NormalizedJob, error) {
		if boards[tok] == nil {
			return nil, errors.New("refused")
		}
		return boards[tok], nil
	})
	if err == nil {
		t.Error("one failed board fails the run")
	}

	// A manual capture past its age limit comes back as expired, not as an error.
	raw, expired, err := FetchCompanyBoards(context.Background(), "tesla", []string{"t"}, func(context.Context, string, string) ([]model.NormalizedJob, error) {
		return nil, fetch.NewFailure(fetch.KindManualExpired, "too old", 0)
	})
	if err != nil || expired == nil || len(raw) != 0 {
		t.Errorf("expired capture: %v %v %v", raw, expired, err)
	}
}

func TestInternshipTitle(t *testing.T) {
	for title, want := range map[string]bool{
		"Software Engineering Intern":   true,
		"INTERNSHIP - Data":             true,
		"Co-op Student":                 true,
		"Coop":                          true,
		"Apprenticeship Program":        true,
		"Project\u00a0Intern":           true, // JS \s includes NBSP
		"Campus Hire Intern":            true,
		"Year at Acme":                  true,
		"Internal Tools Engineer":       false,
		"Senior Engineer":               false,
		"Inſternship":                   false,
		"Internſhip":                    true, // \b is ASCII: "intern" ends before ſ
		"Iñtern":                        false,
		"International Business Intern": true,
	} {
		if got := IsInternshipTitle(title); got != want {
			t.Errorf("IsInternshipTitle(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestSlice16(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"a🚀b", 2, "a\xed\xa0\xbd"}, // the cut leaves the high surrogate
		{"a🚀b", 3, "a🚀"},
		{"é", 1, "é"},
	}
	for _, c := range cases {
		if got := Slice16(c.in, c.n); got != c.want {
			t.Errorf("Slice16(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

// Ports the ingest part of packages/ingest/src/state.test.ts: three runs
// over one Greenhouse board, then a dump.
func TestIngestStateRoundTrip(t *testing.T) {
	intern := model.NormalizedJob{ExternalID: "1", Title: "Software Engineer Intern (Summer 2027)",
		ApplyURL: "https://job-boards.greenhouse.io/khanacademy/jobs/1", Description: "Software engineering internship, React."}
	designer := model.NormalizedJob{ExternalID: "2", Title: "Senior Product Designer",
		ApplyURL: "https://job-boards.greenhouse.io/khanacademy/jobs/2", Description: "Design role."}
	var board []model.NormalizedJob
	fetcher := func(_ context.Context, ats, token string) ([]model.NormalizedJob, error) {
		if ats == model.Greenhouse && token == "khanacademy" {
			return board, nil
		}
		return nil, fetch.NewFailure(fetch.KindNotFound, "HTTP 404", 404)
	}
	dir := filepath.Join(t.TempDir(), "state")
	only := map[string]bool{"khan-academy": true}
	t0 := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	ctx := context.Background()

	st := state.New()
	board = []model.NormalizedJob{intern, designer}
	s, err := Run(ctx, st, Options{Fetch: fetcher, Now: t0, Only: only})
	if err != nil {
		t.Fatal(err)
	}
	if s.JobsUpserted != 1 {
		t.Errorf("one internship ingested, got %d", s.JobsUpserted)
	}
	if !s.LastSeenSweepSkipped {
		t.Error("first run ever skips the sweep")
	}
	if err := state.Write(dir, st); err != nil {
		t.Fatal(err)
	}

	st, _ = state.Load(dir)
	if len(st.Jobs) != 1 {
		t.Fatalf("jobs %d", len(st.Jobs))
	}
	var job *state.Job
	for _, j := range st.Jobs {
		job = j
	}
	if !job.IsActive {
		t.Error("job active")
	}
	firstSeen := job.FirstSeenAt
	if !regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d*[1-9])?\+00:00$`).MatchString(firstSeen) {
		t.Errorf("Postgres timestamp format: %q", firstSeen)
	}

	board = []model.NormalizedJob{designer} // #24: the internship closed
	s, err = Run(ctx, st, Options{Fetch: fetcher, Now: t0.Add(day), Only: only, SkipRegistrySync: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.JobsDeactivated != 1 {
		t.Errorf("closed posting retired, got %d", s.JobsDeactivated)
	}
	state.Write(dir, st)
	st, _ = state.Load(dir)
	if st.Jobs[job.ID].IsActive {
		t.Error("retired job still active")
	}
	if st.Jobs[job.ID].FirstSeenAt != firstSeen {
		t.Error("first_seen_at kept byte for byte")
	}

	board = []model.NormalizedJob{intern} // reposted: same id, same first_seen_at
	if _, err := Run(ctx, st, Options{Fetch: fetcher, Now: t0.Add(2 * day), Only: only, SkipRegistrySync: true}); err != nil {
		t.Fatal(err)
	}
	if len(st.Jobs) != 1 || !st.Jobs[job.ID].IsActive {
		t.Error("same fingerprint, same job, active again")
	}
	if len(st.Runs) != 3 {
		t.Errorf("runs %d", len(st.Runs))
	}
	if err := state.Write(dir, st); err != nil {
		t.Fatal(err)
	}
	jobsFile, _ := os.ReadFile(filepath.Join(dir, state.JobsFile))
	if strings.Contains(string(jobsFile), "last_seen_at") {
		t.Error("per-run timestamps stay out of jobs.jsonl")
	}

	out := filepath.Join(t.TempDir(), "dump")
	r, err := dump.FromState(st, out, t0.Add(2*day), dump.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count != 1 {
		t.Errorf("dump count %d", r.Count)
	}
	healthJSON, _ := os.ReadFile(filepath.Join(out, "health.json"))
	if n := strings.Count(string(healthJSON), `"zero_match":`); n != 3 {
		t.Errorf("health lists %d runs, want 3", n)
	}

	// Two active jobs with the same URL prefix in one company are refused.
	a := *st.Jobs[job.ID]
	a.ID = a.ID[:8] + "-ffff-4fff-8fff-ffffffffffff"
	a.Fingerprint = "other"
	st.Jobs["x"] = &a
	if !strings.Contains(strings.Join(state.Validate(st), ","), "share a URL prefix") {
		t.Error("shared prefix not reported")
	}
	if err := state.Write(dir, st); err == nil || !strings.Contains(err.Error(), "refusing to write state") {
		t.Errorf("write: %v", err)
	}
}
