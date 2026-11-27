// Package registry loads and checks the community registry: companies
// (data/companies/*.yaml), the Tier 1 list (data/curated/tier-1.yaml), the
// apply-link allowlist (data/apply-hosts.tsv plus hosts implied by board
// tokens), company and job aliases, and hand-delisted jobs.
//
// The TypeScript compiled these into generated modules because the site had
// no filesystem on Vercel; here they are embedded (package data) and parsed
// once at startup, so the generators and their --check steps are gone. The
// `apply-hosts` command prints the effective allowlist for review.
package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/dnexdev/openintern/data"
)

// Registry is a loaded, validated registry.
type Registry struct {
	// Companies is every entry of every file, files in name order, so a slug
	// listed twice appears twice (validate-companies reports it).
	Companies []CompanyYAML
	// Tier1 is data/curated/tier-1.yaml in file order.
	Tier1 []string
	// Warnings are non-fatal findings (unknown keys, which are ignored).
	Warnings []string

	unique         []CompanyYAML
	bySlug         map[string]int
	tier1          map[string]bool
	applyHosts     Table
	companyAliases map[string]string
	jobAliases     [][2]string // sorted by old id
	jobAliasMap    map[string]string
}

// CompaniesDir, Tier1Path, ... are paths inside a data/-shaped fs.FS.
const (
	CompaniesDir       = "companies"
	Tier1Path          = "curated/tier-1.yaml"
	ApplyHostsPath     = "apply-hosts.tsv"
	CompanyAliasesPath = "company-aliases.tsv"
	JobAliasesPath     = "job-aliases.tsv"
	ExpectedDeltaPath  = "expected-delta.json"
)

// LoadCompanies is loadCompanyYamlFiles: every company of every *.yaml or
// *.yml file in companies/, files in lexicographic order. The first invalid
// file fails the load, as zod's parse does.
func LoadCompanies(fsys fs.FS) ([]CompanyYAML, []string, error) {
	entries, err := fs.ReadDir(fsys, CompaniesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("registry: %w", err)
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && (strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml")) {
			names = append(names, n)
		}
	}
	// Later files win on duplicate slugs (ats-new before tech-*).
	sort.Strings(names)
	var all []CompanyYAML
	var warnings []string
	for _, n := range names {
		src, err := fs.ReadFile(fsys, CompaniesDir+"/"+n)
		if err != nil {
			return nil, nil, fmt.Errorf("registry: %w", err)
		}
		list, unknown, err := ParseCompaniesFile("data/companies/"+n, src)
		if err != nil {
			return nil, nil, err
		}
		for i := range list {
			list[i].File = n
		}
		all = append(all, list...)
		warnings = append(warnings, unknown...)
	}
	return all, warnings, nil
}

// LoadTier1 is loadTier1Curated.
func LoadTier1(fsys fs.FS) ([]string, error) {
	src, err := fs.ReadFile(fsys, Tier1Path)
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}
	return ParseTier1("data/"+Tier1Path, src)
}

// Load reads and validates the whole registry from a data/-shaped fs.FS.
func Load(fsys fs.FS) (*Registry, error) {
	companies, warnings, err := LoadCompanies(fsys)
	if err != nil {
		return nil, err
	}
	r := &Registry{Companies: companies, Warnings: warnings, bySlug: map[string]int{}}
	for _, c := range companies {
		if i, ok := r.bySlug[c.Slug]; ok {
			r.unique[i] = c
			continue
		}
		r.bySlug[c.Slug] = len(r.unique)
		r.unique = append(r.unique, c)
	}
	var errs []error
	if r.Tier1, err = LoadTier1(fsys); err != nil {
		errs = append(errs, err)
	}
	r.tier1 = map[string]bool{}
	for _, s := range r.Tier1 {
		r.tier1[s] = true
	}
	if r.applyHosts, err = buildApplyHosts(fsys, companies); err != nil {
		errs = append(errs, err)
	}
	if err := r.loadAliases(fsys); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return r, nil
}

// Unique is the registry by slug, later files winning, in first-seen order
// (the iteration order of the TypeScript Map built the same way).
func (r *Registry) Unique() []CompanyYAML { return r.unique }

// Company returns the winning entry for slug.
func (r *Registry) Company(slug string) (CompanyYAML, bool) {
	i, ok := r.bySlug[slug]
	if !ok {
		return CompanyYAML{}, false
	}
	return r.unique[i], true
}

// IsTier1 reports whether slug is on the curated Tier 1 list.
func (r *Registry) IsTier1(slug string) bool { return r.tier1[slug] }

var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)

// Default is the registry compiled into the binary. The embedded files are
// checked by this package's tests, so a load error is a build that skipped
// them; it panics rather than serve an empty registry.
func Default() *Registry {
	defaultOnce.Do(func() { defaultReg, defaultErr = Load(data.FS) })
	if defaultErr != nil {
		panic(fmt.Sprintf("registry: embedded data is invalid: %v", defaultErr))
	}
	return defaultReg
}

// Package-level helpers on the embedded registry, for the website and API.

// ApplyURLAllowed is the publishing gate on the embedded allowlist, in the
// mode OPENINTERN_APPLY_HOSTS selects.
func ApplyURLAllowed(companySlug, url string) bool {
	return Default().ApplyURLAllowed(companySlug, url, ApplyHostModeFromEnv())
}

// CanonicalCompanySlug follows merges and renames (company-aliases.tsv).
func CanonicalCompanySlug(slug string) string { return Default().CanonicalCompanySlug(slug) }

// JobAlias is the job that replaced a retired job id, or "".
func JobAlias(id string) string { return Default().JobAlias(id) }

// JobAliasByPrefix is the replacement for a retired job whose id starts with
// this 8-hex prefix, or "".
func JobAliasByPrefix(prefix8 string) string { return Default().JobAliasByPrefix(prefix8) }

// Tier1Slugs is the curated Tier 1 list.
func Tier1Slugs() []string { return Default().Tier1 }
