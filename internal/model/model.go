// Package model holds the records every part of OpenIntern shares: the
// flat-file state (companies.jsonl, jobs.jsonl, last_seen.tsv, runs.jsonl),
// the public dump (jobs.json, companies.json) and what an ATS adapter returns.
//
// Field order is the serialized key order. State files must stay byte-identical
// to the phase D TypeScript writer and to Postgres row_to_json exports, and dump
// files must stay identical to the published format, so never reorder fields,
// and add new ones only at the end with a matching change in the writer.
//
// Encoding rules every writer of these records must follow (see
// docs/go-rewrite/PLAN.md, "Byte compatibility"):
//   - no HTML escaping (json.Encoder.SetEscapeHTML(false));
//   - U+2028 and U+2029 written raw, as JavaScript's JSON.stringify does;
//   - nil slices written as [] wherever the TypeScript wrote arrays;
//   - timestamps kept as the strings that were read, never re-formatted.
package model

// ATS names, as stored in companies.ats and data/companies/*.yaml.
const (
	Greenhouse        = "greenhouse"
	Lever             = "lever"
	Ashby             = "ashby"
	Workable          = "workable"
	SmartRecruiters   = "smartrecruiters"
	Recruitee         = "recruitee"
	Rippling          = "rippling"
	BambooHR          = "bamboohr"
	Workday           = "workday"
	Citadel           = "citadel"
	CitadelSecurities = "citadel_securities"
	Tesla             = "tesla"
	ByteDance         = "bytedance"
	TikTok            = "tiktok"
	Phenom            = "phenom"
	Avature           = "avature"
	ICIMS             = "icims"
	Eightfold         = "eightfold"
	Google            = "google"
	IBM               = "ibm"
	DEShaw            = "deshaw"
	Shopify           = "shopify"
	X                 = "x"
)

// AllATS lists every ATS name in schema order (the Postgres enum order).
var AllATS = []string{
	Greenhouse, Lever, Ashby, Workable, SmartRecruiters, Recruitee, Rippling,
	BambooHR, Workday, Citadel, CitadelSecurities, Tesla, ByteDance, TikTok,
	Phenom, Avature, ICIMS, Eightfold, Google, IBM, DEShaw, Shopify, X,
}

// TermYear is an internship term with its year, e.g. {"summer", 2027}.
type TermYear struct {
	Term string `json:"term"`
	Year int    `json:"year"`
}

// Company is one line of companies.jsonl. Timestamps are Postgres-style
// strings ("2026-10-23T12:00:00.123+00:00"); see PLAN.md.
type Company struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	ATS        string  `json:"ats"`
	BoardToken string  `json:"board_token"`
	CareersURL *string `json:"careers_url"`
	WebsiteURL *string `json:"website_url"`
	Active     bool    `json:"active"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// Job is one job in state. Field order is jobs.jsonl key order (the Postgres
// column order). LastSeenAt and UpdatedAt are volatile: they live in
// last_seen.tsv, not jobs.jsonl, so unchanged jobs give no diff.
type Job struct {
	ID              string     `json:"id"`
	CompanyID       string     `json:"company_id"`
	ExternalJobID   string     `json:"external_job_id"`
	Title           string     `json:"title"`
	Locations       []string   `json:"locations"`
	ApplyURL        string     `json:"apply_url"`
	Excerpt         *string    `json:"excerpt"`
	IsRemote        bool       `json:"is_remote"`
	IsActive        bool       `json:"is_active"`
	Source          string     `json:"source"`
	PostedAt        *string    `json:"posted_at"`
	FirstSeenAt     string     `json:"first_seen_at"`
	LastSeenAt      string     `json:"-"` // last_seen.tsv column 2
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"-"` // last_seen.tsv column 3
	Terms           []string   `json:"terms"`
	CohortYear      *int       `json:"cohort_year"`
	DurationMonths  []int      `json:"duration_months"`
	Roles           []string   `json:"roles"`
	Regions         []string   `json:"regions"`
	TermYears       []TermYear `json:"term_years"`
	Fingerprint     string     `json:"fingerprint"`
	NormalizedTitle string     `json:"normalized_title"`
	RoleFamilyID    string     `json:"role_family_id"`
}

// Run is one line of runs.jsonl (an ingest run for one company).
type Run struct {
	ID        string  `json:"id"`
	CompanyID string  `json:"company_id"`
	Status    string  `json:"status"`
	JobCount  int     `json:"job_count"`
	Error     *string `json:"error"`
	RanAt     string  `json:"ran_at"`
}

// NormalizedJob is what an ATS adapter returns for one posting, before
// classification (TS: NormalizedJob in packages/ingest/src/ats.ts).
type NormalizedJob struct {
	ExternalID  string
	Title       string
	Locations   []string
	ApplyURL    string
	Excerpt     *string
	PostedAt    *string // RFC 3339 / ISO 8601 UTC, or nil when unknown
	Description string
}

// DumpCompanyRef is the company block inside each dump job.
type DumpCompanyRef struct {
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	ATS        string  `json:"ats"`
	WebsiteURL *string `json:"website_url"` // absent (nil) in dumps before 2026-10
	CareersURL *string `json:"careers_url"`
}

// DumpJob is one entry of jobs.json "jobs". Timestamps are JS
// toISOString() strings ("2026-11-13T14:38:58.955Z").
type DumpJob struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	Company         DumpCompanyRef `json:"company"`
	Locations       []string       `json:"locations"`
	ApplyURL        string         `json:"apply_url"`
	Excerpt         *string        `json:"excerpt"`
	Terms           []string       `json:"terms"`
	TermYears       []TermYear     `json:"term_years"`
	DurationMonths  []int          `json:"duration_months"`
	CohortYear      *int           `json:"cohort_year"`
	Roles           []string       `json:"roles"`
	Regions         []string       `json:"regions"`
	IsRemote        bool           `json:"is_remote"`
	Source          string         `json:"source"`
	PostedAt        *string        `json:"posted_at"`
	FirstSeenAt     string         `json:"first_seen_at"`
	LastSeenAt      string         `json:"last_seen_at"`
	RoleFamilyID    string         `json:"role_family_id"`   // absent ("") in dumps before 2026-11
	NormalizedTitle string         `json:"normalized_title"` // absent ("") in dumps before 2026-11
}

// JobsDump is jobs.json.
type JobsDump struct {
	GeneratedAt string    `json:"generatedAt"`
	Count       int       `json:"count"`
	Jobs        []DumpJob `json:"jobs"`
}

// DumpCompany is one entry of companies.json "companies".
type DumpCompany struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	ATS        string  `json:"ats"`
	CareersURL *string `json:"careers_url"`
	WebsiteURL *string `json:"website_url"`
	Active     bool    `json:"active"`
	ActiveJobs int     `json:"active_jobs"`
}

// CompaniesDump is companies.json.
type CompaniesDump struct {
	GeneratedAt string        `json:"generatedAt"`
	Count       int           `json:"count"`
	Companies   []DumpCompany `json:"companies"`
}
