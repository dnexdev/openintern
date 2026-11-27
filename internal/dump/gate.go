package dump

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// GateOptions configures Gate.
type GateOptions struct {
	// Table is the apply-host allowlist built into this binary; nil uses
	// the embedded registry's.
	Table registry.Table
	// Enforce makes off-allowlist apply URLs a problem (OPENINTERN_APPLY_HOSTS
	// =enforce); otherwise they are counted and warned about.
	Enforce bool
	// Approved reports whether data/expected-delta.json approves a scope
	// ("total", "ats:<name>") today.
	Approved func(scope string) bool
}

// GateResult is what the dump gate found.
type GateResult struct {
	Problems []string // integrity, allowlist and denylist failures
	Holds    []string // circuit-breaker swings against the previous release
	OffList  int      // off-allowlist apply URLs (warned about in warn mode)
	Enforce  bool
	Jobs     int
}

// Gate is scripts/dump-gate.mjs, the publish gate for the daily dump. It
// holds the release when a file does not match MANIFEST.json, an apply URL
// is not https or not on the allowlist (enforce mode), a delisted job is
// present, or, against the previous release (prevDir, optional), the job
// total moves more than 10%, an ATS drops more than 15% and more than 10
// jobs, or an ATS with at least 5 jobs drops to zero, unless approved.
// Unreadable or malformed files are errors, where the script crashed.
func Gate(newDir, prevDir string, opts GateOptions) (GateResult, error) {
	res := GateResult{Enforce: opts.Enforce}
	table := opts.Table
	if table == nil {
		table = registry.Default().ApplyHosts()
	}
	approved := opts.Approved
	if approved == nil {
		approved = func(string) bool { return false }
	}

	// 1. Integrity.
	manifest, err := readJSON(filepath.Join(newDir, "MANIFEST.json"))
	if err != nil {
		return res, err
	}
	files, _ := get(manifest, "files").(jsjson.Object)
	for _, m := range files {
		buf, err := os.ReadFile(filepath.Join(newDir, m.Key))
		if err != nil {
			return res, err
		}
		sum := sha256.Sum256(buf)
		entry, _ := m.Value.(jsjson.Object)
		want, _ := get(entry, "sha256").(string)
		size, _ := get(entry, "bytes").(float64)
		if hex.EncodeToString(sum[:]) != want || float64(len(buf)) != size {
			res.Problems = append(res.Problems, m.Key+": does not match MANIFEST.json")
		}
	}

	// 2. Every apply link is https and on the allowlist built into this binary.
	dump, err := readJSON(filepath.Join(newDir, "jobs.json"))
	if err != nil {
		return res, err
	}
	denyFile, err := readJSON(filepath.Join(newDir, "denylist.json"))
	if err != nil {
		return res, err
	}
	deny := map[string]bool{}
	for _, j := range list(get(denyFile, "jobs")) {
		deny[strings.ToLower(str(get(j, "id")))] = true
	}
	jobs := list(get(dump, "jobs"))
	res.Jobs = len(jobs)
	for _, j := range jobs {
		id := str(get(j, "id"))
		if deny[strings.ToLower(id)] {
			res.Problems = append(res.Problems, "delisted job "+id+" is in the dump")
		}
		slug, url := str(get(get(j, "company"), "slug")), str(get(j, "apply_url"))
		if !registry.CheckApplyURL(slug, url, table).OK {
			res.OffList++
			if opts.Enforce {
				res.Problems = append(res.Problems, "off-allowlist apply URL for "+slug+": "+url)
			}
		}
	}

	// 3. Circuit breaker against the previous release.
	if prevDir == "" {
		return res, nil
	}
	if _, err := os.Stat(filepath.Join(prevDir, "jobs.json")); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return res, nil
		}
		return res, err
	}
	prev, err := readJSON(filepath.Join(prevDir, "jobs.json"))
	if err != nil {
		return res, err
	}
	prevJobs := list(get(prev, "jobs"))
	beforeOrder, before := countATS(prevJobs)
	_, after := countATS(jobs)
	prevTotal, newTotal := len(prevJobs), len(jobs)
	if prevTotal > 0 && math.Abs(float64(newTotal-prevTotal))/float64(prevTotal) > 0.1 && !approved("total") {
		res.Holds = append(res.Holds, fmt.Sprintf("total %d → %d (more than 10%%)", prevTotal, newTotal))
	}
	for _, ats := range beforeOrder {
		b, a := before[ats], after[ats]
		drop := b - a
		if a == 0 && b >= 5 && !approved("ats:"+ats) {
			res.Holds = append(res.Holds, fmt.Sprintf("%s %d → 0", ats, b))
		} else if drop > 10 && float64(drop)/float64(b) > 0.15 && !approved("ats:"+ats) {
			res.Holds = append(res.Holds, fmt.Sprintf("%s %d → %d", ats, b, a))
		}
	}
	return res, nil
}

// Report renders the result as the script printed it: what goes to stdout,
// what goes to stderr, and the exit code (0 publish, 1 hold).
func (r GateResult) Report() (stdout, stderr string, code int) {
	var out, errb strings.Builder
	if r.OffList > 0 && !r.Enforce {
		errb.WriteString("dump-gate: " + strconv.Itoa(r.OffList) + " apply URLs off the allowlist (warn mode)\n")
	}
	if len(r.Problems) > 0 || len(r.Holds) > 0 {
		for _, p := range r.Problems {
			errb.WriteString("dump-gate: " + p + "\n")
		}
		for _, h := range r.Holds {
			errb.WriteString("dump-gate: hold: " + h + "\n")
		}
		errb.WriteString("dump-gate: not publishing. Approve an expected change in data/expected-delta.json if it is real.\n")
		return out.String(), errb.String(), 1
	}
	out.WriteString("dump-gate: ok (" + strconv.Itoa(r.Jobs) + " jobs)\n")
	return out.String(), errb.String(), 0
}

func readJSON(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := jsjson.Parse(jsjson.DecodeUTF8(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

func get(v any, key string) any {
	o, _ := v.(jsjson.Object)
	x, ok := o.Get(key)
	if !ok {
		return nil
	}
	return x
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

// str is a JSON value as the script's string operations saw it.
func str(v any) string {
	if v == nil {
		return "undefined"
	}
	return jsString(v)
}

// countATS counts jobs by company.ats, keys in first-seen order.
func countATS(jobs []any) ([]string, map[string]int) {
	var order []string
	by := map[string]int{}
	for _, j := range jobs {
		ats := str(get(get(j, "company"), "ats"))
		if _, ok := by[ats]; !ok {
			order = append(order, ats)
		}
		by[ats]++
	}
	return order, by
}
