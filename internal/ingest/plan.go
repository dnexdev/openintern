package ingest

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dnexdev/openintern/internal/classify"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/safedate"
)

// jsSpace is JavaScript's \s outside unicode mode.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// internshipTitle is INTERNSHIP_TITLE_ONLY: the title looks like an
// internship or co-op (before the tech filter). The JavaScript regexp is
// case-insensitive without the u flag, so only ASCII letters fold; matching
// an ASCII-lowercased title against a lowercase pattern does the same (Go's
// (?i) would also fold ſ to s). \b and \w are ASCII in both engines.
var internshipTitle = regexp.MustCompile(`\b(intern|internship|co-?op|coop|apprentice(ship)?|project` + jsSpace + `+intern|campus` + jsSpace + `+\w+` + jsSpace + `+intern|year` + jsSpace + `+at` + jsSpace + `+\w+)\b`)

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
}

// IsInternshipTitle is INTERNSHIP_TITLE_ONLY.test(title).
func IsInternshipTitle(title string) bool { return internshipTitle.MatchString(asciiLower(title)) }

// SuspectedBlockMinJobs: a company that had at least this many active jobs
// and now lists none is flagged.
const SuspectedBlockMinJobs = 5

// ExistingJob is a stored job as the plan sees it.
type ExistingJob struct {
	ID            string
	ExternalJobID string
	IsActive      bool
	Fingerprint   string
	FirstSeenAt   time.Time
	PostedAt      *time.Time
}

// JobUpsert is a job this run writes.
type JobUpsert struct {
	CompanyID       string
	ExternalJobID   string
	Fingerprint     string
	Title           string
	NormalizedTitle string
	RoleFamilyID    string
	Locations       []string
	ApplyURL        string
	Excerpt         *string
	Terms           []string
	TermYears       []model.TermYear
	DurationMonths  []int
	CohortYear      *int
	Roles           []string
	Regions         []string
	IsRemote        bool
	IsActive        bool
	Source          string
	PostedAt        *time.Time
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	UpdatedAt       time.Time
}

// ApplyHostViolation is an apply link off the allowlist, one per company,
// host and reason.
type ApplyHostViolation struct {
	Company string
	Host    *string
	Reason  string
	Example string
}

// CompanyFunnel is one company's fetched → title → tech → upsert counts.
type CompanyFunnel struct {
	Slug                                                string
	Fetched, InternshipTitle, TechPass, Upserted, Stale int
}

// RunRecord is the ingest run a plan records.
type RunRecord struct {
	Status   string // "ok" or "error"
	JobCount int
	Error    string
}

// CompanyRunPlan is what one company's run changes.
type CompanyRunPlan struct {
	Upserts             []JobUpsert
	RetireIDs           []string
	Run                 RunRecord
	ZeroMatch           bool
	SuspectedBlock      bool
	StaleDeactivated    int
	ApplyHostViolations []ApplyHostViolation
	Funnel              CompanyFunnel
}

// PlanCompany is the company a plan is for.
type PlanCompany struct{ ID, Slug, ATS string }

// PlanInput is planCompanyRun's input.
type PlanInput struct {
	Company PlanCompany
	// Raw is every board's postings, or nil when Expired.
	Raw []model.NormalizedJob
	// Expired is set when the source is a manual capture past its age limit.
	Expired           *fetch.Failure
	Existing          []ExistingJob
	PriorEmptyRuns    int
	Now               time.Time
	EnforceApplyHosts bool
	// CheckApplyURL is the allowlist check; nil uses the embedded registry.
	CheckApplyURL func(companySlug, url string) registry.ApplyCheck
}

// SightJob is where an existing active job stands after this run's fetch
// (input to proven.ShouldRetireJob).
func SightJob(onBoard *model.NormalizedJob, matched bool) proven.JobSighting {
	if onBoard == nil {
		return proven.JobSightingGone
	}
	if matched {
		return proven.JobSightingMatched
	}
	if jsTrim(onBoard.Description) != "" {
		return proven.JobSightingRejected
	}
	return proven.JobSightingUnverified
}

// safePostedAt is safeDate(j.postedAt) for an adapter's timestamp.
func safePostedAt(s *string) *time.Time {
	if s == nil {
		return nil
	}
	return safedate.ParseString(*s)
}

// PlanCompanyRun is planCompanyRun: what one company's run changes,
// decided without touching storage.
func PlanCompanyRun(in PlanInput) CompanyRunPlan {
	check := in.CheckApplyURL
	if check == nil {
		reg := registry.Default()
		check = reg.CheckApplyURL
	}
	company, raw, now := in.Company, in.Raw, in.Now
	titles, descriptionMissing := 0, 0
	var filtered []model.NormalizedJob
	for _, j := range raw {
		if IsInternshipTitle(j.Title) {
			titles++
		}
		if classify.IsTechInternship(j.Title, j.Description) {
			filtered = append(filtered, j)
		}
		if jsTrim(j.Description) == "" {
			descriptionMissing++
		}
	}
	byFingerprint := map[string]ExistingJob{}
	for _, row := range in.Existing {
		byFingerprint[row.Fingerprint] = row
	}
	matched := map[string]bool{}
	upserts := []JobUpsert{}
	var violations []ApplyHostViolation
	seenViolation := map[string]bool{}
	stale := 0
	maxAge := proven.MaxAgeDaysNoTerm

	for _, j := range filtered {
		if c := check(company.Slug, j.ApplyURL); !c.OK {
			host := "null"
			if c.Host != nil {
				host = *c.Host
			}
			key := host + " " + c.Reason
			if !seenViolation[key] {
				seenViolation[key] = true
				violations = append(violations, ApplyHostViolation{Company: company.Slug, Host: c.Host, Reason: c.Reason, Example: j.ApplyURL})
			}
			// Held: not ingested, and an existing copy is retired as no longer matching.
			if in.EnforceApplyHosts {
				continue
			}
		}
		matched[j.ExternalID] = true
		text := j.Title + " " + j.Description
		isRemote := classify.LooksRemote(j.Locations, j.Title)
		cohortYear := classify.ExtractCohortYear(text, now)
		termYears := classify.ExtractTermYears(text, cohortYear, now)
		normalized := classify.NormalizeTitle(j.Title)
		fingerprint := classify.JobFingerprint(company.ID, company.ATS, j.ExternalID)
		prior, hasPrior := byFingerprint[fingerprint]
		firstSeen := now
		if hasPrior {
			firstSeen = prior.FirstSeenAt
		}
		postedAt := safePostedAt(j.PostedAt)
		if postedAt == nil && hasPrior {
			postedAt = prior.PostedAt
		}
		anchor := firstSeen
		if postedAt != nil {
			anchor = *postedAt
		}
		// Bend-proven to be exactly !isFreshJob (law fr_stale_iff_not_fresh).
		isStale := proven.IsStale(proven.FreshnessFacts{TermYears: termYearsLike(termYears), Anchor: anchor, Now: now, MaxAgeDays: &maxAge})
		if isStale {
			stale++
		}
		excerpt := (*string)(nil)
		if j.Excerpt != nil {
			excerpt = classify.ExcerptFromHtml(*j.Excerpt)
		}
		if excerpt == nil {
			excerpt = classify.ExcerptFromHtml(j.Description)
		}
		durations := classify.ExtractDurationMonthsList(text)
		if durations == nil {
			durations = []int{}
		}
		upserts = append(upserts, JobUpsert{
			CompanyID:       company.ID,
			ExternalJobID:   j.ExternalID,
			Fingerprint:     fingerprint,
			Title:           j.Title,
			NormalizedTitle: normalized,
			RoleFamilyID:    classify.RoleFamilyId(company.Slug, normalized),
			Locations:       j.Locations,
			ApplyURL:        j.ApplyURL,
			Excerpt:         excerpt,
			Terms:           classify.ExtractTerms(text),
			TermYears:       termYears,
			DurationMonths:  durations,
			CohortYear:      cohortYear,
			Roles:           classify.ExtractRoles(j.Title, j.Description),
			Regions:         classify.ExtractRegions(j.Locations, isRemote),
			IsRemote:        isRemote,
			IsActive:        !isStale,
			Source:          company.ATS,
			PostedAt:        postedAt,
			FirstSeenAt:     firstSeen,
			LastSeenAt:      now,
			UpdatedAt:       now,
		})
	}

	// Per-job retirement, Bend-proven (laws dp_*): a job gone from a board that
	// was fetched successfully is retired; one whose details failed to load is
	// kept; an empty board retires everything only on the third empty run.
	run := proven.BoardRunListed
	switch {
	case in.Expired != nil:
		run = proven.BoardRunExpired
	case len(raw) == 0:
		run = proven.BoardRunEmpty
	}
	// new Map(entries) keeps the last entry for a repeated key.
	onBoard := map[string]*model.NormalizedJob{}
	for i := range raw {
		onBoard[raw[i].ExternalID] = &raw[i]
	}
	activeBefore := 0
	retire := []string{}
	for _, row := range in.Existing {
		if !row.IsActive {
			continue
		}
		activeBefore++
		if proven.ShouldRetireJob(run, int64(in.PriorEmptyRuns), SightJob(onBoard[row.ExternalJobID], matched[row.ExternalJobID])) {
			retire = append(retire, row.ID)
		}
	}

	if in.Expired != nil {
		return CompanyRunPlan{
			Upserts:             []JobUpsert{},
			RetireIDs:           retire,
			Run:                 RunRecord{Status: "error", Error: Slice16(in.Expired.Error(), 2000)},
			ApplyHostViolations: []ApplyHostViolation{},
			Funnel:              CompanyFunnel{Slug: company.Slug},
		}
	}

	zeroMatch := len(raw) > 0 && len(filtered) == 0
	msg := "funnel fetched=" + strconv.Itoa(len(raw)) + " title=" + strconv.Itoa(titles) + " tech=" + strconv.Itoa(len(filtered)) +
		" upserted=" + strconv.Itoa(len(upserts)) + " description_missing=" + strconv.Itoa(descriptionMissing)
	if zeroMatch {
		msg = "zero_match: no tech internships classified; " + msg
	}
	if violations == nil {
		violations = []ApplyHostViolation{}
	}
	return CompanyRunPlan{
		Upserts:             upserts,
		RetireIDs:           retire,
		Run:                 RunRecord{Status: "ok", JobCount: len(filtered), Error: msg},
		ZeroMatch:           zeroMatch,
		SuspectedBlock:      run == proven.BoardRunEmpty && activeBefore >= SuspectedBlockMinJobs,
		StaleDeactivated:    stale,
		ApplyHostViolations: violations,
		Funnel: CompanyFunnel{Slug: company.Slug, Fetched: len(raw), InternshipTitle: titles,
			TechPass: len(filtered), Upserted: len(upserts), Stale: stale},
	}
}

func termYearsLike(tys []model.TermYear) []proven.TermYearLike {
	out := make([]proven.TermYearLike, len(tys))
	for i, ty := range tys {
		out[i] = proven.TermYearLike{Term: ty.Term, Year: ty.Year}
	}
	return out
}

// isJSSpace is JavaScript's String.prototype.trim whitespace.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func jsTrim(s string) string { return strings.TrimFunc(s, isJSSpace) }

// Slice16 is s.slice(0, n) in UTF-16 code units. Cutting a surrogate pair
// leaves the lone high surrogate, kept as WTF-8 like the state codec does.
func Slice16(s string, n int) string {
	units := 0
	for i := 0; i < len(s); {
		if units >= n {
			return s[:i]
		}
		// A lone surrogate already in s (WTF-8, ED A0..BF xx) is one unit.
		if s[i] == 0xed && i+2 < len(s) && s[i+1] >= 0xa0 && s[i+1] <= 0xbf {
			units++
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r >= 0x10000 {
			if units+2 > n {
				hi := 0xd800 + (r-0x10000)>>10
				return s[:i] + string([]byte{0xed, 0x80 | byte(hi>>6&0x3f), 0x80 | byte(hi&0x3f)})
			}
			units += 2
		} else {
			units++
		}
		i += size
	}
	return s
}
