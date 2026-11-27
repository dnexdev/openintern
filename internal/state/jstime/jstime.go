// Package jstime handles instants the way the TypeScript pipeline did with
// JavaScript Dates: milliseconds since the epoch, parsed by Date.parse and
// printed by toISOString. Postgres-style state timestamps
// ("2026-11-21T10:00:00.123456+00:00") parse to millisecond precision, as in
// JavaScript, so comparisons and sort orders match the TypeScript exactly.
package jstime

import (
	"strconv"
	"strings"
	"time"
)

// Parse is Date.parse for the ISO 8601 forms V8 accepts: date only
// ("2026-11-21", "2026-11", "2026"; UTC), and date-time with "T", "t" or a
// space, optional seconds and fraction (any number of digits, truncated to
// milliseconds), and an optional "Z" or numeric offset ("+00:00", "+0000",
// "+00"; none means local time). Years may be extended ("+010000"). Days past
// the end of a month roll over, as V8's fallback parser does. Other formats
// (RFC 1123 and the like) are reported as unparseable.
func Parse(s string) (ms int64, ok bool) {
	p := 0
	num := func(n int) (int, bool) {
		if p+n > len(s) {
			return 0, false
		}
		v := 0
		for i := 0; i < n; i++ {
			c := s[p+i]
			if c < '0' || c > '9' {
				return 0, false
			}
			v = v*10 + int(c-'0')
		}
		p += n
		return v, true
	}
	var year int
	switch {
	case len(s) > 0 && (s[0] == '+' || s[0] == '-'):
		neg := s[0] == '-'
		p = 1
		y, ok := num(6)
		if !ok || (neg && y == 0) {
			return 0, false
		}
		year = y
		if neg {
			year = -y
		}
	default:
		y, ok := num(4)
		if !ok {
			return 0, false
		}
		year = y
	}
	month, day := 1, 1
	if p < len(s) && s[p] == '-' {
		p++
		m, ok := num(2)
		if !ok || m < 1 || m > 12 {
			return 0, false
		}
		month = m
		if p < len(s) && s[p] == '-' {
			p++
			d, ok := num(2)
			if !ok || d < 1 || d > 31 {
				return 0, false
			}
			day = d
		}
	}
	if p == len(s) {
		return msOf(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)), true
	}
	if s[p] != 'T' && s[p] != 't' && s[p] != ' ' {
		return 0, false
	}
	p++
	hour, ok := num(2)
	if !ok || p >= len(s) || s[p] != ':' {
		return 0, false
	}
	p++
	minute, ok := num(2)
	if !ok {
		return 0, false
	}
	sec, nanos := 0, 0
	if p < len(s) && s[p] == ':' {
		p++
		if sec, ok = num(2); !ok {
			return 0, false
		}
		if p < len(s) && s[p] == '.' {
			p++
			start := p
			for p < len(s) && s[p] >= '0' && s[p] <= '9' {
				p++
			}
			if p == start {
				return 0, false
			}
			frac := (s[start:p] + "000")[:3]
			f, _ := strconv.Atoi(frac)
			nanos = f * int(time.Millisecond)
		}
	}
	if minute > 59 || sec > 59 || hour > 24 || (hour == 24 && (minute != 0 || sec != 0 || nanos != 0)) {
		return 0, false
	}
	loc := time.Local
	if p < len(s) {
		switch c := s[p]; {
		case c == 'Z' || c == 'z':
			loc = time.UTC
			p++
		case c == '+' || c == '-':
			p++
			oh, ok := num(2)
			if !ok {
				return 0, false
			}
			om := 0
			if p < len(s) && s[p] == ':' {
				p++
				if om, ok = num(2); !ok {
					return 0, false
				}
			} else if p < len(s) {
				if om, ok = num(2); !ok {
					return 0, false
				}
			}
			if oh > 23 || om > 59 {
				return 0, false
			}
			off := oh*3600 + om*60
			if c == '-' {
				off = -off
			}
			loc = time.FixedZone("", off)
		default:
			return 0, false
		}
	}
	if p != len(s) {
		return 0, false
	}
	t := time.Date(year, time.Month(month), day, hour, minute, sec, nanos, loc)
	return msOf(t), true
}

func msOf(t time.Time) int64 { return t.UnixMilli() }

// MaxMS bounds a JavaScript Date: ±8.64e15 ms.
const MaxMS = 8_640_000_000_000_000

// ISO is Date#toISOString: "2026-11-21T10:00:00.000Z", years outside
// 0..9999 as "+010000"/"-000001". ok is false where JavaScript throws.
func ISO(ms int64) (string, bool) {
	if ms > MaxMS || ms < -MaxMS {
		return "", false
	}
	t := time.UnixMilli(ms).UTC()
	var b strings.Builder
	y := t.Year()
	switch {
	case y >= 0 && y <= 9999:
		b.WriteString(pad(y, 4))
	case y < 0:
		b.WriteString("-" + pad(-y, 6))
	default:
		b.WriteString("+" + pad(y, 6))
	}
	b.WriteString("-" + pad(int(t.Month()), 2) + "-" + pad(t.Day(), 2) + "T" + pad(t.Hour(), 2) + ":" +
		pad(t.Minute(), 2) + ":" + pad(t.Second(), 2) + "." + pad(t.Nanosecond()/int(time.Millisecond), 3) + "Z")
	return b.String(), true
}

// MustISO is ISO for instants known to be in range.
func MustISO(ms int64) string {
	s, ok := ISO(ms)
	if !ok {
		panic("jstime: instant out of range: " + strconv.FormatInt(ms, 10))
	}
	return s
}

func pad(n, width int) string {
	s := strconv.Itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// PgTimestamp is pgTimestamp in state.ts: the instant as Postgres row_to_json
// prints a timestamptz in UTC, fraction trimmed of trailing zeros (none when
// whole), offset "+00:00".
func PgTimestamp(ms int64) string {
	iso := MustISO(ms)
	whole, frac, _ := strings.Cut(strings.TrimSuffix(iso, "Z"), ".")
	frac = strings.TrimRight(frac, "0")
	if frac != "" {
		return whole + "." + frac + "+00:00"
	}
	return whole + "+00:00"
}

// FromTime is the JavaScript Date for a Go time (millisecond precision).
func FromTime(t time.Time) int64 { return t.UnixMilli() }

// ToTime converts milliseconds to a UTC time.
func ToTime(ms int64) time.Time { return time.UnixMilli(ms).UTC() }
