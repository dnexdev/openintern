package board

// Funnel is the per-run classifier funnel stored in health.json.
type Funnel struct {
	Fetched            *int `json:"fetched"`
	Title              *int `json:"title"`
	Tech               *int `json:"tech"`
	Upserted           *int `json:"upserted"`
	DescriptionMissing *int `json:"descriptionMissing"`
}

type FunnelTotals struct {
	Fetched            int `json:"fetched"`
	Title              int `json:"title"`
	Tech               int `json:"tech"`
	Upserted           int `json:"upserted"`
	DescriptionMissing int `json:"descriptionMissing"`
}

type HealthRun struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	JobCount    int     `json:"job_count"`
	Error       *string `json:"error"`
	RanAt       string  `json:"ran_at"`
	CompanySlug *string `json:"company_slug"`
	CompanyName *string `json:"company_name"`
	CompanyATS  *string `json:"company_ats"`
	Funnel      Funnel  `json:"funnel"`
	ZeroMatch   bool    `json:"zero_match"`
}

type HealthFailure struct {
	CompanySlug *string `json:"company_slug"`
	CompanyName *string `json:"company_name"`
	CompanyATS  *string `json:"company_ats"`
	Error       *string `json:"error"`
	RanAt       string  `json:"ran_at"`
}

type ProprietaryFailure struct {
	CompanySlug *string `json:"company_slug"`
	CompanyATS  *string `json:"company_ats"`
	Error       *string `json:"error"`
	RanAt       string  `json:"ran_at"`
}

type ProprietaryZeroJobRun struct {
	CompanySlug *string `json:"company_slug"`
	CompanyATS  *string `json:"company_ats"`
	JobCount    int     `json:"job_count"`
	Error       *string `json:"error"`
	RanAt       string  `json:"ran_at"`
}

type HealthStats struct {
	ActiveJobs           int     `json:"active_jobs"`
	TotalJobs            int     `json:"total_jobs"`
	ActiveCompanies      int     `json:"active_companies"`
	TotalCompanies       int     `json:"total_companies"`
	LastSuccessfulIngest *string `json:"last_successful_ingest"`
	ZeroMatchCompanies   int     `json:"zero_match_companies"`
}

type HealthPipeline struct {
	ZeroMatchCompanies []string        `json:"zero_match_companies"`
	RecentFailures     []HealthFailure `json:"recent_failures"`
	ProprietaryATS     struct {
		Failures    []ProprietaryFailure    `json:"failures"`
		ZeroJobRuns []ProprietaryZeroJobRun `json:"zero_job_runs"`
	} `json:"proprietary_ats"`
}

// HealthSnapshot is health.json as published alongside the jobs dump.
type HealthSnapshot struct {
	GeneratedAt                  string         `json:"generatedAt"`
	Stats                        HealthStats    `json:"stats"`
	Pipeline                     HealthPipeline `json:"pipeline"`
	FunnelTotalsLatestPerCompany FunnelTotals   `json:"funnel_totals_latest_per_company"`
	RecentIngestRuns             []HealthRun    `json:"recent_ingest_runs"`
}
