// Package health builds the pipeline health snapshot: health.json in the
// daily dump, which the site's /health page and /api/v1/health serve as is.
// Ports packages/db/src/health-snapshot.ts (healthSnapshotFromRecords, the
// flat-file builder; the SQL builder went with the database) and
// ingest-health.ts.
package health

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/state"
	"github.com/dnexdev/openintern/internal/state/jsjson"
	"github.com/dnexdev/openintern/internal/state/jstime"
)

// ProprietaryATS are the scraped career sites the snapshot reports on
// separately, since they break without the board going away.
var ProprietaryATS = map[string]bool{
	"citadel":            true,
	"citadel_securities": true,
	"tesla":              true,
	"bytedance":          true,
	"tiktok":             true,
}

// Funnel is the classifier funnel an ingest run records in its error text;
// nil where the count is missing or too large to be real.
type Funnel struct {
	Fetched            *int64
	Title              *int64
	Tech               *int64
	Upserted           *int64
	DescriptionMissing *int64
}

var funnelRe = regexp.MustCompile(`funnel fetched=([0-9]+) title=([0-9]+) tech=([0-9]+) upserted=([0-9]+)(?: description_missing=([0-9]+))?`)

// count is Number(digits) when below the Nat limit.
func count(digits string, present bool) *int64 {
	if !present {
		return nil
	}
	d := strings.TrimLeft(digits, "0")
	if len(d) > 15 { // 2^48 has 15 digits
		return nil
	}
	var n int64
	for _, c := range d {
		n = n*10 + int64(c-'0')
	}
	if n >= proven.NatLimit {
		return nil
	}
	return &n
}

// ParseFunnel is parseFunnel: the counts of "funnel fetched=… title=… tech=…
// upserted=…[ description_missing=…]" anywhere in a run's error text.
func ParseFunnel(errText *string) Funnel {
	if errText == nil || *errText == "" {
		return Funnel{}
	}
	m := funnelRe.FindStringSubmatchIndex(*errText)
	if m == nil {
		return Funnel{}
	}
	group := func(i int) *int64 {
		if m[2*i] < 0 {
			return nil
		}
		return count((*errText)[m[2*i]:m[2*i+1]], true)
	}
	return Funnel{Fetched: group(1), Title: group(2), Tech: group(3), Upserted: group(4), DescriptionMissing: group(5)}
}

// SumFunnels adds funnels, missing counts as zero (proven law).
func SumFunnels(funnels []Funnel) proven.FunnelTotals {
	in := make([]proven.FunnelCounts, len(funnels))
	for i, f := range funnels {
		in[i] = proven.FunnelCounts{Fetched: f.Fetched, Title: f.Title, Tech: f.Tech, Upserted: f.Upserted, DescriptionMissing: f.DescriptionMissing}
	}
	return proven.SumFunnels(in)
}

// IsZeroMatchRun reports a run whose board listed jobs but none matched.
func IsZeroMatchRun(errText *string) bool {
	return errText != nil && proven.IsZeroMatchRun(*errText)
}

// Run is one ingest run in the snapshot, with its company resolved.
type Run struct {
	ID          string
	Status      string
	JobCount    int
	Error       *string
	RanAt       string // toISOString()
	CompanySlug *string
	CompanyName *string
	CompanyATS  *string
	Funnel      Funnel
	ZeroMatch   bool
}

// Failure is an entry of pipeline.recent_failures.
type Failure struct {
	CompanySlug, CompanyName, CompanyATS *string
	Error                                *string
	RanAt                                string
}

// ProprietaryFailure is an entry of pipeline.proprietary_ats.failures.
type ProprietaryFailure struct {
	CompanySlug, CompanyATS *string
	Error                   *string
	RanAt                   string
}

// ProprietaryZeroJobRun is an entry of pipeline.proprietary_ats.zero_job_runs.
type ProprietaryZeroJobRun struct {
	CompanySlug, CompanyATS *string
	JobCount                int
	Error                   *string
	RanAt                   string
}

// Stats is the snapshot's stats block.
type Stats struct {
	ActiveJobs, TotalJobs           int
	ActiveCompanies, TotalCompanies int
	LastSuccessfulIngest            *string
	ZeroMatchCompanies              int
}

// Snapshot is health.json. JSON renders it byte for byte as the TypeScript
// JSON.stringify(snapshot, null, 2) did.
type Snapshot struct {
	GeneratedAt         string
	Stats               Stats
	ZeroMatchCompanies  []string
	RecentFailures      []Failure
	ProprietaryFailures []ProprietaryFailure
	ProprietaryZeroJobs []ProprietaryZeroJobRun
	FunnelTotals        proven.FunnelTotals // latest run per company
	RecentIngestRuns    []Run               // newest first, up to 100
}

// RecentRunsLimit and RecentFailuresLimit bound the snapshot's lists.
const (
	RecentRunsLimit     = 100
	RecentFailuresLimit = 20
)

// run is a state run with what the builder needs resolved.
type run struct {
	r   *state.Run
	key string // company_id as a Map key: kind + value
	co  *state.Company
}

// ranAt is new Date(ran_at).toISOString(), which throws on a bad date only
// when a run is actually rendered.
func (r run) ranAt() string {
	ms, ok := jstime.Parse(r.r.RanAt)
	if !ok {
		panic(fmt.Sprintf("run %s: invalid ran_at %q", r.r.ID, r.r.RanAt))
	}
	return jstime.MustISO(ms)
}

// companyKey distinguishes the company_id values a JavaScript Map would:
// strings, null and undefined (other JSON types do not occur in exports).
func companyKey(v any) string {
	switch x := v.(type) {
	case string:
		return "s" + x
	case nil:
		return "n"
	}
	if v == state.Undefined {
		return "u"
	}
	return "o" + string(jsjson.Marshal(v))
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

// FromState is healthSnapshotFromRecords over a whole state. It fails where
// the TypeScript threw: a run whose ran_at is not a date, or funnel totals
// past the proven Nat limit.
func FromState(st *state.State, now time.Time) (snap Snapshot, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("health snapshot: %v", p)
		}
	}()
	snap.GeneratedAt, err = iso(now.UnixMilli())
	if err != nil {
		return snap, err
	}

	runs := make([]run, len(st.Runs))
	for i, r := range st.Runs {
		cid := r.Value("company_id")
		runs[i] = run{r: r, key: companyKey(cid)}
		if s, ok := cid.(string); ok {
			runs[i].co = st.Companies[s]
		}
	}
	// Newest first by ran_at as a string under localeCompare (not as a
	// date), stable on state order, as the TypeScript sorted.
	sort.SliceStable(runs, func(i, j int) bool { return state.LocaleCompare(runs[j].r.RanAt, runs[i].r.RanAt) < 0 })

	seen := map[string]bool{}
	var latest []run
	for _, r := range runs {
		if !seen[r.key] {
			seen[r.key] = true
			latest = append(latest, r)
		}
	}
	recent := runs
	if len(recent) > RecentRunsLimit {
		recent = recent[:RecentRunsLimit]
	}

	for _, j := range st.Jobs {
		if truthy(j.Value("is_active")) {
			snap.Stats.ActiveJobs++
		}
	}
	snap.Stats.TotalJobs = len(st.Jobs)
	for _, c := range st.Companies {
		if truthy(c.Value("active")) {
			snap.Stats.ActiveCompanies++
		}
	}
	snap.Stats.TotalCompanies = len(st.Companies)
	for _, r := range runs {
		if r.r.Status == "ok" {
			s := r.ranAt()
			snap.Stats.LastSuccessfulIngest = &s
			break
		}
	}

	snap.ZeroMatchCompanies = []string{}
	for _, r := range latest {
		if r.r.Status == "ok" && IsZeroMatchRun(r.r.Error) && r.co != nil && r.co.Slug != "" {
			snap.ZeroMatchCompanies = append(snap.ZeroMatchCompanies, r.co.Slug)
		}
	}
	sort.SliceStable(snap.ZeroMatchCompanies, func(i, j int) bool {
		return state.ByCodeUnit(snap.ZeroMatchCompanies[i], snap.ZeroMatchCompanies[j]) < 0
	})
	snap.Stats.ZeroMatchCompanies = len(snap.ZeroMatchCompanies)

	snap.RecentFailures = []Failure{}
	for _, r := range recent {
		if r.r.Status == "error" && len(snap.RecentFailures) < RecentFailuresLimit {
			slug, name, ats := r.company()
			snap.RecentFailures = append(snap.RecentFailures, Failure{slug, name, ats, r.r.Error, r.ranAt()})
		}
	}

	snap.ProprietaryFailures = []ProprietaryFailure{}
	snap.ProprietaryZeroJobs = []ProprietaryZeroJobRun{}
	for _, r := range latest {
		if r.co == nil || r.co.ATS == "" || !ProprietaryATS[r.co.ATS] {
			continue
		}
		slug, _, ats := r.company()
		if r.r.Status == "error" {
			snap.ProprietaryFailures = append(snap.ProprietaryFailures, ProprietaryFailure{slug, ats, r.r.Error, r.ranAt()})
		}
		if r.r.Status == "ok" && (IsZeroMatchRun(r.r.Error) || r.r.JobCount == 0) {
			snap.ProprietaryZeroJobs = append(snap.ProprietaryZeroJobs, ProprietaryZeroJobRun{slug, ats, r.r.JobCount, r.r.Error, r.ranAt()})
		}
	}

	funnels := make([]Funnel, len(latest))
	for i, r := range latest {
		funnels[i] = ParseFunnel(r.r.Error)
	}
	snap.FunnelTotals = SumFunnels(funnels)

	snap.RecentIngestRuns = make([]Run, len(recent))
	for i, r := range recent {
		slug, name, ats := r.company()
		snap.RecentIngestRuns[i] = Run{
			ID: r.r.ID, Status: r.r.Status, JobCount: r.r.JobCount, Error: r.r.Error,
			RanAt: r.ranAt(), CompanySlug: slug, CompanyName: name, CompanyATS: ats,
			Funnel: ParseFunnel(r.r.Error), ZeroMatch: IsZeroMatchRun(r.r.Error),
		}
	}
	return snap, nil
}

func (r run) company() (slug, name, ats *string) {
	if r.co == nil {
		return nil, nil, nil
	}
	s, n, a := r.co.Slug, r.co.Name, r.co.ATS
	return &s, &n, &a
}

func iso(ms int64) (string, error) {
	s, ok := jstime.ISO(ms)
	if !ok {
		return "", fmt.Errorf("invalid time value")
	}
	return s, nil
}

func str(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func num(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func obj(kv ...any) jsjson.Object {
	o := make(jsjson.Object, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		o = append(o, jsjson.Member{Key: kv[i].(string), Value: kv[i+1]})
	}
	return o
}

func funnelValue(f Funnel) jsjson.Object {
	return obj("fetched", num(f.Fetched), "title", num(f.Title), "tech", num(f.Tech),
		"upserted", num(f.Upserted), "descriptionMissing", num(f.DescriptionMissing))
}

// Value is the snapshot as a JSON value in the TypeScript key order.
func (s Snapshot) Value() jsjson.Object {
	zm := make([]any, len(s.ZeroMatchCompanies))
	for i, v := range s.ZeroMatchCompanies {
		zm[i] = v
	}
	failures := make([]any, len(s.RecentFailures))
	for i, f := range s.RecentFailures {
		failures[i] = obj("company_slug", str(f.CompanySlug), "company_name", str(f.CompanyName),
			"company_ats", str(f.CompanyATS), "error", str(f.Error), "ran_at", f.RanAt)
	}
	pf := make([]any, len(s.ProprietaryFailures))
	for i, f := range s.ProprietaryFailures {
		pf[i] = obj("company_slug", str(f.CompanySlug), "company_ats", str(f.CompanyATS), "error", str(f.Error), "ran_at", f.RanAt)
	}
	pz := make([]any, len(s.ProprietaryZeroJobs))
	for i, f := range s.ProprietaryZeroJobs {
		pz[i] = obj("company_slug", str(f.CompanySlug), "company_ats", str(f.CompanyATS), "job_count", f.JobCount,
			"error", str(f.Error), "ran_at", f.RanAt)
	}
	runs := make([]any, len(s.RecentIngestRuns))
	for i, r := range s.RecentIngestRuns {
		runs[i] = obj("id", r.ID, "status", r.Status, "job_count", r.JobCount, "error", str(r.Error), "ran_at", r.RanAt,
			"company_slug", str(r.CompanySlug), "company_name", str(r.CompanyName), "company_ats", str(r.CompanyATS),
			"funnel", funnelValue(r.Funnel), "zero_match", r.ZeroMatch)
	}
	ft := s.FunnelTotals
	return obj(
		"generatedAt", s.GeneratedAt,
		"stats", obj("active_jobs", s.Stats.ActiveJobs, "total_jobs", s.Stats.TotalJobs,
			"active_companies", s.Stats.ActiveCompanies, "total_companies", s.Stats.TotalCompanies,
			"last_successful_ingest", str(s.Stats.LastSuccessfulIngest), "zero_match_companies", s.Stats.ZeroMatchCompanies),
		"pipeline", obj("zero_match_companies", zm, "recent_failures", failures,
			"proprietary_ats", obj("failures", pf, "zero_job_runs", pz)),
		"funnel_totals_latest_per_company", obj("fetched", ft.Fetched, "title", ft.Title, "tech", ft.Tech,
			"upserted", ft.Upserted, "descriptionMissing", ft.DescriptionMissing),
		"recent_ingest_runs", runs,
	)
}

// JSON is health.json: two-space indent, no trailing newline.
func (s Snapshot) JSON() []byte { return jsjson.MarshalIndent(s.Value(), "  ") }
