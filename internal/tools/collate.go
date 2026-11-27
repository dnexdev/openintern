package tools

import (
	"unicode"

	"github.com/dnexdev/openintern/internal/unicodex"
)

// The reports sort company names with String.prototype.localeCompare,
// which in Node is ICU's root collation: punctuation and symbols before
// digits before letters, accents a secondary difference, case a tertiary one
// (lowercase first). localeCompare below reproduces that for the names the
// community lists carry: ASCII exactly, Latin letters with accents through
// NFKD, the common typographic punctuation by table, and anything else by
// script group then code point. TestLocaleCompareMatchesNode checks it
// against Node on every name in the recorded lists plus crafted cases.

// asciiOrder is the root-collation order of printable ASCII (probed from
// Node 26 by sorting every printable character); letters follow, case
// ignored at the primary level.
const asciiOrder = " _-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$0123456789"

const weightStep = 100

var asciiWeight = func() [128]int {
	var w [128]int
	for i := 0; i < len(asciiOrder); i++ {
		w[asciiOrder[i]] = (i + 1) * weightStep
	}
	base := (len(asciiOrder) + 1) * weightStep
	for c := 'a'; c <= 'z'; c++ {
		w[c] = base + int(c-'a')*weightStep
		w[c-'a'+'A'] = w[c]
	}
	return w
}()

// typographic places punctuation that NFKD leaves alone next to its ASCII
// relative, as the root collation does.
var typographic = map[rune]int{
	'‐': asciiWeight['-'] + 1, // hyphen
	'‑': asciiWeight['-'] + 2, // non-breaking hyphen
	'‒': asciiWeight['-'] + 3, // figure dash
	'–': asciiWeight['-'] + 4, // en dash
	'—': asciiWeight['-'] + 5, // em dash
	'‘': asciiWeight['\''] + 1,
	'’': asciiWeight['\''] + 2,
	'‚': asciiWeight['\''] + 3,
	'“': asciiWeight['"'] + 1,
	'”': asciiWeight['"'] + 2,
	'„': asciiWeight['"'] + 3,
	'«': asciiWeight['"'] + 4,
	'»': asciiWeight['"'] + 5,
	'·': asciiWeight['.'] + 1,
	'•': asciiWeight['*'] + 1,
	'©': asciiWeight['^'] + 1,
	'®': asciiWeight['^'] + 2,
	'°': asciiWeight['^'] + 3,
	'§': asciiWeight['&'] - 2,
	'¶': asciiWeight['&'] - 1,
}

// Weights outside the ASCII table. Every primary is weight<<21 | sub, so
// characters in one group order by code point (sub) at the primary level.
var (
	mathSymbolWeight  = asciiWeight['+'] - 1 // → ± ×: just before '+'
	otherSymbolWeight = asciiWeight['~'] + 1 // ★ and emoji: after '~', before '$'
	otherPunctWeight  = asciiWeight['$'] - 1
	otherLetterWeight = asciiWeight['z'] + weightStep // other scripts: after Latin
)

// letterLike are Latin letters NFKD does not decompose that the root
// collation sorts as a base letter (plus a secondary mark) or an expansion.
var letterLike = map[rune]struct {
	base string
	mark rune
}{
	'Ø': {"O", 0x0338}, 'ø': {"o", 0x0338},
	'Ł': {"L", 0x0335}, 'ł': {"l", 0x0335},
	'Đ': {"D", 0x0335}, 'đ': {"d", 0x0335},
	'Æ': {"AE", 0}, 'æ': {"ae", 0},
	'Œ': {"OE", 0}, 'œ': {"oe", 0},
	'ß': {"ss", 0},
}

// secondaryOrder is the root collation's order of common accents (acute
// before grave, ...); other marks follow by code point.
var secondaryOrder = func() map[rune]int {
	m := map[rune]int{}
	for i, r := range []rune{0x0301, 0x0300, 0x0306, 0x0302, 0x030C, 0x030A, 0x0308, 0x030B, 0x0303, 0x0307, 0x0335, 0x0338, 0x0327, 0x0328, 0x0304} {
		m[r] = i + 1
	}
	return m
}()

func secondaryWeight(r rune) int {
	if r == 0 {
		return 0
	}
	if w, ok := secondaryOrder[r]; ok {
		return w
	}
	return 100 + int(r)
}

type collElem struct {
	primary   int
	secondary int // weight of the combining mark that follows, 0 for none
	tertiary  int // 0 lowercase, 1 compatibility form, 2 uppercase, 3 both
}

func elem(weight, sub int) collElem { return collElem{primary: weight<<21 | sub} }

func collationElems(s string) []collElem {
	var out []collElem
	addRune := func(r rune, compat bool) {
		var e collElem
		switch {
		case r < 128:
			w := asciiWeight[r]
			if w == 0 {
				return
			}
			e = elem(w, 0)
			if r >= 'A' && r <= 'Z' {
				e.tertiary = 2
			}
		case unicodex.IsCombiningMark(r):
			if n := len(out); n > 0 && out[n-1].secondary == 0 {
				out[n-1].secondary = secondaryWeight(r)
			}
			return
		case typographic[r] != 0:
			e = elem(typographic[r], 0)
		case unicode.IsLetter(r):
			e = elem(otherLetterWeight, int(r))
			if unicode.IsUpper(r) {
				e.tertiary = 2
			}
		case unicode.IsSpace(r):
			e = elem(asciiWeight[' '], 0)
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			return // ignorable
		case unicode.Is(unicode.Sm, r):
			e = elem(mathSymbolWeight, int(r))
		case unicode.IsPunct(r):
			e = elem(otherPunctWeight, int(r))
		default:
			e = elem(otherSymbolWeight, int(r))
		}
		if compat {
			e.tertiary |= 1
		}
		out = append(out, e)
	}
	for _, r := range s {
		if r < 128 {
			addRune(r, false)
			continue
		}
		if ll, ok := letterLike[r]; ok {
			for _, b := range ll.base {
				addRune(b, false)
			}
			if ll.mark != 0 {
				addRune(ll.mark, false)
			}
			continue
		}
		d := unicodex.NFKD(string(r))
		// A compatibility decomposition (ﬁ, Ｆ, Ⅸ) is a tertiary difference;
		// a canonical one (é) is not.
		compat := unicodex.NFC(d) != string(r)
		for _, x := range d {
			addRune(x, compat && !unicodex.IsCombiningMark(x))
		}
	}
	return out
}

// localeCompare is a.localeCompare(b) (root collation, see above).
func localeCompare(a, b string) int {
	ea, eb := collationElems(a), collationElems(b)
	for i := 0; i < len(ea) && i < len(eb); i++ {
		if ea[i].primary != eb[i].primary {
			return sign(ea[i].primary - eb[i].primary)
		}
	}
	if len(ea) != len(eb) {
		return sign(len(ea) - len(eb))
	}
	for i := range ea {
		if ea[i].secondary != eb[i].secondary {
			return sign(ea[i].secondary - eb[i].secondary)
		}
	}
	for i := range ea {
		if ea[i].tertiary != eb[i].tertiary {
			return sign(ea[i].tertiary - eb[i].tertiary)
		}
	}
	return 0
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
