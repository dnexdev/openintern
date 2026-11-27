package state

import (
	"math"
	"sort"

	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// field maps one JSON key to a Go field. set reports whether the JSON value
// fits the field; a value that does not is kept verbatim in the Shape.
type field[T any] struct {
	key string
	get func(*T) any
	set func(*T, any) bool
	// omit, when set, leaves the key out of a record whose value is its
	// zero (optional keys added after the database export format).
	omit func(*T) bool
}

// optional marks f as written only when omit reports false.
func optional[T any](f field[T], omit func(*T) bool) field[T] {
	f.omit = omit
	return f
}

func strField[T any](key string, p func(*T) *string) field[T] {
	return field[T]{key: key, get: func(r *T) any { return *p(r) }, set: func(r *T, v any) bool {
		s, ok := v.(string)
		if ok {
			*p(r) = s
		}
		return ok
	}}
}

func nstrField[T any](key string, p func(*T) **string) field[T] {
	return field[T]{key: key, get: func(r *T) any {
		if s := *p(r); s != nil {
			return *s
		}
		return nil
	}, set: func(r *T, v any) bool {
		switch x := v.(type) {
		case nil:
			*p(r) = nil
			return true
		case string:
			*p(r) = &x
			return true
		}
		return false
	}}
}

func boolField[T any](key string, p func(*T) *bool) field[T] {
	return field[T]{key: key, get: func(r *T) any { return *p(r) }, set: func(r *T, v any) bool {
		b, ok := v.(bool)
		if ok {
			*p(r) = b
		}
		return ok
	}}
}

// integral is a JSON number that a Go int holds exactly.
func integral(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, false
	}
	return int(f), true
}

func intField[T any](key string, p func(*T) *int) field[T] {
	return field[T]{key: key, get: func(r *T) any { return *p(r) }, set: func(r *T, v any) bool {
		n, ok := integral(v)
		if ok {
			*p(r) = n
		}
		return ok
	}}
}

func nintField[T any](key string, p func(*T) **int) field[T] {
	return field[T]{key: key, get: func(r *T) any {
		if n := *p(r); n != nil {
			return *n
		}
		return nil
	}, set: func(r *T, v any) bool {
		if v == nil {
			*p(r) = nil
			return true
		}
		n, ok := integral(v)
		if ok {
			*p(r) = &n
		}
		return ok
	}}
}

func strsField[T any](key string, p func(*T) *[]string) field[T] {
	return field[T]{key: key, get: func(r *T) any {
		if *p(r) == nil {
			return []string{}
		}
		return *p(r)
	}, set: func(r *T, v any) bool {
		list, ok := v.([]any)
		if !ok {
			return false
		}
		out := make([]string, len(list))
		for i, e := range list {
			s, ok := e.(string)
			if !ok {
				return false
			}
			out[i] = s
		}
		*p(r) = out
		return true
	}}
}

func intsField[T any](key string, p func(*T) *[]int) field[T] {
	return field[T]{key: key, get: func(r *T) any {
		if *p(r) == nil {
			return []int{}
		}
		return *p(r)
	}, set: func(r *T, v any) bool {
		list, ok := v.([]any)
		if !ok {
			return false
		}
		out := make([]int, len(list))
		for i, e := range list {
			n, ok := integral(e)
			if !ok {
				return false
			}
			out[i] = n
		}
		*p(r) = out
		return true
	}}
}

// TermYearsValue is term_years as written: [{"term":...,"year":...}].
func TermYearsValue(tys []model.TermYear) []any {
	out := make([]any, len(tys))
	for i, ty := range tys {
		out[i] = jsjson.Object{{Key: "term", Value: ty.Term}, {Key: "year", Value: ty.Year}}
	}
	return out
}

func termYearsField[T any](key string, p func(*T) *[]model.TermYear) field[T] {
	return field[T]{key: key, get: func(r *T) any { return TermYearsValue(*p(r)) }, set: func(r *T, v any) bool {
		list, ok := v.([]any)
		if !ok {
			return false
		}
		out := make([]model.TermYear, len(list))
		for i, e := range list {
			o, ok := e.(jsjson.Object)
			if !ok || len(o) != 2 || o[0].Key != "term" || o[1].Key != "year" {
				return false
			}
			term, ok := o[0].Value.(string)
			if !ok {
				return false
			}
			year, ok := integral(o[1].Value)
			if !ok {
				return false
			}
			out[i] = model.TermYear{Term: term, Year: year}
		}
		*p(r) = out
		return true
	}}
}

func decode[T any](o jsjson.Object, fields []field[T], rec *T, shape *Shape) {
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		known[f.key] = true
		v, ok := o.Get(f.key)
		if !ok {
			if shape.missing == nil {
				shape.missing = map[string]bool{}
			}
			shape.missing[f.key] = true
			continue
		}
		if !f.set(rec, v) {
			if shape.raw == nil {
				shape.raw = map[string]any{}
			}
			shape.raw[f.key] = v
		}
	}
	for _, m := range o {
		if !known[m.Key] {
			shape.extra = append(shape.extra, m)
		}
	}
}

// encode is ordered() in state.ts: the record's keys in column order (absent
// ones skipped), then unknown keys sorted by code unit, minus skip.
func encode[T any](rec *T, shape *Shape, fields []field[T], skip map[string]bool) jsjson.Object {
	out := make(jsjson.Object, 0, len(fields)+len(shape.extra))
	for _, f := range fields {
		if skip[f.key] || shape.missing[f.key] {
			continue
		}
		if v, ok := shape.raw[f.key]; ok {
			out = append(out, jsjson.Member{Key: f.key, Value: v})
			continue
		}
		if f.omit != nil && f.omit(rec) {
			continue
		}
		out = append(out, jsjson.Member{Key: f.key, Value: f.get(rec)})
	}
	if len(shape.extra) > 0 {
		extra := append(jsjson.Object(nil), shape.extra...)
		sort.SliceStable(extra, func(i, j int) bool { return ByCodeUnit(extra[i].Key, extra[j].Key) < 0 })
		out = append(out, extra...)
		// Assigning keys to a JavaScript object puts array-index keys first.
		out = jsjson.JSOrder(out)
	}
	return out
}

var companyFields = []field[model.Company]{
	strField("id", func(c *model.Company) *string { return &c.ID }),
	strField("name", func(c *model.Company) *string { return &c.Name }),
	strField("slug", func(c *model.Company) *string { return &c.Slug }),
	strField("ats", func(c *model.Company) *string { return &c.ATS }),
	strField("board_token", func(c *model.Company) *string { return &c.BoardToken }),
	nstrField("careers_url", func(c *model.Company) **string { return &c.CareersURL }),
	nstrField("website_url", func(c *model.Company) **string { return &c.WebsiteURL }),
	boolField("active", func(c *model.Company) *bool { return &c.Active }),
	strField("created_at", func(c *model.Company) *string { return &c.CreatedAt }),
	strField("updated_at", func(c *model.Company) *string { return &c.UpdatedAt }),
}

var jobFields = []field[model.Job]{
	strField("id", func(j *model.Job) *string { return &j.ID }),
	strField("company_id", func(j *model.Job) *string { return &j.CompanyID }),
	strField("external_job_id", func(j *model.Job) *string { return &j.ExternalJobID }),
	strField("title", func(j *model.Job) *string { return &j.Title }),
	strsField("locations", func(j *model.Job) *[]string { return &j.Locations }),
	strField("apply_url", func(j *model.Job) *string { return &j.ApplyURL }),
	nstrField("excerpt", func(j *model.Job) **string { return &j.Excerpt }),
	boolField("is_remote", func(j *model.Job) *bool { return &j.IsRemote }),
	boolField("is_active", func(j *model.Job) *bool { return &j.IsActive }),
	strField("source", func(j *model.Job) *string { return &j.Source }),
	nstrField("posted_at", func(j *model.Job) **string { return &j.PostedAt }),
	strField("first_seen_at", func(j *model.Job) *string { return &j.FirstSeenAt }),
	strField("last_seen_at", func(j *model.Job) *string { return &j.LastSeenAt }),
	strField("created_at", func(j *model.Job) *string { return &j.CreatedAt }),
	strField("updated_at", func(j *model.Job) *string { return &j.UpdatedAt }),
	strsField("terms", func(j *model.Job) *[]string { return &j.Terms }),
	nintField("cohort_year", func(j *model.Job) **int { return &j.CohortYear }),
	intsField("duration_months", func(j *model.Job) *[]int { return &j.DurationMonths }),
	strsField("roles", func(j *model.Job) *[]string { return &j.Roles }),
	strsField("regions", func(j *model.Job) *[]string { return &j.Regions }),
	termYearsField("term_years", func(j *model.Job) *[]model.TermYear { return &j.TermYears }),
	strField("fingerprint", func(j *model.Job) *string { return &j.Fingerprint }),
	strField("normalized_title", func(j *model.Job) *string { return &j.NormalizedTitle }),
	strField("role_family_id", func(j *model.Job) *string { return &j.RoleFamilyID }),
	optional(intField("apply_strikes", func(j *model.Job) *int { return &j.ApplyStrikes }),
		func(j *model.Job) bool { return j.ApplyStrikes == 0 }),
	optional(strField("apply_struck_at", func(j *model.Job) *string { return &j.ApplyStruckAt }),
		func(j *model.Job) bool { return j.ApplyStruckAt == "" }),
}

var runFields = []field[model.Run]{
	strField("id", func(r *model.Run) *string { return &r.ID }),
	strField("company_id", func(r *model.Run) *string { return &r.CompanyID }),
	strField("status", func(r *model.Run) *string { return &r.Status }),
	intField("job_count", func(r *model.Run) *int { return &r.JobCount }),
	nstrField("error", func(r *model.Run) **string { return &r.Error }),
	strField("ran_at", func(r *model.Run) *string { return &r.RanAt }),
}

func init() {
	// The codecs must list exactly the column order the writer promises.
	check := func(name string, keys []string, n int, key func(int) string) {
		if len(keys) != n {
			panic("state: " + name + " codec out of sync with key order")
		}
		for i, k := range keys {
			if key(i) != k {
				panic("state: " + name + " codec out of sync with key order at " + k)
			}
		}
	}
	check("company", CompanyKeys, len(companyFields), func(i int) string { return companyFields[i].key })
	check("job", JobKeys, len(jobFields), func(i int) string { return jobFields[i].key })
	check("run", RunKeys, len(runFields), func(i int) string { return runFields[i].key })
}

func decodeCompany(o jsjson.Object) *Company {
	c := &Company{}
	decode(o, companyFields, &c.Company, &c.Shape)
	return c
}

func decodeJob(o jsjson.Object) *Job {
	j := &Job{}
	decode(o, jobFields, &j.Job, &j.Shape)
	return j
}

func decodeRun(o jsjson.Object) *Run {
	r := &Run{}
	decode(o, runFields, &r.Run, &r.Shape)
	return r
}

func encodeCompany(c *Company) jsjson.Object { return encode(&c.Company, &c.Shape, companyFields, nil) }

func encodeJob(j *Job, skipVolatile bool) jsjson.Object {
	var skip map[string]bool
	if skipVolatile {
		skip = jobVolatile
	}
	return encode(&j.Job, &j.Shape, jobFields, skip)
}

func encodeRun(r *Run) jsjson.Object { return encode(&r.Run, &r.Shape, runFields, nil) }
