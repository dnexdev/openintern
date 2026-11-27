package board

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

type familyAcc struct {
	roleFamilyID    string
	titles          []string
	normalizedTitle string
	company         FamilyCompany
	roles           []string
	regions         []string
	terms           []string
	durationMonths  []int
	excerpt         *string
	source          string
	sortTime        time.Time
	postings        []JobPosting
}

func (s *Store) liveJobs(ctx context.Context) ([]model.DumpJob, *model.JobsDump, error) {
	ks := s.RefreshKillSwitch(ctx)
	dump, err := s.jobsDump(ctx)
	if err != nil {
		return nil, nil, err
	}
	now := s.now()
	jobs := make([]model.DumpJob, 0, len(dump.Jobs))
	for _, job := range dump.Jobs {
		if s.publishable(job, ks) && s.isPublic(job, now) {
			jobs = append(jobs, job)
		}
	}
	return jobs, dump, nil
}

func dataSource(dump *model.JobsDump) DataSource {
	return DataSource{Kind: "dump", GeneratedAt: dump.GeneratedAt}
}

func matchesAnyString(haystack, needles []string) bool {
	for _, needle := range needles {
		for _, value := range haystack {
			if value == needle {
				return true
			}
		}
	}
	return false
}

func matchesAnyInt(haystack, needles []int) bool {
	for _, needle := range needles {
		for _, value := range haystack {
			if value == needle {
				return true
			}
		}
	}
	return false
}

func parseTimestamp(value string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, value)
	return t, err == nil
}

func sortTimestamp(job model.DumpJob, familySort FamilySort) time.Time {
	if familySort == SortPosted && job.PostedAt != nil {
		if t, ok := parseTimestamp(*job.PostedAt); ok {
			return t
		}
	}
	t, _ := parseTimestamp(job.FirstSeenAt)
	return t
}

func filterJobs(jobs []model.DumpJob, opts FamilyQuery) []model.DumpJob {
	query := strings.ToLower(opts.Query)
	filtered := make([]model.DumpJob, 0, len(jobs))
	for _, job := range jobs {
		if query != "" && !strings.Contains(strings.ToLower(job.Title), query) {
			continue
		}
		if opts.Company != "" && job.Company.Slug != opts.Company {
			continue
		}
		if len(opts.Roles) > 0 && !matchesAnyString(job.Roles, opts.Roles) {
			continue
		}
		if len(opts.Regions) > 0 && !matchesAnyString(job.Regions, opts.Regions) {
			continue
		}
		if len(opts.Terms) > 0 && !matchesAnyString(job.Terms, opts.Terms) {
			continue
		}
		if len(opts.Durations) > 0 && !matchesAnyInt(job.DurationMonths, opts.Durations) {
			continue
		}
		if opts.PostedAfter != nil {
			if job.PostedAt == nil {
				continue
			}
			posted, ok := parseTimestamp(*job.PostedAt)
			if !ok || posted.Before(*opts.PostedAfter) {
				continue
			}
		}
		filtered = append(filtered, job)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return sortTimestamp(filtered[i], opts.Sort).After(sortTimestamp(filtered[j], opts.Sort))
	})
	return filtered
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func nonNilInts(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	return append([]int(nil), values...)
}

func nonNilTermYears(values []model.TermYear) []model.TermYear {
	if len(values) == 0 {
		return []model.TermYear{}
	}
	return append([]model.TermYear(nil), values...)
}

func appendUniqueStrings(dst []string, values []string) []string {
	for _, value := range values {
		found := false
		for _, have := range dst {
			if have == value {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, value)
		}
	}
	return dst
}

func appendUniqueInts(dst []int, values []int) []int {
	for _, value := range values {
		found := false
		for _, have := range dst {
			if have == value {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, value)
		}
	}
	sort.Ints(dst)
	return dst
}

func formatPostingLocations(locations []string, remote bool) string {
	filtered := make([]string, 0, len(locations))
	for _, location := range locations {
		if location = strings.TrimSpace(location); location != "" {
			filtered = append(filtered, location)
		}
	}
	if len(filtered) > 0 {
		return strings.Join(filtered, " · ")
	}
	if remote {
		return "Remote"
	}
	return "Location n/a"
}

func familyKey(job model.DumpJob) string {
	if job.RoleFamilyID != "" {
		return job.RoleFamilyID
	}
	return job.Company.Slug + ":" + job.ID
}

func normalizedTitle(job model.DumpJob) string {
	if job.NormalizedTitle != "" {
		return job.NormalizedTitle
	}
	return job.Title
}

func familiesFromJobs(jobs []model.DumpJob, opts FamilyQuery) []familyAcc {
	byKey := make(map[string]*familyAcc)
	families := make([]*familyAcc, 0, len(jobs))
	for _, job := range jobs {
		key := familyKey(job)
		acc := byKey[key]
		if acc == nil {
			acc = &familyAcc{
				roleFamilyID:    key,
				normalizedTitle: normalizedTitle(job),
				company: FamilyCompany{
					Name:       job.Company.Name,
					Slug:       job.Company.Slug,
					ATS:        job.Company.ATS,
					WebsiteURL: job.Company.WebsiteURL,
					CareersURL: job.Company.CareersURL,
				},
				roles:          nonNilStrings(job.Roles),
				regions:        nonNilStrings(job.Regions),
				terms:          nonNilStrings(job.Terms),
				durationMonths: nonNilInts(job.DurationMonths),
				excerpt:        job.Excerpt,
				source:         job.Source,
				postings:       []JobPosting{},
			}
			byKey[key] = acc
			families = append(families, acc)
		}
		acc.titles = append(acc.titles, job.Title)
		acc.roles = appendUniqueStrings(acc.roles, job.Roles)
		acc.regions = appendUniqueStrings(acc.regions, job.Regions)
		acc.terms = appendUniqueStrings(acc.terms, job.Terms)
		acc.durationMonths = appendUniqueInts(acc.durationMonths, job.DurationMonths)
		t := sortTimestamp(job, opts.Sort)
		if t.After(acc.sortTime) {
			acc.sortTime = t
		}
		if acc.excerpt == nil && job.Excerpt != nil {
			acc.excerpt = job.Excerpt
		}
		acc.postings = append(acc.postings, JobPosting{
			ID:          job.ID,
			Title:       job.Title,
			Location:    formatPostingLocations(job.Locations, job.IsRemote),
			Locations:   nonNilStrings(job.Locations),
			PostedAt:    job.PostedAt,
			ApplyURL:    job.ApplyURL,
			IsRemote:    job.IsRemote,
			FirstSeenAt: job.FirstSeenAt,
		})
	}
	out := make([]familyAcc, len(families))
	for i, family := range families {
		out[i] = *family
	}
	return out
}

func diversifyByCompany(sorted []familyAcc) []familyAcc {
	queues := make(map[string][]familyAcc)
	order := make([]string, 0)
	for _, family := range sorted {
		slug := family.company.Slug
		if _, ok := queues[slug]; !ok {
			order = append(order, slug)
		}
		queues[slug] = append(queues[slug], family)
	}
	out := make([]familyAcc, 0, len(sorted))
	for len(out) < len(sorted) {
		progressed := false
		for _, slug := range order {
			queue := queues[slug]
			if len(queue) == 0 {
				continue
			}
			out = append(out, queue[0])
			queues[slug] = queue[1:]
			progressed = true
		}
		if !progressed {
			break
		}
	}
	return out
}

func orderFamilies(families []familyAcc, familySort FamilySort) []familyAcc {
	sort.SliceStable(families, func(i, j int) bool {
		return families[i].sortTime.After(families[j].sortTime)
	})
	if familySort != SortPrestige {
		return diversifyByCompany(families)
	}
	prestigious := make([]familyAcc, 0, len(families))
	rest := make([]familyAcc, 0, len(families))
	for _, family := range families {
		if isTier1Employer(family.company.Slug) {
			prestigious = append(prestigious, family)
		} else {
			rest = append(rest, family)
		}
	}
	return append(diversifyByCompany(prestigious), diversifyByCompany(rest)...)
}

func displayTitle(titles []string, normalized string) string {
	if len(titles) == 0 {
		return normalized
	}
	best := titles[0]
	for _, title := range titles[1:] {
		if len(title) < len(best) {
			best = title
		}
	}
	return best
}

func isoMilliseconds(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Query filters, groups, sorts and paginates the public job corpus.
func (s *Store) Query(ctx context.Context, opts FamilyQuery) (FamilyPage, error) {
	jobs, dump, err := s.liveJobs(ctx)
	if err != nil {
		return FamilyPage{}, err
	}
	if opts.Sort != SortPosted && opts.Sort != SortPrestige {
		opts.Sort = SortFirstSeen
	}
	if opts.Limit < 0 {
		opts.Limit = 0
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	families := orderFamilies(familiesFromJobs(filterJobs(jobs, opts), opts), opts.Sort)
	total := len(families)
	start := min(opts.Offset, total)
	end := total
	if opts.Limit >= 0 {
		end = min(start+opts.Limit, total)
	}
	page := make([]JobFamily, 0, end-start)
	for _, family := range families[start:end] {
		page = append(page, JobFamily{
			RoleFamilyID:   family.roleFamilyID,
			Title:          displayTitle(family.titles, family.normalizedTitle),
			Company:        family.company,
			Roles:          nonNilStrings(family.roles),
			Regions:        nonNilStrings(family.regions),
			Terms:          nonNilStrings(family.terms),
			DurationMonths: nonNilInts(family.durationMonths),
			Excerpt:        family.excerpt,
			Source:         family.source,
			FirstSeenAt:    isoMilliseconds(family.sortTime),
			Postings:       append([]JobPosting(nil), family.postings...),
		})
	}
	if page == nil {
		page = []JobFamily{}
	}
	return FamilyPage{Families: page, Total: total, DataSource: dataSource(dump)}, nil
}
