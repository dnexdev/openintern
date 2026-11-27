// Package htmlx extracts plain text from HTML and markdown cell noise.
// Output is for matching and excerpts. It is not a browser XSS sanitizer
// and is never re-embedded as HTML.
package htmlx

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dnexdev/openintern/internal/unicodex"
)

// JSSpace is the body of a character class matching JavaScript's \s
// (Go's \s is ASCII only). Use it as "["+JSSpace+"]".
const JSSpace = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var (
	brTag  = regexp.MustCompile(`(?i)<br[` + JSSpace + `]*/?>`)
	closeP = regexp.MustCompile(`(?i)</p>`)
	anyTag = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	apos   = regexp.MustCompile(`&#0?39;`)
)

// Strip removes tags, decodes a few entities and drops a leading run of
// emoji. &amp; is decoded last so "&amp;quot;" stays "&quot;".
func Strip(html string) string {
	s := brTag.ReplaceAllString(html, " ")
	s = anyTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = apos.ReplaceAllString(s, "'")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = stripLeadingNoise(s)
	s = collapseJSSpace(s)
	return TrimJS(s)
}

// StripTags turns job-description HTML into plain text. Breaks and paragraphs
// become whitespace; the following collapse matches the TypeScript helper,
// which folds those newlines into spaces.
func StripTags(html string) string {
	s := brTag.ReplaceAllString(html, "\n")
	s = closeP.ReplaceAllString(s, "\n")
	s = anyTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = collapseJSSpace(s)
	return TrimJS(s)
}

// TrimJS is String.prototype.trim: it strips JavaScript whitespace and line
// terminators, which unlike strings.TrimSpace leaves U+0085 alone.
func TrimJS(s string) string {
	return strings.TrimFunc(s, IsJSSpace)
}

// collapseJSSpace folds JavaScript \s+ into a single ASCII space. Other
// bytes are copied as they are, so WTF-8 lone surrogates (kept by the JSON
// layer for JavaScript parity) survive.
func collapseJSSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prev := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if IsJSSpace(r) {
			if !prev {
				b.WriteByte(' ')
				prev = true
			}
		} else {
			prev = false
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// stripLeadingNoise removes the prefix matched by
// /^[\s\p{Extended_Pictographic}\uFE0F\u200D🔥🔒🛂🇺🇸🎓]+/u. Inside a class the
// flag 🇺🇸 is its two code points, U+1F1FA and U+1F1F8; regional indicators are
// not Extended_Pictographic, so those two are listed on their own.
// (\s is JavaScript's Unicode whitespace, not Go's unicode.IsSpace, which
// also matches U+0085).
func stripLeadingNoise(s string) string {
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			break
		}
		if !isLeadingNoise(r) {
			break
		}
		i += size
	}
	return s[i:]
}

func isLeadingNoise(r rune) bool {
	if IsJSSpace(r) || r == 0xFE0F || r == 0x200D || r == 0x1F1FA || r == 0x1F1F8 {
		return true
	}
	return unicodex.IsExtendedPictographic(r)
}

// IsJSSpace reports JavaScript \s with the unicode flag: WhiteSpace or a
// line terminator. U+0085 is deliberately excluded.
func IsJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00A0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	return unicode.Is(unicode.Zs, r)
}
