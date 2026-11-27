package ingest

import (
	"context"
	"hash/fnv"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/applycheck"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state"
	"github.com/dnexdev/openintern/internal/state/jstime"
)

// Apply-link probe defaults (see ApplyProbeOptions).
const (
	DefaultProbeMaxPerRun   = 300
	DefaultProbeMaxPerHost  = 40
	DefaultProbeRotation    = 7
	DefaultProbePace        = 500 * time.Millisecond
	DefaultProbeBudget      = 10 * time.Minute
	probeRetiredListedLimit = 20
)

// ApplyProbeOptions turns on the apply-link probe: after the boards are
// read, ingest checks the apply links of jobs it saw this run and retires a
// job once its link has been dead StrikesToRetire times in a row
// (internal/applycheck). Zero values take the defaults above.
type ApplyProbeOptions struct {
	// Probe checks one link; applycheck.Probe in production.
	Probe func(ctx context.Context, url string) applycheck.Result
	// MaxPerRun and MaxPerHost cap the requests of one run.
	MaxPerRun, MaxPerHost int
	// RotationDays: a job without strikes is checked once every this many
	// days (by a hash of its id); jobs with strikes are checked every run
	// once applycheck.MinStrikeGap has passed.
	RotationDays int
	// Pace is the wait between two requests (all hosts; requests rotate
	// between hosts, so one host sees them further apart).
	Pace time.Duration
	// Budget stops the probe phase after this long; the rest wait.
	Budget time.Duration
	// Sleep waits between requests; nil uses a timer. Tests stub it.
	Sleep func(ctx context.Context, d time.Duration) error
	// Approved reports a data/expected-delta.json approval for a scope
	// ("company:<slug>"); an approved company is not held. nil approves
	// nothing.
	Approved func(scope string) bool
}

// ApplyProbeRetired is a job the probe took off the board.
type ApplyProbeRetired struct {
	Company, ID, URL, Reason string
}

// ApplyProbeSummary is what the probe phase did.
type ApplyProbeSummary struct {
	Probed, Alive, Dead, Inconclusive int
	// Strikes counts strikes added this run; Retired the jobs that reached
	// StrikesToRetire; Cleared the retired-for-dead-link jobs whose link
	// works again (the next run's upsert brings them back).
	Strikes, Retired, Cleared int
	// Deferred were due but over a cap, the time budget, or on a host that
	// refused or throttled earlier in the run.
	Deferred int
	// HostsBacked are hosts that answered 401/403/429; the run stopped
	// probing them.
	HostsBacked []string
	RetiredJobs []ApplyProbeRetired
	// HeldCompanies list at least SuspectedBlockMinJobs jobs and every one
	// of their links has strikes. That looks like a moved, switched-off or
	// robot-blocking careers site rather than closed postings, so no job of
	// theirs is retired for its link and a person is asked to look (Digest)
	// unless data/expected-delta.json approves "company:<slug>", which the
	// state gate's collapse check honours too.
	HeldCompanies []string
}

type probeCandidate struct {
	job      *state.Job
	slug     string
	host     string
	strikes  applycheck.Strikes
	retired  bool // already retired for a dead link, still listed
	priority int
	order    uint32
}

func fnv32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// jobStrikes reads a job's strike record. A struck_at that is not a date
// makes the strike old enough to be due, never younger.
func jobStrikes(j *state.Job) applycheck.Strikes {
	if j.ApplyStrikes <= 0 {
		return applycheck.Strikes{}
	}
	s := applycheck.Strikes{Count: j.ApplyStrikes}
	if ms, ok := jstime.Parse(j.ApplyStruckAt); ok {
		s.Last = jstime.ToTime(ms)
	}
	return s
}

// probeable is an https apply link with no credentials or port: the only
// links the probe will request.
func probeable(check registry.ApplyCheck) bool {
	if check.OK {
		return true
	}
	return check.Reason == registry.ReasonOffList || check.Reason == registry.ReasonUnknownCompany
}

// probeApplyLinks is the probe phase of Run. seen are the jobs a board
// listed this run; jobs of failed fetches, dropped postings and manual
// captures (whose sites refuse automated requests) are never probed.
func probeApplyLinks(ctx context.Context, st *state.State, reg *registry.Registry, po ApplyProbeOptions, seen map[string]bool, now time.Time, ts string) *ApplyProbeSummary {
	sum := &ApplyProbeSummary{HostsBacked: []string{}, RetiredJobs: []ApplyProbeRetired{}, HeldCompanies: []string{}}
	if po.Probe == nil {
		return sum
	}
	maxRun, maxHost, rotation := po.MaxPerRun, po.MaxPerHost, po.RotationDays
	if maxRun <= 0 {
		maxRun = DefaultProbeMaxPerRun
	}
	if maxHost <= 0 {
		maxHost = DefaultProbeMaxPerHost
	}
	if rotation <= 0 {
		rotation = DefaultProbeRotation
	}
	pace, budget := po.Pace, po.Budget
	if pace <= 0 {
		pace = DefaultProbePace
	}
	if budget <= 0 {
		budget = DefaultProbeBudget
	}
	sleep := po.Sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	day := now.UnixMilli() / (24 * 60 * 60 * 1000)
	dayKey := "/" + time.UnixMilli(now.UnixMilli()).UTC().Format("2006-01-02")

	// listed are the jobs the probe is about, by company: seen this run,
	// not a manual capture, active or retired for a dead link. A company
	// with a struck job is suspect: all of its listed jobs are probed every
	// run, so a site that fails every link shows up as one (the hold below).
	listed := map[string][]*state.Job{}
	suspect := map[string]bool{}
	for _, j := range sortedJobs(st) {
		if !seen[j.ID] {
			continue
		}
		c := st.Companies[j.CompanyID]
		if c == nil || ManualATS[c.ATS] {
			continue
		}
		if !truthy(j.Value("is_active")) && !jobStrikes(j).Retired() {
			continue
		}
		listed[j.CompanyID] = append(listed[j.CompanyID], j)
		if j.ApplyStrikes > 0 {
			suspect[j.CompanyID] = true
		}
	}
	var cands []probeCandidate
	for _, j := range sortedJobs(st) {
		if !seen[j.ID] {
			continue
		}
		c := st.Companies[j.CompanyID]
		if c == nil || ManualATS[c.ATS] {
			continue
		}
		s := jobStrikes(j)
		active := truthy(j.Value("is_active"))
		retired := !active && s.Retired()
		if !active && !retired {
			continue
		}
		if !s.Due(now) || !probeable(reg.CheckApplyURL(c.Slug, j.ApplyURL)) {
			continue
		}
		u, err := url.Parse(j.ApplyURL)
		if err != nil || u.Hostname() == "" {
			continue
		}
		inTurn := int64(fnv32(j.ID))%int64(rotation) == day%int64(rotation)
		p := -1
		switch {
		case active && s.Count > 0:
			p = 0 // confirm a strike
		case active && suspect[j.CompanyID]:
			p = 1 // a sibling is struck: is it the whole site?
		case active && inTurn:
			p = 2
		case retired && inTurn:
			p = 3 // has the link come back?
		}
		if p < 0 {
			continue
		}
		cands = append(cands, probeCandidate{job: j, slug: c.Slug, host: strings.ToLower(u.Hostname()),
			strikes: s, retired: retired, priority: p, order: fnv32(j.ID + dayKey)})
	}
	sort.SliceStable(cands, func(a, b int) bool {
		if cands[a].priority != cands[b].priority {
			return cands[a].priority < cands[b].priority
		}
		return cands[a].order < cands[b].order
	})

	// Caps, then rotate between hosts so one host's requests are spread out.
	perHost := map[string][]probeCandidate{}
	var hosts []string
	taken := 0
	for _, c := range cands {
		if taken >= maxRun || len(perHost[c.host]) >= maxHost {
			sum.Deferred++
			continue
		}
		if perHost[c.host] == nil {
			hosts = append(hosts, c.host)
		}
		perHost[c.host] = append(perHost[c.host], c)
		taken++
	}
	var queue []probeCandidate
	for i := 0; len(queue) < taken; i++ {
		for _, h := range hosts {
			if i < len(perHost[h]) {
				queue = append(queue, perHost[h][i])
			}
		}
	}

	pctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	backed := map[string]bool{}
	sent := 0
	type pending struct {
		c      probeCandidate
		next   applycheck.Strikes
		reason string
	}
	var retiring []pending
	for _, c := range queue {
		if backed[c.host] || pctx.Err() != nil {
			sum.Deferred++
			continue
		}
		if sent > 0 {
			if err := sleep(pctx, pace); err != nil {
				sum.Deferred++
				continue
			}
		}
		sent++
		res := po.Probe(pctx, c.job.ApplyURL)
		if pctx.Err() != nil && res.Reason == "cancelled" {
			sum.Deferred++
			continue
		}
		sum.Probed++
		switch res.Verdict {
		case applycheck.Alive:
			sum.Alive++
		case applycheck.Dead:
			sum.Dead++
		default:
			sum.Inconclusive++
			if res.Status == 401 || res.Status == 403 || res.Status == 429 {
				backed[c.host] = true
				sum.HostsBacked = append(sum.HostsBacked, c.host)
			}
		}
		next := applycheck.Next(c.strikes, res.Verdict, now)
		if next == c.strikes {
			continue
		}
		if !c.retired && next.Retired() {
			retiring = append(retiring, pending{c, next, res.Reason})
			continue
		}
		struckAt := c.job.ApplyStruckAt
		if next.Count > c.strikes.Count {
			sum.Strikes++
			struckAt = ts
		}
		c.job.SetApplyStrikes(next.Count, struckAt)
		if c.retired && next.Count == 0 {
			sum.Cleared++
		}
	}

	// Hold a company with at least SuspectedBlockMinJobs listed jobs whose
	// every listed link has strikes: that is a moved or blocked site, not a
	// set of closed postings. Jobs about to retire stay at their strikes.
	allStruck := map[string]bool{}
	for cid, jobs := range listed {
		if len(jobs) < SuspectedBlockMinJobs {
			continue
		}
		all := true
		for _, j := range jobs {
			if j.ApplyStrikes == 0 {
				all = false
				break
			}
		}
		allStruck[cid] = all
	}
	held := map[string]bool{}
	for _, p := range retiring {
		cid := p.c.job.CompanyID
		approved := po.Approved != nil && po.Approved("company:"+p.c.slug)
		if allStruck[cid] && !approved {
			if !held[cid] {
				held[cid] = true
				sum.HeldCompanies = append(sum.HeldCompanies, p.c.slug)
			}
			continue // record unchanged: probed again next run, held again
		}
		sum.Strikes++
		p.c.job.SetApplyStrikes(p.next.Count, ts)
		p.c.job.Deactivate(ts)
		sum.Retired++
		sum.RetiredJobs = append(sum.RetiredJobs, ApplyProbeRetired{Company: p.c.slug, ID: p.c.job.ID, URL: p.c.job.ApplyURL, Reason: p.reason})
	}
	sort.Strings(sum.HostsBacked)
	sort.Strings(sum.HeldCompanies)
	sort.SliceStable(sum.RetiredJobs, func(a, b int) bool {
		x, y := sum.RetiredJobs[a], sum.RetiredJobs[b]
		if x.Company != y.Company {
			return x.Company < y.Company
		}
		return x.URL < y.URL
	})
	return sum
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
