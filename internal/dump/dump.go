// Package dump writes the daily public dump from flat-file state: jobs.json,
// jobs.csv, companies.json, health.json, denylist.json, LICENSE-DATA.txt, the
// .gz copies and MANIFEST.json. Ports packages/ingest/src/dump.ts
// (writeDumpFiles) and dump-state.ts (writeDumpsFromState); the database
// query path went with the database.
//
// Every file but the .gz copies is byte-identical to what the TypeScript
// wrote for the same state and clock. The gzip streams decompress to the
// same bytes but are not the same bytes (Go's deflate is not zlib's), so
// MANIFEST.json differs in their two entries only.
package dump

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/health"
	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state"
	"github.com/dnexdev/openintern/internal/state/jsjson"
	"github.com/dnexdev/openintern/internal/state/jstime"
)

// LicenseText is LICENSE-DATA.txt.
const LicenseText = "OpenIntern listing data. Terms: https://github.com/dnexdev/openintern/blob/main/DATA.md\n" +
	"\n" +
	"Factual fields (title, company, location, dates, apply link) are provided as-is.\n" +
	"Excerpts remain the employers' copyright and are included for indexing only; no licence is granted.\n" +
	"Company names and logos are trademarks of their owners. No warranty.\n" +
	"Mirrors must drop the ids in denylist.json and delete snapshots older than 30 days.\n"

// CSVHeader is the first line of jobs.csv. New columns go at the end:
// spreadsheets address columns by position.
const CSVHeader = "id,title,company,company_slug,locations,apply_url,excerpt,terms,term_years,duration_months,cohort_year,roles,regions,is_remote,source,posted_at,first_seen_at,last_seen_at,role_family_id,normalized_title"

// Options injects the publishing gates; nil fields use the registry
// compiled into the binary (apply-host mode from OPENINTERN_APPLY_HOSTS).
type Options struct {
	ApplyURLAllowed func(companySlug, url string) bool
	IsDelisted      func(jobID string) bool
	Delisted        []registry.Delisted // denylist.json
}

func (o Options) withDefaults() Options {
	if o.ApplyURLAllowed == nil {
		mode := registry.ApplyHostModeFromEnv()
		reg := registry.Default()
		o.ApplyURLAllowed = func(slug, url string) bool { return reg.ApplyURLAllowed(slug, url, mode) }
	}
	if o.IsDelisted == nil {
		o.IsDelisted = registry.IsDelisted
	}
	if o.Delisted == nil {
		o.Delisted = registry.DelistedJobs()
	}
	return o
}

// Row is one public job with its company, as jobs.json and jobs.csv print it.
type Row struct {
	Job         *state.Job
	Company     *state.Company
	PostedAt    *string // toISOString(), nil when the job has none
	FirstSeenAt string
	LastSeenAt  string
}

// Result is what FromState wrote.
type Result struct {
	JSONPath, CSVPath, HealthPath, CompaniesPath, ManifestPath string
	Count                                                      int
}

// FromState is writeDumpsFromState: the public jobs of st (active, not
// delisted, at least one role, fresh at now, apply link allowed) sorted by
// id, every company, and the health snapshot.
func FromState(st *state.State, outDir string, now time.Time, opts Options) (Result, error) {
	opts = opts.withDefaults()
	rows, err := PublicRows(st, now, opts)
	if err != nil {
		return Result{}, err
	}
	snap, err := health.FromState(st, now)
	if err != nil {
		return Result{}, err
	}
	return WriteFiles(outDir, rows, SortedCompanies(st), snap, now, opts)
}

// SortedCompanies is every company by slug (code-unit order), then id.
func SortedCompanies(st *state.State) []*state.Company {
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

// date is new Date(s) for a state timestamp, failing where toISOString
// would throw.
func date(s, what, id string) (int64, error) {
	ms, ok := jstime.Parse(s)
	if !ok {
		return 0, fmt.Errorf("job %s: invalid %s %q", id, what, s)
	}
	return ms, nil
}

// PublicRows filters and sorts the jobs a dump publishes.
func PublicRows(st *state.State, now time.Time, opts Options) (rows []Row, err error) {
	opts = opts.withDefaults()
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("dump: %v", p)
		}
	}()
	for _, j := range st.Jobs {
		c, ok := st.Companies[j.CompanyID]
		if !ok || !truthy(j.Value("is_active")) || opts.IsDelisted(j.ID) || jsLength(j.Value("roles")) == 0 {
			continue
		}
		var posted *int64
		if p := j.Value("posted_at"); truthy(p) {
			ms, err := date(jsString(p), "posted_at", j.ID)
			if err != nil {
				return nil, err
			}
			posted = &ms
		}
		first, err := date(j.FirstSeenAt, "first_seen_at", j.ID)
		if err != nil {
			return nil, err
		}
		var postedAt *time.Time
		if posted != nil {
			t := jstime.ToTime(*posted)
			postedAt = &t
		}
		fresh := health.IsFreshJob(termYears(j.Value("term_years")), postedAt, jstime.ToTime(first), now)
		if !fresh || !opts.ApplyURLAllowed(c.Slug, j.ApplyURL) {
			continue
		}
		last, err := date(j.LastSeenAt, "last_seen_at", j.ID)
		if err != nil {
			return nil, err
		}
		row := Row{Job: j, Company: c, FirstSeenAt: jstime.MustISO(first), LastSeenAt: jstime.MustISO(last)}
		if posted != nil {
			s := jstime.MustISO(*posted)
			row.PostedAt = &s
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(a, b int) bool { return state.ByCodeUnit(rows[a].Job.ID, rows[b].Job.ID) < 0 })
	return rows, nil
}

// termYears reads term_years as the TypeScript did: any list of objects
// with a term and a year, whatever its key order.
func termYears(v any) []proven.TermYearLike {
	list, _ := v.([]any)
	out := make([]proven.TermYearLike, 0, len(list))
	for _, e := range list {
		o, _ := e.(jsjson.Object)
		term, _ := o.Get("term")
		year, _ := o.Get("year")
		t, _ := term.(string)
		y, _ := year.(float64)
		if n, ok := year.(int); ok {
			y = float64(n)
		}
		out = append(out, proven.TermYearLike{Term: t, Year: int(y)})
	}
	return out
}

func obj(kv ...any) jsjson.Object {
	o := make(jsjson.Object, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		if kv[i+1] == state.Undefined {
			continue // JSON.stringify drops undefined properties
		}
		o = append(o, jsjson.Member{Key: kv[i].(string), Value: kv[i+1]})
	}
	return o
}

func strOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func jobValue(r Row) jsjson.Object {
	j, c := r.Job, r.Company
	return obj(
		"id", j.ID,
		"title", j.Value("title"),
		"company", obj(
			"name", c.Value("name"),
			"slug", c.Value("slug"),
			"ats", c.Value("ats"),
			"website_url", c.Value("website_url"),
			"careers_url", c.Value("careers_url"),
		),
		"locations", j.Value("locations"),
		"apply_url", j.Value("apply_url"),
		"excerpt", j.Value("excerpt"),
		"terms", j.Value("terms"),
		"term_years", j.Value("term_years"),
		"duration_months", j.Value("duration_months"),
		"cohort_year", j.Value("cohort_year"),
		"roles", j.Value("roles"),
		"regions", j.Value("regions"),
		"is_remote", j.Value("is_remote"),
		"source", j.Value("source"),
		"posted_at", strOrNil(r.PostedAt),
		"first_seen_at", r.FirstSeenAt,
		"last_seen_at", r.LastSeenAt,
		"role_family_id", j.Value("role_family_id"),
		"normalized_title", j.Value("normalized_title"),
	)
}

// JobsJSON is jobs.json.
func JobsJSON(rows []Row, generatedAt string) []byte {
	jobs := make([]any, len(rows))
	for i, r := range rows {
		jobs[i] = jobValue(r)
	}
	return jsjson.MarshalIndent(obj("generatedAt", generatedAt, "count", len(rows), "jobs", jobs), "  ")
}

// joinList is `Array.isArray(v) ? v.join("; ") : ""`.
func joinList(v any, item func(any) string) string {
	var parts []string
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			parts = append(parts, item(e))
		}
	case []string:
		return strings.Join(x, "; ")
	case []int:
		for _, n := range x {
			parts = append(parts, jsString(n))
		}
	default:
		return ""
	}
	return strings.Join(parts, "; ")
}

// joinItem is how Array.prototype.join prints an element.
func joinItem(e any) string {
	if e == nil || e == state.Undefined {
		return ""
	}
	return jsString(e)
}

func termYearItem(e any) string {
	o, _ := e.(jsjson.Object)
	t, ok := o.Get("term")
	if !ok {
		t = state.Undefined
	}
	y, ok := o.Get("year")
	if !ok {
		y = state.Undefined
	}
	return jsString(t) + "-" + jsString(y)
}

// csvEscape quotes a cell holding a quote, comma or newline.
func csvEscape(v string) string {
	if strings.ContainsAny(v, "\",\n") {
		return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
	}
	return v
}

// JobsCSV is jobs.csv: header and one line per row, no trailing newline.
func JobsCSV(rows []Row) []byte {
	var b bytes.Buffer
	b.WriteString(CSVHeader)
	for _, r := range rows {
		j, c := r.Job, r.Company
		excerpt := ""
		if e := j.Value("excerpt"); e != nil && e != state.Undefined {
			excerpt = jsString(e)
		}
		cohort := ""
		if y := j.Value("cohort_year"); y != nil && y != state.Undefined {
			cohort = jsString(y)
		}
		posted := ""
		if r.PostedAt != nil {
			posted = *r.PostedAt
		}
		cells := []string{
			j.ID,
			jsString(j.Value("title")),
			jsString(c.Value("name")),
			jsString(c.Value("slug")),
			joinList(j.Value("locations"), joinItem),
			jsString(j.Value("apply_url")),
			excerpt,
			joinList(j.Value("terms"), joinItem),
			joinList(j.Value("term_years"), termYearItem),
			joinList(j.Value("duration_months"), joinItem),
			cohort,
			joinList(j.Value("roles"), joinItem),
			joinList(j.Value("regions"), joinItem),
			jsString(j.Value("is_remote")),
			jsString(j.Value("source")),
			posted,
			r.FirstSeenAt,
			r.LastSeenAt,
			jsString(j.Value("role_family_id")),
			jsString(j.Value("normalized_title")),
		}
		b.WriteByte('\n')
		for i, cell := range cells {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(csvEscape(cell))
		}
	}
	return []byte(jsjson.EncodeUTF8(b.String()))
}

// CompaniesJSON is companies.json: every company with its count of public
// jobs.
func CompaniesJSON(rows []Row, companies []*state.Company, generatedAt string) []byte {
	live := map[string]int{}
	for _, r := range rows {
		live[r.Company.Slug]++
	}
	list := make([]any, len(companies))
	for i, c := range companies {
		list[i] = obj(
			"id", c.Value("id"),
			"name", c.Value("name"),
			"slug", c.Value("slug"),
			"ats", c.Value("ats"),
			"careers_url", c.Value("careers_url"),
			"website_url", c.Value("website_url"),
			"active", c.Value("active"),
			"active_jobs", live[c.Slug],
		)
	}
	return jsjson.MarshalIndent(obj("generatedAt", generatedAt, "count", len(companies), "companies", list), "  ")
}

// DenylistJSON is denylist.json: mirrors must drop these ids too (DATA.md).
func DenylistJSON(delisted []registry.Delisted, generatedAt string) []byte {
	jobs := make([]any, len(delisted))
	for i, d := range delisted {
		jobs[i] = obj("id", d.ID, "reason", d.Reason)
	}
	return jsjson.MarshalIndent(obj("generatedAt", generatedAt, "jobs", jobs), "  ")
}

// Gzip compresses at level 9 with the header zlib writes on Unix (no name,
// no mtime, OS 3).
func Gzip(b []byte) []byte {
	var out bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&out, gzip.BestCompression)
	zw.OS = 3
	zw.Write(b)
	zw.Close()
	return out.Bytes()
}

// WriteFiles is writeDumpFiles: every dump file from public rows (sorted by
// id), the company list and the health snapshot.
func WriteFiles(outDir string, rows []Row, companies []*state.Company, snap health.Snapshot, now time.Time, opts Options) (Result, error) {
	opts = opts.withDefaults()
	generatedAt, ok := jstime.ISO(now.UnixMilli())
	if !ok {
		return Result{}, fmt.Errorf("dump: invalid time value")
	}
	if err := os.MkdirAll(outDir, 0o777); err != nil {
		return Result{}, err
	}
	res := Result{
		JSONPath:      filepath.Join(outDir, "jobs.json"),
		CSVPath:       filepath.Join(outDir, "jobs.csv"),
		CompaniesPath: filepath.Join(outDir, "companies.json"),
		HealthPath:    filepath.Join(outDir, "health.json"),
		ManifestPath:  filepath.Join(outDir, "MANIFEST.json"),
		Count:         len(rows),
	}
	jobsJSON := JobsJSON(rows, generatedAt)
	jobsCSV := JobsCSV(rows)
	writes := []struct {
		path string
		data []byte
	}{
		{res.JSONPath, jobsJSON},
		{res.CSVPath, jobsCSV},
		{res.CompaniesPath, CompaniesJSON(rows, companies, generatedAt)},
		{res.HealthPath, snap.JSON()},
		{filepath.Join(outDir, "denylist.json"), DenylistJSON(opts.Delisted, generatedAt)},
		{filepath.Join(outDir, "LICENSE-DATA.txt"), []byte(LicenseText)},
		{res.JSONPath + ".gz", Gzip(jobsJSON)},
		{res.CSVPath + ".gz", Gzip(jobsCSV)},
	}
	for _, w := range writes {
		if err := os.WriteFile(w.path, w.data, 0o666); err != nil {
			return Result{}, err
		}
	}

	var byATS jsjson.Object
	idx := map[string]int{}
	for _, r := range rows {
		ats := r.Company.ATS
		if i, ok := idx[ats]; ok {
			byATS[i].Value = byATS[i].Value.(int) + 1
			continue
		}
		idx[ats] = len(byATS)
		byATS = append(byATS, jsjson.Member{Key: ats, Value: 1})
	}
	// Every file in the directory, as readdir lists it, MANIFEST.json aside.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return Result{}, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name() != "MANIFEST.json" {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return state.ByCodeUnit(names[i], names[j]) < 0 })
	var files jsjson.Object
	for _, name := range names {
		buf, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			return Result{}, err
		}
		sum := sha256.Sum256(buf)
		files = append(files, jsjson.Member{Key: name, Value: obj("sha256", hex.EncodeToString(sum[:]), "bytes", len(buf))})
	}
	manifest := obj("generatedAt", generatedAt, "jobs", len(rows), "companies", len(companies),
		"byAts", jsjson.JSOrder(nonNil(byATS)), "files", jsjson.JSOrder(nonNil(files)))
	if err := os.WriteFile(res.ManifestPath, jsjson.MarshalIndent(manifest, "  "), 0o666); err != nil {
		return Result{}, err
	}
	return res, nil
}

func nonNil(o jsjson.Object) jsjson.Object {
	if o == nil {
		return jsjson.Object{}
	}
	return o
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && x == x
	case int:
		return x != 0
	case string:
		return x != ""
	}
	return v != state.Undefined
}

// jsLength is `v?.length ?? 0` for a JSON value.
func jsLength(v any) int {
	switch x := v.(type) {
	case []any:
		return len(x)
	case []string:
		return len(x)
	case string:
		return jsjson.Len16(x)
	}
	return 0
}

// jsString is String(v) for a JSON value.
func jsString(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprint(x)
	case float64:
		return string(jsjson.AppendNumber(nil, x))
	case []any:
		return joinListComma(x)
	case []string:
		return strings.Join(x, ",")
	case []int:
		parts := make([]string, len(x))
		for i, n := range x {
			parts[i] = fmt.Sprint(n)
		}
		return strings.Join(parts, ",")
	case jsjson.Object:
		return "[object Object]"
	}
	if v == state.Undefined {
		return "undefined"
	}
	return fmt.Sprint(v)
}

func joinListComma(x []any) string {
	parts := make([]string, len(x))
	for i, e := range x {
		parts[i] = joinItem(e)
	}
	return strings.Join(parts, ",")
}
