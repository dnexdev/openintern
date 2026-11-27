package web

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/board"
	"github.com/dnexdev/openintern/internal/proven"
)

const apiCacheControl = "public, s-maxage=3600, stale-while-revalidate=86400"

var apiRoleSet = stringSet(board.RoleOptions)
var apiRegionSet = stringSet(board.RegionOptions)
var apiSeasonSet = stringSet(board.TermOptions)

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func valuesFor(request *http.Request, keys ...string) []string {
	var values []string
	query := request.URL.Query()
	for _, key := range keys {
		for _, value := range query[key] {
			values = append(values, strings.Split(value, ",")...)
		}
	}
	return values
}

func filteredValues(values []string, allowed map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[value]; ok {
			out = append(out, value)
		}
	}
	return out
}

func parseJavaScriptDate(value string) (*time.Time, bool) {
	layouts := []string{time.RFC3339Nano, "2006-01-02", time.RFC1123, time.RFC1123Z}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed, true
		}
	}
	return nil, false
}

func rawFirst(request *http.Request, key string) []string {
	values, ok := request.URL.Query()[key]
	if !ok || len(values) == 0 {
		return nil
	}
	return values[:1]
}

func addDataSourceHeaders(header http.Header, source board.DataSource) {
	if source.Kind == "dump" {
		header.Set("X-Data-Source", "dump")
		header.Set("X-Dump-Generated-At", source.GeneratedAt)
		return
	}
	header.Set("X-Data-Source", source.Kind)
}

func writeAPIUnavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "3600")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "Database unavailable"})
}

type jobsResponse struct {
	Jobs       []apiFamily `json:"jobs"`
	Page       int64       `json:"page"`
	Limit      int64       `json:"limit"`
	Total      int         `json:"total"`
	TotalPages int         `json:"total_pages"`
	HasMore    bool        `json:"has_more"`
}

type apiFamily struct {
	RoleFamilyID string             `json:"role_family_id"`
	Company      apiFamilyCompany   `json:"company"`
	Title        string             `json:"title"`
	Postings     []apiFamilyPosting `json:"postings"`
}

type apiFamilyCompany struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	ATS  string `json:"ats"`
}

type apiFamilyPosting struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Location string  `json:"location"`
	PostedAt *string `json:"posted_at"`
	ApplyURL string  `json:"apply_url"`
}

func (s *server) jobsAPI(w http.ResponseWriter, request *http.Request) {
	decision := s.limiter.step("jobs:" + clientIP(request))
	if !decision.OK {
		now := s.now().UnixMilli()
		retry := int64(math.Ceil(float64(decision.ResetAt-now) / 1000))
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt, 10))
		w.Header().Set("Retry-After", strconv.FormatInt(retry, 10))
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "Rate limit exceeded. Use daily dumps or self-host for bulk access."})
		return
	}

	query := request.URL.Query()
	postedAfterRaw := query.Get("posted_after")
	var postedAfter *time.Time
	if parsed, ok := parseJavaScriptDate(postedAfterRaw); postedAfterRaw != "" && ok {
		postedAfter = parsed
	}
	roles := filteredValues(valuesFor(request, "role"), apiRoleSet)
	regions := filteredValues(valuesFor(request, "region"), apiRegionSet)
	seasonsRaw := valuesFor(request, "season", "term")
	for i, season := range seasonsRaw {
		if strings.ToLower(strings.TrimSpace(season)) == "autumn" {
			seasonsRaw[i] = "fall"
		}
	}
	seasons := filteredValues(seasonsRaw, apiSeasonSet)
	durations := make([]int, 0)
	for _, value := range valuesFor(request, "duration_months", "duration") {
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil && number >= 1 && number <= 24 && number == math.Trunc(number) {
			durations = append(durations, int(number))
		}
	}
	page := proven.ParsePage(rawFirst(request, "page")...)
	limit := proven.ParseLimit(rawFirst(request, "limit")...)
	offset := proven.PageOffset(page, limit)
	familySort := board.SortFirstSeen
	switch query.Get("sort") {
	case "posted":
		familySort = board.SortPosted
	case "prestige":
		familySort = board.SortPrestige
	}
	result, err := s.store.Query(request.Context(), board.FamilyQuery{
		Query:       strings.TrimSpace(query.Get("q")),
		Company:     s.store.CanonicalCompanySlug(strings.TrimSpace(query.Get("company"))),
		Roles:       roles,
		Regions:     regions,
		Terms:       seasons,
		Durations:   durations,
		PostedAfter: postedAfter,
		Sort:        familySort,
		Limit:       int(limit),
		Offset:      int(offset),
	})
	if err != nil {
		writeAPIUnavailable(w)
		return
	}
	families := make([]apiFamily, 0, len(result.Families))
	for _, family := range result.Families {
		postings := make([]apiFamilyPosting, 0, len(family.Postings))
		for _, posting := range family.Postings {
			postings = append(postings, apiFamilyPosting{
				ID: posting.ID, Title: posting.Title, Location: posting.Location,
				PostedAt: posting.PostedAt, ApplyURL: posting.ApplyURL,
			})
		}
		families = append(families, apiFamily{
			RoleFamilyID: family.RoleFamilyID,
			Company:      apiFamilyCompany{Name: family.Company.Name, Slug: family.Company.Slug, ATS: family.Company.ATS},
			Title:        family.Title,
			Postings:     postings,
		})
	}
	totalPages := max(1, (result.Total+int(limit)-1)/int(limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	w.Header().Set("Cache-Control", apiCacheControl)
	addDataSourceHeaders(w.Header(), result.DataSource)
	writeJSON(w, http.StatusOK, jobsResponse{
		Jobs: families, Page: page, Limit: limit, Total: result.Total,
		TotalPages: totalPages, HasMore: int(page) < totalPages,
	})
}

type apiJobResponse struct {
	Data apiJob `json:"data"`
}

type apiJob struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	Locations      []string      `json:"locations"`
	ApplyURL       string        `json:"apply_url"`
	Excerpt        *string       `json:"excerpt"`
	Terms          []string      `json:"terms"`
	TermYears      any           `json:"term_years"`
	DurationMonths []int         `json:"duration_months"`
	CohortYear     *int          `json:"cohort_year"`
	Roles          []string      `json:"roles"`
	Regions        []string      `json:"regions"`
	IsRemote       bool          `json:"is_remote"`
	IsActive       bool          `json:"is_active"`
	Source         string        `json:"source"`
	PostedAt       *string       `json:"posted_at"`
	FirstSeenAt    string        `json:"first_seen_at"`
	LastSeenAt     string        `json:"last_seen_at"`
	Company        apiJobCompany `json:"company"`
}

type apiJobCompany struct {
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	ATS        string  `json:"ats"`
	CareersURL *string `json:"careers_url"`
	WebsiteURL *string `json:"website_url"`
}

func (s *server) jobAPI(w http.ResponseWriter, request *http.Request) {
	decision := s.limiter.step("job:" + clientIP(request))
	if !decision.OK {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "Rate limit exceeded"})
		return
	}
	id := request.PathValue("id")
	job, source, err := s.store.Job(request.Context(), id)
	if err != nil {
		writeAPIUnavailable(w)
		return
	}
	if job == nil {
		if replacement := s.store.JobAlias(id); replacement != "" {
			w.Header().Set("Location", "/api/v1/jobs/"+replacement)
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.WriteHeader(http.StatusPermanentRedirect)
			return
		}
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "Not found"})
		return
	}
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	w.Header().Set("Cache-Control", apiCacheControl)
	addDataSourceHeaders(w.Header(), source)
	writeJSON(w, http.StatusOK, apiJobResponse{Data: apiJob{
		ID: job.ID, Title: job.Title, Locations: job.Locations, ApplyURL: job.ApplyURL,
		Excerpt: job.Excerpt, Terms: job.Terms, TermYears: job.TermYears,
		DurationMonths: job.DurationMonths, CohortYear: job.CohortYear,
		Roles: job.Roles, Regions: job.Regions, IsRemote: job.IsRemote,
		IsActive: job.IsActive, Source: job.Source, PostedAt: job.PostedAt,
		FirstSeenAt: job.FirstSeenAt, LastSeenAt: job.LastSeenAt,
		Company: apiJobCompany{
			Name: job.CompanyName, Slug: job.CompanySlug, ATS: job.CompanyATS,
			CareersURL: job.CompanyCareersURL, WebsiteURL: job.CompanyWebsiteURL,
		},
	}})
}

type companiesResponse struct {
	Data  []apiCompany `json:"data"`
	Total int          `json:"total"`
}

type apiCompany struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	ATS        string  `json:"ats"`
	CareersURL *string `json:"careers_url"`
	WebsiteURL *string `json:"website_url"`
	Active     bool    `json:"active"`
	ActiveJobs int     `json:"active_jobs"`
}

func (s *server) companiesAPI(w http.ResponseWriter, request *http.Request) {
	decision := s.limiter.step("companies:" + clientIP(request))
	if !decision.OK {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "Rate limit exceeded"})
		return
	}
	rows, source, err := s.store.Companies(request.Context(), request.URL.Query().Get("active") != "false")
	if err != nil {
		writeAPIUnavailable(w)
		return
	}
	companies := make([]apiCompany, 0, len(rows))
	for _, company := range rows {
		companies = append(companies, apiCompany{
			ID: company.ID, Name: company.Name, Slug: company.Slug, ATS: company.ATS,
			CareersURL: company.CareersURL, WebsiteURL: company.WebsiteURL,
			Active: company.Active, ActiveJobs: company.ActiveJobs,
		})
	}
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	w.Header().Set("Cache-Control", apiCacheControl)
	addDataSourceHeaders(w.Header(), source)
	writeJSON(w, http.StatusOK, companiesResponse{Data: companies, Total: len(companies)})
}

const seasonNote = "Live board focuses on Fall/Winter 2026–27 and Summer 2027; many Summer 2026 boards are closed."
const healthTriage = "Check /health → pnpm recover-tokens → fix data/companies YAML → pnpm validate-tokens → re-run ingest."

type healthStatsResponse struct {
	ActiveJobs           int     `json:"active_jobs"`
	TotalJobs            *int    `json:"total_jobs"`
	ActiveCompanies      int     `json:"active_companies"`
	TotalCompanies       *int    `json:"total_companies"`
	LastSuccessfulIngest *string `json:"last_successful_ingest"`
	ZeroMatchCompanies   *int    `json:"zero_match_companies"`
	SeasonNote           string  `json:"season_note"`
}

type healthPipelineResponse struct {
	ZeroMatchCompanies []string              `json:"zero_match_companies"`
	RecentFailures     []board.HealthFailure `json:"recent_failures"`
	ProprietaryATS     any                   `json:"proprietary_ats"`
	Triage             string                `json:"triage"`
}

type healthResponse struct {
	Status                       string                 `json:"status"`
	Product                      string                 `json:"product"`
	Version                      string                 `json:"version"`
	SnapshotGeneratedAt          *string                `json:"snapshot_generated_at"`
	Stats                        healthStatsResponse    `json:"stats"`
	Pipeline                     healthPipelineResponse `json:"pipeline"`
	FunnelTotalsLatestPerCompany any                    `json:"funnel_totals_latest_per_company"`
	RecentIngestRuns             []board.HealthRun      `json:"recent_ingest_runs"`
}

func intPointer(value int) *int { return &value }

func (s *server) healthAPI(w http.ResponseWriter, request *http.Request) {
	snapshot, snapshotErr := s.store.HealthSnapshot(request.Context())
	status, dumpErr := s.store.DumpStatus(request.Context())
	if snapshot != nil && snapshotErr == nil {
		generated := snapshot.GeneratedAt
		runs := snapshot.RecentIngestRuns
		if len(runs) > 50 {
			runs = runs[:50]
		}
		w.Header().Set("Cache-Control", "public, s-maxage=3600, stale-while-revalidate=600")
		w.Header().Set("X-Data-Source", "snapshot")
		w.Header().Set("X-Snapshot-Generated-At", snapshot.GeneratedAt)
		if dumpErr == nil {
			w.Header().Set("X-Dump-Generated-At", status.GeneratedAt)
		}
		writeJSON(w, http.StatusOK, healthResponse{
			Status: "ok", Product: "OpenIntern", Version: "0.1.0",
			SnapshotGeneratedAt: &generated,
			Stats: healthStatsResponse{
				ActiveJobs: snapshot.Stats.ActiveJobs, TotalJobs: intPointer(snapshot.Stats.TotalJobs),
				ActiveCompanies: snapshot.Stats.ActiveCompanies, TotalCompanies: intPointer(snapshot.Stats.TotalCompanies),
				LastSuccessfulIngest: snapshot.Stats.LastSuccessfulIngest,
				ZeroMatchCompanies:   intPointer(snapshot.Stats.ZeroMatchCompanies), SeasonNote: seasonNote,
			},
			Pipeline: healthPipelineResponse{
				ZeroMatchCompanies: snapshot.Pipeline.ZeroMatchCompanies,
				RecentFailures:     snapshot.Pipeline.RecentFailures,
				ProprietaryATS:     snapshot.Pipeline.ProprietaryATS,
				Triage:             healthTriage,
			},
			FunnelTotalsLatestPerCompany: snapshot.FunnelTotalsLatestPerCompany,
			RecentIngestRuns:             runs,
		})
		return
	}
	if dumpErr != nil {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "3600")
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "Health data unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "public, s-maxage=3600, stale-while-revalidate=600")
	w.Header().Set("X-Data-Source", "snapshot")
	w.Header().Set("X-Dump-Generated-At", status.GeneratedAt)
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "degraded", Product: "OpenIntern", Version: "0.1.0",
		SnapshotGeneratedAt: nil,
		Stats: healthStatsResponse{
			ActiveJobs: status.ActiveJobs, TotalJobs: nil,
			ActiveCompanies: status.ActiveCompanies, TotalCompanies: nil,
			LastSuccessfulIngest: nil, ZeroMatchCompanies: nil, SeasonNote: seasonNote,
		},
		Pipeline: healthPipelineResponse{
			ZeroMatchCompanies: []string{}, RecentFailures: []board.HealthFailure{},
			ProprietaryATS: struct {
				Failures    []board.ProprietaryFailure    `json:"failures"`
				ZeroJobRuns []board.ProprietaryZeroJobRun `json:"zero_job_runs"`
			}{Failures: []board.ProprietaryFailure{}, ZeroJobRuns: []board.ProprietaryZeroJobRun{}},
			Triage: healthTriage,
		},
		FunnelTotalsLatestPerCompany: nil,
		RecentIngestRuns:             []board.HealthRun{},
	})
}

type statusResponse struct {
	Status          string   `json:"status"`
	DumpGeneratedAt string   `json:"dump_generated_at"`
	DumpAgeHours    *float64 `json:"dump_age_hours"`
	MaxDumpAgeHours int      `json:"max_dump_age_hours"`
	ActiveJobs      int      `json:"active_jobs"`
}

type downStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (s *server) statusAPI(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	status, err := s.store.DumpStatus(request.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, downStatusResponse{Status: "down", Error: "Dump unavailable"})
		return
	}
	generated, ok := parseJavaScriptDate(status.GeneratedAt)
	var age *float64
	fresh := false
	if ok {
		rawAge := float64(s.now().Sub(*generated)) / float64(time.Hour)
		value := math.Round(rawAge*10) / 10
		age = &value
		fresh = rawAge <= 30
	}
	state := "stale"
	httpStatus := http.StatusServiceUnavailable
	if fresh {
		state = "ok"
		httpStatus = http.StatusOK
	}
	writeJSON(w, httpStatus, statusResponse{
		Status: state, DumpGeneratedAt: status.GeneratedAt, DumpAgeHours: age,
		MaxDumpAgeHours: 30, ActiveJobs: status.ActiveJobs,
	})
}
