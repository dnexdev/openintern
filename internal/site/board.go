package site

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/proven"
)

// boardPageSize is the number of role families per board page.
const boardPageSize = 27

var regionLabels = map[string]string{
	"remote": "Remote",
	"us":     "US",
	"canada": "Canada",
	"europe": "UK/Europe",
	"other":  "Other",
}

type option struct {
	Value   string
	Label   string
	Checked bool
}

type pageLink struct {
	N       int
	Href    string
	Current bool
	Gap     bool // an ellipsis between page numbers
}

type familyView struct {
	ID          string
	Title       string
	DetailPath  string
	CompanyName string
	CompanySlug string
	Mark        companyMark
	Tier1       bool
	Roles       string
	Term        string
	Region      string
	Duration    string
	Location    string
	Picker      bool
	PickerLabel string
	ApplyRows   []board.ApplyRow
	ApplyURL    string
	JobID       string
	JobIDs      string
	Excerpt     string
	SeenAgo     string
	SeenAt      string
	ReportURL   string
}

type boardView struct {
	Query          string
	Company        string
	CompanyOptions []option
	Roles          []option
	Regions        []option
	Terms          []option
	Durations      []option
	Sort           []option
	HasFilters     bool
	ActiveCount    int
	RoleCount      int
	RegionCount    int
	TermCount      int
	DurationCount  int
	Total          int
	Families       []familyView
	Page           int
	TotalPages     int
	Pages          []pageLink
	Prev, Next     string
	Error          bool
}

func (h *handler) jobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	search := h.store.ParseBoardSearchParams(r.URL.Query())
	page := int(search.Page)
	v := boardView{
		Query:   search.Query,
		Company: search.Company,
		Page:    page,
	}
	result, err := h.store.Query(ctx, board.FamilyQuery{
		Query:     search.Query,
		Company:   search.Company,
		Roles:     search.Roles,
		Regions:   search.Regions,
		Terms:     search.Terms,
		Durations: search.Durations,
		Sort:      search.Sort,
		Limit:     boardPageSize,
		Offset:    int(proven.PageOffset(search.Page, boardPageSize)),
	})
	companies, _, cerr := h.store.CompanyOptions(ctx)
	if err != nil || cerr != nil {
		v.Error = true
	}
	now := h.opts.now()
	if err == nil {
		v.Total = result.Total
		for _, f := range result.Families {
			row := familyRow(f, now, h.jobPath)
			row.Mark = h.companyMark(f.Company.Slug, f.Company.Name)
			v.Families = append(v.Families, row)
		}
	}

	v.CompanyOptions = append(v.CompanyOptions, option{Value: "", Label: "All companies", Checked: search.Company == ""})
	for _, c := range companies {
		v.CompanyOptions = append(v.CompanyOptions, option{Value: c.Slug, Label: c.Name, Checked: c.Slug == search.Company})
	}
	for _, role := range board.RoleOptions {
		v.Roles = append(v.Roles, option{Value: role, Label: role, Checked: slices.Contains(search.Roles, role)})
	}
	for _, region := range board.RegionOptions {
		v.Regions = append(v.Regions, option{Value: region, Label: regionLabels[region], Checked: slices.Contains(search.Regions, region)})
	}
	for _, term := range board.TermOptions {
		v.Terms = append(v.Terms, option{Value: term, Label: capitalize(term), Checked: slices.Contains(search.Terms, term)})
	}
	for _, d := range board.DurationOptions {
		v.Durations = append(v.Durations, option{Value: strconv.Itoa(d), Label: strconv.Itoa(d) + " mo", Checked: slices.Contains(search.Durations, d)})
	}
	for _, s := range []struct{ value, label string }{
		{string(board.SortFirstSeen), "Newest first seen"},
		{string(board.SortPosted), "Newest posted"},
		{string(board.SortPrestige), "Prestige sort"},
	} {
		v.Sort = append(v.Sort, option{Value: s.value, Label: s.label, Checked: string(search.Sort) == s.value})
	}
	v.RoleCount, v.RegionCount, v.TermCount, v.DurationCount = len(search.Roles), len(search.Regions), len(search.Terms), len(search.Durations)
	v.ActiveCount = v.RoleCount + v.RegionCount + v.TermCount + v.DurationCount
	if search.Query != "" {
		v.ActiveCount++
	}
	if search.Company != "" {
		v.ActiveCount++
	}
	v.HasFilters = v.ActiveCount > 0

	v.TotalPages = max(1, (v.Total+boardPageSize-1)/boardPageSize)
	filters := filterValues(search)
	href := func(n int) string {
		q := url.Values{}
		for k, vs := range filters {
			q[k] = vs
		}
		q.Set("page", strconv.Itoa(n))
		return "/jobs?" + encodeOrdered(q)
	}
	v.Pages = pageWindow(page, v.TotalPages, href)
	if page > 1 && page <= v.TotalPages {
		v.Prev = href(page - 1)
	}
	if page < v.TotalPages {
		v.Next = href(page + 1)
	}

	h.render(w, http.StatusOK, cacheDynamic, "jobs", view{
		Meta: meta{
			Title:       "Jobs · OpenIntern",
			Description: "Browse free tech internship listings. No account required. Filter by role, location, term, and company.",
			Canonical:   SiteURL + "/jobs",
		},
		Nav:     "jobs",
		Stale:   h.stale(r),
		Aliases: jobAliasesJS(),
		Data:    v,
	})
}

// filterValues rebuilds the board query string in the reference order: q,
// company, role, region, term, duration, then sort when it is not the default.
func filterValues(s board.BoardSearch) url.Values {
	q := url.Values{}
	if s.Query != "" {
		q.Set("q", s.Query)
	}
	if s.Company != "" {
		q.Set("company", s.Company)
	}
	q["role"] = s.Roles
	q["region"] = s.Regions
	q["term"] = s.Terms
	for _, d := range s.Durations {
		q.Add("duration", strconv.Itoa(d))
	}
	if s.Sort != board.SortFirstSeen && s.Sort != "" {
		q.Set("sort", string(s.Sort))
	}
	return q
}

var queryOrder = []string{"q", "company", "role", "region", "term", "duration", "sort", "page"}

func encodeOrdered(q url.Values) string {
	var parts []string
	for _, k := range queryOrder {
		for _, v := range q[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

// pageWindow lists the first and last page, the two pages either side of the
// current one, and gaps between them.
func pageWindow(current, total int, href func(int) string) []pageLink {
	if total <= 1 {
		return nil
	}
	var out []pageLink
	last := 0
	for n := 1; n <= total; n++ {
		if n != 1 && n != total && (n < current-2 || n > current+2) {
			continue
		}
		if last != 0 && n > last+1 {
			out = append(out, pageLink{Gap: true})
		}
		out = append(out, pageLink{N: n, Href: href(n), Current: n == current})
		last = n
	}
	return out
}

func familyPostings(f board.JobFamily) []board.FamilyPosting {
	out := make([]board.FamilyPosting, 0, len(f.Postings))
	for _, p := range f.Postings {
		out = append(out, board.FamilyPosting{ID: p.ID, Title: p.Title, Location: p.Location, Locations: p.Locations, ApplyURL: p.ApplyURL})
	}
	return out
}

func familyRow(f board.JobFamily, now time.Time, jobPath func(companySlug, title, id string) string) familyView {
	postings := familyPostings(f)
	primary := board.FamilyPosting{}
	if len(postings) > 0 {
		primary = postings[0]
	}
	detail := jobPath(f.Company.Slug, f.Title, primary.ID)
	ids := make([]string, 0, len(postings))
	for _, p := range postings {
		ids = append(ids, p.ID)
	}
	v := familyView{
		ID:          f.RoleFamilyID,
		Title:       f.Title,
		DetailPath:  detail,
		CompanyName: f.Company.Name,
		CompanySlug: f.Company.Slug,
		Tier1:       board.IsTier1Employer(f.Company.Slug),
		Roles:       strings.Join(f.Roles[:min(2, len(f.Roles))], ", "),
		Location:    board.FamilyMetaLabel(f.Title, postings),
		Picker:      board.NeedsApplyPicker(postings),
		PickerLabel: board.FamilyPickerButtonLabel(postings),
		ApplyURL:    primary.ApplyURL,
		JobID:       primary.ID,
		JobIDs:      strings.Join(ids, " "),
		SeenAgo:     timeAgo(f.FirstSeenAt, now),
		SeenAt:      f.FirstSeenAt,
		ReportURL:   board.ReportIssueURL(primary.ID, f.Title, f.Company.Name, primary.ApplyURL, SiteURL+detail),
	}
	if v.Picker {
		v.ApplyRows = board.FamilyApplyRows(f.Title, postings)
	}
	if len(f.Terms) > 0 {
		v.Term = capitalize(f.Terms[0])
	}
	if len(f.Regions) > 0 {
		v.Region = regionLabel(f.Regions[0])
	}
	v.Duration = durationLabel(f.DurationMonths)
	if f.Excerpt != nil {
		v.Excerpt = *f.Excerpt
	}
	return v
}

func regionLabel(r string) string {
	if l, ok := regionLabels[r]; ok {
		return l
	}
	return capitalize(r)
}

// durationLabel prints "4–6 mo" for a contiguous run of months and "3/6 mo"
// otherwise, as the reference board does.
func durationLabel(months []int) string {
	if len(months) == 0 {
		return ""
	}
	m := slices.Clone(months)
	slices.Sort(m)
	contiguous := true
	for i := 1; i < len(m); i++ {
		if m[i] != m[i-1]+1 {
			contiguous = false
		}
	}
	if contiguous && len(m) > 1 {
		return strconv.Itoa(m[0]) + "–" + strconv.Itoa(m[len(m)-1]) + " mo"
	}
	parts := make([]string, len(m))
	for i, n := range m {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, "/") + " mo"
}

// timeAgo matches the reference board's relative first-seen label.
func timeAgo(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return ""
	}
	sec := int(now.Sub(t).Seconds())
	switch {
	case sec < 60:
		return strconv.Itoa(sec) + "s ago"
	case sec < 3600:
		return strconv.Itoa(sec/60) + "m ago"
	case sec < 86400:
		return strconv.Itoa(sec/3600) + "h ago"
	default:
		return strconv.Itoa(sec/86400) + "d ago"
	}
}
