package board

import (
	"time"

	"github.com/dnexdev/openintern/internal/jobslug"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/registry"
)

func defaultPublic(job model.DumpJob, now time.Time) (public bool) {
	defer func() {
		if recover() != nil {
			public = false
		}
	}()
	anchor := job.FirstSeenAt
	if job.PostedAt != nil {
		anchor = *job.PostedAt
	}
	anchorTime, err := time.Parse(time.RFC3339Nano, anchor)
	if err != nil {
		return false
	}
	termYears := make([]proven.TermYearLike, len(job.TermYears))
	for i, termYear := range job.TermYears {
		termYears[i] = proven.TermYearLike{Term: termYear.Term, Year: termYear.Year}
	}
	return proven.IsPublic(int64(len(job.Roles)), proven.FreshnessFacts{
		TermYears: termYears,
		Anchor:    anchorTime,
		Now:       now,
	})
}

func (s *Store) fillDefaults() {
	if s.canonicalCompanySlug == nil {
		s.canonicalCompanySlug = registry.CanonicalCompanySlug
	}
	if s.jobAlias == nil {
		s.jobAlias = registry.JobAlias
	}
	if s.jobAliasByPrefix == nil {
		s.jobAliasByPrefix = registry.JobAliasByPrefix
	}
	if s.jobPath == nil {
		s.jobPath = jobslug.JobPath
	}
	if s.applyURLAllowed == nil {
		s.applyURLAllowed = registry.ApplyURLAllowed
	}
	if s.isDelisted == nil {
		s.isDelisted = registry.IsDelisted
	}
	if s.isPublic == nil {
		s.isPublic = defaultPublic
	}
}
