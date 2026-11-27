package state

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/registry/weburl"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// MaxNewJobsPerCompany is how many jobs one company may add in one run
// before the state gate holds the write.
const MaxNewJobsPerCompany = 200

// GateResult is what the state gate found.
type GateResult struct {
	// Problems refuse the write (the script printed the first 50).
	Problems []string
	// NewHosts are apply hosts of active jobs not seen in the current state,
	// sorted, for the reviewer.
	NewHosts []string
	// Jobs and Companies count the candidate's records.
	Jobs, Companies int
}

// GateOptions configures Gate.
type GateOptions struct {
	// Approved reports whether data/expected-delta.json approves a scope
	// ("company:<slug>") today.
	Approved func(scope string) bool
}

// Gate is scripts/state-gate.mjs: the write gate for flat-file state. It
// refuses a candidate that is malformed (bad JSON, duplicate ids, slugs or
// fingerprints, jobs of unknown companies, two active jobs sharing a URL
// prefix in a company), changes an existing job's id, company or
// first_seen_at, adds more than MaxNewJobsPerCompany jobs to one company in
// one run, or drops a company with at least 5 active jobs to zero, unless
// approved. It reads the raw files rather than Load, so it judges exactly
// what is on disk, whatever wrote it.
func Gate(curDir, newDir string, opts GateOptions) (GateResult, error) {
	var res GateResult
	approved := opts.Approved
	if approved == nil {
		approved = func(string) bool { return false }
	}
	read := func(dir, name string) ([]jsjson.Object, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var out []jsjson.Object
		n := 0
		for _, l := range strings.Split(jsjson.DecodeUTF8(b), "\n") {
			if jsTrimEmpty(l) {
				continue
			}
			n++
			v, err := jsjson.Parse(l)
			if err != nil {
				res.Problems = append(res.Problems, name+":"+strconv.Itoa(n)+" in "+dir+": invalid JSON")
				continue
			}
			if !jsTruthy(v) {
				continue // .filter(Boolean)
			}
			o, _ := v.(jsjson.Object) // other values have no fields
			out = append(out, o)
		}
		return out, nil
	}
	curCompanies, err := read(curDir, CompaniesFile)
	if err != nil {
		return res, err
	}
	_ = curCompanies // read for its JSON check, as the script did
	curJobs, err := read(curDir, JobsFile)
	if err != nil {
		return res, err
	}
	nextCompanies, err := read(newDir, CompaniesFile)
	if err != nil {
		return res, err
	}
	nextJobs, err := read(newDir, JobsFile)
	if err != nil {
		return res, err
	}
	res.Jobs, res.Companies = len(nextJobs), len(nextCompanies)
	problem := func(s string) { res.Problems = append(res.Problems, s) }

	// Structure.
	companyIDs := map[any]jsjson.Object{}
	slugs := map[any]bool{}
	for _, c := range nextCompanies {
		id, slug := jsKey(c, "id"), jsKey(c, "slug")
		if _, dup := companyIDs[id]; dup {
			problem("duplicate company id " + fieldStr(c, "id"))
		}
		if slugs[slug] {
			problem("duplicate company slug " + fieldStr(c, "slug"))
		}
		companyIDs[id] = c
		slugs[slug] = true
	}
	jobIDs := map[any]bool{}
	fingerprints := map[any]bool{}
	prefixes := map[string]string{}
	for _, j := range nextJobs {
		id, fp := jsKey(j, "id"), jsKey(j, "fingerprint")
		if jobIDs[id] {
			problem("duplicate job id " + fieldStr(j, "id"))
		}
		if fingerprints[fp] {
			problem("duplicate fingerprint " + fieldStr(j, "fingerprint"))
		}
		jobIDs[id] = true
		fingerprints[fp] = true
		if _, ok := companyIDs[jsKey(j, "company_id")]; !ok {
			problem("job " + fieldStr(j, "id") + " has unknown company " + fieldStr(j, "company_id"))
		}
		if truthy(j, "is_active") {
			// The script crashed on a non-string id; here it is a problem.
			idStr, ok := getAny(j, "id").(string)
			if !ok {
				problem("job " + fieldStr(j, "id") + " has an id that is not a string")
				continue
			}
			key := fieldStr(j, "company_id") + " " + URLPrefix(idStr)
			if other, ok := prefixes[key]; ok {
				problem("active jobs " + other + " and " + idStr + " share a URL prefix")
			}
			prefixes[key] = idStr
		}
	}

	// Identity never changes.
	before := map[any]jsjson.Object{}
	var beforeOrder []any
	for _, j := range curJobs {
		k := jsKey(j, "id")
		if _, seen := before[k]; !seen {
			beforeOrder = append(beforeOrder, k)
		}
		before[k] = j
	}
	for _, j := range nextJobs {
		b, ok := before[jsKey(j, "id")]
		if !ok {
			continue
		}
		if !strictEqual(b, j, "company_id") {
			problem("job " + fieldStr(j, "id") + " moved company")
		}
		if !strictEqual(b, j, "first_seen_at") {
			problem("job " + fieldStr(j, "id") + " first_seen_at changed")
		}
	}
	for _, k := range beforeOrder {
		if !jobIDs[k] {
			problem("job " + fieldStr(before[k], "id") + " disappeared (jobs are never deleted)")
		}
	}

	// Growth and collapse per company.
	slugOf := func(cid any) string { // `${companyIds.get(cid)?.slug}`
		c, ok := companyIDs[cid]
		if !ok {
			return "undefined"
		}
		return fieldStr(c, "slug")
	}
	added := countBy(nextJobs, func(j jsjson.Object) bool { _, seen := before[jsKey(j, "id")]; return !seen })
	for _, e := range added {
		slug := slugOf(e.key)
		if e.n > MaxNewJobsPerCompany && !approved("company:"+slug) {
			problem(slug + " added " + strconv.Itoa(e.n) + " jobs in one run (limit " + strconv.Itoa(MaxNewJobsPerCompany) + ")")
		}
	}
	activeBefore := countBy(curJobs, func(j jsjson.Object) bool { return truthy(j, "is_active") })
	activeAfter := map[any]int{}
	for _, e := range countBy(nextJobs, func(j jsjson.Object) bool { return truthy(j, "is_active") }) {
		activeAfter[e.key] = e.n
	}
	for _, e := range activeBefore {
		slug := e.label // companyIds.get(cid)?.slug ?? cid
		if c, ok := companyIDs[e.key]; ok {
			if v, present := c.Get("slug"); present && v != nil {
				slug = fieldStr(c, "slug")
			}
		}
		stillActive := true
		if c, ok := companyIDs[e.key]; ok {
			if v, present := c.Get("active"); present && v == false {
				stillActive = false
			}
		}
		if e.n >= 5 && activeAfter[e.key] == 0 && stillActive && !approved("company:"+slug) {
			problem(slug + " dropped from " + strconv.Itoa(e.n) + " active jobs to 0")
		}
	}

	// New apply hosts, for the reviewer.
	known := map[string]bool{}
	for _, j := range curJobs {
		known[hostOf(fieldStr(j, "apply_url"))] = true
	}
	fresh := map[string]bool{}
	for _, j := range nextJobs {
		if truthy(j, "is_active") {
			if h := hostOf(fieldStr(j, "apply_url")); !known[h] {
				fresh[h] = true
			}
		}
	}
	for h := range fresh {
		res.NewHosts = append(res.NewHosts, h)
	}
	sort.Slice(res.NewHosts, func(a, b int) bool { return ByCodeUnit(res.NewHosts[a], res.NewHosts[b]) < 0 })
	return res, nil
}

func hostOf(u string) string {
	p, err := weburl.Parse(u)
	if err != nil {
		return "(malformed)"
	}
	return p.Hostname
}

type countEntry struct {
	key   any
	label string // `${company_id}`
	n     int
}

// countBy counts jobs per company_id in first-seen order (a JavaScript Map).
func countBy(jobs []jsjson.Object, pred func(jsjson.Object) bool) []countEntry {
	idx := map[any]int{}
	var out []countEntry
	for _, j := range jobs {
		if !pred(j) {
			continue
		}
		k := jsKey(j, "company_id")
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, countEntry{key: k, label: fieldStr(j, "company_id")})
		}
		out[i].n++
	}
	return out
}

// undefinedKey and nullKey stand for undefined and null as Map keys.
type (
	undefinedKey struct{}
	nullKey      struct{}
	refKey       struct{ p *int }
)

func getAny(o jsjson.Object, key string) any {
	v, _ := o.Get(key)
	return v
}

// jsKey is o[key] as a JavaScript Map/Set key (SameValueZero): primitives by
// value, objects and arrays each distinct.
func jsKey(o jsjson.Object, key string) any {
	v, ok := o.Get(key)
	if !ok {
		return undefinedKey{}
	}
	switch x := v.(type) {
	case nil:
		return nullKey{}
	case string, bool:
		return x
	case float64:
		if x == 0 {
			return float64(0) // -0 and 0 are the same key
		}
		return x
	}
	return refKey{new(int)}
}

// strictEqual is a[key] === b[key].
func strictEqual(a, b jsjson.Object, key string) bool {
	ka, kb := jsKey(a, key), jsKey(b, key)
	if _, ref := ka.(refKey); ref {
		return false
	}
	return ka == kb
}

// field is `${o[key]}`.
func fieldStr(o jsjson.Object, key string) string { return jsString(o, key) }

func truthy(o jsjson.Object, key string) bool {
	v, ok := o.Get(key)
	return ok && jsTruthy(v)
}

func jsTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

// Report renders the result as the script printed it: what goes to stdout,
// what goes to stderr, and the exit code (0 ok, 1 refused).
func (r GateResult) Report() (stdout, stderr string, code int) {
	var out, errb strings.Builder
	if len(r.NewHosts) > 0 {
		out.WriteString("state-gate: new apply hosts: " + strings.Join(r.NewHosts, ", ") + "\n")
	}
	if len(r.Problems) > 0 {
		for i, p := range r.Problems {
			if i == 50 {
				break
			}
			errb.WriteString("state-gate: " + p + "\n")
		}
		errb.WriteString("state-gate: refusing to publish (" + strconv.Itoa(len(r.Problems)) + " problem(s))\n")
		return out.String(), errb.String(), 1
	}
	out.WriteString("state-gate: ok (" + strconv.Itoa(r.Jobs) + " jobs, " + strconv.Itoa(r.Companies) + " companies)\n")
	return out.String(), errb.String(), 0
}
