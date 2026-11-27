package proven

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

type expectedCapture struct {
	Value json.RawMessage `json:"value"`
	Error *struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

type actualCapture struct {
	value any
	err   *struct {
		name    string
		message string
	}
}

func callCaptured(fn func() any) (got actualCapture) {
	defer func() {
		if p := recover(); p != nil {
			name := "Error"
			if _, ok := p.(*RangeError); ok {
				name = "RangeError"
			}
			message := fmt.Sprint(p)
			got.err = &struct {
				name    string
				message string
			}{name: name, message: message}
		}
	}()
	got.value = fn()
	return got
}

func assertCaptured(t *testing.T, want expectedCapture, fn func() any) {
	t.Helper()
	got := callCaptured(fn)
	if want.Error != nil {
		if got.err == nil {
			t.Fatalf("wanted %s %q, got value %#v", want.Error.Name, want.Error.Message, got.value)
		}
		if got.err.name != want.Error.Name || got.err.message != want.Error.Message {
			t.Fatalf("error mismatch: want %s %q, got %s %q", want.Error.Name, want.Error.Message, got.err.name, got.err.message)
		}
		return
	}
	if got.err != nil {
		t.Fatalf("unexpected %s: %s", got.err.name, got.err.message)
	}
	var wantValue any
	if err := json.Unmarshal(want.Value, &wantValue); err != nil {
		t.Fatalf("decode expected value: %v", err)
	}
	encoded, err := json.Marshal(got.value)
	if err != nil {
		t.Fatalf("encode actual value: %v", err)
	}
	var gotValue any
	if err := json.Unmarshal(encoded, &gotValue); err != nil {
		t.Fatalf("decode actual value: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("value mismatch: want %s, got %s", want.Value, encoded)
	}
}

func TestProvenModelVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		Constants struct {
			NatLimit             int64 `json:"NAT_LIMIT"`
			DayMS                int64 `json:"DAY_MS"`
			MaxAgeDaysNoTerm     int64 `json:"MAX_AGE_DAYS_NO_TERM"`
			LastSeenStaleDays    int64 `json:"LAST_SEEN_STALE_DAYS"`
			RetireAfterEmptyRuns int64 `json:"RETIRE_AFTER_EMPTY_RUNS"`
			StrikesToRetire      int64 `json:"STRIKES_TO_RETIRE"`
			StrikeGapHours       int64 `json:"STRIKE_GAP_HOURS"`
		} `json:"constants"`
		RateLimit []struct {
			Prev     *RateBucket `json:"prev"`
			Now      int64       `json:"now"`
			Limit    int64       `json:"limit"`
			WindowMS int64       `json:"windowMs"`
			expectedCapture
		} `json:"rateLimit"`
		ApplyBy []struct {
			Input TermYearLike `json:"input"`
			ISO   string       `json:"iso"`
			MS    int64        `json:"ms"`
		} `json:"applyBy"`
		Freshness []struct {
			Input struct {
				TermYears  []TermYearLike `json:"termYears"`
				Anchor     int64          `json:"anchor"`
				Now        int64          `json:"now"`
				MaxAgeDays *int64         `json:"maxAgeDays"`
			} `json:"input"`
			Fresh   expectedCapture `json:"fresh"`
			Stale   expectedCapture `json:"stale"`
			Public0 expectedCapture `json:"public0"`
			Public2 expectedCapture `json:"public2"`
		} `json:"freshness"`
		StaleLastSeen []struct {
			Seen    int64  `json:"seen"`
			Now     int64  `json:"now"`
			MaxDays *int64 `json:"maxDays"`
			expectedCapture
		} `json:"staleLastSeen"`
		Picker []struct {
			Postings  int64           `json:"postings"`
			Locations int64           `json:"locations"`
			Needs     expectedCapture `json:"needs"`
			Qualify   expectedCapture `json:"qualify"`
			Openings  expectedCapture `json:"openings"`
		} `json:"picker"`
		Deactivation []struct {
			Run    BoardRun        `json:"run"`
			Prior  int64           `json:"prior"`
			Seen   JobSighting     `json:"seen"`
			Retire expectedCapture `json:"retire"`
			Next   expectedCapture `json:"next"`
		} `json:"deactivation"`
		ApplyStrikes []struct {
			Count   int64           `json:"count"`
			Due     bool            `json:"due"`
			Verdict ApplyVerdict    `json:"verdict"`
			Next    expectedCapture `json:"next"`
			Retired expectedCapture `json:"retired"`
		} `json:"applyStrikes"`
		Funnels []struct {
			Funnels []FunnelCounts `json:"funnels"`
			expectedCapture
		} `json:"funnels"`
		ZeroMatch []struct {
			Error *string `json:"error"`
			Value bool    `json:"value"`
		} `json:"zeroMatch"`
		Pagination []struct {
			Raw     json.RawMessage `json:"raw"`
			IsArray bool            `json:"isArray"`
			Page    int64           `json:"page"`
			Limit   int64           `json:"limit"`
		} `json:"pagination"`
		Offsets []struct {
			Page  int64 `json:"page"`
			Limit int64 `json:"limit"`
			expectedCapture
		} `json:"offsets"`
		SlugSegments []struct {
			Base  string `json:"base"`
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"slugSegments"`
		SlugParses []struct {
			Slug  string  `json:"slug"`
			Value *string `json:"value"`
		} `json:"slugParses"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}

	if vectors.Constants.NatLimit != NatLimit || vectors.Constants.DayMS != DayMS ||
		vectors.Constants.MaxAgeDaysNoTerm != MaxAgeDaysNoTerm ||
		vectors.Constants.LastSeenStaleDays != LastSeenStaleDays ||
		vectors.Constants.RetireAfterEmptyRuns != RetireAfterEmptyRuns ||
		vectors.Constants.StrikesToRetire != StrikesToRetire ||
		vectors.Constants.StrikeGapHours != StrikeGapHours {
		t.Fatalf("constant mismatch: %#v", vectors.Constants)
	}

	for i, tc := range vectors.RateLimit {
		t.Run(fmt.Sprintf("rate/%d", i), func(t *testing.T) {
			assertCaptured(t, tc.expectedCapture, func() any { return RateLimitStep(tc.Prev, tc.Now, tc.Limit, tc.WindowMS) })
		})
	}
	for i, tc := range vectors.ApplyBy {
		t.Run(fmt.Sprintf("apply-by/%d", i), func(t *testing.T) {
			got := ApplyByDate(tc.Input)
			if got.UnixMilli() != tc.MS || got.Format("2006-01-02T15:04:05.000Z") != tc.ISO {
				t.Fatalf("want %s/%d, got %s/%d", tc.ISO, tc.MS, got.Format(time.RFC3339Nano), got.UnixMilli())
			}
		})
	}
	for i, tc := range vectors.Freshness {
		t.Run(fmt.Sprintf("freshness/%d", i), func(t *testing.T) {
			facts := FreshnessFacts{TermYears: tc.Input.TermYears, Anchor: time.UnixMilli(tc.Input.Anchor).UTC(), Now: time.UnixMilli(tc.Input.Now).UTC(), MaxAgeDays: tc.Input.MaxAgeDays}
			assertCaptured(t, tc.Fresh, func() any { return IsFresh(facts) })
			assertCaptured(t, tc.Stale, func() any { return IsStale(facts) })
			assertCaptured(t, tc.Public0, func() any { return IsPublic(0, facts) })
			assertCaptured(t, tc.Public2, func() any { return IsPublic(2, facts) })
		})
	}
	for i, tc := range vectors.StaleLastSeen {
		t.Run(fmt.Sprintf("last-seen/%d", i), func(t *testing.T) {
			fn := func() any {
				if tc.MaxDays == nil {
					return IsStaleByLastSeen(time.UnixMilli(tc.Seen), time.UnixMilli(tc.Now))
				}
				return IsStaleByLastSeen(time.UnixMilli(tc.Seen), time.UnixMilli(tc.Now), *tc.MaxDays)
			}
			assertCaptured(t, tc.expectedCapture, fn)
		})
	}
	for i, tc := range vectors.Picker {
		t.Run(fmt.Sprintf("picker/%d", i), func(t *testing.T) {
			assertCaptured(t, tc.Needs, func() any { return NeedsPicker(tc.Postings, tc.Locations) })
			assertCaptured(t, tc.Qualify, func() any { return QualifySingleTitle(tc.Locations) })
			assertCaptured(t, tc.Openings, func() any { return SaysOpenings(tc.Postings, tc.Locations) })
		})
	}
	for i, tc := range vectors.Deactivation {
		t.Run(fmt.Sprintf("deactivation/%d", i), func(t *testing.T) {
			assertCaptured(t, tc.Retire, func() any { return ShouldRetireJob(tc.Run, tc.Prior, tc.Seen) })
			assertCaptured(t, tc.Next, func() any { return NextEmptyRuns(tc.Run, tc.Prior) })
		})
	}
	if len(vectors.ApplyStrikes) == 0 {
		t.Fatal("no applyStrikes vectors")
	}
	for i, tc := range vectors.ApplyStrikes {
		t.Run(fmt.Sprintf("applyStrikes/%d", i), func(t *testing.T) {
			assertCaptured(t, tc.Next, func() any { return NextStrikes(tc.Count, tc.Due, tc.Verdict) })
			assertCaptured(t, tc.Retired, func() any { return StrikesRetired(tc.Count) })
		})
	}
	for i, tc := range vectors.Funnels {
		t.Run(fmt.Sprintf("funnels/%d", i), func(t *testing.T) {
			assertCaptured(t, tc.expectedCapture, func() any { return SumFunnels(tc.Funnels) })
		})
	}
	for i, tc := range vectors.ZeroMatch {
		text := ""
		if tc.Error != nil {
			text = *tc.Error
		}
		if got := IsZeroMatchRun(text); got != tc.Value {
			t.Errorf("zero-match/%d: want %v, got %v", i, tc.Value, got)
		}
	}
	for i, tc := range vectors.Pagination {
		var values []string
		if tc.IsArray {
			if err := json.Unmarshal(tc.Raw, &values); err != nil {
				t.Fatalf("pagination/%d: %v", i, err)
			}
		} else if string(tc.Raw) != "null" {
			var value string
			if err := json.Unmarshal(tc.Raw, &value); err != nil {
				t.Fatalf("pagination/%d: %v", i, err)
			}
			values = []string{value}
		}
		if got := ParsePage(values...); got != tc.Page {
			t.Errorf("pagination/%d page: want %d, got %d", i, tc.Page, got)
		}
		if got := ParseLimit(values...); got != tc.Limit {
			t.Errorf("pagination/%d limit: want %d, got %d", i, tc.Limit, got)
		}
	}
	for i, tc := range vectors.Offsets {
		t.Run(fmt.Sprintf("offset/%d", i), func(t *testing.T) {
			assertCaptured(t, tc.expectedCapture, func() any { return PageOffset(tc.Page, tc.Limit) })
		})
	}
	for i, tc := range vectors.SlugSegments {
		if got := JobSlugSegment(tc.Base, tc.ID); got != tc.Value {
			t.Errorf("segment/%d: want %q, got %q", i, tc.Value, got)
		}
	}
	for i, tc := range vectors.SlugParses {
		got := ParseIdPrefix(tc.Slug)
		if !reflect.DeepEqual(got, tc.Value) {
			t.Errorf("slug parse/%d: want %#v, got %#v", i, tc.Value, got)
		}
	}
}
