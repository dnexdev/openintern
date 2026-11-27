// Package ingest runs the daily ingest against flat-file state: registry
// sync, auto-disable, fetching every active company's boards, classifying
// postings, per-job retirement, the last-seen sweep with its outage guard,
// and run history. Ports packages/ingest/src/ingest-files.ts, plan.ts and
// the parts of ingest.ts they use; the Postgres path went with the database.
//
// The fetcher, clock and id generator are injected, so a run is
// reproducible: the same state, fetch results, clock and ids give the same
// state files as the TypeScript.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dnexdev/openintern/internal/applycheck"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/health"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state"
	"github.com/dnexdev/openintern/internal/state/jstime"
)

// ConsecutiveFailureLimit is how many daily runs in a row must fail before
// a company is auto-disabled.
const ConsecutiveFailureLimit = 7

// DisabledRecheckDays: an auto-disabled company is retried once this many
// days after its last run.
const DisabledRecheckDays = 7

// OutageGapDays: a gap longer than this between successful runs counts as an
// outage.
const OutageGapDays = 2

// Concurrency is how many companies are fetched at once.
const Concurrency = 8

// ManualATS are sources read from committed manual captures
// (data/manual/<ats>.json); they are never auto-disabled.
var ManualATS = map[string]bool{"tesla": true, "citadel": true, "citadel_securities": true}

// HistoryRun is a past run as the policies see it.
type HistoryRun struct {
	Status string
	Error  *string
	RanAt  time.Time
}

// CountPriorEmptyRuns is countPriorEmptyRuns: empty runs in a row before
// this one, from the company's recent runs (newest first). Failed runs are
// skipped: they leave the count unchanged (law dp_failed_keeps_count); the
// first run that listed postings ends it.
func CountPriorEmptyRuns(runs []HistoryRun) int {
	empty := 0
	for _, r := range runs {
		if r.Status != "ok" {
			continue
		}
		if f := health.ParseFunnel(r.Error).Fetched; f == nil || *f != 0 {
			break
		}
		empty++
	}
	return empty
}

// AutoDisable is what autoDisableState decides.
type AutoDisable string

const (
	Enabled  AutoDisable = "enabled"
	Disabled AutoDisable = "disabled"
	Recheck  AutoDisable = "recheck"
)

// AutoDisableState is autoDisableState. Auto-disable is derived from run
// history, so the YAML `active` flag stays the human switch and registry
// sync can't silently undo it. A company is disabled when its last
// ConsecutiveFailureLimit runs all failed, unless it is exempt (tier-1,
// manual captures). It is still fetched once every DisabledRecheckDays.
func AutoDisableState(recent []HistoryRun, exempt bool, now time.Time) AutoDisable {
	if exempt || len(recent) < ConsecutiveFailureLimit {
		return Enabled
	}
	for _, r := range recent[:ConsecutiveFailureLimit] {
		if r.Status != "error" {
			return Enabled
		}
	}
	if now.UnixMilli()-recent[0].RanAt.UnixMilli() >= DisabledRecheckDays*proven.DayMS {
		return Recheck
	}
	return Disabled
}

// ShouldSkipLastSeenSweep is shouldSkipLastSeenSweep. The last-seen sweep
// retires jobs not seen for LastSeenStaleDays of wall time. After an outage
// (Neon down, ingest disabled) every job looks unseen, so the first run back
// would retire jobs that are still open. Skip the sweep when the previous
// successful run is older than OutageGapDays; it resumes on the next run,
// once this run has refreshed last_seen_at.
func ShouldSkipLastSeenSweep(lastSuccessfulRunAt *time.Time, now time.Time) bool {
	if lastSuccessfulRunAt == nil {
		return true
	}
	return now.UnixMilli()-lastSuccessfulRunAt.UnixMilli() > OutageGapDays*proven.DayMS
}

// Fetcher reads one board; the signature of ats.Fetch. A failed read
// returns an error; *fetch.Failure with KindManualExpired marks a manual
// capture past its age limit.
type Fetcher func(ctx context.Context, atsName, boardToken string) ([]model.NormalizedJob, error)

// FetchAllBoards is fetchAllBoards: every board of a company, merged by
// posting id (the same requisition is often listed on several Workday
// sites). One failed board fails the whole run, so a partial fetch never
// retires the other boards' jobs.
func FetchAllBoards(ctx context.Context, ats string, tokens []string, fetchBoard Fetcher) ([]model.NormalizedJob, error) {
	seen := map[string]bool{}
	out := []model.NormalizedJob{}
	for _, token := range tokens {
		jobs, err := fetchBoard(ctx, ats, token)
		if err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if seen[j.ExternalID] {
				continue
			}
			seen[j.ExternalID] = true
			out = append(out, j)
		}
	}
	return out, nil
}

// FetchCompanyBoards is fetchCompanyBoards. A manual capture past its age
// limit is not an error to wait out: it comes back as expired so its jobs
// come down now (law dp_expired_retires). Every other failure is returned
// and recorded as a failed run, which retires nothing.
func FetchCompanyBoards(ctx context.Context, ats string, tokens []string, fetchBoard Fetcher) (raw []model.NormalizedJob, expired *fetch.Failure, err error) {
	raw, err = FetchAllBoards(ctx, ats, tokens, fetchBoard)
	if err != nil {
		var f *fetch.Failure
		if errors.As(err, &f) && f.Kind == fetch.KindManualExpired {
			return []model.NormalizedJob{}, f, nil
		}
		return nil, nil, err
	}
	return raw, nil, nil
}

// ShadowResult is one honest-request shadow test run beside a legacy
// request (logged only).
type ShadowResult struct {
	Source      string
	OK          bool
	Status      *int
	Count       *float64
	LegacyCount int
	Error       *string // absent from the JSON when nil
}

// Failure is a company whose fetch failed.
type Failure struct{ Slug, Error string }

// FunnelTotals sums the per-company funnels.
type FunnelTotals struct{ Fetched, InternshipTitle, TechPass, Upserted int }

// Summary is IngestSummary.
type Summary struct {
	Companies           int
	JobsUpserted        int
	JobsDeactivated     int
	StaleDeactivated    int
	LastSeenDeactivated int
	// LastSeenSweepSkipped is true when the pipeline had been down.
	LastSeenSweepSkipped bool
	ZeroMatchCompanies   []string
	// AutoDisabled are companies this run's failure auto-disabled; a person
	// should confirm.
	AutoDisabled []string
	// AutoDisabledSkipped are auto-disabled companies not fetched this run.
	AutoDisabledSkipped []string
	// SuspectedBlocks had several active jobs and now list none.
	SuspectedBlocks     []string
	Shadow              []ShadowResult
	ApplyHostViolations []ApplyHostViolation
	Failures            []Failure
	// Funnel is sorted by fetched, descending, then slug.
	Funnel       []CompanyFunnel
	FunnelTotals FunnelTotals
	// ApplyProbe is the apply-link probe phase, nil when it was off.
	ApplyProbe *ApplyProbeSummary
}

// Options configures Run.
type Options struct {
	// Fetch reads one board (ats.Fetch in production).
	Fetch Fetcher
	// Now is the run's clock; zero means time.Now().
	Now time.Time
	// NewID makes record ids; nil means state.NewID (random UUIDs).
	NewID func() string
	// Registry supplies companies, Tier 1 and the apply-host allowlist; nil
	// means the registry compiled into the binary.
	Registry *registry.Registry
	// SkipRegistrySync leaves companies as they are (syncRegistry: false).
	SkipRegistrySync bool
	// Only limits the run to these slugs (OPENINTERN_ONLY_SLUGS); nil means
	// every active company.
	Only map[string]bool
	// EnforceApplyHosts holds off-allowlist links (OPENINTERN_APPLY_HOSTS=enforce).
	EnforceApplyHosts bool
	// Shadow returns the shadow tests run during the fetches, if any.
	Shadow func() []ShadowResult
	// ApplyProbe, when set, checks the apply links of the jobs boards listed
	// this run and retires jobs whose link stays dead (applyprobe.go). nil
	// sends no probes; jobs already retired for a dead link stay retired
	// either way while their board lists the same link.
	ApplyProbe *ApplyProbeOptions
}

// SyncRegistry is syncRegistryIntoState: registry companies become company
// records (new slugs get ids), and active companies no longer in the
// registry go inactive with their jobs. It returns those orphans' slugs.
func SyncRegistry(st *state.State, reg *registry.Registry, now time.Time, newID func() string) []string {
	ts := state.PgTimestamp(now.UnixMilli())
	existing := map[string]*state.Company{}
	for _, c := range sortedCompanies(st) {
		existing[c.Slug] = c
	}
	inRegistry := map[string]bool{}
	for _, y := range reg.Unique() {
		inRegistry[y.Slug] = true
		prev := existing[y.Slug]
		next := &state.Company{Company: model.Company{
			Name: y.Name, Slug: y.Slug, ATS: y.ATS, BoardToken: y.BoardToken,
			CareersURL: y.CareersURL, WebsiteURL: y.WebsiteURL, Active: y.Active,
		}}
		if prev != nil {
			next.ID, next.CreatedAt, next.UpdatedAt = prev.ID, prev.CreatedAt, prev.UpdatedAt
		} else {
			next.ID, next.CreatedAt, next.UpdatedAt = newID(), ts, ts
		}
		if prev == nil || companyChanged(prev, next) {
			next.UpdatedAt = ts
		}
		st.Companies[next.ID] = next
	}
	var orphans []string
	for _, c := range sortedCompanies(st) {
		if truthy(c.Value("active")) && !inRegistry[c.Slug] {
			c.SetActive(false, ts)
			orphans = append(orphans, c.Slug)
			for _, j := range st.Jobs {
				if j.CompanyID == c.ID && truthy(j.Value("is_active")) {
					j.Deactivate(ts)
				}
			}
		}
	}
	return orphans
}

// companyChanged is JSON.stringify({...prev, updated_at: ""}) !==
// JSON.stringify({...next, updated_at: ""}): any field differs, or the
// stored record had keys or value types the fresh one does not.
func companyChanged(prev, next *state.Company) bool {
	if !prev.Shape.Plain() {
		return true
	}
	a, b := prev.Company, next.Company
	return a.ID != b.ID || a.Name != b.Name || a.Slug != b.Slug || a.ATS != b.ATS || a.BoardToken != b.BoardToken ||
		!eqPtr(a.CareersURL, b.CareersURL) || !eqPtr(a.WebsiteURL, b.WebsiteURL) || a.Active != b.Active ||
		a.CreatedAt != b.CreatedAt
}

func eqPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// sortedCompanies is the state's company order (slug, then id): the order
// the TypeScript Map had for a loaded state.
func sortedCompanies(st *state.State) []*state.Company {
	out := make([]*state.Company, 0, len(st.Companies))
	for _, c := range st.Companies {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if d := state.ByCodeUnit(out[i].Slug, out[j].Slug); d != 0 {
			return d < 0
		}
		return state.ByCodeUnit(out[i].ID, out[j].ID) < 0
	})
	return out
}

func sortedJobs(st *state.State) []*state.Job {
	out := make([]*state.Job, 0, len(st.Jobs))
	for _, j := range st.Jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool { return state.ByCodeUnit(out[i].ID, out[j].ID) < 0 })
	return out
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && x == x
	case string:
		return x != ""
	}
	return v != state.Undefined
}

// date is new Date(s) for a stored timestamp; ok is false for Invalid Date.
func date(s string) (time.Time, bool) {
	ms, ok := jstime.Parse(s)
	if !ok {
		return time.Time{}, false
	}
	return jstime.ToTime(ms), true
}

type fetched struct {
	raw     []model.NormalizedJob
	expired *fetch.Failure
	err     error
}

// Run is runIngestFiles: one ingest over st, changed in place. It fails
// only where the TypeScript threw (a stored date that is not a date); a
// failed board is a failed run in the summary.
func Run(ctx context.Context, st *state.State, opts Options) (Summary, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.Truncate(time.Millisecond)
	newID := opts.NewID
	if newID == nil {
		newID = state.NewID
	}
	reg := opts.Registry
	if reg == nil {
		reg = registry.Default()
	}
	if opts.Fetch == nil {
		return Summary{}, errors.New("ingest: no fetcher")
	}
	ts := state.PgTimestamp(now.UnixMilli())

	// The previous successful run, before this run records anything. A run
	// whose ran_at is not a date makes the maximum NaN, which never skips.
	var lastOk *time.Time
	nan := false
	var maxMS int64
	for _, r := range st.Runs {
		if r.Status != "ok" {
			continue
		}
		ms, ok := jstime.Parse(r.RanAt)
		if !ok {
			nan = true
			continue
		}
		if lastOk == nil || ms > maxMS {
			maxMS = ms
			t := jstime.ToTime(ms)
			lastOk = &t
		}
	}
	skipSweep := ShouldSkipLastSeenSweep(lastOk, now)
	if nan {
		skipSweep = false
	}

	if !opts.SkipRegistrySync {
		SyncRegistry(st, reg, now, newID)
	}

	extraBoards := map[string][]string{}
	for _, c := range reg.Unique() {
		extraBoards[c.Slug] = c.ExtraBoardTokens
	}
	history, err := buildHistory(st.Runs)
	if err != nil {
		return Summary{}, err
	}

	var active []*state.Company
	for _, c := range sortedCompanies(st) {
		if truthy(c.Value("active")) && (opts.Only == nil || opts.Only[c.Slug]) {
			active = append(active, c)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return state.LocaleCompare(active[i].Slug, active[j].Slug) < 0 })
	exempt := func(c *state.Company) bool { return reg.IsTier1(c.Slug) || ManualATS[c.ATS] }
	sum := Summary{
		ZeroMatchCompanies: []string{}, AutoDisabled: []string{}, AutoDisabledSkipped: []string{},
		SuspectedBlocks: []string{}, ApplyHostViolations: []ApplyHostViolation{}, Failures: []Failure{},
		Funnel: []CompanyFunnel{},
	}
	var toFetch []*state.Company
	for _, c := range active {
		if AutoDisableState(history[c.ID], exempt(c), now) != Disabled {
			toFetch = append(toFetch, c)
		} else {
			sum.AutoDisabledSkipped = append(sum.AutoDisabledSkipped, c.Slug)
		}
	}

	jobsByCompany := map[string][]*state.Job{}
	byFingerprint := map[string]*state.Job{}
	for _, j := range sortedJobs(st) {
		jobsByCompany[j.CompanyID] = append(jobsByCompany[j.CompanyID], j)
		byFingerprint[j.Fingerprint] = j
	}
	check := reg.CheckApplyURL
	seen := map[string]bool{} // jobs a board listed this run

	for i := 0; i < len(toFetch); i += Concurrency {
		chunk := toFetch[i:min(i+Concurrency, len(toFetch))]
		results := make([]fetched, len(chunk))
		var wg sync.WaitGroup
		for k, c := range chunk {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if p := recover(); p != nil {
						results[k] = fetched{err: fmt.Errorf("%v", p)}
					}
				}()
				tokens := append([]string{c.BoardToken}, extraBoards[c.Slug]...)
				raw, expired, err := FetchCompanyBoards(ctx, c.ATS, tokens, opts.Fetch)
				results[k] = fetched{raw, expired, err}
			}()
		}
		wg.Wait()

		for k, company := range chunk {
			res := results[k]
			prior := history[company.ID]
			if res.err != nil {
				message := res.err.Error()
				sum.Failures = append(sum.Failures, Failure{company.Slug, message})
				sum.Funnel = append(sum.Funnel, CompanyFunnel{Slug: company.Slug})
				ex := exempt(company)
				withThis := append([]HistoryRun{{Status: "error", RanAt: now}}, prior...)
				if AutoDisableState(prior, ex, now) == Enabled && AutoDisableState(withThis, ex, now) == Disabled {
					sum.AutoDisabled = append(sum.AutoDisabled, company.Slug)
				}
				msg := Slice16(message, 2000)
				st.Runs = append(st.Runs, &state.Run{Run: model.Run{ID: newID(), CompanyID: company.ID, Status: "error", Error: &msg, RanAt: ts}})
				continue
			}
			own := jobsByCompany[company.ID]
			existing := make([]ExistingJob, len(own))
			for x, j := range own {
				first, ok := date(j.FirstSeenAt)
				if !ok {
					return sum, fmt.Errorf("ingest: job %s: invalid first_seen_at %q", j.ID, j.FirstSeenAt)
				}
				e := ExistingJob{ID: j.ID, ExternalJobID: j.ExternalJobID, IsActive: truthy(j.Value("is_active")), Fingerprint: j.Fingerprint, FirstSeenAt: first}
				if j.PostedAt != nil && *j.PostedAt != "" {
					p, ok := date(*j.PostedAt)
					if !ok {
						return sum, fmt.Errorf("ingest: job %s: invalid posted_at %q", j.ID, *j.PostedAt)
					}
					e.PostedAt = &p
				}
				existing[x] = e
			}
			plan := PlanCompanyRun(PlanInput{
				Company:           PlanCompany{ID: company.ID, Slug: company.Slug, ATS: company.ATS},
				Raw:               res.raw,
				Expired:           res.expired,
				Existing:          existing,
				PriorEmptyRuns:    CountPriorEmptyRuns(prior),
				Now:               now,
				EnforceApplyHosts: opts.EnforceApplyHosts,
				CheckApplyURL:     check,
			})
			for _, u := range plan.Upserts {
				prev := byFingerprint[u.Fingerprint]
				rec := &state.Job{Job: model.Job{
					CompanyID: u.CompanyID, ExternalJobID: u.ExternalJobID, Fingerprint: u.Fingerprint,
					Title: u.Title, NormalizedTitle: u.NormalizedTitle, RoleFamilyID: u.RoleFamilyID,
					Locations: nonNilStrings(u.Locations), ApplyURL: u.ApplyURL, Excerpt: u.Excerpt,
					Terms: nonNilStrings(u.Terms), TermYears: u.TermYears, DurationMonths: u.DurationMonths,
					CohortYear: u.CohortYear, Roles: nonNilStrings(u.Roles), Regions: nonNilStrings(u.Regions),
					IsRemote: u.IsRemote, IsActive: u.IsActive, Source: u.Source,
					LastSeenAt: ts, UpdatedAt: ts,
				}}
				if prev != nil {
					rec.ID = prev.ID
				} else {
					rec.ID = newID()
				}
				// Keep stored strings when the instant is unchanged (byte-for-byte state).
				switch {
				case prev != nil && prev.PostedAt != nil && *prev.PostedAt != "" && u.PostedAt != nil && sameInstant(*prev.PostedAt, *u.PostedAt):
					rec.PostedAt = prev.PostedAt
				case u.PostedAt != nil:
					s := state.PgTimestamp(u.PostedAt.UnixMilli())
					rec.PostedAt = &s
				}
				// Dead-link strikes belong to the link: they carry over while the
				// board lists the same URL, and a job retired for a dead link
				// stays retired until the link changes or a probe finds it
				// working again (internal/applycheck).
				if prev != nil && prev.ApplyURL == u.ApplyURL && prev.ApplyStrikes > 0 {
					rec.ApplyStrikes, rec.ApplyStruckAt = prev.ApplyStrikes, prev.ApplyStruckAt
					if prev.ApplyStrikes >= applycheck.StrikesToRetire {
						rec.IsActive = false
					}
				}
				if prev != nil {
					rec.FirstSeenAt, rec.CreatedAt = prev.FirstSeenAt, prev.CreatedAt
				} else {
					rec.FirstSeenAt, rec.CreatedAt = state.PgTimestamp(u.FirstSeenAt.UnixMilli()), ts
				}
				st.Jobs[rec.ID] = rec
				seen[rec.ID] = true
				byFingerprint[rec.Fingerprint] = rec
				if prev == nil {
					own = append(own, rec)
				}
			}
			for _, id := range plan.RetireIDs {
				if j := st.Jobs[id]; j != nil && truthy(j.Value("is_active")) {
					j.Deactivate(ts)
					sum.JobsDeactivated++
				}
			}
			jobsByCompany[company.ID] = own
			runErr := plan.Run.Error
			st.Runs = append(st.Runs, &state.Run{Run: model.Run{ID: newID(), CompanyID: company.ID, Status: plan.Run.Status,
				JobCount: plan.Run.JobCount, Error: &runErr, RanAt: ts}})
			sum.JobsUpserted += len(plan.Upserts)
			sum.StaleDeactivated += plan.StaleDeactivated
			if plan.ZeroMatch {
				sum.ZeroMatchCompanies = append(sum.ZeroMatchCompanies, company.Slug)
			}
			if plan.SuspectedBlock {
				sum.SuspectedBlocks = append(sum.SuspectedBlocks, company.Slug)
			}
			sum.ApplyHostViolations = append(sum.ApplyHostViolations, plan.ApplyHostViolations...)
			sum.Funnel = append(sum.Funnel, plan.Funnel)
		}
	}

	if opts.ApplyProbe != nil {
		sum.ApplyProbe = probeApplyLinks(ctx, st, reg, *opts.ApplyProbe, seen, now, ts)
	}

	if !skipSweep {
		cutoff := now.UnixMilli() - proven.LastSeenStaleDays*proven.DayMS
		for _, j := range st.Jobs {
			if !truthy(j.Value("is_active")) {
				continue
			}
			// new Date(bad).getTime() is NaN, and NaN < cutoff is false.
			if ms, ok := jstime.Parse(j.LastSeenAt); ok && ms < cutoff {
				j.Deactivate(ts)
				sum.LastSeenDeactivated++
			}
		}
	}

	keepAfter := now.UnixMilli() - state.RunRetentionDays*proven.DayMS
	kept := st.Runs[:0]
	for _, r := range st.Runs {
		if ms, ok := jstime.Parse(r.RanAt); ok && ms >= keepAfter {
			kept = append(kept, r)
		}
	}
	st.Runs = kept

	sort.SliceStable(sum.Funnel, func(i, j int) bool {
		a, b := sum.Funnel[i], sum.Funnel[j]
		if a.Fetched != b.Fetched {
			return a.Fetched > b.Fetched
		}
		return state.LocaleCompare(a.Slug, b.Slug) < 0
	})
	for _, f := range sum.Funnel {
		sum.FunnelTotals.Fetched += f.Fetched
		sum.FunnelTotals.InternshipTitle += f.InternshipTitle
		sum.FunnelTotals.TechPass += f.TechPass
		sum.FunnelTotals.Upserted += f.Upserted
	}
	sum.Companies = len(toFetch)
	sum.LastSeenSweepSkipped = skipSweep
	sum.Shadow = []ShadowResult{}
	if opts.Shadow != nil {
		sum.Shadow = append(sum.Shadow, opts.Shadow()...)
	}
	return sum, nil
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func sameInstant(stored string, t time.Time) bool {
	ms, ok := jstime.Parse(stored)
	return ok && ms == t.UnixMilli()
}

// buildHistory is each company's runs, newest first by Date.parse(ran_at),
// stable on state order.
func buildHistory(runs []*state.Run) (map[string][]HistoryRun, error) {
	type item struct {
		r  *state.Run
		ms int64
		ok bool
	}
	items := make([]item, len(runs))
	for i, r := range runs {
		ms, ok := jstime.Parse(r.RanAt)
		items[i] = item{r, ms, ok}
	}
	sort.SliceStable(items, func(i, j int) bool {
		// NaN differences compare as equal.
		return items[i].ok && items[j].ok && items[i].ms > items[j].ms
	})
	out := map[string][]HistoryRun{}
	for _, it := range items {
		cid, ok := it.r.Value("company_id").(string)
		if !ok {
			continue // a null company_id belongs to no company that can be fetched
		}
		h := HistoryRun{Status: it.r.Status, Error: it.r.Error}
		if it.ok {
			h.RanAt = jstime.ToTime(it.ms)
		}
		out[cid] = append(out[cid], h)
	}
	return out, nil
}

// OnlySlugs parses OPENINTERN_ONLY_SLUGS: comma-separated, trimmed, empty
// entries dropped; nil when the variable is blank.
func OnlySlugs(env string) map[string]bool {
	env = jsTrim(env)
	if env == "" {
		return nil
	}
	out := map[string]bool{}
	for _, s := range strings.Split(env, ",") {
		if s = jsTrim(s); s != "" {
			out[s] = true
		}
	}
	return out
}
