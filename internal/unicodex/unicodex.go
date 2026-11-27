// Package unicodex holds the Unicode operations the standard library lacks:
// NFKC/NFKD normalization (copied golang.org/x/text tables, Unicode 17.0) and
// the Extended_Pictographic property (generated from emoji-data.txt).
//
// These match what the previous TypeScript code got from Node 26
// (String.prototype.normalize and /\p{Extended_Pictographic}/u), so job slugs
// and normalized titles stay byte-identical.
package unicodex

import (
	"sort"
	"unicode"

	"github.com/dnexdev/openintern/internal/third_party/golang.org/x/text/unicode/norm"
)

//go:generate go run ./internal/gen

// NFKC returns s in Normalization Form KC, like s.normalize("NFKC") in JS.
func NFKC(s string) string { return norm.NFKC.String(s) }

// NFKD returns s in Normalization Form KD, like s.normalize("NFKD") in JS.
func NFKD(s string) string { return norm.NFKD.String(s) }

// NFC returns s in Normalization Form C.
func NFC(s string) string { return norm.NFC.String(s) }

// IsExtendedPictographic reports whether r has the Extended_Pictographic
// property, like /\p{Extended_Pictographic}/u in JS.
func IsExtendedPictographic(r rune) bool {
	i := sort.Search(len(pictographicRanges), func(i int) bool { return pictographicRanges[i][1] >= r })
	return i < len(pictographicRanges) && pictographicRanges[i][0] <= r
}

// IsCombiningMark reports whether r is in general category M (Mn, Mc, Me),
// the class JS code strips after NFKD with /\p{M}/gu or /[̀-ͯ]/g.
// Callers porting a narrower JS range must keep that range instead.
func IsCombiningMark(r rune) bool { return unicode.Is(unicode.M, r) }
