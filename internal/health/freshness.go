package health

import (
	"time"

	"github.com/dnexdev/openintern/internal/proven"
)

// MaxAgeDaysNoTerm is the max age of a job without an explicit term/year.
const MaxAgeDaysNoTerm = proven.MaxAgeDaysNoTerm

// IsFreshJob is isFreshJob (packages/db/src/freshness.ts): the live-job
// freshness predicate shared by the board, API, sitemap, detail pages and
// dumps. Bend-proven (law fr_stale_iff_not_fresh) to be exactly the
// opposite of ingest's staleness. The anchor is postedAt, else firstSeenAt.
func IsFreshJob(termYears []proven.TermYearLike, postedAt *time.Time, firstSeenAt, now time.Time) bool {
	anchor := firstSeenAt
	if postedAt != nil {
		anchor = *postedAt
	}
	return proven.IsFresh(proven.FreshnessFacts{TermYears: termYears, Anchor: anchor, Now: now})
}
