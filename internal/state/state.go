// Package state is the flat-file state (plan phase D): the same data as the
// old Postgres tables, kept as sorted JSONL in a git repository so every
// change is a reviewable diff.
//
//	companies.jsonl  one company per line, sorted by slug
//	jobs.jsonl       every job ever seen (inactive too), sorted by id; no
//	                 per-run timestamps, so unchanged jobs give no diff
//	last_seen.tsv    id, last_seen_at, updated_at (changes every run)
//	runs.jsonl       ingest runs of the last RunRetentionDays
//
// Field names and timestamp strings match Postgres row_to_json output, so a
// state seeded from scripts/export-state.sh keeps ids and first_seen_at byte
// for byte. A state written by the TypeScript writer loads and re-writes
// byte-identical here: records keep keys they do not know (written last,
// sorted), keys they lack, and values whose JSON type does not fit the Go
// field (a null company_id on a run of a deleted company, say). Writes go to
// a temporary directory and replace the files at once.
package state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
	"github.com/dnexdev/openintern/internal/state/jstime"
)

// RunRetentionDays is how long runs.jsonl keeps ingest runs.
const RunRetentionDays = 60

// File names inside a state directory.
const (
	CompaniesFile = "companies.jsonl"
	JobsFile      = "jobs.jsonl"
	LastSeenFile  = "last_seen.tsv"
	RunsFile      = "runs.jsonl"
)

// Key order of each file = the table's physical column order, which is the
// order row_to_json uses in scripts/export-state.sh. Records written by file
// ingest then match exported ones byte for byte. A migration that adds a
// column appends it here, in internal/model and in the codecs below.
//
// apply_strikes and apply_struck_at (dead apply links, internal/applycheck)
// came after the database and are optional: written only when a job has
// strikes, absent otherwise. A record without them reads as no strikes.
var (
	CompanyKeys = []string{"id", "name", "slug", "ats", "board_token", "careers_url", "website_url", "active", "created_at", "updated_at"}
	JobKeys     = []string{
		"id", "company_id", "external_job_id", "title", "locations", "apply_url", "excerpt", "is_remote", "is_active",
		"source", "posted_at", "first_seen_at", "last_seen_at", "created_at", "updated_at", "terms", "cohort_year",
		"duration_months", "roles", "regions", "term_years", "fingerprint", "normalized_title", "role_family_id",
		"apply_strikes", "apply_struck_at",
	}
	RunKeys = []string{"id", "company_id", "status", "job_count", "error", "ran_at"}
)

// jobVolatile keys live in last_seen.tsv, not jobs.jsonl.
var jobVolatile = map[string]bool{"last_seen_at": true, "updated_at": true}

// Shape remembers how a record's JSON differed from its Go fields, so it is
// written back as it was read: keys the record lacked, values kept verbatim
// because their type does not fit the field, and keys the code does not know.
// A record built by ingest has an empty Shape.
type Shape struct {
	missing map[string]bool
	raw     map[string]any
	extra   jsjson.Object
}

// Set marks key as now holding its Go field's value (TS: assigning the
// property). Ingest calls it through the record methods when it changes a
// field in place.
func (s *Shape) Set(key string) {
	delete(s.missing, key)
	delete(s.raw, key)
}

// Plain reports whether the record round-trips through its Go fields alone.
func (s *Shape) Plain() bool { return len(s.missing) == 0 && len(s.raw) == 0 && len(s.extra) == 0 }

// Company is one line of companies.jsonl.
type Company struct {
	model.Company
	Shape Shape
}

// Job is one job: a line of jobs.jsonl plus its last_seen.tsv columns.
type Job struct {
	model.Job
	Shape Shape
}

// Run is one line of runs.jsonl. A run whose company was deleted from the
// database has a null company_id; Shape keeps it.
type Run struct {
	model.Run
	Shape Shape
}

// SetActive changes Active in place (registry sync of an orphaned company).
func (c *Company) SetActive(active bool, updatedAt string) {
	c.Active = active
	c.UpdatedAt = updatedAt
	c.Shape.Set("active")
	c.Shape.Set("updated_at")
}

// Deactivate retires a job in place.
func (j *Job) Deactivate(updatedAt string) {
	j.IsActive = false
	j.UpdatedAt = updatedAt
	j.Shape.Set("is_active")
	j.Shape.Set("updated_at")
}

// SetApplyStrikes records a job's dead-link strikes in place. A zero count
// clears both keys from the record.
func (j *Job) SetApplyStrikes(count int, struckAt string) {
	if count <= 0 {
		count, struckAt = 0, ""
	}
	j.ApplyStrikes = count
	j.ApplyStruckAt = struckAt
	j.Shape.Set("apply_strikes")
	j.Shape.Set("apply_struck_at")
}

// State is the whole state in memory.
type State struct {
	Companies map[string]*Company // by id
	Jobs      map[string]*Job     // by id
	Runs      []*Run
}

// New is an empty state.
func New() *State {
	return &State{Companies: map[string]*Company{}, Jobs: map[string]*Job{}}
}

// ByCodeUnit is the TypeScript byCodeUnit comparator: plain UTF-16 code-unit
// order, stable across Node and ICU versions, unlike localeCompare.
func ByCodeUnit(a, b string) int { return jsjson.CompareCodeUnits(a, b) }

// PgTimestamp formats an instant (ms since the epoch) as Postgres prints it.
func PgTimestamp(ms int64) string { return jstime.PgTimestamp(ms) }

// NewID is a random (version 4) UUID, lowercase, as crypto.randomUUID.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, c := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[c>>4], hex[c&15])
	}
	return string(out)
}

// jsTrimEmpty reports whether JavaScript's line.trim() is empty.
func jsTrimEmpty(l string) bool {
	for _, r := range l {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
			continue
		}
		if r >= 0x2000 && r <= 0x200a {
			continue
		}
		return false
	}
	return true
}

// parseJSONL parses the non-blank lines of text. Errors count lines the way
// the TypeScript did: among non-blank lines only.
func parseJSONL(text, label string) ([]jsjson.Object, error) {
	var out []jsjson.Object
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if jsTrimEmpty(l) {
			continue
		}
		n++
		v, err := jsjson.Parse(l)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: invalid JSON", label, n)
		}
		obj, ok := v.(jsjson.Object)
		if !ok {
			// JSON.parse accepted it; the TypeScript then treated it as a
			// record with no fields. A state line is always an object.
			return nil, fmt.Errorf("%s:%d: not a JSON object", label, n)
		}
		out = append(out, obj)
	}
	return out, nil
}

func readOptional(path string) (string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return jsjson.DecodeUTF8(b), nil
}

// Load is loadState. Missing files are empty.
func Load(dir string) (*State, error) {
	read := func(name string) (string, error) { return readOptional(filepath.Join(dir, name)) }
	ctext, err := read(CompaniesFile)
	if err != nil {
		return nil, err
	}
	jtext, err := read(JobsFile)
	if err != nil {
		return nil, err
	}
	rtext, err := read(RunsFile)
	if err != nil {
		return nil, err
	}
	ltext, err := read(LastSeenFile)
	if err != nil {
		return nil, err
	}
	companies, err := parseJSONL(ctext, CompaniesFile)
	if err != nil {
		return nil, err
	}
	jobs, err := parseJSONL(jtext, JobsFile)
	if err != nil {
		return nil, err
	}
	runs, err := parseJSONL(rtext, RunsFile)
	if err != nil {
		return nil, err
	}
	lastSeen := map[string][2]string{}
	for _, l := range strings.Split(ltext, "\n") {
		cols := strings.Split(l, "\t")
		if len(cols) >= 3 && cols[0] != "" && cols[1] != "" && cols[2] != "" {
			lastSeen[cols[0]] = [2]string{cols[1], cols[2]}
		}
	}
	st := New()
	for _, o := range companies {
		c := decodeCompany(o)
		st.Companies[c.ID] = c
	}
	for _, o := range jobs {
		j := decodeJob(o)
		// The record's own last_seen_at/updated_at (if any) give way to the TSV.
		j.Shape.Set("last_seen_at")
		j.Shape.Set("updated_at")
		if ls, ok := lastSeen[jsString(o, "id")]; ok {
			j.LastSeenAt, j.UpdatedAt = ls[0], ls[1]
		} else {
			j.LastSeenAt, j.UpdatedAt = jsString(o, "first_seen_at"), jsString(o, "created_at")
		}
		st.Jobs[j.ID] = j
	}
	for _, o := range runs {
		st.Runs = append(st.Runs, decodeRun(o))
	}
	return st, nil
}

// FromExport is stateFromExport: seed a state from scripts/export-state.sh
// output (row_to_json per table). Jobs there carry last_seen_at and
// updated_at themselves.
func FromExport(exportDir string) (*State, error) {
	read := func(name string) (string, error) { return readOptional(filepath.Join(exportDir, name)) }
	ctext, err := read("public.companies.jsonl")
	if err != nil {
		return nil, err
	}
	jtext, err := read("public.jobs.jsonl")
	if err != nil {
		return nil, err
	}
	rtext, err := read("public.ingest_runs.jsonl")
	if err != nil {
		return nil, err
	}
	companies, err := parseJSONL(ctext, "companies")
	if err != nil {
		return nil, err
	}
	jobs, err := parseJSONL(jtext, "jobs")
	if err != nil {
		return nil, err
	}
	runs, err := parseJSONL(rtext, "ingest_runs")
	if err != nil {
		return nil, err
	}
	if len(companies) == 0 {
		return nil, fmt.Errorf("no public.companies.jsonl in %s", exportDir)
	}
	st := New()
	for _, o := range companies {
		c := decodeCompany(o)
		st.Companies[c.ID] = c
	}
	for _, o := range jobs {
		j := decodeJob(o)
		st.Jobs[j.ID] = j
	}
	for _, o := range runs {
		st.Runs = append(st.Runs, decodeRun(o))
	}
	return st, nil
}

// Validate is validateState: problems that make a state unsafe to write;
// empty when it is sound.
func Validate(st *State) []string {
	var problems []string
	slugs := map[string]bool{}
	for _, c := range sortedCompanies(st) {
		if slugs[c.Slug] {
			problems = append(problems, "duplicate company slug "+c.Slug)
		}
		slugs[c.Slug] = true
	}
	fingerprints := map[string]bool{}
	prefixes := map[string]string{}
	for _, j := range sortedJobs(st) {
		if _, ok := st.Companies[j.CompanyID]; !ok {
			problems = append(problems, fmt.Sprintf("job %s has unknown company %s", j.ID, j.CompanyID))
		}
		if fingerprints[j.Fingerprint] {
			problems = append(problems, "duplicate fingerprint "+j.Fingerprint)
		}
		fingerprints[j.Fingerprint] = true
		if j.IsActive {
			// Job URLs carry company + the id's first 8 hex digits; they must be unique.
			key := j.CompanyID + " " + URLPrefix(j.ID)
			if other, ok := prefixes[key]; ok {
				problems = append(problems, fmt.Sprintf("active jobs %s and %s share a URL prefix", other, j.ID))
			}
			prefixes[key] = j.ID
		}
	}
	return problems
}

// URLPrefix is the first 8 characters of a job id without dashes (the id
// part of a job URL), counted in UTF-16 code units like the TypeScript slice.
func URLPrefix(id string) string {
	s := strings.ReplaceAll(id, "-", "")
	n := 0
	for i := range s {
		if n == 8 {
			return s[:i]
		}
		n++
	}
	return s
}

// sortedCompanies returns companies by slug, then id: the order writeState
// uses. Validate reports problems in this order and jobs in id order (the
// TypeScript used Map insertion order, which for a loaded state is the same
// file order; only jobs added in the same run can be listed differently).
func sortedCompanies(st *State) []*Company {
	out := make([]*Company, 0, len(st.Companies))
	for _, c := range st.Companies {
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if d := ByCodeUnit(out[i].Slug, out[j].Slug); d != 0 {
			return d < 0
		}
		return ByCodeUnit(out[i].ID, out[j].ID) < 0
	})
	return out
}

func sortedJobs(st *State) []*Job {
	out := make([]*Job, 0, len(st.Jobs))
	for _, j := range st.Jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool { return ByCodeUnit(out[i].ID, out[j].ID) < 0 })
	return out
}

// SortedRuns orders runs by ran_at (milliseconds, as Date.parse reads them),
// then id, as writeState does.
func SortedRuns(runs []*Run) []*Run {
	out := append([]*Run(nil), runs...)
	sort.SliceStable(out, func(i, j int) bool {
		a, aok := jstime.Parse(out[i].RanAt)
		b, bok := jstime.Parse(out[j].RanAt)
		if aok && bok && a != b {
			return a < b
		}
		return ByCodeUnit(out[i].ID, out[j].ID) < 0
	})
	return out
}

// Write is writeState: refuses an unsound state, then writes the four files
// to a temporary directory next to dir and moves them into place.
func Write(dir string, st *State) error {
	if problems := Validate(st); len(problems) > 0 {
		if len(problems) > 20 {
			problems = problems[:20]
		}
		return fmt.Errorf("refusing to write state:\n  %s", strings.Join(problems, "\n  "))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(abs), ".state-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	files := Encode(st)
	for _, name := range []string{CompaniesFile, JobsFile, LastSeenFile, RunsFile} {
		if err := os.WriteFile(filepath.Join(tmp, name), files[name], 0o666); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	for _, name := range []string{CompaniesFile, JobsFile, LastSeenFile, RunsFile} {
		if err := os.Rename(filepath.Join(tmp, name), filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// Encode renders the four state files without validating.
func Encode(st *State) map[string][]byte {
	companies := sortedCompanies(st)
	jobs := sortedJobs(st)
	var cb, jb, lb, rb []byte
	for _, c := range companies {
		cb = append(append(cb, jsjson.Marshal(encodeCompany(c))...), '\n')
	}
	for _, j := range jobs {
		jb = append(append(jb, jsjson.Marshal(encodeJob(j, true))...), '\n')
		lb = append(lb, tsvValue(j, "id")+"\t"+tsvValue(j, "last_seen_at")+"\t"+tsvValue(j, "updated_at")+"\n"...)
	}
	for _, r := range SortedRuns(st.Runs) {
		rb = append(append(rb, jsjson.Marshal(encodeRun(r))...), '\n')
	}
	return map[string][]byte{CompaniesFile: cb, JobsFile: jb, LastSeenFile: lb, RunsFile: rb}
}

// tsvValue is `${j[key]}` in a template string.
func tsvValue(j *Job, key string) string {
	if j.Shape.missing[key] {
		return "undefined"
	}
	if v, ok := j.Shape.raw[key]; ok {
		return jsString1(v)
	}
	switch key {
	case "id":
		return j.ID
	case "last_seen_at":
		return j.LastSeenAt
	}
	return j.UpdatedAt
}

// jsString1 is String(v) for a parsed JSON value.
func jsString1(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return string(jsjson.AppendNumber(nil, x))
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if e != nil {
				parts[i] = jsString1(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

// jsString is the string value of key in a raw record, as a template string
// would print it ("undefined" when absent).
func jsString(o jsjson.Object, key string) string {
	v, ok := o.Get(key)
	if !ok {
		return "undefined"
	}
	return jsString1(v)
}

// Undefined is the value Value reports for a key the record's JSON lacked.
var Undefined = undefinedValue{}

type undefinedValue struct{}

// value is the JavaScript value of key on a record: the verbatim JSON value
// when it did not fit the Go field, Undefined when absent, else the field.
func value[T any](rec *T, shape *Shape, fields []field[T], key string) any {
	if shape.missing[key] {
		return Undefined
	}
	if v, ok := shape.raw[key]; ok {
		return v
	}
	for _, f := range fields {
		if f.key == key {
			return f.get(rec)
		}
	}
	if v, ok := shape.extra.Get(key); ok {
		return v
	}
	return Undefined
}

// Value is what the TypeScript saw at c[key]: a string, bool, float64, nil,
// []any, jsjson.Object, or Undefined. Readers that must treat odd records
// as the TypeScript did (a null company_id on a run) use it.
func (c *Company) Value(key string) any { return value(&c.Company, &c.Shape, companyFields, key) }

// Value is what the TypeScript saw at j[key]; see Company.Value.
func (j *Job) Value(key string) any { return value(&j.Job, &j.Shape, jobFields, key) }

// Value is what the TypeScript saw at r[key]; see Company.Value.
func (r *Run) Value(key string) any { return value(&r.Run, &r.Shape, runFields, key) }
