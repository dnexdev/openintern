package state

import "unicode/utf8"

// asciiPrimary is the primary collation weight of each printable ASCII
// character in the root locale that Node's String.prototype.localeCompare
// uses (ICU, punctuation non-ignorable): space, then punctuation and
// symbols in the order below, then digits, then letters with case ignored.
// Probed from Node 26 by sorting every printable ASCII character.
var asciiPrimary = func() [128]int {
	var w [128]int
	order := " _-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$0123456789"
	for i := 0; i < len(order); i++ {
		w[order[i]] = i + 1
	}
	for c := 'a'; c <= 'z'; c++ {
		w[c] = len(order) + 1 + int(c-'a')
		w[c-'a'+'A'] = w[c]
	}
	return w
}()

// collationKey yields the primary weight and case bit of each character of
// s, skipping characters ICU ignores (ASCII control characters). Characters
// outside ASCII sort after letters by code point: an approximation of the
// root collation that is exact for the ASCII strings it is used on
// (timestamps, slugs).
func collationKey(s string) (primary []int, upper []bool) {
	for _, r := range s {
		switch {
		case r < 128 && asciiPrimary[r] == 0:
			continue
		case r < 128:
			primary = append(primary, asciiPrimary[r])
			upper = append(upper, r >= 'A' && r <= 'Z')
		case r == utf8.RuneError:
			primary = append(primary, 1000+0xfffd)
			upper = append(upper, false)
		default:
			primary = append(primary, 1000+int(r))
			upper = append(upper, false)
		}
	}
	return primary, upper
}

// LocaleCompare is a.localeCompare(b) as Node computes it for ASCII
// strings: primary weights first, then case (lowercase first), and equal
// when only ignorable characters differ.
func LocaleCompare(a, b string) int {
	pa, ua := collationKey(a)
	pb, ub := collationKey(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	if len(pa) != len(pb) {
		if len(pa) < len(pb) {
			return -1
		}
		return 1
	}
	for i := range ua {
		if ua[i] != ub[i] {
			if ub[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
