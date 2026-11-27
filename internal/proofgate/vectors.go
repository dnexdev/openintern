package proofgate

// Vectors regenerates internal/proven/testdata/vectors.json from the Bend
// model itself, so the Go port is checked against the checked source with no
// compiled JavaScript and no TypeScript in the loop.
//
// The inputs are the ones the TypeScript generator used. Every decision comes
// from bend: the generator writes a throwaway program whose main calls one
// Bend def per line and prints the result, and runs it next to a copy of
// packages/proven/bend (outside the guarded proof tree, which may never hold
// a main or IO). What the Go side adds is the wrapper contract of the original
// packages/proven/src/index.ts, which marshalled values and decided nothing:
// RangeError for an input that is not a Bend Nat or an unknown enum tag, the
// defaults (max age, last-seen days), term names to constructors, and
// {years_back, month0, day} to a UTC instant. Bend's runtime keeps Nats below
// 2^48 and aborts the program past that, so when a call overflows the
// generator records that call's error and reruns the rest.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/state/jsjson"
)

const natLimit int64 = 1 << 48

// The line bend's runtime prints on stderr when a Nat leaves the immediate range.
const bendOverflow = "bend: a Nat past the largest immediate 2^48-1"

type obj = jsjson.Object
type mem = jsjson.Member

// bendCall is one def call; expr is a Bend expression of type String.
type bendCall struct {
	def  string // module:def, as the TypeScript wrapper named it in errors
	expr string
}

type outcome struct {
	line     string
	overflow bool
}

// capture is the result of one wrapper call: a RangeError raised before bend
// was called, or the index of the Bend call that answers it.
type capture struct {
	rangeErr string
	call     int
	decode   func(string) (any, error)
}

type vgen struct {
	calls   []bendCall
	results []outcome
}

// pre collects the first wrapper error of a call, as the first throw in
// JavaScript's left-to-right argument evaluation would.
type pre struct{ msg string }

func (p *pre) nat(n int64, what string) string {
	if p.msg == "" && (n < 0 || n >= natLimit) {
		p.msg = fmt.Sprintf("%s must be an integer in [0, 2^48), got %d", what, n)
	}
	return natExpr(n)
}

func (p *pre) fail(msg string) {
	if p.msg == "" {
		p.msg = msg
	}
}

func (g *vgen) call(p *pre, def, expr string, decode func(string) (any, error)) capture {
	if p != nil && p.msg != "" {
		return capture{rangeErr: p.msg}
	}
	g.calls = append(g.calls, bendCall{def: def, expr: expr})
	return capture{call: len(g.calls) - 1, decode: decode}
}

// members is TypeScript's `...capture(fn)`: {value} or {error: {name, message}}.
func (g *vgen) members(c capture) ([]mem, error) {
	if c.rangeErr != "" {
		return []mem{{Key: "error", Value: obj{{Key: "name", Value: "RangeError"}, {Key: "message", Value: c.rangeErr}}}}, nil
	}
	r := g.results[c.call]
	if r.overflow {
		msg := fmt.Sprintf("proven %s: %s", g.calls[c.call].def, bendOverflow)
		return []mem{{Key: "error", Value: obj{{Key: "name", Value: "Error"}, {Key: "message", Value: msg}}}}, nil
	}
	v, err := c.decode(r.line)
	if err != nil {
		return nil, fmt.Errorf("%s: bend printed %q: %w", g.calls[c.call].def, r.line, err)
	}
	return []mem{{Key: "value", Value: v}}, nil
}

func (g *vgen) captureObj(c capture) (obj, error) {
	m, err := g.members(c)
	return obj(m), err
}

// value is a call that the wrapper never guards (no capture in the vectors).
func (g *vgen) value(c capture) (any, error) {
	if c.rangeErr != "" || g.results[c.call].overflow {
		return nil, fmt.Errorf("%s: unexpected error for an uncaptured call", g.calls[c.call].def)
	}
	return c.decode(g.results[c.call].line)
}

// natExpr writes n as a Bend Nat expression. Literals stop at 2^32-1; larger
// values are hi*2^32+lo (no intermediate passes 2^48 for n < 2^48). Values
// outside [0, 2^48) never reach bend: the wrapper refuses them first.
func natExpr(n int64) string {
	if n < 0 || n >= natLimit {
		return "0n"
	}
	if n <= 0xFFFFFFFF {
		return fmt.Sprintf("%dn", n)
	}
	return fmt.Sprintf("Nat.add(Nat.mul(%dn, Nat.mul(65536n, 65536n)), %dn)", n>>32, n&0xFFFFFFFF)
}

func strExpr(s string) (string, error) {
	if strings.ContainsAny(s, "\"\\\n\r") {
		return "", fmt.Errorf("vector input %q needs escaping, which the generator does not do", s)
	}
	return `"` + s + `"`, nil
}

func decodeBool(line string) (any, error) {
	switch line {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return nil, fmt.Errorf("not a Bool")
}

func decodeNat(line string) (any, error) {
	n, err := strconv.ParseInt(line, 10, 64)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("not a Nat")
	}
	return n, nil
}

func decodeNats(line string, n int) ([]int64, error) {
	f := strings.Split(line, " ")
	if len(f) != n {
		return nil, fmt.Errorf("want %d fields", n)
	}
	out := make([]int64, n)
	for i, s := range f {
		v, err := decodeNat(s)
		if err != nil {
			return nil, err
		}
		out[i] = v.(int64)
	}
	return out, nil
}

func decodeString(line string) (any, error) { return line, nil }

func decodeMaybeString(line string) (any, error) {
	if line == "N" {
		return nil, nil
	}
	if s, ok := strings.CutPrefix(line, "S"); ok {
		return s, nil
	}
	return nil, fmt.Errorf("not a Maybe String")
}

func decodeVerdict(line string) (any, error) {
	f := strings.SplitN(line, " ", 2)
	if len(f) != 2 {
		return nil, fmt.Errorf("not a Verdict")
	}
	ok, err := decodeBool(f[0])
	if err != nil {
		return nil, err
	}
	n, err := decodeNats(f[1], 3)
	if err != nil {
		return nil, err
	}
	return obj{{Key: "ok", Value: ok}, {Key: "remaining", Value: n[0]},
		{Key: "bucket", Value: obj{{Key: "count", Value: n[1]}, {Key: "resetAt", Value: n[2]}}}}, nil
}

func decodeTotals(line string) (any, error) {
	n, err := decodeNats(line, 5)
	if err != nil {
		return nil, err
	}
	return obj{{Key: "fetched", Value: n[0]}, {Key: "title", Value: n[1]}, {Key: "tech", Value: n[2]},
		{Key: "upserted", Value: n[3]}, {Key: "descriptionMissing", Value: n[4]}}, nil
}

// The helpers print each Bend result as one line.
const vectorsPrelude = `import Base
import ./packages/proven/bend/rate_limit.bend as RL
import ./packages/proven/bend/freshness.bend as FR
import ./packages/proven/bend/apply_picker.bend as AP
import ./packages/proven/bend/deactivation.bend as DP
import ./packages/proven/bend/apply_strikes.bend as AS
import ./packages/proven/bend/funnel.bend as FN
import ./packages/proven/bend/pagination.bend as PG
import ./packages/proven/bend/job_slug.bend as JS

def show_bool(x: Bool) -> String:
  match x:
    case True{}:
      "true"
    case False{}:
      "false"

def show_verdict(x: RL.Verdict) -> String:
  match x:
    case RL.Verdict{o, r, RL.Bucket{c, t}}:
      show_bool(o) ++ " " ++ Nat.show(r) ++ " " ++ Nat.show(c) ++ " " ++ Nat.show(t)

def show_apply_by(x: FR.ApplyBy) -> String:
  match x:
    case FR.ApplyBy{y, m, d}:
      Nat.show(y) ++ " " ++ Nat.show(m) ++ " " ++ Nat.show(d)

def show_totals(x: FN.Totals) -> String:
  match x:
    case FN.Totals{a, b, c, d, e}:
      Nat.show(a) ++ " " ++ Nat.show(b) ++ " " ++ Nat.show(c) ++ " " ++ Nat.show(d) ++ " " ++ Nat.show(e)

def show_maybe(x: Maybe<&2, String>) -> String:
  match x:
    case None{}:
      "N"
    case Some{s}:
      "S" ++ s

def main() -> IO(Unit):
  do IO<Unit>:
`

// run evaluates every pending call with bend, rerunning after each overflow.
func (g *vgen) run(ctx context.Context, gate *Gate) error {
	tree, err := os.ReadDir(filepath.Join(gate.Root, BendDir))
	if err != nil {
		return err
	}
	var files []string
	for _, e := range tree {
		if strings.HasSuffix(e.Name(), ".bend") {
			files = append(files, filepath.ToSlash(filepath.Join(BendDir, e.Name())))
		}
	}
	tmp, err := copyTree(gate.Root, files)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	for start := len(g.results); start < len(g.calls); {
		var src strings.Builder
		src.WriteString(vectorsPrelude)
		for _, c := range g.calls[start:] {
			fmt.Fprintf(&src, "    IO.print(%s)\n", c.expr)
		}
		if err := os.WriteFile(filepath.Join(tmp, "vectors.bend"), []byte(src.String()), 0o644); err != nil {
			return err
		}
		r, err := gate.bend(ctx, tmp, "vectors.bend")
		if err != nil {
			return err
		}
		var lines []string
		sc := bufio.NewScanner(strings.NewReader(r.stdout))
		sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		want := len(g.calls) - start
		switch {
		case r.status == 0 && len(lines) == want && jsTrim(r.stderr) == "":
			for _, l := range lines {
				g.results = append(g.results, outcome{line: l})
			}
			start += want
		case r.status != 0 && len(lines) < want && jsTrim(r.stderr) == bendOverflow:
			for _, l := range lines {
				g.results = append(g.results, outcome{line: l})
			}
			g.results = append(g.results, outcome{overflow: true})
			start += len(lines) + 1
		default:
			return fail("vectors program failed (exit status %d, %d of %d results):\n%s", r.status, len(lines), want, r.combined())
		}
	}
	return nil
}

// Vectors returns the bytes of internal/proven/testdata/vectors.json computed
// from the current Bend model.
func (g *Gate) Vectors(ctx context.Context) ([]byte, error) {
	v := &vgen{}

	// Phase 1: the constants and term dates the inputs are built from.
	constDefs := []string{"day_ms", "max_age_days", "last_seen_days"}
	var consts []capture
	for _, d := range constDefs {
		consts = append(consts, v.call(nil, "freshness:"+d, "Nat.show(FR."+d+"())", decodeNat))
	}
	consts = append(consts, v.call(nil, "deactivation:retire_after_empty", "Nat.show(DP.retire_after_empty())", decodeNat))
	consts = append(consts, v.call(nil, "apply_strikes:strikes_to_retire", "Nat.show(AS.strikes_to_retire())", decodeNat))
	consts = append(consts, v.call(nil, "apply_strikes:strike_gap_hours", "Nat.show(AS.strike_gap_hours())", decodeNat))
	ctors := []string{"Summer", "Fall", "Winter", "Spring", "Other"}
	applyBy := map[string]capture{}
	for _, c := range ctors {
		applyBy[c] = v.call(nil, "freshness:apply_by", "show_apply_by(FR.apply_by(FR."+c+"{}))", func(l string) (any, error) { return decodeNats(l, 3) })
	}
	if err := v.run(ctx, g); err != nil {
		return nil, err
	}
	cv := make([]int64, len(consts))
	for i, c := range consts {
		x, err := v.value(c)
		if err != nil {
			return nil, err
		}
		cv[i] = x.(int64)
	}
	dayMS, maxAgeDays, lastSeenDays, retireAfter, strikesToRetire, strikeGapHours := cv[0], cv[1], cv[2], cv[3], cv[4], cv[5]
	ab := map[string][]int64{}
	for c, cp := range applyBy {
		x, err := v.value(cp)
		if err != nil {
			return nil, err
		}
		ab[c] = x.([]int64)
	}
	termCtor := func(term string) string {
		switch term {
		case "summer":
			return "Summer"
		case "fall":
			return "Fall"
		case "winter":
			return "Winter"
		case "spring":
			return "Spring"
		}
		return "Other"
	}
	// applyByDate: Date.UTC(year - years_back, month0, day, 23, 59, 59).
	applyByDate := func(term string, year int64) time.Time {
		a := ab[termCtor(term)]
		return time.Date(int(year-a[0]), time.Month(a[1]+1), int(a[2]), 23, 59, 59, 0, time.UTC)
	}

	// Phase 2: every vector.
	var build []func() (mem, error)
	section := func(key string, entries []func() (any, error)) {
		build = append(build, func() (mem, error) {
			list := make([]any, 0, len(entries))
			for _, e := range entries {
				x, err := e()
				if err != nil {
					return mem{}, fmt.Errorf("%s: %w", key, err)
				}
				list = append(list, x)
			}
			return mem{Key: key, Value: list}, nil
		})
	}
	// entry joins fixed members with call results, in order.
	type part struct {
		key  string
		val  any
		cap  capture
		mode int // 0 fixed value, 1 spread capture, 2 nested capture, 3 plain value
	}
	entry := func(parts ...part) func() (any, error) {
		return func() (any, error) {
			var o obj
			for _, p := range parts {
				switch p.mode {
				case 0:
					o = append(o, mem{Key: p.key, Value: p.val})
				case 1:
					m, err := v.members(p.cap)
					if err != nil {
						return nil, err
					}
					o = append(o, m...)
				case 2:
					c, err := v.captureObj(p.cap)
					if err != nil {
						return nil, err
					}
					o = append(o, mem{Key: p.key, Value: c})
				case 3:
					x, err := v.value(p.cap)
					if err != nil {
						return nil, err
					}
					o = append(o, mem{Key: p.key, Value: x})
				}
			}
			return o, nil
		}
	}
	fixed := func(key string, val any) part { return part{key: key, val: val} }
	spread := func(c capture) part { return part{cap: c, mode: 1} }
	nested := func(key string, c capture) part { return part{key: key, cap: c, mode: 2} }
	// uncaptured is a member whose value comes from a call that never throws.
	uncaptured := func(key string, c capture) part { return part{key: key, cap: c, mode: 3} }

	// Rate limiter.
	type bucket struct{ count, resetAt int64 }
	rateStep := func(prev *bucket, now, limit, window int64) capture {
		p := &pre{}
		if window < 1 {
			p.fail(fmt.Sprintf("windowMs must be at least 1, got %d", window))
		}
		prevExpr := "None{}"
		if prev != nil {
			prevExpr = "Some{RL.Bucket{" + p.nat(prev.count, "bucket count") + ", " + p.nat(prev.resetAt, "bucket resetAt") + "}}"
		}
		expr := "show_verdict(RL.step(" + prevExpr + ", " + p.nat(now, "now") + ", " + p.nat(limit, "limit") + ", " + p.nat(window, "windowMs") + "))"
		return v.call(p, "rate_limit:step", expr, decodeVerdict)
	}
	bucketJSON := func(b *bucket) any {
		if b == nil {
			return nil
		}
		return obj{{Key: "count", Value: b.count}, {Key: "resetAt", Value: b.resetAt}}
	}
	var rate []func() (any, error)
	rateCase := func(prev *bucket, now, limit, window int64) {
		c := rateStep(prev, now, limit, window)
		rate = append(rate, entry(fixed("prev", bucketJSON(prev)), fixed("now", now), fixed("limit", limit), fixed("windowMs", window), spread(c)))
	}
	for _, now := range []int64{0, 1, 999, 1_700_000_000_000, natLimit - 2} {
		for _, limit := range []int64{0, 1, 2, 5} {
			for _, window := range []int64{1, 2, 60_000} {
				if now+window >= natLimit {
					continue
				}
				prevs := []*bucket{nil, {0, now}, {1, now}, {limit, now}}
				if now+1 < natLimit {
					prevs = append(prevs, &bucket{0, now + 1}, &bucket{1, now + 1}, &bucket{limit, now + 1})
				}
				for _, prev := range prevs {
					rateCase(prev, now, limit, window)
				}
			}
		}
	}
	for _, c := range []struct {
		prev               *bucket
		now, limit, window int64
	}{
		{nil, -1, 1, 1},
		{nil, natLimit, 1, 1},
		{nil, 0, -1, 1},
		{nil, 0, natLimit, 1},
		{nil, 0, 1, 0},
		{nil, natLimit - 1, 1, 1},
		{&bucket{natLimit, 2}, 1, 1, 1},
		{&bucket{0, natLimit}, 1, 1, 1},
	} {
		rateCase(c.prev, c.now, c.limit, c.window)
	}
	section("rateLimit", rate)

	// Apply-by dates (computed in Go from the phase-1 apply_by results).
	terms := []string{"summer", "fall", "winter", "spring", "other", "constructor"}
	var applyEntries []func() (any, error)
	for _, term := range terms {
		for _, year := range []int64{1970, 2026, 2027} {
			d := applyByDate(term, year)
			applyEntries = append(applyEntries, entry(
				fixed("input", obj{{Key: "term", Value: term}, {Key: "year", Value: year}}),
				fixed("iso", d.Format("2006-01-02T15:04:05.000Z")),
				fixed("ms", d.UnixMilli())))
		}
	}
	section("applyBy", applyEntries)

	// Freshness.
	type termYear struct {
		term string
		year int64
	}
	type freshIn struct {
		termYears   []termYear
		anchor, now int64
		maxAgeDays  *int64
	}
	i64 := func(n int64) *int64 { return &n }
	var freshInputs []freshIn
	for _, anchor := range []int64{0, 1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), natLimit - dayMS - 1} {
		for _, delta := range []int64{0, 1, dayMS, 120 * dayMS, 120*dayMS + 1} {
			now := anchor + delta
			if now >= natLimit {
				continue
			}
			freshInputs = append(freshInputs,
				freshIn{nil, anchor, now, nil},
				freshIn{nil, anchor, now, i64(0)},
				freshIn{nil, anchor, now, i64(1)})
		}
	}
	for _, term := range terms {
		end := applyByDate(term, 2027).UnixMilli()
		for _, delta := range []int64{-1, 0, 1} {
			freshInputs = append(freshInputs, freshIn{[]termYear{{term, 2027}}, 0, end + delta, nil})
		}
	}
	freshInputs = append(freshInputs, freshIn{nil, natLimit - 1, natLimit - 1, i64(1)})
	freshArgs := func(p *pre, f freshIn) string {
		ends := make([]string, len(f.termYears))
		for i, ty := range f.termYears {
			ends[i] = p.nat(applyByDate(ty.term, ty.year).UnixMilli(), "date")
		}
		days := maxAgeDays
		if f.maxAgeDays != nil {
			days = *f.maxAgeDays
		}
		return "[" + strings.Join(ends, ", ") + "], " + p.nat(f.anchor, "date") + ", " + p.nat(f.now, "date") + ", " + p.nat(days*dayMS, "value")
	}
	var freshEntries []func() (any, error)
	for _, f := range freshInputs {
		pf, ps, p0, p2 := &pre{}, &pre{}, &pre{}, &pre{}
		fresh := v.call(pf, "freshness:is_fresh", "show_bool(FR.is_fresh("+freshArgs(pf, f)+"))", decodeBool)
		stale := v.call(ps, "freshness:is_stale", "show_bool(FR.is_stale("+freshArgs(ps, f)+"))", decodeBool)
		r0 := p0.nat(0, "value")
		pub0 := v.call(p0, "freshness:is_public", "show_bool(FR.is_public("+r0+", "+freshArgs(p0, f)+"))", decodeBool)
		r2 := p2.nat(2, "value")
		pub2 := v.call(p2, "freshness:is_public", "show_bool(FR.is_public("+r2+", "+freshArgs(p2, f)+"))", decodeBool)
		tys := make([]any, len(f.termYears))
		for i, ty := range f.termYears {
			tys[i] = obj{{Key: "term", Value: ty.term}, {Key: "year", Value: ty.year}}
		}
		input := obj{{Key: "termYears", Value: tys}, {Key: "anchor", Value: f.anchor}, {Key: "now", Value: f.now}}
		if f.maxAgeDays != nil {
			input = append(input, mem{Key: "maxAgeDays", Value: *f.maxAgeDays})
		}
		freshEntries = append(freshEntries, entry(fixed("input", input), nested("fresh", fresh), nested("stale", stale), nested("public0", pub0), nested("public2", pub2)))
	}
	section("freshness", freshEntries)

	var lastSeen []func() (any, error)
	for _, c := range []struct {
		seen, now int64
		maxDays   *int64
	}{
		{0, 0, nil},
		{time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC).UnixMilli(), nil},
		{time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC).UnixMilli() + 1, nil},
		{1, 1, i64(0)},
		{natLimit - 1, natLimit - 1, i64(1)},
	} {
		p := &pre{}
		days := lastSeenDays
		var maxJSON any
		if c.maxDays != nil {
			days, maxJSON = *c.maxDays, *c.maxDays
		}
		expr := "show_bool(FR.stale_last_seen(" + p.nat(c.seen, "date") + ", " + p.nat(c.now, "date") + ", " + p.nat(days*dayMS, "value") + "))"
		cp := v.call(p, "freshness:stale_last_seen", expr, decodeBool)
		lastSeen = append(lastSeen, entry(fixed("seen", c.seen), fixed("now", c.now), fixed("maxDays", maxJSON), spread(cp)))
	}
	section("staleLastSeen", lastSeen)

	// Apply picker.
	var picker []func() (any, error)
	pickerCase := func(postings, locations int64) {
		pn := &pre{}
		needs := v.call(pn, "apply_picker:needs_picker", "show_bool(AP.needs_picker("+pn.nat(postings, "value")+", "+pn.nat(locations, "value")+"))", decodeBool)
		pq := &pre{}
		qualify := v.call(pq, "apply_picker:qualify_single_title", "show_bool(AP.qualify_single_title("+pq.nat(locations, "value")+"))", decodeBool)
		po := &pre{}
		openings := v.call(po, "apply_picker:says_openings", "show_bool(AP.says_openings("+po.nat(postings, "value")+", "+po.nat(locations, "value")+"))", decodeBool)
		picker = append(picker, entry(fixed("postings", postings), fixed("locations", locations), nested("needs", needs), nested("qualify", qualify), nested("openings", openings)))
	}
	for _, postings := range []int64{0, 1, 2, 3, natLimit - 1} {
		for _, locations := range []int64{0, 1, 2, 3, natLimit - 1} {
			pickerCase(postings, locations)
		}
	}
	pickerCase(-1, -1)
	pickerCase(natLimit, natLimit)
	section("picker", picker)

	// Deactivation.
	runTag := map[string]string{"failed": "Failed", "empty": "Empty", "listed": "Listed", "expired": "Expired"}
	seenTag := map[string]string{"gone": "Gone", "rejected": "Rejected", "unverified": "Unverified", "matched": "Matched"}
	tag := func(p *pre, table map[string]string, key, what string) string {
		t, ok := table[key]
		if !ok {
			p.fail("unknown " + what + ": " + key)
		}
		return "DP." + t + "{}"
	}
	var deact []func() (any, error)
	deactCase := func(run string, prior int64, seen string) {
		pr := &pre{}
		retireExpr := "show_bool(DP.retire(" + tag(pr, runTag, run, "board run") + ", " + pr.nat(prior, "priorEmptyRuns") + ", " + tag(pr, seenTag, seen, "job sighting") + "))"
		retire := v.call(pr, "deactivation:retire", retireExpr, decodeBool)
		pn := &pre{}
		nextExpr := "Nat.show(DP.next_empty(" + tag(pn, runTag, run, "board run") + ", " + pn.nat(prior, "priorEmptyRuns") + "))"
		next := v.call(pn, "deactivation:next_empty", nextExpr, decodeNat)
		deact = append(deact, entry(fixed("run", run), fixed("prior", prior), fixed("seen", seen), nested("retire", retire), nested("next", next)))
	}
	for _, run := range []string{"failed", "empty", "listed", "expired"} {
		for _, prior := range []int64{0, 1, 2, 3, 4, natLimit - 1} {
			for _, seen := range []string{"gone", "rejected", "unverified", "matched"} {
				deactCase(run, prior, seen)
			}
		}
	}
	deactCase("unknown", 0, "gone")
	deactCase("listed", 0, "unknown")
	section("deactivation", deact)

	// Dead apply links.
	verdictTag := map[string]string{"alive": "Alive", "dead": "Dead", "inconclusive": "Inconclusive"}
	var strikes []func() (any, error)
	strikeCase := func(count int64, due bool, verdict string) {
		pn := &pre{}
		t, ok := verdictTag[verdict]
		if !ok {
			pn.fail("unknown apply verdict: " + verdict)
		}
		dueTag := "False{}"
		if due {
			dueTag = "True{}"
		}
		next := v.call(pn, "apply_strikes:next", "Nat.show(AS.next("+pn.nat(count, "count")+", "+dueTag+", AS."+t+"{}))", decodeNat)
		pr := &pre{}
		retired := v.call(pr, "apply_strikes:retired", "show_bool(AS.retired("+pr.nat(count, "count")+"))", decodeBool)
		strikes = append(strikes, entry(fixed("count", count), fixed("due", due), fixed("verdict", verdict), nested("next", next), nested("retired", retired)))
	}
	for _, count := range []int64{0, 1, 2, 3, 4, natLimit - 1} {
		for _, due := range []bool{false, true} {
			for _, verdict := range []string{"alive", "dead", "inconclusive"} {
				strikeCase(count, due, verdict)
			}
		}
	}
	strikeCase(-1, true, "dead")
	strikeCase(0, true, "unknown")
	section("applyStrikes", strikes)

	// Funnel totals.
	type funnel [5]*int64 // fetched, title, tech, upserted, descriptionMissing
	z, one, maxN, half := i64(0), i64(1), i64(natLimit-1), i64(1<<47)
	funnelSets := [][]funnel{
		{},
		{{nil, nil, nil, nil, nil}},
		{{i64(10), i64(5), i64(3), i64(2), nil}, {nil, i64(1), nil, i64(4), i64(7)}},
		{{z, one, maxN, z, one}},
		{{half, z, z, z, z}, {half, z, z, z, z}},
	}
	funnelKeys := []string{"fetched", "title", "tech", "upserted", "descriptionMissing"}
	var funnels []func() (any, error)
	for _, set := range funnelSets {
		p := &pre{}
		items := make([]string, len(set))
		js := make([]any, len(set))
		for i, f := range set {
			fields := make([]string, 5)
			o := obj{}
			for k, n := range f {
				if n == nil {
					fields[k] = "None{}"
					o = append(o, mem{Key: funnelKeys[k], Value: nil})
				} else {
					fields[k] = "Some{" + p.nat(*n, "value") + "}"
					o = append(o, mem{Key: funnelKeys[k], Value: *n})
				}
			}
			items[i] = "FN.Funnel{" + strings.Join(fields, ", ") + "}"
			js[i] = o
		}
		cp := v.call(p, "funnel:total", "show_totals(FN.total(["+strings.Join(items, ", ")+"]))", decodeTotals)
		funnels = append(funnels, entry(fixed("funnels", js), spread(cp)))
	}
	section("funnels", funnels)

	var zero []func() (any, error)
	for _, e := range []*string{nil, strp(""), strp("zero_match:"), strp("zero_match: board had 12 jobs"), strp("funnel fetched=1"), strp("HTTP 500"), strp(" zero_match:x")} {
		var errJSON any
		var val part
		if e != nil {
			errJSON = *e
		}
		if e == nil || *e == "" {
			// isZeroMatchRun(error): error ? is_zero_match(error) : false
			val = fixed("value", false)
		} else {
			s, err := strExpr(*e)
			if err != nil {
				return nil, err
			}
			val = uncaptured("value", v.call(nil, "funnel:is_zero_match", "show_bool(FN.is_zero_match("+s+"))", decodeBool))
		}
		zero = append(zero, entry(fixed("error", errJSON), val))
	}
	section("zeroMatch", zero)

	// Pagination.
	type param struct {
		isArray bool
		null    bool
		values  []string
	}
	str := func(s string) param { return param{values: []string{s}} }
	params := []param{{null: true}, {null: true}, str(""), str(" "), str("0"), str("1"), str("3"), str(" 5 "), str("+2"), str("-2"), str("2.5"), str("2abc"), str("0003"), str("100"), str("101"), str("250000"), str("99999999999999999999"), {isArray: true, values: []string{"4", "9"}}, {isArray: true, values: []string{}}}
	var pages []func() (any, error)
	for _, prm := range params {
		raw := "None{}"
		if len(prm.values) > 0 {
			s, err := strExpr(jsTrim(prm.values[0]))
			if err != nil {
				return nil, err
			}
			raw = "Some{" + s + "}"
		}
		var rawJSON any
		switch {
		case prm.isArray:
			l := make([]any, len(prm.values))
			for i, s := range prm.values {
				l[i] = s
			}
			rawJSON = l
		case !prm.null:
			rawJSON = prm.values[0]
		}
		page := v.call(nil, "pagination:page", "Nat.show(PG.page("+raw+"))", decodeNat)
		limit := v.call(nil, "pagination:limit", "Nat.show(PG.limit("+raw+"))", decodeNat)
		pages = append(pages, entry(fixed("raw", rawJSON), fixed("isArray", prm.isArray), uncaptured("page", page), uncaptured("limit", limit)))
	}
	section("pagination", pages)

	var offsets []func() (any, error)
	for _, c := range [][2]int64{{0, 0}, {1, 27}, {2, 1}, {3, 27}, {100000, 100}, {natLimit - 1, 0}, {natLimit - 1, 1}, {natLimit - 1, natLimit - 1}, {-1, 1}, {natLimit, 1}} {
		p := &pre{}
		cp := v.call(p, "pagination:offset", "Nat.show(PG.offset("+p.nat(c[0], "value")+", "+p.nat(c[1], "value")+"))", decodeNat)
		offsets = append(offsets, entry(fixed("page", c[0]), fixed("limit", c[1]), spread(cp)))
	}
	section("offsets", offsets)

	// Job slugs.
	var segments []func() (any, error)
	for _, base := range []string{"", "job", "software-intern", "über-praktikum", "日本語"} {
		for _, id := range []string{"", "12345678", "1a2b3c4d-0000-0000-0000-000000000000", "-12345678", "12-34-56-78", "abcdef"} {
			b, err := strExpr(base)
			if err != nil {
				return nil, err
			}
			i, err := strExpr(id)
			if err != nil {
				return nil, err
			}
			cp := v.call(nil, "job_slug:segment", "JS.segment("+b+", "+i+")", decodeString)
			segments = append(segments, entry(fixed("base", base), fixed("id", id), uncaptured("value", cp)))
		}
	}
	section("slugSegments", segments)

	var parses []func() (any, error)
	for _, slug := range []string{"", "12345678", "-12345678", "a-1234567", "a-12345678", "a-ABCDEF12", "a-1234567g", "a--deadbeef", "日本語-0123abcd", "emoji😀-deadbeef", "a-١٢٣٤٥٦٧٨"} {
		s, err := strExpr(slug)
		if err != nil {
			return nil, err
		}
		cp := v.call(nil, "job_slug:parse", "show_maybe(JS.parse("+s+"))", decodeMaybeString)
		parses = append(parses, entry(fixed("slug", slug), uncaptured("value", cp)))
	}
	section("slugParses", parses)

	if err := v.run(ctx, g); err != nil {
		return nil, err
	}

	doc := obj{{Key: "constants", Value: obj{
		{Key: "NAT_LIMIT", Value: natLimit},
		{Key: "DAY_MS", Value: dayMS},
		{Key: "MAX_AGE_DAYS_NO_TERM", Value: maxAgeDays},
		{Key: "LAST_SEEN_STALE_DAYS", Value: lastSeenDays},
		{Key: "RETIRE_AFTER_EMPTY_RUNS", Value: retireAfter},
		{Key: "STRIKES_TO_RETIRE", Value: strikesToRetire},
		{Key: "STRIKE_GAP_HOURS", Value: strikeGapHours},
	}}}
	for _, b := range build {
		m, err := b()
		if err != nil {
			return nil, err
		}
		doc = append(doc, m)
	}
	return append(jsjson.MarshalIndent(doc, "  "), '\n'), nil
}

func strp(s string) *string { return &s }
