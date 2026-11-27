package board

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

func jobDetail(job model.DumpJob) *JobDetail {
	return &JobDetail{
		ID:                job.ID,
		Title:             job.Title,
		Locations:         nonNilStrings(job.Locations),
		ApplyURL:          job.ApplyURL,
		Excerpt:           job.Excerpt,
		Terms:             nonNilStrings(job.Terms),
		TermYears:         nonNilTermYears(job.TermYears),
		DurationMonths:    nonNilInts(job.DurationMonths),
		CohortYear:        job.CohortYear,
		Roles:             nonNilStrings(job.Roles),
		Regions:           nonNilStrings(job.Regions),
		IsRemote:          job.IsRemote,
		IsActive:          true,
		Source:            job.Source,
		PostedAt:          job.PostedAt,
		FirstSeenAt:       job.FirstSeenAt,
		LastSeenAt:        job.LastSeenAt,
		CompanyName:       job.Company.Name,
		CompanySlug:       job.Company.Slug,
		CompanyATS:        job.Company.ATS,
		CompanyWebsiteURL: job.Company.WebsiteURL,
		CompanyCareersURL: job.Company.CareersURL,
	}
}

// Job finds a public job ID case-insensitively, as the API does.
func (s *Store) Job(ctx context.Context, id string) (*JobDetail, DataSource, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return nil, DataSource{}, err
	}
	for _, job := range jobs {
		if strings.EqualFold(job.ID, id) {
			return jobDetail(job), dataSource(dump), nil
		}
	}
	return nil, dataSource(dump), nil
}

// JobByCompanyAndIDPrefix resolves the eight-hex suffix used in page URLs. It
// returns nil when no job or more than one job matches.
func (s *Store) JobByCompanyAndIDPrefix(ctx context.Context, companySlug, idPrefix string) (*JobDetail, DataSource, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return nil, DataSource{}, err
	}
	prefix := strings.ToLower(idPrefix)
	var match *JobDetail
	for _, job := range jobs {
		if job.Company.Slug != companySlug {
			continue
		}
		id := strings.ReplaceAll(strings.ToLower(job.ID), "-", "")
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		if match != nil {
			return nil, dataSource(dump), nil
		}
		match = jobDetail(job)
	}
	return match, dataSource(dump), nil
}

func compareCompanyName(a, b string) bool {
	al, bl := strings.ToLower(a), strings.ToLower(b)
	if al != bl {
		return al < bl
	}
	return a < b
}

// Companies returns companies.json when available. Older releases are
// represented by the companies visible in jobs.json.
func (s *Store) Companies(ctx context.Context, activeOnly bool) ([]Company, DataSource, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return nil, DataSource{}, err
	}
	counts := make(map[string]int)
	for _, job := range jobs {
		counts[job.Company.Slug]++
	}
	registry, available := s.companiesDump(ctx)
	rows := make([]Company, 0)
	if available {
		for _, company := range registry {
			if activeOnly && !company.Active {
				continue
			}
			rows = append(rows, Company{
				ID:         company.ID,
				Name:       company.Name,
				Slug:       company.Slug,
				ATS:        company.ATS,
				CareersURL: company.CareersURL,
				WebsiteURL: company.WebsiteURL,
				Active:     company.Active,
				ActiveJobs: counts[company.Slug],
			})
		}
	} else {
		bySlug := make(map[string]*Company)
		for _, job := range jobs {
			company := bySlug[job.Company.Slug]
			if company == nil {
				company = &Company{
					ID:         job.Company.Slug,
					Name:       job.Company.Name,
					Slug:       job.Company.Slug,
					ATS:        job.Company.ATS,
					CareersURL: nil,
					WebsiteURL: nil,
					Active:     true,
				}
				bySlug[job.Company.Slug] = company
				rows = append(rows, *company)
			}
			company.ActiveJobs++
			for i := range rows {
				if rows[i].Slug == company.Slug {
					rows[i].ActiveJobs = company.ActiveJobs
					break
				}
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return compareCompanyName(rows[i].Name, rows[j].Name) })
	if rows == nil {
		rows = []Company{}
	}
	return rows, dataSource(dump), nil
}

func (s *Store) Company(ctx context.Context, slug string) (*Company, DataSource, error) {
	rows, source, err := s.Companies(ctx, false)
	if err != nil {
		return nil, DataSource{}, err
	}
	slug = s.CanonicalCompanySlug(slug)
	for i := range rows {
		if rows[i].Slug == slug {
			company := rows[i]
			return &company, source, nil
		}
	}
	return nil, source, nil
}

func (s *Store) CompanyOptions(ctx context.Context) ([]CompanyOption, DataSource, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return nil, DataSource{}, err
	}
	bySlug := make(map[string]string)
	for _, job := range jobs {
		bySlug[job.Company.Slug] = job.Company.Name
	}
	options := make([]CompanyOption, 0, len(bySlug))
	for slug, name := range bySlug {
		options = append(options, CompanyOption{Slug: slug, Name: name})
	}
	sort.SliceStable(options, func(i, j int) bool { return compareCompanyName(options[i].Name, options[j].Name) })
	if options == nil {
		options = []CompanyOption{}
	}
	return options, dataSource(dump), nil
}

func (s *Store) CorpusStats(ctx context.Context) (CorpusStats, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return CorpusStats{}, err
	}
	slugs := make(map[string]struct{})
	for _, job := range jobs {
		slugs[job.Company.Slug] = struct{}{}
	}
	generatedAt := dump.GeneratedAt
	return CorpusStats{
		ActiveCompanies: len(slugs),
		LastIngest:      &generatedAt,
		DataSource:      dataSource(dump),
	}, nil
}

var previewUSCities = []string{
	"San Francisco", "New York", "Seattle", "Austin", "Boston", "Los Angeles",
	"Chicago", "Denver", "Portland", "San Jose", "Palo Alto", "Mountain View",
	"Cupertino", "Redmond", "Menlo Park", "Washington",
}

var previewOtherCities = []string{"Toronto", "Vancouver", "Montreal", "Ottawa", "Waterloo", "Calgary"}
var heroPrimarySlugs = []string{"openai", "cloudflare", "notion", "palantir", "figma", "databricks"}
var heroBackupSlugs = []string{"anthropic", "airbnb", "stripe", "anduril", "scale-ai", "jump-trading"}

func jobCard(job model.DumpJob) JobCard {
	return JobCard{
		ID:                job.ID,
		Title:             job.Title,
		Locations:         nonNilStrings(job.Locations),
		ApplyURL:          job.ApplyURL,
		Excerpt:           job.Excerpt,
		Terms:             nonNilStrings(job.Terms),
		TermYears:         nonNilTermYears(job.TermYears),
		DurationMonths:    nonNilInts(job.DurationMonths),
		Roles:             nonNilStrings(job.Roles),
		Regions:           nonNilStrings(job.Regions),
		IsRemote:          job.IsRemote,
		Source:            job.Source,
		PostedAt:          job.PostedAt,
		FirstSeenAt:       job.FirstSeenAt,
		CompanyName:       job.Company.Name,
		CompanySlug:       job.Company.Slug,
		CompanyWebsiteURL: job.Company.WebsiteURL,
		CompanyCareersURL: job.Company.CareersURL,
	}
}

func locationBlob(job JobCard) string {
	locations := append([]string(nil), job.Locations...)
	if job.IsRemote {
		locations = append(locations, "Remote")
	}
	return strings.ToLower(strings.Join(locations, " "))
}

func previewLocationScore(job JobCard) int {
	blob := locationBlob(job)
	for _, city := range previewUSCities {
		if strings.Contains(blob, strings.ToLower(city)) {
			return 4
		}
	}
	for _, region := range job.Regions {
		if region == "us" {
			return 3
		}
	}
	if job.IsRemote || strings.Contains(blob, "remote") {
		return 2
	}
	for _, city := range previewOtherCities {
		if strings.Contains(blob, strings.ToLower(city)) {
			return 1
		}
	}
	return 0
}

func PreviewLocationLabel(job JobCard) string {
	blob := locationBlob(job)
	for _, city := range append(append([]string(nil), previewUSCities...), previewOtherCities...) {
		if strings.Contains(blob, strings.ToLower(city)) {
			return city
		}
	}
	if job.IsRemote || strings.Contains(blob, "remote") {
		return "Remote"
	}
	if len(job.Locations) > 0 && strings.TrimSpace(job.Locations[0]) != "" {
		return strings.TrimSpace(job.Locations[0])
	}
	return "Remote"
}

func (s *Store) HeroPreview(ctx context.Context) (HeroPreview, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return HeroPreview{}, err
	}
	allowed := make(map[string]struct{})
	for _, slug := range append(append([]string(nil), heroPrimarySlugs...), heroBackupSlugs...) {
		allowed[slug] = struct{}{}
	}
	bySlug := make(map[string]JobCard)
	for _, raw := range jobs {
		if _, ok := allowed[raw.Company.Slug]; !ok {
			continue
		}
		card := jobCard(raw)
		previous, exists := bySlug[raw.Company.Slug]
		if !exists || previewLocationScore(card) > previewLocationScore(previous) {
			bySlug[raw.Company.Slug] = card
		}
	}
	preview := make([]JobCard, 0, 6)
	for _, slug := range heroPrimarySlugs {
		if card, ok := bySlug[slug]; ok {
			preview = append(preview, card)
		}
	}
	for _, slug := range heroBackupSlugs {
		if len(preview) >= 6 {
			break
		}
		if card, ok := bySlug[slug]; ok {
			preview = append(preview, card)
		}
	}
	return HeroPreview{Preview: preview, Total: len(jobs), DataSource: dataSource(dump)}, nil
}

func (s *Store) DumpStatus(ctx context.Context) (DumpStatus, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return DumpStatus{}, err
	}
	companies := make(map[string]struct{})
	for _, job := range jobs {
		companies[job.Company.Slug] = struct{}{}
	}
	return DumpStatus{
		GeneratedAt:     dump.GeneratedAt,
		ActiveJobs:      len(jobs),
		ActiveCompanies: len(companies),
	}, nil
}

func (s *Store) SitemapEntries(ctx context.Context, limit int) ([]SitemapEntry, error) {
	if s.jobPath == nil {
		return nil, errors.New("board: job path dependency is not available")
	}
	jobs, _, err := s.liveJobs(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 0 || limit > len(jobs) {
		limit = len(jobs)
	}
	entries := make([]SitemapEntry, 0, limit)
	for _, job := range jobs[:limit] {
		entries = append(entries, SitemapEntry{
			Path:         s.jobPath(job.Company.Slug, job.Title, job.ID),
			LastModified: job.LastSeenAt,
		})
	}
	return entries, nil
}

func (s *Store) StaleData(ctx context.Context) StaleState {
	dump, err := s.jobsDump(ctx)
	if err != nil {
		return StaleState{}
	}
	generated, ok := parseTimestamp(dump.GeneratedAt)
	if !ok {
		return StaleState{Available: true, Stale: true, GeneratedAt: dump.GeneratedAt}
	}
	age := s.now().Sub(generated)
	return StaleState{Available: true, Stale: age > MaxDumpAge, GeneratedAt: dump.GeneratedAt, Age: age}
}

func (s *Store) CanonicalCompanySlug(slug string) string {
	return s.canonicalCompanySlug(slug)
}

func (s *Store) JobAlias(id string) string {
	return s.jobAlias(id)
}

func (s *Store) JobAliasByPrefix(prefix string) string {
	return s.jobAliasByPrefix(prefix)
}

func FormatIngestAge(iso string, now time.Time) string {
	if iso == "" {
		return ""
	}
	then, ok := parseTimestamp(iso)
	if !ok {
		return ""
	}
	seconds := int(now.Sub(then).Seconds())
	switch {
	case seconds < 60:
		return "just now"
	case seconds < 3600:
		return strconv.Itoa(seconds/60) + "m ago"
	case seconds < 86400:
		return strconv.Itoa(seconds/3600) + "h ago"
	default:
		return strconv.Itoa(seconds/86400) + "d ago"
	}
}

func formEscape(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "~", "%7E")
	return strings.ReplaceAll(escaped, "%2A", "*")
}

func ReportIssueURL(jobID, title, companyName, applyURL, pageURL string) string {
	issueTitle := truncateUTF16("Listing: "+companyName+" — "+title, 120)
	parts := []string{
		"template=" + formEscape("listing.yml"),
		"title=" + formEscape(issueTitle),
		"url=" + formEscape(pageURL+" (job "+jobID+")"),
		"details=" + formEscape("Apply URL: "+applyURL+"\n\n"),
	}
	return "https://github.com/dnexdev/openintern/issues/new?" + strings.Join(parts, "&")
}
