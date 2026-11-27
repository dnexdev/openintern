package proven

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func expectRangePanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if p := recover(); p == nil {
			t.Fatal("expected RangeError panic")
		} else if _, ok := p.(*RangeError); !ok {
			t.Fatalf("expected RangeError, got %T: %v", p, p)
		}
	}()
	fn()
}

func expectErrorPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if p := recover(); p == nil {
			t.Fatal("expected error panic")
		}
	}()
	fn()
}

func TestRateLimitWrapperReferenceReplay(t *testing.T) {
	type oldBucket struct{ count, resetAt int64 }
	oldStep := func(bucket *oldBucket, now, limit, window int64) (RateVerdict, *oldBucket) {
		if bucket == nil || bucket.resetAt <= now {
			verdict := RateVerdict{OK: true, Remaining: limit - 1, Bucket: RateBucket{Count: 1, ResetAt: now + window}}
			return verdict, &oldBucket{count: 1, resetAt: now + window}
		}
		if bucket.count >= limit {
			verdict := RateVerdict{Bucket: RateBucket{Count: bucket.count, ResetAt: bucket.resetAt}}
			return verdict, bucket
		}
		verdict := RateVerdict{OK: true, Remaining: limit - bucket.count - 1, Bucket: RateBucket{Count: bucket.count + 1, ResetAt: bucket.resetAt}}
		return verdict, &oldBucket{count: bucket.count + 1, resetAt: bucket.resetAt}
	}
	r := rand.New(rand.NewSource(42))
	for run := 0; run < 200; run++ {
		limit, window := int64(1+r.Intn(5)), int64(1+r.Intn(50))
		now := int64(1_700_000_000_000 + r.Intn(1_000))
		var current *RateBucket
		var old *oldBucket
		for step := 0; step < 30; step++ {
			now += int64(r.Intn(10))
			got := RateLimitStep(current, now, limit, window)
			want, nextOld := oldStep(old, now, limit, window)
			if got != want {
				t.Fatalf("run %d step %d: got %#v want %#v", run, step, got, want)
			}
			current, old = &got.Bucket, nextOld
		}
	}
	if RateLimitStep(nil, 1_000, 0, 60_000).OK {
		t.Fatal("limit zero admitted request")
	}
	for _, tc := range [][3]int64{{1_000, -1, 60_000}, {-1, 60, 60_000}, {NatLimit, 60, 60_000}, {1_000, 60, 0}} {
		t.Run("invalid", func(t *testing.T) {
			expectRangePanic(t, func() { RateLimitStep(nil, tc[0], tc[1], tc[2]) })
		})
	}
	if !RateLimitStep(nil, NatLimit-60_001, 60, 60_000).OK {
		t.Fatal("valid near-limit request denied")
	}
	expectErrorPanic(t, func() { RateLimitStep(nil, NatLimit-1_000, 60, 1_000) })
}

func oldApplyBy(term string, year int) time.Time {
	switch term {
	case "summer":
		return time.Date(year, 4, 30, 23, 59, 59, 0, time.UTC)
	case "fall":
		return time.Date(year, 8, 31, 23, 59, 59, 0, time.UTC)
	case "winter", "spring":
		return time.Date(year-1, 12, 31, 23, 59, 59, 0, time.UTC)
	default:
		return time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)
	}
}

func oldFresh(termYears []TermYearLike, anchor, now time.Time) bool {
	if len(termYears) != 0 {
		for _, ty := range termYears {
			if !oldApplyBy(ty.Term, ty.Year).Before(now) {
				return true
			}
		}
		return false
	}
	return now.Sub(anchor) <= time.Duration(MaxAgeDaysNoTerm)*24*time.Hour
}

func TestFreshnessWrapperReferenceReplay(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	terms := []string{"summer", "fall", "winter", "spring"}
	for i := 0; i < 2_000; i++ {
		termYears := make([]TermYearLike, r.Intn(3))
		for j := range termYears {
			termYears[j] = TermYearLike{Term: terms[r.Intn(len(terms))], Year: 2024 + r.Intn(4)}
		}
		now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(r.Intn(3*365))*24*time.Hour + time.Duration(r.Int63n(int64(24*time.Hour))))
		anchor := now.Add(-time.Duration(r.Intn(300)-50) * 24 * time.Hour)
		facts := FreshnessFacts{TermYears: termYears, Anchor: anchor, Now: now}
		want := oldFresh(termYears, anchor, now)
		if IsFresh(facts) != want || IsStale(facts) == want || IsPublic(0, facts) || IsPublic(int64(1+r.Intn(3)), facts) != want {
			t.Fatalf("freshness mismatch at case %d", i)
		}
	}
	for _, tc := range []struct {
		term string
		year int
		iso  string
	}{{"summer", 2026, "2026-04-30T23:59:59Z"}, {"fall", 2026, "2026-08-31T23:59:59Z"}, {"winter", 2027, "2026-12-31T23:59:59Z"}, {"spring", 2027, "2026-12-31T23:59:59Z"}, {"autumnal", 2026, "2026-12-31T23:59:59Z"}, {"constructor", 2026, "2026-12-31T23:59:59Z"}} {
		if got := ApplyByDate(TermYearLike{Term: tc.term, Year: tc.year}).Format(time.RFC3339); got != tc.iso {
			t.Errorf("%s: got %s want %s", tc.term, got, tc.iso)
		}
	}
	end := ApplyByDate(TermYearLike{Term: "summer", Year: 2026})
	if !IsFresh(FreshnessFacts{TermYears: []TermYearLike{{Term: "summer", Year: 2026}}, Anchor: end, Now: end}) {
		t.Fatal("term not fresh at final instant")
	}
	if !IsStale(FreshnessFacts{TermYears: []TermYearLike{{Term: "summer", Year: 2026}}, Anchor: end, Now: end.Add(time.Millisecond)}) {
		t.Fatal("term not stale one millisecond later")
	}
	expectRangePanic(t, func() { IsFresh(FreshnessFacts{Anchor: time.UnixMilli(-1), Now: time.Now()}) })
	expectRangePanic(t, func() { IsFresh(FreshnessFacts{Anchor: time.UnixMilli(NatLimit), Now: time.Now()}) })
	seen := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if IsStaleByLastSeen(seen, seen.Add(14*24*time.Hour)) || !IsStaleByLastSeen(seen, seen.Add(14*24*time.Hour+time.Millisecond)) {
		t.Fatal("last-seen boundary mismatch")
	}
}

func TestApplyPickerWrapperReferenceGrid(t *testing.T) {
	for postings := int64(0); postings < 5; postings++ {
		for locations := int64(0); locations < 5; locations++ {
			want := postings > 1 || postings == 1 && locations > 1
			if NeedsPicker(postings, locations) != want || SaysOpenings(postings, locations) != (locations < postings) {
				t.Fatalf("postings=%d locations=%d", postings, locations)
			}
		}
		if QualifySingleTitle(postings) != (postings > 1) {
			t.Fatalf("qualify %d", postings)
		}
	}
	expectRangePanic(t, func() { NeedsPicker(-1, 0) })
}

func TestDeactivationWrapperReferenceGrid(t *testing.T) {
	runs := []BoardRun{BoardRunFailed, BoardRunEmpty, BoardRunListed, BoardRunExpired}
	for _, run := range runs {
		for prior := int64(0); prior < 6; prior++ {
			for _, seen := range allSightings {
				want := run == BoardRunExpired || run == BoardRunEmpty && prior+1 >= 3 || run == BoardRunListed && (seen == JobSightingGone || seen == JobSightingRejected)
				if ShouldRetireJob(run, prior, seen) != want {
					t.Fatalf("run=%s prior=%d seen=%s", run, prior, seen)
				}
			}
			wantNext := prior
			if run == BoardRunEmpty {
				wantNext++
			} else if run == BoardRunListed {
				wantNext = 0
			}
			if NextEmptyRuns(run, prior) != wantNext {
				t.Fatalf("next run=%s prior=%d", run, prior)
			}
		}
	}
	expectRangePanic(t, func() { ShouldRetireJob("lisetd", 0, JobSightingGone) })
	expectRangePanic(t, func() { ShouldRetireJob(BoardRunListed, 0, "constructor") })
	expectRangePanic(t, func() { ShouldRetireJob(BoardRunListed, -1, JobSightingGone) })
}

func TestFunnelWrapperReferenceCases(t *testing.T) {
	got := SumFunnels([]FunnelCounts{
		{Fetched: int64ptr(10), Title: int64ptr(5), Tech: int64ptr(3), Upserted: int64ptr(2)},
		{Title: int64ptr(1), Upserted: int64ptr(4), DescriptionMissing: int64ptr(7)},
	})
	want := FunnelTotals{Fetched: 10, Title: 6, Tech: 3, Upserted: 6, DescriptionMissing: 7}
	if got != want || SumFunnels(nil) != (FunnelTotals{}) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for text, expected := range map[string]bool{"zero_match: board had 12 jobs": true, "funnel fetched=1": false, "": false, "zero_matc": false} {
		if IsZeroMatchRun(text) != expected {
			t.Errorf("zero match %q", text)
		}
	}
	expectRangePanic(t, func() { SumFunnels([]FunnelCounts{{Fetched: int64ptr(NatLimit)}}) })
	expectErrorPanic(t, func() {
		SumFunnels([]FunnelCounts{{Fetched: int64ptr(NatLimit / 2)}, {Fetched: int64ptr(NatLimit / 2)}})
	})
}

func TestPaginationWrapperReferenceCases(t *testing.T) {
	cases := []struct {
		raw  []string
		page int64
		lim  int64
	}{{nil, 1, 27}, {[]string{"3"}, 3, 3}, {[]string{"4", "9"}, 4, 4}, {[]string{" 5 "}, 5, 5}, {[]string{"0"}, 1, 27}, {[]string{"-2"}, 1, 27}, {[]string{"abc"}, 1, 27}, {[]string{"2.5"}, 1, 27}, {[]string{"99999999999999999999"}, 1, 27}, {[]string{"250000"}, 100000, 100}}
	for _, tc := range cases {
		if got := ParsePage(tc.raw...); got != tc.page {
			t.Errorf("page %v=%d want %d", tc.raw, got, tc.page)
		}
		if got := ParseLimit(tc.raw...); got != tc.lim {
			t.Errorf("limit %v=%d want %d", tc.raw, got, tc.lim)
		}
	}
	if PageOffset(1, 27) != 0 || PageOffset(3, 27) != 54 || PageOffset(100000, 100) != 9_999_900 {
		t.Fatal("offset cases failed")
	}
}

func TestJobSlugWrapperReferenceCases(t *testing.T) {
	parses := map[string]*string{
		"software-intern-1a2b3c4d": stringPointer("1a2b3c4d"),
		"x-ABCDEF12":               stringPointer("abcdef12"),
		"-12345678":                stringPointer("12345678"),
		"12345678":                 nil,
		"a-1234567":                nil,
		"a-1234567g":               nil,
		"très-élevé-deadbeef":      stringPointer("deadbeef"),
		"":                         nil,
		"a--deadbeef":              stringPointer("deadbeef"),
		"日本語-0123abcd":             stringPointer("0123abcd"),
	}
	for slug, want := range parses {
		if got := ParseIdPrefix(slug); !reflect.DeepEqual(got, want) {
			t.Errorf("parse %q=%#v want %#v", slug, got, want)
		}
	}
	r := rand.New(rand.NewSource(42))
	hex := "0123456789abcdef"
	bases := []string{"swe-intern", "job", "über-praktikum", "a"}
	for i := 0; i < 300; i++ {
		id := make([]byte, 32)
		for j := range id {
			id[j] = hex[r.Intn(len(hex))]
		}
		uuid := string(id[:8]) + "-" + string(id[8:12]) + "-" + string(id[12:16]) + "-" + string(id[16:20]) + "-" + string(id[20:])
		segment := JobSlugSegment(bases[r.Intn(len(bases))], uuid)
		if got := ParseIdPrefix(segment); got == nil || *got != string(id[:8]) {
			t.Fatalf("roundtrip %d: %q %#v", i, segment, got)
		}
	}
}

func stringPointer(value string) *string { return &value }
