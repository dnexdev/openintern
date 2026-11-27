// Package safedate coerces ATS timestamps into the range the freshness rules
// accept. It never returns a time whose Unix millisecond is outside
// [0, 2^48).
package safedate

import (
	"encoding/json"
	"math"
	"time"
)

// MaxMS is the latest instant the freshness code accepts (2^48 ms, year 10889).
const MaxMS int64 = 1 << 48

// Parse matches safe-date.ts: nil and "" are nil, numbers below 1e12 are
// seconds, strings are read like `new Date(s)`, and anything outside
// [1970, 2^48 ms) is nil.
func Parse(value any) *time.Time {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		return ParseString(v)
	case float64:
		return parseNumber(v)
	case float32:
		return parseNumber(float64(v))
	case int:
		return parseNumber(float64(v))
	case int64:
		return parseNumber(float64(v))
	case int32:
		return parseNumber(float64(v))
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return nil
		}
		return parseNumber(f)
	case time.Time:
		return inRange(v)
	case *time.Time:
		if v == nil {
			return nil
		}
		return inRange(*v)
	default:
		return nil
	}
}

func parseNumber(v float64) *time.Time {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	ms := v
	if v < 1e12 {
		ms = v * 1000
	}
	return ParseMillis(ms)
}

// ParseString is `safeDate(new Date(value))` for a string: V8's Date.parse
// (see DateParse), then the [0, 2^48) range check. Forms without a zone are
// local time, as in JavaScript.
func ParseString(value string) *time.Time {
	ms, ok := DateParse(value)
	if !ok {
		return nil
	}
	return ParseMillis(ms)
}

// ParseMillis is `safeDate(new Date(ms))`: no seconds heuristic, the value
// is truncated to an integer like TimeClip, then range-checked.
func ParseMillis(ms float64) *time.Time {
	if math.IsNaN(ms) || math.IsInf(ms, 0) || math.Abs(ms) > 8.64e15 {
		return nil
	}
	return inRange(time.UnixMilli(int64(math.Trunc(ms))).UTC())
}

func inRange(t time.Time) *time.Time {
	ms := t.UnixMilli()
	if ms < 0 || ms >= MaxMS {
		return nil
	}
	u := t.UTC()
	return &u
}

// FormatISO formats t like Date.toISOString (millisecond precision, Z).
func FormatISO(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}

// ISOPtr formats a parsed instant, or nil.
func ISOPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := FormatISO(*t)
	return &s
}
