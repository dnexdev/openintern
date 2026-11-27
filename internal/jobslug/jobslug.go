// Package jobslug builds and parses public job URL segments.
package jobslug

import (
	"strings"
	"unicode"

	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/unicodex"
)

func sliceUTF16(s string, end int) string {
	if end <= 0 {
		return ""
	}
	units := 0
	for index, r := range s {
		width := 1
		if r > 0xffff {
			width = 2
		}
		if units+width > end {
			return s[:index]
		}
		units += width
		if units == end {
			return s[:index+len(string(r))]
		}
	}
	return s
}

// SlugifyTitle converts a title to the Unicode-aware base used in public URLs.
func SlugifyTitle(title string) string {
	title = strings.ToLower(unicodex.NFKC(title))
	var slug strings.Builder
	slug.Grow(len(title))
	pendingDash := false
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if pendingDash && slug.Len() != 0 {
				slug.WriteByte('-')
			}
			pendingDash = false
			slug.WriteRune(r)
		} else {
			pendingDash = true
		}
	}
	return sliceUTF16(slug.String(), 80)
}

// JobSlugSegment returns the title slug followed by the eight-character id
// prefix. Empty title slugs use "job".
func JobSlugSegment(title, id string) string {
	base := SlugifyTitle(title)
	if base == "" {
		base = "job"
	}
	return proven.JobSlugSegment(base, id)
}

// JobPath returns the public path for one company and job.
func JobPath(companySlug, title, id string) string {
	return "/jobs/" + companySlug + "/" + JobSlugSegment(title, id)
}

// ParseIdPrefixFromJobSlug returns the trailing lowercase id prefix.
func ParseIdPrefixFromJobSlug(jobSlug string) *string {
	return proven.ParseIdPrefix(jobSlug)
}
