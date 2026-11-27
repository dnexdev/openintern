// Package proven is the Go implementation of the decision logic specified by
// LAWS.bend and modeled in packages/proven/bend.
package proven

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// NatLimit is the exclusive upper bound of a Bend Nat.
const NatLimit int64 = 1 << 48

// NAT_LIMIT preserves the name of the TypeScript wrapper's exported constant.
const NAT_LIMIT = NatLimit

const (
	DayMS                int64 = 86_400_000
	MaxAgeDaysNoTerm     int64 = 120
	LastSeenStaleDays    int64 = 14
	RetireAfterEmptyRuns int64 = 3
	StrikesToRetire      int64 = 3
	StrikeGapHours       int64 = 20

	DAY_MS                  = DayMS
	MAX_AGE_DAYS_NO_TERM    = MaxAgeDaysNoTerm
	LAST_SEEN_STALE_DAYS    = LastSeenStaleDays
	RETIRE_AFTER_EMPTY_RUNS = RetireAfterEmptyRuns
)

// RangeError is raised for values that cannot be represented as a Bend Nat.
// These are programmer errors, matching the TypeScript wrapper's RangeError.
type RangeError struct {
	Message string
}

func (e *RangeError) Error() string { return e.Message }

func rangePanic(what string, n int64) {
	panic(&RangeError{Message: fmt.Sprintf("%s must be an integer in [0, 2^48), got %d", what, n)})
}

func nat(n int64, what string) int64 {
	if n < 0 || n >= NatLimit {
		rangePanic(what, n)
	}
	return n
}

func natAdd(a, b int64, definition string) int64 {
	if a > NatLimit-1-b {
		panic(fmt.Errorf("proven %s: bend: a Nat past the largest immediate 2^48-1", definition))
	}
	return a + b
}

func natMul(a, b int64, definition string) int64 {
	if a != 0 && b > (NatLimit-1)/a {
		panic(fmt.Errorf("proven %s: bend: a Nat past the largest immediate 2^48-1", definition))
	}
	return a * b
}

// RateBucket is one fixed-window rate-limit bucket.
type RateBucket struct {
	Count   int64 `json:"count"`
	ResetAt int64 `json:"resetAt"`
}

// RateVerdict is the result of applying one request to a rate-limit bucket.
type RateVerdict struct {
	OK        bool       `json:"ok"`
	Remaining int64      `json:"remaining"`
	Bucket    RateBucket `json:"bucket"`
}

func rateFresh(now, limit, window int64) RateVerdict {
	resetAt := natAdd(now, window, "rate_limit:step")
	if limit == 0 {
		return RateVerdict{Bucket: RateBucket{ResetAt: resetAt}}
	}
	return RateVerdict{
		OK:        true,
		Remaining: limit - 1,
		Bucket:    RateBucket{Count: 1, ResetAt: resetAt},
	}
}

func rateAdmit(count, limit, resetAt int64) RateVerdict {
	if count >= limit {
		return RateVerdict{Bucket: RateBucket{Count: count, ResetAt: resetAt}}
	}
	return RateVerdict{
		OK:        true,
		Remaining: limit - count - 1,
		Bucket:    RateBucket{Count: count + 1, ResetAt: resetAt},
	}
}

func rateBump(ok bool, count int64) int64 {
	if ok {
		return count + 1
	}
	return count
}

// RateLimitStep applies one request to a key's fixed-window bucket. Invalid Nat
// inputs panic with RangeError, as the TypeScript wrapper throws RangeError.
func RateLimitStep(prev *RateBucket, now, limit, windowMs int64) RateVerdict {
	if windowMs < 1 {
		panic(&RangeError{Message: fmt.Sprintf("windowMs must be at least 1, got %d", windowMs)})
	}
	if prev != nil {
		nat(prev.Count, "bucket count")
		nat(prev.ResetAt, "bucket resetAt")
	}
	nat(now, "now")
	nat(limit, "limit")
	nat(windowMs, "windowMs")
	if prev == nil || prev.ResetAt <= now {
		return rateFresh(now, limit, windowMs)
	}
	return rateAdmit(prev.Count, limit, prev.ResetAt)
}

// TermYearLike identifies an internship term and cohort year.
type TermYearLike struct {
	Term string `json:"term"`
	Year int    `json:"year"`
}

// FreshnessFacts contains the values used by the freshness policy. A nil
// MaxAgeDays uses MaxAgeDaysNoTerm; a non-nil zero is an explicit zero-day age.
type FreshnessFacts struct {
	TermYears  []TermYearLike
	Anchor     time.Time
	Now        time.Time
	MaxAgeDays *int64
}

// ApplyByDate returns the last instant, in UTC, when a student can usefully
// apply for a term.
func ApplyByDate(ty TermYearLike) time.Time {
	year, month, day := ty.Year, time.December, 31
	switch ty.Term {
	case "summer":
		month, day = time.April, 30
	case "fall":
		month, day = time.August, 31
	case "winter", "spring":
		year--
	}
	return time.Date(year, month, day, 23, 59, 59, 0, time.UTC)
}

func epochMS(t time.Time, what string) int64 {
	return nat(t.UnixMilli(), what)
}

func freshnessValues(f FreshnessFacts, definition string) (ends []int64, anchor, now, maxMS int64) {
	ends = make([]int64, len(f.TermYears))
	for i, ty := range f.TermYears {
		ends[i] = epochMS(ApplyByDate(ty), "date")
	}
	anchor = epochMS(f.Anchor, "date")
	now = epochMS(f.Now, "date")
	maxDays := MaxAgeDaysNoTerm
	if f.MaxAgeDays != nil {
		maxDays = *f.MaxAgeDays
	}
	// The TypeScript wrapper multiplies before validating the Nat.
	if maxDays < 0 || maxDays >= NatLimit || maxDays > (NatLimit-1)/DayMS {
		rangePanic("value", maxDays*DayMS)
	}
	maxMS = maxDays * DayMS
	_ = definition
	return ends, anchor, now, maxMS
}

func anyLive(ends []int64, now int64) bool {
	for _, end := range ends {
		if now <= end {
			return true
		}
	}
	return false
}

func allPast(ends []int64, now int64) bool {
	for _, end := range ends {
		if end >= now {
			return false
		}
	}
	return true
}

func freshFromValues(ends []int64, anchor, now, maxMS int64, definition string) bool {
	if len(ends) != 0 {
		return anyLive(ends, now)
	}
	return now <= natAdd(anchor, maxMS, definition)
}

func staleFromValues(ends []int64, anchor, now, maxMS int64, definition string) bool {
	if len(ends) != 0 {
		return allPast(ends, now)
	}
	return natAdd(anchor, maxMS, definition) < now
}

// IsFresh reports whether a listing remains useful to applicants.
func IsFresh(f FreshnessFacts) bool {
	ends, anchor, now, maxMS := freshnessValues(f, "freshness:is_fresh")
	return freshFromValues(ends, anchor, now, maxMS, "freshness:is_fresh")
}

// IsStale is exactly the complement of IsFresh.
func IsStale(f FreshnessFacts) bool {
	ends, anchor, now, maxMS := freshnessValues(f, "freshness:is_stale")
	return staleFromValues(ends, anchor, now, maxMS, "freshness:is_stale")
}

// IsPublic reports whether a fresh listing has at least one role tag.
func IsPublic(roleCount int64, f FreshnessFacts) bool {
	nat(roleCount, "value")
	ends, anchor, now, maxMS := freshnessValues(f, "freshness:is_public")
	return roleCount > 0 && freshFromValues(ends, anchor, now, maxMS, "freshness:is_public")
}

// IsStaleByLastSeen reports whether no ingest has seen the listing for longer
// than maxDays. Omit maxDays to use LastSeenStaleDays.
func IsStaleByLastSeen(lastSeenAt, now time.Time, maxDays ...int64) bool {
	days := LastSeenStaleDays
	if len(maxDays) != 0 {
		days = maxDays[0]
	}
	seenMS := epochMS(lastSeenAt, "date")
	nowMS := epochMS(now, "date")
	if days < 0 || days >= NatLimit || days > (NatLimit-1)/DayMS {
		rangePanic("value", days*DayMS)
	}
	return natAdd(seenMS, days*DayMS, "freshness:stale_last_seen") < nowMS
}

// NeedsPicker reports whether an apply action must show a posting picker.
func NeedsPicker(postingCount, firstPostingLocationCount int64) bool {
	nat(postingCount, "value")
	nat(firstPostingLocationCount, "value")
	return postingCount >= 2 || postingCount == 1 && firstPostingLocationCount >= 2
}

// QualifySingleTitle reports whether a lone posting's title names locations.
func QualifySingleTitle(locationCount int64) bool {
	nat(locationCount, "value")
	return locationCount >= 2
}

// SaysOpenings reports whether repeated locations require "openings" copy.
func SaysOpenings(postingCount, uniqueLocationCount int64) bool {
	nat(postingCount, "value")
	nat(uniqueLocationCount, "value")
	return uniqueLocationCount < postingCount
}

// BoardRun describes what one ingest run learned about a company's board.
type BoardRun string

const (
	BoardRunFailed  BoardRun = "failed"
	BoardRunEmpty   BoardRun = "empty"
	BoardRunListed  BoardRun = "listed"
	BoardRunExpired BoardRun = "expired"
)

// JobSighting describes where an existing job stands on a listed board.
type JobSighting string

const (
	JobSightingGone       JobSighting = "gone"
	JobSightingRejected   JobSighting = "rejected"
	JobSightingUnverified JobSighting = "unverified"
	JobSightingMatched    JobSighting = "matched"
)

func validBoardRun(run BoardRun) bool {
	return run == BoardRunFailed || run == BoardRunEmpty || run == BoardRunListed || run == BoardRunExpired
}

func validJobSighting(seen JobSighting) bool {
	return seen == JobSightingGone || seen == JobSightingRejected || seen == JobSightingUnverified || seen == JobSightingMatched
}

// ShouldRetireJob applies the board-run deactivation policy.
func ShouldRetireJob(run BoardRun, priorEmptyRuns int64, seen JobSighting) bool {
	if !validBoardRun(run) {
		panic(&RangeError{Message: fmt.Sprintf("unknown board run: %s", run)})
	}
	if !validJobSighting(seen) {
		panic(&RangeError{Message: fmt.Sprintf("unknown job sighting: %s", seen)})
	}
	nat(priorEmptyRuns, "priorEmptyRuns")
	switch run {
	case BoardRunFailed:
		return false
	case BoardRunExpired:
		return true
	case BoardRunEmpty:
		return natAdd(1, priorEmptyRuns, "deactivation:retire") >= RetireAfterEmptyRuns
	default:
		return seen == JobSightingGone || seen == JobSightingRejected
	}
}

// NextEmptyRuns returns the company's consecutive-empty-run count after a run.
func NextEmptyRuns(run BoardRun, priorEmptyRuns int64) int64 {
	if !validBoardRun(run) {
		panic(&RangeError{Message: fmt.Sprintf("unknown board run: %s", run)})
	}
	nat(priorEmptyRuns, "priorEmptyRuns")
	switch run {
	case BoardRunEmpty:
		return natAdd(1, priorEmptyRuns, "deactivation:next_empty")
	case BoardRunListed:
		return 0
	default:
		return priorEmptyRuns
	}
}

// ApplyVerdict is what one probe of an apply link found.
type ApplyVerdict string

const (
	ApplyAlive        ApplyVerdict = "alive"
	ApplyDead         ApplyVerdict = "dead"
	ApplyInconclusive ApplyVerdict = "inconclusive"
)

// StrikesRetired reports whether a job with count dead-link strikes is off
// the board.
func StrikesRetired(count int64) bool {
	nat(count, "count")
	return count >= StrikesToRetire
}

// NextStrikes is a job's strike count after one probe. due means at least
// StrikeGapHours have passed since the last strike (ignored at count 0).
func NextStrikes(count int64, due bool, v ApplyVerdict) int64 {
	nat(count, "count")
	switch v {
	case ApplyAlive:
		return 0
	case ApplyInconclusive:
		return count
	case ApplyDead:
		if StrikesRetired(count) {
			return count
		}
		if count == 0 || due {
			return natAdd(count, 1, "apply_strikes:next")
		}
		return count
	}
	panic(&RangeError{Message: fmt.Sprintf("unknown apply verdict: %s", v)})
}

// FunnelCounts holds the optional counts parsed from one ingest-health record.
type FunnelCounts struct {
	Fetched            *int64 `json:"fetched"`
	Title              *int64 `json:"title"`
	Tech               *int64 `json:"tech"`
	Upserted           *int64 `json:"upserted"`
	DescriptionMissing *int64 `json:"descriptionMissing"`
}

// FunnelTotals is the sum of a set of ingest-health records.
type FunnelTotals struct {
	Fetched            int64 `json:"fetched"`
	Title              int64 `json:"title"`
	Tech               int64 `json:"tech"`
	Upserted           int64 `json:"upserted"`
	DescriptionMissing int64 `json:"descriptionMissing"`
}

func funnelValue(n *int64) int64 {
	if n == nil {
		return 0
	}
	return nat(*n, "value")
}

func plusFunnels(a, b FunnelTotals) FunnelTotals {
	return FunnelTotals{
		Fetched:            natAdd(a.Fetched, b.Fetched, "funnel:total"),
		Title:              natAdd(a.Title, b.Title, "funnel:total"),
		Tech:               natAdd(a.Tech, b.Tech, "funnel:total"),
		Upserted:           natAdd(a.Upserted, b.Upserted, "funnel:total"),
		DescriptionMissing: natAdd(a.DescriptionMissing, b.DescriptionMissing, "funnel:total"),
	}
}

// SumFunnels sums optional funnel counts, treating nil as zero.
func SumFunnels(funnels []FunnelCounts) FunnelTotals {
	var total FunnelTotals
	for _, f := range funnels {
		total = plusFunnels(total, FunnelTotals{
			Fetched:            funnelValue(f.Fetched),
			Title:              funnelValue(f.Title),
			Tech:               funnelValue(f.Tech),
			Upserted:           funnelValue(f.Upserted),
			DescriptionMissing: funnelValue(f.DescriptionMissing),
		})
	}
	return total
}

// IsZeroMatchRun reports whether an ingest error has the zero-match tag.
func IsZeroMatchRun(errorText string) bool {
	return strings.HasPrefix(errorText, "zero_match:")
}

func parseNatText(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	var n int64
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int64(c - '0')
		if n > (NatLimit-1-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}

func parseClamped(raw []string, fallback, maximum int64) int64 {
	if len(raw) == 0 {
		return fallback
	}
	n, ok := parseNatText(strings.TrimSpace(raw[0]))
	if !ok || n == 0 {
		return fallback
	}
	if n > maximum {
		return maximum
	}
	return n
}

// ParsePage parses the first value, if any, as a page in [1, 100000].
func ParsePage(raw ...string) int64 { return parseClamped(raw, 1, 100_000) }

// ParseLimit parses the first value, if any, as a page size in [1, 100].
func ParseLimit(raw ...string) int64 { return parseClamped(raw, 27, 100) }

// PageOffset returns the number of rows before page.
func PageOffset(page, limit int64) int64 {
	nat(page, "value")
	nat(limit, "value")
	if page == 0 {
		return 0
	}
	return natMul(page-1, limit, "pagination:offset")
}

func stripDashes(s string) string { return strings.ReplaceAll(s, "-", "") }

func takeRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

func jobIDSuffix(id string) string { return takeRunes(stripDashes(id), 8) }

// JobSlugSegment joins a title base to the first eight id characters after
// removing dashes.
func JobSlugSegment(base, id string) string { return base + "-" + jobIDSuffix(id) }

func isHexASCII(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
}

// ParseIdPrefix returns the lowercased trailing eight-hex-digit id prefix.
func ParseIdPrefix(slug string) *string {
	runes := []rune(slug)
	if len(runes) < 9 || runes[len(runes)-9] != '-' {
		return nil
	}
	tail := runes[len(runes)-8:]
	for _, r := range tail {
		if !isHexASCII(r) {
			return nil
		}
	}
	value := strings.ToLower(string(tail))
	return &value
}
