package safedate

import (
	"math"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	if Parse(nil) != nil {
		t.Fatal("nil")
	}
	if Parse("") != nil || Parse("not-a-date") != nil || Parse("nope") != nil {
		t.Fatal("invalid string")
	}
	if Parse(math.NaN()) != nil {
		t.Fatal("NaN")
	}
	if Parse(time.Time{}) != nil {
		t.Fatal("zero time")
	}
	got := Parse(float64(1785740400))
	if got == nil || got.UTC().Year() < 2026 {
		t.Fatalf("seconds: %v", got)
	}
	if FormatISO(*got) != "2026-08-03T07:00:00.000Z" {
		t.Fatalf("seconds iso %s", FormatISO(*got))
	}
	iso := Parse("2026-08-01T00:00:00Z")
	if iso == nil || FormatISO(*iso) != "2026-08-01T00:00:00.000Z" {
		t.Fatalf("iso %v", iso)
	}
	day := Parse("2026-08-01")
	if day == nil || FormatISO(*day) != "2026-08-01T00:00:00.000Z" {
		t.Fatalf("date-only %v", day)
	}
	// Already milliseconds: no extra *1000.
	ms := Parse(float64(1785740400000))
	if ms == nil || FormatISO(*ms) != "2026-08-03T07:00:00.000Z" {
		t.Fatalf("millis %v", ms)
	}
	if ParseMillis(1785740400) == nil || ParseMillis(1785740400).Year() >= 2026 {
		t.Fatal("ParseMillis does not treat small numbers as seconds")
	}
	if Parse(float64(-1)) != nil {
		t.Fatal("negative")
	}
	if Parse(float64(MaxMS)) != nil {
		t.Fatal("upper bound is exclusive")
	}
}
