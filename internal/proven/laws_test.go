package proven

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

func propertyRand() *rand.Rand { return rand.New(rand.NewSource(0x4f70656e496e7465)) }

func int64ptr(n int64) *int64 { return &n }

func randomRateCase(r *rand.Rand) (*RateBucket, int64, int64, int64) {
	now := r.Int63n(1_000_000)
	limit := r.Int63n(100)
	window := r.Int63n(10_000) + 1
	if r.Intn(4) == 0 {
		return nil, now, limit, window
	}
	return &RateBucket{Count: r.Int63n(130), ResetAt: r.Int63n(1_010_000)}, now, limit, window
}

func TestLaw_rl_remaining(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 2_000; i++ {
		prev, now, limit, window := randomRateCase(r)
		got := RateLimitStep(prev, now, limit, window)
		want := limit - got.Bucket.Count
		if want < 0 {
			want = 0
		}
		if got.Remaining != want {
			t.Fatalf("remaining=%d, want %d for %#v", got.Remaining, want, got)
		}
	}
}

func TestLaw_rl_bounded(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 2_000; i++ {
		prev, now, limit, window := randomRateCase(r)
		got := RateLimitStep(prev, now, limit, window)
		if got.OK && got.Bucket.Count > limit {
			t.Fatalf("admitted count %d above limit %d", got.Bucket.Count, limit)
		}
	}
}

func TestLaw_rl_zero_limit(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		prev, now, _, window := randomRateCase(r)
		if RateLimitStep(prev, now, 0, window).OK {
			t.Fatalf("admitted with zero limit: %#v", prev)
		}
	}
}

func TestLaw_rl_expired(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		reset := r.Int63n(1_000_000)
		now := reset + r.Int63n(1_000)
		limit, window := r.Int63n(100), r.Int63n(1_000)+1
		prev := &RateBucket{Count: r.Int63n(200), ResetAt: reset}
		if got, want := RateLimitStep(prev, now, limit, window), RateLimitStep(nil, now, limit, window); got != want {
			t.Fatalf("expired bucket used: got %#v want %#v", got, want)
		}
	}
}

func TestLaw_rl_open_window(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		now := r.Int63n(1_000_000)
		reset := now + 1 + r.Int63n(1_000)
		count, limit := r.Int63n(200), r.Int63n(100)
		got := RateLimitStep(&RateBucket{Count: count, ResetAt: reset}, now, limit, 1)
		if want := rateAdmit(count, limit, reset); got != want {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestLaw_rl_admit_count(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 2_000; i++ {
		count, limit := r.Int63n(200), r.Int63n(100)
		got := rateAdmit(count, limit, 7)
		if got.Bucket.Count != rateBump(got.OK, count) {
			t.Fatalf("count=%d ok=%v input=%d", got.Bucket.Count, got.OK, count)
		}
	}
}

func TestLaw_rl_admit_keeps_window(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		reset := r.Int63n(1_000_000)
		if got := rateAdmit(r.Int63n(200), r.Int63n(100), reset); got.Bucket.ResetAt != reset {
			t.Fatalf("reset moved from %d to %d", reset, got.Bucket.ResetAt)
		}
	}
}

func TestLaw_rl_fresh_window(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		now, window := r.Int63n(1_000_000), r.Int63n(1_000)+1
		if got := rateFresh(now, r.Int63n(100), window).Bucket.ResetAt; got != now+window {
			t.Fatalf("reset=%d, want %d", got, now+window)
		}
	}
}

func TestLaw_rl_admit_progress(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 2_000; i++ {
		count, extra := r.Int63n(200), r.Int63n(50)
		limit := count + 1 + extra
		if !rateAdmit(count, limit, 7).OK {
			t.Fatalf("denied under limit: count=%d limit=%d", count, limit)
		}
	}
}

func TestLaw_rl_fresh_progress(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		now, extra, window := r.Int63n(1_000_000), r.Int63n(50), r.Int63n(1_000)+1
		if !rateFresh(now, 1+extra, window).OK {
			t.Fatalf("fresh denied for limit=%d", 1+extra)
		}
	}
}

func randomEnds(r *rand.Rand) []int64 {
	ends := make([]int64, r.Intn(5))
	for i := range ends {
		ends[i] = r.Int63n(1_000_000)
	}
	return ends
}

func TestLaw_fr_stale_iff_not_fresh(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 2_000; i++ {
		ends := randomEnds(r)
		anchor, now, maxMS := r.Int63n(1_000_000), r.Int63n(1_000_000), r.Int63n(1_000_000)
		fresh := freshFromValues(ends, anchor, now, maxMS, "test")
		stale := staleFromValues(ends, anchor, now, maxMS, "test")
		if stale == fresh {
			t.Fatalf("fresh=%v stale=%v ends=%v anchor=%d now=%d max=%d", fresh, stale, ends, anchor, now, maxMS)
		}
	}
}

func TestLaw_fr_stale_monotone(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 2_000; i++ {
		ends := randomEnds(r)
		anchor, now, maxMS, k := r.Int63n(1_000_000), r.Int63n(1_000_000), r.Int63n(1_000_000), r.Int63n(1_000_000)
		if staleFromValues(ends, anchor, now, maxMS, "test") && !staleFromValues(ends, anchor, now+k, maxMS, "test") {
			t.Fatalf("staleness reversed after %d", k)
		}
	}
}

func TestLaw_fr_last_seen_monotone(t *testing.T) {
	r := propertyRand()
	base := time.UnixMilli(0).UTC()
	for i := 0; i < 1_000; i++ {
		seen := r.Int63n(1_000_000)
		now := r.Int63n(1_000_000)
		k := r.Int63n(1_000_000)
		if IsStaleByLastSeen(base.Add(time.Duration(seen)*time.Millisecond), base.Add(time.Duration(now)*time.Millisecond), 0) &&
			!IsStaleByLastSeen(base.Add(time.Duration(seen)*time.Millisecond), base.Add(time.Duration(now+k)*time.Millisecond), 0) {
			t.Fatal("last-seen staleness reversed")
		}
	}
}

func TestLaw_fr_term_live_at_end(t *testing.T) {
	for _, term := range []string{"summer", "fall", "winter", "spring", "other"} {
		end := ApplyByDate(TermYearLike{Term: term, Year: 2028})
		facts := FreshnessFacts{TermYears: []TermYearLike{{Term: term, Year: 2028}}, Anchor: time.UnixMilli(0), Now: end}
		if !IsFresh(facts) {
			t.Fatalf("%s not live at end", term)
		}
	}
}

func TestLaw_fr_age_live_at_limit(t *testing.T) {
	anchor := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, maxDays := range []int64{0, 1, 30, 120} {
		facts := FreshnessFacts{Anchor: anchor, Now: anchor.Add(time.Duration(maxDays) * 24 * time.Hour), MaxAgeDays: int64ptr(maxDays)}
		if !IsFresh(facts) {
			t.Fatalf("not fresh at %d-day limit", maxDays)
		}
	}
}

func TestLaw_fr_public_needs_role(t *testing.T) {
	facts := FreshnessFacts{Anchor: time.UnixMilli(0), Now: time.UnixMilli(0)}
	if IsPublic(0, facts) {
		t.Fatal("roleless listing is public")
	}
}

func TestLaw_fr_public_is_fresh(t *testing.T) {
	r := propertyRand()
	base := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 1_000; i++ {
		now := base.Add(time.Duration(r.Intn(300)) * 24 * time.Hour)
		facts := FreshnessFacts{Anchor: base, Now: now}
		if IsPublic(int64(1+r.Intn(20)), facts) != IsFresh(facts) {
			t.Fatal("public differs from fresh when roles exist")
		}
	}
}

func assertApplyBy(t *testing.T, term string, year int, want time.Time) {
	t.Helper()
	if got := ApplyByDate(TermYearLike{Term: term, Year: year}); !got.Equal(want) {
		t.Fatalf("%s: got %s want %s", term, got, want)
	}
}

func TestLaw_fr_summer_apply_by(t *testing.T) {
	assertApplyBy(t, "summer", 2027, time.Date(2027, 4, 30, 23, 59, 59, 0, time.UTC))
}
func TestLaw_fr_fall_apply_by(t *testing.T) {
	assertApplyBy(t, "fall", 2027, time.Date(2027, 8, 31, 23, 59, 59, 0, time.UTC))
}
func TestLaw_fr_winter_apply_by(t *testing.T) {
	assertApplyBy(t, "winter", 2027, time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC))
}
func TestLaw_fr_spring_apply_by(t *testing.T) {
	if got, want := ApplyByDate(TermYearLike{Term: "spring", Year: 2027}), ApplyByDate(TermYearLike{Term: "winter", Year: 2027}); got != want {
		t.Fatalf("spring %s != winter %s", got, want)
	}
}
func TestLaw_fr_other_apply_by(t *testing.T) {
	assertApplyBy(t, "other", 2027, time.Date(2027, 12, 31, 23, 59, 59, 0, time.UTC))
}
func TestLaw_fr_constants(t *testing.T) {
	if DayMS != 86_400_000 || MaxAgeDaysNoTerm != 120 || LastSeenStaleDays != 14 {
		t.Fatalf("constants changed: %d/%d/%d", DayMS, MaxAgeDaysNoTerm, LastSeenStaleDays)
	}
}

func TestLaw_ap_empty(t *testing.T) {
	for locations := int64(0); locations < 1_000; locations++ {
		if NeedsPicker(0, locations) {
			t.Fatalf("empty family with %d locations needs picker", locations)
		}
	}
}
func TestLaw_ap_many_postings(t *testing.T) {
	for postings := int64(2); postings < 100; postings++ {
		for locations := int64(0); locations < 10; locations++ {
			if !NeedsPicker(postings, locations) {
				t.Fatalf("%d postings did not need picker", postings)
			}
		}
	}
}
func TestLaw_ap_one_posting(t *testing.T) {
	for locations := int64(0); locations < 1_000; locations++ {
		if NeedsPicker(1, locations) != (locations >= 2) {
			t.Fatalf("one posting, %d locations", locations)
		}
	}
}
func TestLaw_ap_title_matches_picker(t *testing.T) {
	for locations := int64(0); locations < 1_000; locations++ {
		if QualifySingleTitle(locations) != NeedsPicker(1, locations) {
			t.Fatalf("title/picker mismatch at %d", locations)
		}
	}
}
func TestLaw_ap_distinct_says_locations(t *testing.T) {
	for n := int64(0); n < 1_000; n++ {
		if SaysOpenings(n, n) {
			t.Fatalf("%d distinct locations says openings", n)
		}
	}
}
func TestLaw_ap_repeats_say_openings(t *testing.T) {
	for unique := int64(0); unique < 100; unique++ {
		for k := int64(0); k < 20; k++ {
			if !SaysOpenings(1+unique+k, unique) {
				t.Fatalf("unique=%d extra=%d says locations", unique, k)
			}
		}
	}
}

var allSightings = []JobSighting{JobSightingGone, JobSightingRejected, JobSightingUnverified, JobSightingMatched}

func TestLaw_dp_failed_fetch_keeps(t *testing.T) {
	for prior := int64(0); prior < 100; prior++ {
		for _, seen := range allSightings {
			if ShouldRetireJob(BoardRunFailed, prior, seen) {
				t.Fatal("failed fetch retired job")
			}
		}
	}
}
func TestLaw_dp_expired_retires(t *testing.T) {
	for _, seen := range allSightings {
		if !ShouldRetireJob(BoardRunExpired, 7, seen) {
			t.Fatal("expired capture kept job")
		}
	}
}
func TestLaw_dp_expired_keeps_count(t *testing.T) {
	for prior := int64(0); prior < 100; prior++ {
		if NextEmptyRuns(BoardRunExpired, prior) != prior {
			t.Fatal("expired capture changed count")
		}
	}
}
func TestLaw_dp_board_gone_retires(t *testing.T) {
	if !ShouldRetireJob(BoardRunListed, 0, JobSightingGone) {
		t.Fatal("gone job kept")
	}
}
func TestLaw_dp_on_board_keeps(t *testing.T) {
	if ShouldRetireJob(BoardRunListed, 0, JobSightingMatched) {
		t.Fatal("matched job retired")
	}
}
func TestLaw_dp_unverified_keeps(t *testing.T) {
	if ShouldRetireJob(BoardRunListed, 0, JobSightingUnverified) {
		t.Fatal("unverified job retired")
	}
}
func TestLaw_dp_rejected_retires(t *testing.T) {
	if !ShouldRetireJob(BoardRunListed, 0, JobSightingRejected) {
		t.Fatal("rejected job kept")
	}
}
func TestLaw_dp_empty_waits(t *testing.T) {
	for _, seen := range allSightings {
		if ShouldRetireJob(BoardRunEmpty, 0, seen) || ShouldRetireJob(BoardRunEmpty, 1, seen) {
			t.Fatal("first or second empty run retired job")
		}
	}
}
func TestLaw_dp_three_empty_runs_retire(t *testing.T) {
	for prior := int64(0); prior < 100; prior++ {
		thirdPrior := NextEmptyRuns(BoardRunEmpty, NextEmptyRuns(BoardRunEmpty, prior))
		for _, seen := range allSightings {
			if !ShouldRetireJob(BoardRunEmpty, thirdPrior, seen) {
				t.Fatal("third empty run kept job")
			}
		}
	}
}
func TestLaw_dp_retire_after_three(t *testing.T) {
	if RetireAfterEmptyRuns != 3 {
		t.Fatalf("threshold=%d", RetireAfterEmptyRuns)
	}
}
func TestLaw_dp_listed_resets(t *testing.T) {
	for prior := int64(0); prior < 100; prior++ {
		if NextEmptyRuns(BoardRunListed, prior) != 0 {
			t.Fatal("listed board did not reset")
		}
	}
}
func TestLaw_dp_failed_keeps_count(t *testing.T) {
	for prior := int64(0); prior < 100; prior++ {
		if NextEmptyRuns(BoardRunFailed, prior) != prior {
			t.Fatal("failed board changed count")
		}
	}
}
func TestLaw_dp_empty_counts(t *testing.T) {
	for prior := int64(0); prior < 100; prior++ {
		if NextEmptyRuns(BoardRunEmpty, prior) != prior+1 {
			t.Fatal("empty board did not increment")
		}
	}
}

func randomFunnel(r *rand.Rand) FunnelCounts {
	value := func() *int64 {
		if r.Intn(4) == 0 {
			return nil
		}
		n := r.Int63n(1_000)
		return &n
	}
	return FunnelCounts{Fetched: value(), Title: value(), Tech: value(), Upserted: value(), DescriptionMissing: value()}
}

func TestLaw_fn_total_empty(t *testing.T) {
	if got := SumFunnels(nil); got != (FunnelTotals{}) {
		t.Fatalf("empty total=%#v", got)
	}
}
func TestLaw_fn_total_append(t *testing.T) {
	r := propertyRand()
	for i := 0; i < 1_000; i++ {
		xs, ys := make([]FunnelCounts, r.Intn(10)), make([]FunnelCounts, r.Intn(10))
		for j := range xs {
			xs[j] = randomFunnel(r)
		}
		for j := range ys {
			ys[j] = randomFunnel(r)
		}
		joined := append(append([]FunnelCounts(nil), xs...), ys...)
		if got, want := SumFunnels(joined), plusFunnels(SumFunnels(xs), SumFunnels(ys)); got != want {
			t.Fatalf("append total %#v != %#v", got, want)
		}
	}
}
func TestLaw_fn_zero_match_tagged(t *testing.T) {
	for _, rest := range []string{"", "x", " board had 12 jobs", "\nanything"} {
		if !IsZeroMatchRun("zero_match:" + rest) {
			t.Fatalf("tagged error missed: %q", rest)
		}
	}
}
func TestLaw_fn_funnel_not_zero_match(t *testing.T) {
	for _, rest := range []string{"", "zero_match:", " fetched=1"} {
		if IsZeroMatchRun("funnel " + rest) {
			t.Fatalf("funnel report tagged: %q", rest)
		}
	}
}
func TestLaw_fn_errors_not_zero_match(t *testing.T) {
	if IsZeroMatchRun("HTTP 500") || IsZeroMatchRun("") {
		t.Fatal("ordinary error or empty string tagged")
	}
}

func TestLaw_pg_page_min(t *testing.T) {
	for _, raw := range []string{"", "garbage", "-1", "0", "1", "9999999999999999999999"} {
		if ParsePage(raw) < 1 {
			t.Fatalf("page below one for %q", raw)
		}
	}
}
func TestLaw_pg_page_max(t *testing.T) {
	for _, raw := range []string{"", "garbage", "1", "100000", "100001", "9999999999999999999999"} {
		if ParsePage(raw) > 100_000 {
			t.Fatalf("page above cap for %q", raw)
		}
	}
}
func TestLaw_pg_limit_min(t *testing.T) {
	for _, raw := range []string{"", "garbage", "-1", "0", "1"} {
		if ParseLimit(raw) < 1 {
			t.Fatalf("limit below one for %q", raw)
		}
	}
}
func TestLaw_pg_limit_max(t *testing.T) {
	for _, raw := range []string{"", "garbage", "1", "100", "101", "9999999999999999999999"} {
		if ParseLimit(raw) > 100 {
			t.Fatalf("limit above cap for %q", raw)
		}
	}
}
func TestLaw_pg_defaults(t *testing.T) {
	if ParsePage() != 1 || ParseLimit() != 27 {
		t.Fatalf("defaults page=%d limit=%d", ParsePage(), ParseLimit())
	}
}
func TestLaw_pg_offset_exact(t *testing.T) {
	for p := int64(0); p < 1_000; p++ {
		for _, limit := range []int64{0, 1, 27, 100} {
			if got := PageOffset(1+p, limit); got != p*limit {
				t.Fatalf("p=%d limit=%d got=%d", p, limit, got)
			}
		}
	}
}
func TestLaw_pg_parse_digits_only(t *testing.T) {
	if got := []int64{ParsePage("3"), ParsePage("+2"), ParsePage("2abc"), ParsePage("")}; !reflect.DeepEqual(got, []int64{3, 1, 1, 1}) {
		t.Fatalf("got %v", got)
	}
}
func TestLaw_pg_first_page_offset(t *testing.T) {
	for limit := int64(0); limit < 1_000; limit++ {
		if PageOffset(1, limit) != 0 {
			t.Fatalf("limit=%d", limit)
		}
	}
}

func TestLaw_js_suffix_skips_dashes(t *testing.T) {
	for _, id := range []string{"", "12345678", "ab-cd-ef-12", "日本語abcdefghi"} {
		if jobIDSuffix("-"+id) != jobIDSuffix(id) {
			t.Fatalf("leading dash changed suffix for %q", id)
		}
	}
}
func TestLaw_js_hex_digits(t *testing.T) {
	for _, r := range "0123456789abcdefABCDEF" {
		if !isHexASCII(r) {
			t.Fatalf("rejected %q", r)
		}
	}
	for _, r := range "gG-z_١" {
		if isHexASCII(r) {
			t.Fatalf("accepted %q", r)
		}
	}
}
func TestLaw_js_roundtrip(t *testing.T) {
	r := propertyRand()
	hex := "0123456789abcdef"
	for i := 0; i < 2_000; i++ {
		var id strings.Builder
		for j := 0; j < 8; j++ {
			id.WriteByte(hex[r.Intn(len(hex))])
		}
		base := strings.Repeat("x", r.Intn(100)) + []string{"", "-", "日本語", "😀"}[r.Intn(4)]
		slug := base + "-" + id.String()
		got := ParseIdPrefix(slug)
		if got == nil || *got != id.String() {
			t.Fatalf("parse %q = %#v", slug, got)
		}
	}
}

var strikeCounts = []int64{0, 1, 2, 3, 4, 17, NatLimit - 1}

func TestLaw_as_inconclusive_keeps(t *testing.T) {
	for _, c := range strikeCounts {
		for _, due := range []bool{false, true} {
			if got := NextStrikes(c, due, ApplyInconclusive); got != c {
				t.Fatalf("inconclusive probe moved %d strikes to %d (due=%v)", c, got, due)
			}
		}
	}
}

func TestLaw_as_alive_clears(t *testing.T) {
	for _, c := range strikeCounts {
		for _, due := range []bool{false, true} {
			if got := NextStrikes(c, due, ApplyAlive); got != 0 {
				t.Fatalf("alive probe left %d of %d strikes (due=%v)", got, c, due)
			}
		}
	}
}

func TestLaw_as_retire_iff(t *testing.T) {
	for _, d1 := range []bool{false, true} {
		for _, d2 := range []bool{false, true} {
			if StrikesRetired(NextStrikes(NextStrikes(0, d1, ApplyDead), d2, ApplyDead)) {
				t.Fatal("two dead probes retired a job")
			}
		}
		two := NextStrikes(NextStrikes(0, d1, ApplyDead), true, ApplyDead)
		if StrikesRetired(NextStrikes(two, false, ApplyDead)) {
			t.Fatal("a third strike inside the gap counted")
		}
		if !StrikesRetired(NextStrikes(two, true, ApplyDead)) {
			t.Fatal("three counted strikes kept a job")
		}
	}
}

func TestLaw_as_constants(t *testing.T) {
	if StrikesToRetire != 3 || StrikeGapHours != 20 {
		t.Fatalf("strikes %d, gap %dh", StrikesToRetire, StrikeGapHours)
	}
}
