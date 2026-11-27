package ats

import (
	"strings"
	"unicode"

	"github.com/dnexdev/openintern/internal/unicodex"
)

// localeCompare approximates String.prototype.localeCompare (ICU root
// collation), used only to sort manual capture ids for stable diffs:
//
//  1. primary: characters compare by group (whitespace < punctuation <
//     symbols < digits < letters < everything else), letters by their
//     lowercase base form with accents removed, others by code point;
//  2. secondary: accents (the decomposed forms);
//  3. tertiary: lowercase before uppercase;
//  4. last, code-unit order, so the sort is total.
//
// For the ASCII ids captures use (digits, slugs, lowercase words) this is
// ICU's order: "a-b" < "ab", "a" < "B" < "b", "10" < "9".
func localeCompare(a, b string) int {
	if c := cmpKeys(primaryKey(a), primaryKey(b)); c != 0 {
		return c
	}
	if c := cmpKeys(runes(unicodex.NFKD(strings.ToLower(a))), runes(unicodex.NFKD(strings.ToLower(b)))); c != 0 {
		return c
	}
	if c := cmpKeys(caseKey(a), caseKey(b)); c != 0 {
		return c
	}
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func runes(s string) []int {
	var out []int
	for _, r := range s {
		out = append(out, int(r))
	}
	return out
}

func primaryKey(s string) []int {
	var out []int
	for _, r := range unicodex.NFKD(s) {
		if unicodex.IsCombiningMark(r) {
			continue
		}
		var group int
		switch {
		case unicode.IsSpace(r):
			group = 1
		case unicode.IsPunct(r):
			group = 2
		case unicode.IsSymbol(r):
			group = 3
		case unicode.IsDigit(r):
			group = 4
		case unicode.IsLetter(r):
			group = 5
			r = unicode.ToLower(r)
		default:
			group = 6
		}
		out = append(out, group<<21|int(r))
	}
	return out
}

func caseKey(s string) []int {
	var out []int
	for _, r := range s {
		if unicode.IsUpper(r) {
			out = append(out, 1)
		} else {
			out = append(out, 0)
		}
	}
	return out
}

func cmpKeys(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}
