package site

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/dnexdev/openintern/data"
	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/jobslug"
	"github.com/dnexdev/openintern/internal/registry"
)

var (
	aliasOnce sync.Once
	aliasJS   template.JS
)

// jobAliasesJS is the old-id → new-id map the Applied toggle uses to move
// marks when a company merge replaces a job, as JSON for an inline script.
func jobAliasesJS() template.JS {
	aliasOnce.Do(func() {
		aliases := map[string]string{}
		if text, err := fs.ReadFile(data.FS, "job-aliases.tsv"); err == nil {
			if parsed, err := registry.ParseJobAliases(string(text)); err == nil {
				aliases = parsed
			}
		}
		b, _ := json.Marshal(aliases) // escapes <, > and & for a script context
		aliasJS = template.JS(b)
	})
	return aliasJS
}

func (h *handler) stale(r *http.Request) *stale {
	s := h.store.StaleData(r.Context())
	if !s.Available || !s.Stale {
		return nil
	}
	return &stale{Age: board.FormatIngestAge(s.GeneratedAt, h.opts.now())}
}

type previewRow struct {
	Company    string
	Mark       companyMark
	Title      string
	Location   string
	Roles      string
	DetailPath string
}

type homeView struct {
	Error      bool
	Total      int
	Companies  int
	IngestAge  string
	Preview    []previewRow
	CurlSample string
}

const curlSample = `curl "https://openintern.dev/api/v1/jobs?role=software&limit=5"`

func (h *handler) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	v := homeView{CurlSample: curlSample}
	hero, err := h.store.HeroPreview(ctx)
	stats, serr := h.store.CorpusStats(ctx)
	if err != nil || serr != nil {
		v.Error = true
	} else {
		v.Total = hero.Total
		v.Companies = stats.ActiveCompanies
		if stats.LastIngest != nil {
			v.IngestAge = board.FormatIngestAge(*stats.LastIngest, h.opts.now())
		}
		for _, job := range hero.Preview {
			v.Preview = append(v.Preview, previewRow{
				Company:    job.CompanyName,
				Mark:       h.companyMark(job.CompanySlug, job.CompanyName),
				Title:      job.Title,
				Location:   board.PreviewLocationLabel(job),
				Roles:      strings.Join(job.Roles[:min(2, len(job.Roles))], ", "),
				DetailPath: h.jobPath(job.CompanySlug, job.Title, job.ID),
			})
		}
	}
	h.render(w, http.StatusOK, cachePublic, "home", view{
		Meta:  meta{Title: defaultTitle, Description: defaultDescription},
		Stale: h.stale(r),
		Data:  v,
	})
}

var uuidPattern = regexp.MustCompile(`(?i)^[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)

// legacyJob redirects old /jobs/{uuid} links to the canonical job URL.
func (h *handler) legacyJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("company")
	if !uuidPattern.MatchString(id) {
		h.notFound(w, r)
		return
	}
	job, _, err := h.store.Job(r.Context(), id)
	if err != nil {
		h.errorPage(w, r)
		return
	}
	if job == nil {
		if replacement := h.store.JobAlias(id); replacement != "" {
			http.Redirect(w, r, "/jobs/"+replacement, http.StatusPermanentRedirect)
			return
		}
		h.notFound(w, r)
		return
	}
	http.Redirect(w, r, h.jobPath(job.CompanySlug, job.Title, job.ID), http.StatusTemporaryRedirect)
}

type fact struct {
	Label  string
	Values []string
}

type jobView struct {
	ID         string
	Title      string
	Company    string
	CompanyURL string
	Mark       companyMark
	Facts      []fact
	Excerpt    string
	ApplyURL   string
	ReportURL  string
}

var allowedDurations = []int{3, 4, 6, 8, 12}

func (h *handler) jobDetail(w http.ResponseWriter, r *http.Request) {
	company, jobSlug := r.PathValue("company"), r.PathValue("jobSlug")
	prefix := jobslug.ParseIdPrefixFromJobSlug(jobSlug)
	if prefix == nil {
		h.notFound(w, r)
		return
	}
	// Merged or renamed companies keep their old links working.
	if canonical := h.store.CanonicalCompanySlug(company); canonical != company {
		http.Redirect(w, r, "/jobs/"+canonical+"/"+jobSlug, http.StatusPermanentRedirect)
		return
	}
	job, _, err := h.store.JobByCompanyAndIDPrefix(r.Context(), company, *prefix)
	if err != nil {
		h.errorPage(w, r)
		return
	}
	if job == nil {
		if replacement := h.store.JobAliasByPrefix(*prefix); replacement != "" {
			http.Redirect(w, r, "/jobs/"+replacement, http.StatusPermanentRedirect)
			return
		}
		h.notFound(w, r)
		return
	}

	pagePath := h.jobPath(job.CompanySlug, job.Title, job.ID)
	v := jobView{
		ID:         job.ID,
		Title:      job.Title,
		Company:    job.CompanyName,
		CompanyURL: "/jobs?company=" + job.CompanySlug,
		Mark:       h.companyMark(job.CompanySlug, job.CompanyName),
		ApplyURL:   job.ApplyURL,
		ReportURL:  board.ReportIssueURL(job.ID, job.Title, job.CompanyName, job.ApplyURL, SiteURL+pagePath),
	}
	if job.Excerpt != nil {
		v.Excerpt = *job.Excerpt
	}
	locations := job.Locations
	if len(locations) == 0 {
		locations = []string{"Location n/a"}
	}
	v.Facts = append(v.Facts, fact{"Company", []string{job.CompanyName}}, fact{plural(len(locations), "Location", "Locations"), locations})
	if len(job.Roles) > 0 {
		v.Facts = append(v.Facts, fact{"Roles", []string{strings.Join(job.Roles, ", ")}})
	}
	if len(job.Regions) > 0 {
		regions := make([]string, len(job.Regions))
		for i, r := range job.Regions {
			regions[i] = regionLabel(r)
		}
		v.Facts = append(v.Facts, fact{"Region", []string{strings.Join(regions, ", ")}})
	}
	var terms []string
	if len(job.TermYears) > 0 {
		for _, ty := range job.TermYears {
			terms = append(terms, capitalize(ty.Term)+" "+strconv.Itoa(ty.Year))
		}
	} else {
		for _, t := range job.Terms {
			terms = append(terms, capitalize(t))
		}
	}
	if len(terms) > 0 {
		v.Facts = append(v.Facts, fact{"Term", []string{strings.Join(terms, ", ")}})
	}
	var durations []int
	for _, m := range job.DurationMonths {
		if slices.Contains(allowedDurations, m) && !slices.Contains(durations, m) {
			durations = append(durations, m)
		}
	}
	slices.Sort(durations)
	if len(durations) > 0 {
		parts := make([]string, len(durations))
		for i, d := range durations {
			parts[i] = strconv.Itoa(d)
		}
		v.Facts = append(v.Facts, fact{"Duration", []string{strings.Join(parts, "/") + " months"}})
	}
	v.Facts = append(v.Facts,
		fact{"Source", []string{job.Source}},
		fact{"Posted", []string{formatDate(job.PostedAt)}},
		fact{"First seen", []string{formatDate(&job.FirstSeenAt)}},
	)

	loc := "Location n/a"
	if len(job.Locations) > 0 {
		loc = strings.Join(job.Locations, ", ")
	}
	description := truncateUTF16(v.Excerpt, 160)
	if description == "" {
		description = job.Title + " at " + job.CompanyName + " (" + loc + ") — tech internship on OpenIntern."
	}
	h.render(w, http.StatusOK, cacheDynamic, "job", view{
		Meta: meta{
			Title:         job.Title + " at " + job.CompanyName + " · OpenIntern",
			Description:   description,
			Canonical:     SiteURL + pagePath,
			OGTitle:       job.Title + " at " + job.CompanyName,
			OGDescription: description,
			OGType:        "article",
			OGURL:         SiteURL + pagePath,
		},
		Nav:     "jobs",
		Stale:   h.stale(r),
		Aliases: jobAliasesJS(),
		Data:    v,
	})
}

// jobPath links through the canonical company slug: dumps written before a
// company merge still carry the old slug, and linking it would cost every
// reader a redirect.
func (h *handler) jobPath(companySlug, title, id string) string {
	return jobslug.JobPath(h.store.CanonicalCompanySlug(companySlug), title, id)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func formatDate(iso *string) string {
	if iso == nil || len(*iso) < 10 {
		return "—"
	}
	return (*iso)[:10]
}

func (h *handler) docs(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, cachePublic, "docs", view{
		Meta: meta{
			Title:       "API & dumps · OpenIntern",
			Description: "Public OpenIntern API reference, query parameters, rate limits, and daily JSON/CSV dump downloads.",
			Canonical:   SiteURL + "/docs",
		},
		Nav:  "docs",
		Data: struct{ CurlSample string }{curlSample},
	})
}

func (h *handler) privacy(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, cachePublic, "privacy", view{
		Meta: meta{Title: "Privacy · OpenIntern", Description: defaultDescription},
	})
}

func (h *handler) vsInternInsider(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, cachePublic, "vs", view{
		Meta: meta{
			Title:       "OpenIntern vs Intern Insider · OpenIntern",
			Description: "Compare OpenIntern (free, open-source, public API) with Intern Insider (paid subscription, gated board).",
			Canonical:   SiteURL + "/vs/intern-insider",
		},
	})
}

func (h *handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusNotFound, cachePublic, "notfound", view{
		Meta: meta{Title: "Page not found · OpenIntern", Description: defaultDescription, NoIndex: true},
	})
}

func (h *handler) errorPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "60")
	h.render(w, http.StatusServiceUnavailable, "no-store", "error", view{
		Meta: meta{Title: "Listings unavailable · OpenIntern", Description: defaultDescription, NoIndex: true},
	})
}
