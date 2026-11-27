// Package board loads the public flat-file dump and provides the read model
// used by the website and public API.
package board

import (
	"context"
	"net/http"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

const (
	DefaultDumpURL = "https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.json"

	DumpRefreshInterval       = time.Hour
	KillSwitchRefreshInterval = 5 * time.Minute
	HTTPTimeout               = 10 * time.Second
	KillSwitchHTTPTimeout     = 3 * time.Second
	MaxDumpAge                = 30 * time.Hour
)

var (
	RoleOptions     = []string{"software", "backend", "frontend", "fullstack", "data", "ml", "mobile", "security", "devops", "hardware", "quant", "product", "research"}
	RegionOptions   = []string{"remote", "us", "canada", "europe", "other"}
	TermOptions     = []string{"summer", "fall", "winter", "spring"}
	DurationOptions = []int{3, 4, 6, 8, 12}
)

type FamilySort string

const (
	SortFirstSeen FamilySort = "first_seen"
	SortPosted    FamilySort = "posted"
	SortPrestige  FamilySort = "prestige"
)

// Options controls data sources and test seams. Empty source fields use the
// corresponding environment variable and then the production default.
type Options struct {
	DumpSource       string
	HealthSource     string
	KillSwitchSource string
	HTTPClient       *http.Client
	Now              func() time.Time
	DumpRefresh      time.Duration
	KillRefresh      time.Duration

	// Registry callbacks keep the store usable with a test registry. NewStore
	// fills nil callbacks from internal/registry.
	CanonicalCompanySlug func(string) string
	JobAlias             func(string) string
	JobAliasByPrefix     func(string) string
	JobPath              func(companySlug, title, id string) string
	ApplyURLAllowed      func(companySlug, applyURL string) bool
	IsDelisted           func(jobID string) bool

	// IsPublic is a test seam around the Bend-proven freshness decision.
	IsPublic func(job model.DumpJob, now time.Time) bool
}

type DataSource struct {
	Kind        string
	GeneratedAt string
}

type FamilyQuery struct {
	Query       string
	Company     string
	Roles       []string
	Regions     []string
	Terms       []string
	Durations   []int
	PostedAfter *time.Time
	Sort        FamilySort
	Limit       int
	Offset      int
}

type JobPosting struct {
	ID          string
	Title       string
	Location    string
	Locations   []string
	PostedAt    *string
	ApplyURL    string
	IsRemote    bool
	FirstSeenAt string
}

type JobFamily struct {
	RoleFamilyID   string
	Title          string
	Company        FamilyCompany
	Roles          []string
	Regions        []string
	Terms          []string
	DurationMonths []int
	Excerpt        *string
	Source         string
	FirstSeenAt    string
	Postings       []JobPosting
}

type FamilyCompany struct {
	Name       string
	Slug       string
	ATS        string
	WebsiteURL *string
	CareersURL *string
}

type FamilyPage struct {
	Families   []JobFamily
	Total      int
	DataSource DataSource
}

type JobDetail struct {
	ID                string
	Title             string
	Locations         []string
	ApplyURL          string
	Excerpt           *string
	Terms             []string
	TermYears         []model.TermYear
	DurationMonths    []int
	CohortYear        *int
	Roles             []string
	Regions           []string
	IsRemote          bool
	IsActive          bool
	Source            string
	PostedAt          *string
	FirstSeenAt       string
	LastSeenAt        string
	CompanyName       string
	CompanySlug       string
	CompanyATS        string
	CompanyWebsiteURL *string
	CompanyCareersURL *string
}

type Company struct {
	ID         string
	Name       string
	Slug       string
	ATS        string
	CareersURL *string
	WebsiteURL *string
	Active     bool
	ActiveJobs int
}

type CompanyOption struct {
	Slug string
	Name string
}

type CorpusStats struct {
	ActiveCompanies int
	LastIngest      *string
	DataSource      DataSource
}

type JobCard struct {
	ID                string
	Title             string
	Locations         []string
	ApplyURL          string
	Excerpt           *string
	Terms             []string
	TermYears         []model.TermYear
	DurationMonths    []int
	Roles             []string
	Regions           []string
	IsRemote          bool
	Source            string
	PostedAt          *string
	FirstSeenAt       string
	CompanyName       string
	CompanySlug       string
	CompanyWebsiteURL *string
	CompanyCareersURL *string
}

type HeroPreview struct {
	Preview    []JobCard
	Total      int
	DataSource DataSource
}

type StaleState struct {
	Available   bool
	Stale       bool
	GeneratedAt string
	Age         time.Duration
}

type DumpStatus struct {
	GeneratedAt     string
	ActiveJobs      int
	ActiveCompanies int
}

type SitemapEntry struct {
	Path         string
	LastModified string
}

// StoreReader is the board surface used by HTTP handlers and site.Register.
// The concrete *Store also exposes refresh and alias helpers.
type StoreReader interface {
	Query(context.Context, FamilyQuery) (FamilyPage, error)
	Job(context.Context, string) (*JobDetail, DataSource, error)
	JobByCompanyAndIDPrefix(context.Context, string, string) (*JobDetail, DataSource, error)
	Company(context.Context, string) (*Company, DataSource, error)
	Companies(context.Context, bool) ([]Company, DataSource, error)
	CompanyOptions(context.Context) ([]CompanyOption, DataSource, error)
	CorpusStats(context.Context) (CorpusStats, error)
	HeroPreview(context.Context) (HeroPreview, error)
	HealthSnapshot(context.Context) (*HealthSnapshot, error)
	DumpStatus(context.Context) (DumpStatus, error)
	StaleData(context.Context) StaleState
	SitemapEntries(context.Context, int) ([]SitemapEntry, error)
}
