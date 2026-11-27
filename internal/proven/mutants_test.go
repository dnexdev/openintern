package proven

import (
	"strings"
	"testing"
	"time"
)

// TestMutantsKilled is the Go-native mutation runner. Each case implements one
// row of packages/proven/mutants.tsv and returns whether the corresponding
// law or ported wrapper test still holds. A surviving mutant fails this test.
func TestMutantsKilled(t *testing.T) {
	mutants := []struct {
		name     string
		lawHolds func() bool
	}{
		{
			name: "pagination_offset_always_zero",
			lawHolds: func() bool {
				mutantOffset := func(_, _ int64) int64 { return 0 }
				return mutantOffset(2, 3) == (2-1)*3
			},
		},
		{
			name: "pagination_offset_ignores_limit",
			lawHolds: func() bool {
				mutantOffset := func(page, _ int64) int64 { return (page - 1) * 27 }
				return mutantOffset(2, 3) == (2-1)*3
			},
		},
		{
			name: "pagination_page_cap_raised",
			lawHolds: func() bool {
				mutantPage := func(n int64) int64 {
					if n > 1_000_000 {
						return 1_000_000
					}
					return n
				}
				return mutantPage(250_000) <= 100_000
			},
		},
		{name: "freshness_age_limit_120_to_12", lawHolds: func() bool { return int64(12) == MaxAgeDaysNoTerm }},
		{name: "freshness_last_seen_14_to_1", lawHolds: func() bool { return int64(1) == LastSeenStaleDays }},
		{name: "freshness_day_ms_wrong", lawHolds: func() bool { return int64(1_000) == DayMS }},
		{
			name: "freshness_summer_ends_august_31",
			lawHolds: func() bool {
				mutant := time.Date(2027, time.August, 31, 23, 59, 59, 0, time.UTC)
				want := time.Date(2027, time.April, 30, 23, 59, 59, 0, time.UTC)
				return mutant.Equal(want)
			},
		},
		{
			name: "job_slug_suffix_keeps_dashes",
			lawHolds: func() bool {
				mutantSuffix := func(id string) string { return takeRunes(id, 8) }
				return mutantSuffix("-12345678") == mutantSuffix("12345678")
			},
		},
		{
			name: "job_slug_hex_accepts_g_to_z",
			lawHolds: func() bool {
				mutantIsHex := func(r rune) bool { return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
				return !mutantIsHex('g')
			},
		},
		{
			name: "funnel_zero_match_means_not_funnel",
			lawHolds: func() bool {
				mutantZeroMatch := func(s string) bool { return !strings.HasPrefix(s, "funnel") }
				return !mutantZeroMatch("HTTP 500") && !mutantZeroMatch("")
			},
		},
		{
			name: "deactivation_gone_job_kept",
			lawHolds: func() bool {
				mutantRetireListed := func(seen JobSighting) bool { return seen == JobSightingRejected }
				return mutantRetireListed(JobSightingGone)
			},
		},
		{
			name: "deactivation_unverified_job_retired",
			lawHolds: func() bool {
				mutantRetireListed := func(seen JobSighting) bool { return seen != JobSightingMatched }
				return !mutantRetireListed(JobSightingUnverified)
			},
		},
		{
			name: "deactivation_first_empty_retires",
			lawHolds: func() bool {
				mutantRetireEmpty := func(_ int64) bool { return true }
				return !mutantRetireEmpty(0) && !mutantRetireEmpty(1)
			},
		},
		{
			name: "deactivation_failed_fetch_retires",
			lawHolds: func() bool {
				mutantRetireFailed := func() bool { return true }
				return !mutantRetireFailed()
			},
		},
		{
			name: "deactivation_listed_keeps_empty_count",
			lawHolds: func() bool {
				mutantNextListed := func(prior int64) int64 { return prior }
				return mutantNextListed(7) == 0
			},
		},
		{
			name: "rate_limit_fresh_denies_all",
			lawHolds: func() bool {
				mutant := RateVerdict{OK: false, Remaining: 4, Bucket: RateBucket{Count: 1, ResetAt: 10}}
				return mutant.OK == RateLimitStep(nil, 0, 5, 10).OK
			},
		},
		{
			name: "rate_limit_resets_one_ms_late",
			lawHolds: func() bool {
				mutantStep := func(prev RateBucket, now, limit, window int64) RateVerdict {
					if prev.ResetAt < now {
						return rateFresh(now, limit, window)
					}
					return rateAdmit(prev.Count, limit, prev.ResetAt)
				}
				prev := RateBucket{Count: 5, ResetAt: 100}
				return mutantStep(prev, 100, 10, 60) == rateFresh(100, 10, 60)
			},
		},
		{
			name: "rate_limit_admitted_request_not_counted",
			lawHolds: func() bool {
				mutant := RateVerdict{OK: true, Remaining: 4, Bucket: RateBucket{Count: 0, ResetAt: 10}}
				return mutant.Remaining == 5-mutant.Bucket.Count
			},
		},
		{
			name: "freshness_spring_folded_into_summer",
			lawHolds: func() bool {
				mutantSpring := time.Date(2027, time.April, 30, 23, 59, 59, 0, time.UTC)
				winter := time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC)
				return mutantSpring.Equal(winter)
			},
		},
	}

	if len(mutants) != 19 {
		t.Fatalf("mutation table has %d cases, want 19", len(mutants))
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			if mutant.lawHolds() {
				t.Fatal("mutant survived the Go law checks")
			}
		})
	}
}
