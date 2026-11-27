package safedate

import (
	"math"
	"time"
	"unicode"
	"unicode/utf16"
)

// DateParse is JavaScript's Date.parse(s) as V8 implements it: the ES date
// time string format first, then V8's legacy fallback parser ("Nov 10 2026",
// "2026-11-10 12:00:00 UTC", "11/10/2026", RFC 1123 and the like). ok is
// false where JavaScript returns NaN. Forms without a zone are local time.
//
// ATS feeds send all of these (Recruitee writes "2026-11-10 12:00:00 UTC"),
// and the TypeScript adapters handed them to `new Date(s)`, so the port keeps
// V8's exact rules, quirks included ("Nov 10" is 2001-11-10).
func DateParse(s string) (ms float64, ok bool) {
	return dateParseIn(s, time.Local)
}

const (
	kNone          = math.MaxInt32
	maxSignificant = 9
	maxTimeMs      = 8.64e15
)

type tokenTag int

const (
	tagInvalid tokenTag = iota - 6
	tagUnknown
	tagWhiteSpace
	tagNumber
	tagSymbol
	tagEnd
	// Keyword tags are the keyword types, all >= 0.
)

const (
	kwInvalid tokenTag = iota // an unknown word
	kwMonthName
	kwTimeZoneName
	kwTimeSeparator
	kwAMPM
)

type dateToken struct {
	tag    tokenTag
	length int
	value  int
}

func (t dateToken) isNumber() bool                 { return t.tag == tagNumber }
func (t dateToken) isSymbol(c int) bool            { return t.tag == tagSymbol && t.value == c }
func (t dateToken) isWhiteSpace() bool             { return t.tag == tagWhiteSpace }
func (t dateToken) isEnd() bool                    { return t.tag == tagEnd }
func (t dateToken) isInvalid() bool                { return t.tag == tagInvalid }
func (t dateToken) isKeyword() bool                { return t.tag >= 0 }
func (t dateToken) isFixedLengthNumber(n int) bool { return t.isNumber() && t.length == n }
func (t dateToken) isAsciiSign() bool {
	return t.tag == tagSymbol && (t.value == '-' || t.value == '+')
}
func (t dateToken) asciiSign() int { return 44 - t.value }
func (t dateToken) isKeywordZ() bool {
	return t.tag == kwTimeZoneName && t.length == 1 && t.value == 0
}

var keywords = []struct {
	prefix [3]uint16
	tag    tokenTag
	value  int
}{
	{[3]uint16{'j', 'a', 'n'}, kwMonthName, 1},
	{[3]uint16{'f', 'e', 'b'}, kwMonthName, 2},
	{[3]uint16{'m', 'a', 'r'}, kwMonthName, 3},
	{[3]uint16{'a', 'p', 'r'}, kwMonthName, 4},
	{[3]uint16{'m', 'a', 'y'}, kwMonthName, 5},
	{[3]uint16{'j', 'u', 'n'}, kwMonthName, 6},
	{[3]uint16{'j', 'u', 'l'}, kwMonthName, 7},
	{[3]uint16{'a', 'u', 'g'}, kwMonthName, 8},
	{[3]uint16{'s', 'e', 'p'}, kwMonthName, 9},
	{[3]uint16{'o', 'c', 't'}, kwMonthName, 10},
	{[3]uint16{'n', 'o', 'v'}, kwMonthName, 11},
	{[3]uint16{'d', 'e', 'c'}, kwMonthName, 12},
	{[3]uint16{'a', 'm', 0}, kwAMPM, 0},
	{[3]uint16{'p', 'm', 0}, kwAMPM, 12},
	{[3]uint16{'u', 't', 0}, kwTimeZoneName, 0},
	{[3]uint16{'u', 't', 'c'}, kwTimeZoneName, 0},
	{[3]uint16{'z', 0, 0}, kwTimeZoneName, 0},
	{[3]uint16{'g', 'm', 't'}, kwTimeZoneName, 0},
	{[3]uint16{'c', 'd', 't'}, kwTimeZoneName, -5},
	{[3]uint16{'c', 's', 't'}, kwTimeZoneName, -6},
	{[3]uint16{'e', 'd', 't'}, kwTimeZoneName, -4},
	{[3]uint16{'e', 's', 't'}, kwTimeZoneName, -5},
	{[3]uint16{'m', 'd', 't'}, kwTimeZoneName, -6},
	{[3]uint16{'m', 's', 't'}, kwTimeZoneName, -7},
	{[3]uint16{'p', 'd', 't'}, kwTimeZoneName, -7},
	{[3]uint16{'p', 's', 't'}, kwTimeZoneName, -8},
	{[3]uint16{'t', 0, 0}, kwTimeSeparator, 0},
}

func lookupKeyword(pre [3]uint16, length int) (tokenTag, int) {
	for _, k := range keywords {
		if k.prefix == pre && (length <= 3 || k.tag == kwMonthName) {
			return k.tag, k.value
		}
	}
	return kwInvalid, 0
}

// tokenizer walks UTF-16 code units, as V8 does.
type tokenizer struct {
	s    []uint16
	pos  int
	next dateToken
}

func newTokenizer(s string) *tokenizer {
	t := &tokenizer{s: utf16.Encode([]rune(s))}
	t.next = t.scan()
	return t
}

func (t *tokenizer) ch() int {
	if t.pos < len(t.s) {
		return int(t.s[t.pos])
	}
	return 0
}

func (t *tokenizer) atEnd() bool { return t.pos >= len(t.s) }

func (t *tokenizer) isDigit() bool { c := t.ch(); return c >= '0' && c <= '9' && !t.atEnd() }

func (t *tokenizer) isWhiteSpaceChar() bool {
	if t.atEnd() {
		return false
	}
	c := t.ch()
	switch c {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00A0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	if c >= 0xD800 && c <= 0xDFFF {
		return false
	}
	return unicode.Is(unicode.Zs, rune(c))
}

func (t *tokenizer) isAsciiAlphaOrAbove() bool { return !t.atEnd() && t.ch() >= 'A' }

func (t *tokenizer) scan() dateToken {
	pre := t.pos
	if t.atEnd() {
		return dateToken{tag: tagEnd}
	}
	if t.isDigit() {
		n, i := 0, 0
		for t.ch() == '0' && !t.atEnd() {
			t.pos++
		}
		for t.isDigit() {
			if i < maxSignificant {
				n = n*10 + t.ch() - '0'
			}
			i++
			t.pos++
		}
		return dateToken{tag: tagNumber, length: t.pos - pre, value: n}
	}
	switch c := t.ch(); c {
	case ':', '-', '+', '.', ')':
		t.pos++
		return dateToken{tag: tagSymbol, length: 1, value: c}
	}
	if t.isAsciiAlphaOrAbove() && !t.isWhiteSpaceChar() {
		var buf [3]uint16
		n := 0
		for ; t.isAsciiAlphaOrAbove() && !t.isWhiteSpaceChar(); t.pos, n = t.pos+1, n+1 {
			if n < 3 {
				// V8's AsciiAlphaToLower just sets bit 0x20, on any unit.
				buf[n] = uint16(t.ch() | 0x20)
			}
		}
		tag, value := lookupKeyword(buf, n)
		return dateToken{tag: tag, length: n, value: value}
	}
	if t.isWhiteSpaceChar() {
		for t.isWhiteSpaceChar() {
			t.pos++
		}
		return dateToken{tag: tagWhiteSpace, length: t.pos - pre}
	}
	if t.ch() == '(' {
		balance := 0
		for {
			switch t.ch() {
			case ')':
				balance--
			case '(':
				balance++
			}
			t.pos++
			if !(balance > 0 && !t.atEnd() && t.ch() != 0) {
				break
			}
		}
		return dateToken{tag: tagUnknown}
	}
	t.pos++
	return dateToken{tag: tagUnknown}
}

func (t *tokenizer) Next() dateToken {
	r := t.next
	t.next = t.scan()
	return r
}

func (t *tokenizer) Peek() dateToken { return t.next }

func (t *tokenizer) SkipSymbol(c int) bool {
	if t.next.isSymbol(c) {
		t.Next()
		return true
	}
	return false
}

func between(x, lo, hi int) bool { return x >= lo && x <= hi }
func isMonth(x int) bool         { return between(x, 1, 12) }
func isDay(x int) bool           { return between(x, 1, 31) }
func isHour(x int) bool          { return between(x, 0, 23) }
func isMinute(x int) bool        { return between(x, 0, 59) }
func isSecond(x int) bool        { return between(x, 0, 59) }
func isHour12(x int) bool        { return between(x, 0, 12) }
func isMillisecond(x int) bool   { return between(x, 0, 999) }

type dayComposer struct {
	comp       [3]int
	index      int
	namedMonth int
	isISO      bool
}

func (d *dayComposer) add(n int) bool {
	if d.index < 3 {
		d.comp[d.index] = n
		d.index++
		return true
	}
	return false
}

func (d *dayComposer) write() (year, month, day int, ok bool) {
	if d.index < 1 {
		return 0, 0, 0, false
	}
	for d.index < 3 {
		d.comp[d.index] = 1
		d.index++
	}
	year, month, day = 0, kNone, kNone
	if d.namedMonth == kNone {
		if d.isISO || (d.index == 3 && !isDay(d.comp[0])) {
			year, month, day = d.comp[0], d.comp[1], d.comp[2]
		} else {
			month, day = d.comp[0], d.comp[1]
			if d.index == 3 {
				year = d.comp[2]
			}
		}
	} else {
		month = d.namedMonth
		if d.index == 1 {
			day = d.comp[0]
		} else if !isDay(d.comp[0]) {
			year, day = d.comp[0], d.comp[1]
		} else {
			day, year = d.comp[0], d.comp[1]
		}
	}
	if !d.isISO {
		if between(year, 0, 49) {
			year += 2000
		} else if between(year, 50, 99) {
			year += 1900
		}
	}
	if year < -(1<<30) || year > (1<<30)-1 || !isMonth(month) || !isDay(day) {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

type timeComposer struct {
	comp       [4]int
	index      int
	hourOffset int
}

func (c *timeComposer) isEmpty() bool { return c.index == 0 }
func (c *timeComposer) isExpecting(n int) bool {
	return (c.index == 1 && isMinute(n)) || (c.index == 2 && isSecond(n)) || (c.index == 3 && isMillisecond(n))
}
func (c *timeComposer) add(n int) bool {
	if c.index < 4 {
		c.comp[c.index] = n
		c.index++
		return true
	}
	return false
}
func (c *timeComposer) addFinal(n int) bool {
	if !c.add(n) {
		return false
	}
	for c.index < 4 {
		c.comp[c.index] = 0
		c.index++
	}
	return true
}

func (c *timeComposer) write() (h, m, s, ms int, ok bool) {
	for c.index < 4 {
		c.comp[c.index] = 0
		c.index++
	}
	h, m, s, ms = c.comp[0], c.comp[1], c.comp[2], c.comp[3]
	if c.hourOffset != kNone {
		if !isHour12(h) {
			return 0, 0, 0, 0, false
		}
		h %= 12
		h += c.hourOffset
	}
	if !isHour(h) || !isMinute(m) || !isSecond(s) || !isMillisecond(ms) {
		if h != 24 || m != 0 || s != 0 || ms != 0 {
			return 0, 0, 0, 0, false
		}
	}
	return h, m, s, ms, true
}

type tzComposer struct{ sign, hour, minute int }

func (z *tzComposer) set(offsetHours int) {
	if offsetHours < 0 {
		z.sign = -1
	} else {
		z.sign = 1
	}
	z.hour = offsetHours * z.sign
	z.minute = 0
}
func (z *tzComposer) setSign(sign int) {
	if sign < 0 {
		z.sign = -1
	} else {
		z.sign = 1
	}
}
func (z *tzComposer) isExpecting(n int) bool {
	return z.hour != kNone && z.minute == kNone && isMinute(n)
}
func (z *tzComposer) isUTC() bool   { return z.hour == 0 && z.minute == 0 }
func (z *tzComposer) isEmpty() bool { return z.hour == kNone }

// write returns the offset in seconds, or local=true when no zone was given.
func (z *tzComposer) write() (offset int, local bool, ok bool) {
	if z.sign == kNone {
		return 0, true, true
	}
	if z.hour == kNone {
		z.hour = 0
	}
	if z.minute == kNone {
		z.minute = 0
	}
	total := uint64(uint32(z.hour))*3600 + uint64(uint32(z.minute))*60
	total &= 0xFFFFFFFF
	if total > (1<<30)-1 {
		return 0, false, false
	}
	offset = int(total)
	if z.sign < 0 {
		offset = -offset
	}
	return offset, false, true
}

func readMilliseconds(t dateToken) int {
	n, length := t.value, t.length
	if length < 3 {
		if length == 1 {
			n *= 100
		} else if length == 2 {
			n *= 10
		}
	} else if length > 3 {
		if length > maxSignificant {
			length = maxSignificant
		}
		factor := 1
		for {
			factor *= 10
			length--
			if length <= 3 {
				break
			}
		}
		n /= factor
	}
	return n
}

func parseES5(sc *tokenizer, day *dayComposer, tm *timeComposer, tz *tzComposer) dateToken {
	if sc.Peek().isAsciiSign() {
		sign := sc.Next()
		if !sc.Peek().isFixedLengthNumber(6) {
			return sign
		}
		s := sign.asciiSign()
		year := sc.Next().value
		if s < 0 && year == 0 {
			return sign
		}
		day.add(s * year)
	} else if sc.Peek().isFixedLengthNumber(4) {
		day.add(sc.Next().value)
	} else {
		return sc.Next()
	}
	if sc.SkipSymbol('-') {
		if !sc.Peek().isFixedLengthNumber(2) || !isMonth(sc.Peek().value) {
			return sc.Next()
		}
		day.add(sc.Next().value)
		if sc.SkipSymbol('-') {
			if !sc.Peek().isFixedLengthNumber(2) || !isDay(sc.Peek().value) {
				return sc.Next()
			}
			day.add(sc.Next().value)
		}
	}
	if sc.Peek().tag != kwTimeSeparator {
		if !sc.Peek().isEnd() {
			return sc.Next()
		}
	} else {
		sc.Next()
		if !sc.Peek().isFixedLengthNumber(2) || !between(sc.Peek().value, 0, 24) {
			return dateToken{tag: tagInvalid}
		}
		hourIs24 := sc.Peek().value == 24
		tm.add(sc.Next().value)
		if !sc.SkipSymbol(':') {
			return dateToken{tag: tagInvalid}
		}
		if !sc.Peek().isFixedLengthNumber(2) || !isMinute(sc.Peek().value) || (hourIs24 && sc.Peek().value > 0) {
			return dateToken{tag: tagInvalid}
		}
		tm.add(sc.Next().value)
		if sc.SkipSymbol(':') {
			if !sc.Peek().isFixedLengthNumber(2) || !isSecond(sc.Peek().value) || (hourIs24 && sc.Peek().value > 0) {
				return dateToken{tag: tagInvalid}
			}
			tm.add(sc.Next().value)
			if sc.SkipSymbol('.') {
				if !sc.Peek().isNumber() || (hourIs24 && sc.Peek().value > 0) {
					return dateToken{tag: tagInvalid}
				}
				tm.add(readMilliseconds(sc.Next()))
			}
		}
		if sc.Peek().isKeywordZ() {
			sc.Next()
			tz.set(0)
		} else if sc.Peek().isSymbol('+') || sc.Peek().isSymbol('-') {
			if sc.Next().value == '+' {
				tz.setSign(1)
			} else {
				tz.setSign(-1)
			}
			if sc.Peek().isFixedLengthNumber(4) {
				hm := sc.Next().value
				h, m := hm/100, hm%100
				if !isHour(h) || !isMinute(m) {
					return dateToken{tag: tagInvalid}
				}
				tz.hour, tz.minute = h, m
			} else {
				if !sc.Peek().isFixedLengthNumber(2) || !isHour(sc.Peek().value) {
					return dateToken{tag: tagInvalid}
				}
				tz.hour = sc.Next().value
				if !sc.SkipSymbol(':') {
					return dateToken{tag: tagInvalid}
				}
				if !sc.Peek().isFixedLengthNumber(2) || !isMinute(sc.Peek().value) {
					return dateToken{tag: tagInvalid}
				}
				tz.minute = sc.Next().value
			}
		}
		if !sc.Peek().isEnd() {
			return dateToken{tag: tagInvalid}
		}
	}
	if tz.isEmpty() && tm.isEmpty() {
		tz.set(0)
	}
	day.isISO = true
	return dateToken{tag: tagEnd}
}

func dateParseIn(str string, loc *time.Location) (float64, bool) {
	sc := newTokenizer(str)
	day := &dayComposer{namedMonth: kNone}
	tm := &timeComposer{hourOffset: kNone}
	tz := &tzComposer{sign: kNone, hour: kNone, minute: kNone}

	next := parseES5(sc, day, tm, tz)
	if next.isInvalid() {
		return 0, false
	}
	hasReadNumber := day.index > 0
	for token := next; !token.isEnd(); token = sc.Next() {
		switch {
		case token.isNumber():
			hasReadNumber = true
			n := token.value
			if sc.SkipSymbol(':') {
				if sc.SkipSymbol(':') {
					if !tm.isEmpty() {
						return 0, false
					}
					tm.add(n)
					tm.add(0)
				} else {
					if !tm.add(n) {
						return 0, false
					}
					if sc.Peek().isSymbol('.') {
						sc.Next()
					}
				}
			} else if sc.SkipSymbol('.') && tm.isExpecting(n) {
				tm.add(n)
				if !sc.Peek().isNumber() {
					return 0, false
				}
				ms := readMilliseconds(sc.Next())
				if ms < 0 {
					return 0, false
				}
				tm.addFinal(ms)
			} else if tz.isExpecting(n) {
				tz.minute = n
			} else if tm.isExpecting(n) {
				tm.addFinal(n)
				p := sc.Peek()
				if !p.isEnd() && !p.isWhiteSpace() && !p.isKeywordZ() && !p.isAsciiSign() {
					return 0, false
				}
			} else {
				if !day.add(n) {
					return 0, false
				}
				sc.SkipSymbol('-')
			}
		case token.isKeyword():
			if token.tag == kwAMPM && !tm.isEmpty() {
				tm.hourOffset = token.value
			} else if token.tag == kwMonthName {
				day.namedMonth = token.value
				sc.SkipSymbol('-')
			} else if token.tag == kwTimeZoneName && hasReadNumber {
				tz.set(token.value)
			} else {
				if hasReadNumber {
					return 0, false
				}
				if sc.Peek().isNumber() {
					return 0, false
				}
			}
		case token.isAsciiSign() && (tz.isUTC() || !tm.isEmpty()):
			tz.setSign(token.asciiSign())
			n, length := 0, 0
			if sc.Peek().isNumber() {
				nt := sc.Next()
				length, n = nt.length, nt.value
			}
			hasReadNumber = true
			if sc.Peek().isSymbol(':') {
				tz.hour = n
				tz.minute = kNone
			} else if length == 2 || length == 1 {
				tz.hour, tz.minute = n, 0
			} else if length == 4 || length == 3 {
				tz.hour, tz.minute = n/100, n%100
			} else {
				return 0, false
			}
		case (token.isAsciiSign() || token.isSymbol(')')) && hasReadNumber:
			return 0, false
		}
	}

	year, month, dd, ok := day.write()
	if !ok {
		return 0, false
	}
	h, m, s, ms, ok := tm.write()
	if !ok {
		return 0, false
	}
	offset, local, ok := tz.write()
	if !ok {
		return 0, false
	}
	date := makeDate(year, month-1, dd, h, m, s, ms)
	if math.IsNaN(date) {
		return 0, false
	}
	if local {
		// DateCache::kMaxTimeBeforeUTCInMs = kMaxTimeInMs + 10 days.
		if date < -maxTimeMs-864e6 || date > maxTimeMs+864e6 {
			return 0, false
		}
		date = toUTC(date, loc)
	} else {
		date -= float64(offset) * 1000
	}
	if math.Abs(date) > maxTimeMs {
		return 0, false
	}
	return date + 0, true
}

// makeDate is ECMAScript MakeDate(MakeDay(y, m, d), MakeTime(h, min, s, ms))
// for the integer components the parser produces.
func makeDate(year, month, day, h, m, s, ms int) float64 {
	y := float64(year) + math.Floor(float64(month)/12)
	mn := month % 12
	if mn < 0 {
		mn += 12
	}
	if math.Abs(y) > 400000 {
		return math.NaN()
	}
	days := daysFromCivil(int64(y), int64(mn)+1, 1) + int64(day) - 1
	t := float64(h)*3600000 + float64(m)*60000 + float64(s)*1000 + float64(ms)
	return float64(days)*864e5 + t
}

// daysFromCivil counts days from 1970-01-01 to y-m-d (proleptic Gregorian).
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	var era int64
	if y >= 0 {
		era = y / 400
	} else {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// toUTC treats a wall-clock millisecond count as local time in loc.
func toUTC(wall float64, loc *time.Location) float64 {
	if loc == time.UTC || loc == nil {
		return wall
	}
	w := int64(wall)
	sec := w / 1000
	msPart := w % 1000
	if msPart < 0 {
		msPart += 1000
		sec--
	}
	u := time.Unix(sec, 0).UTC()
	t := time.Date(u.Year(), u.Month(), u.Day(), u.Hour(), u.Minute(), u.Second(), 0, loc)
	return float64(t.Unix()*1000 + msPart)
}
