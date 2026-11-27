package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"github.com/dnexdev/openintern/internal/state/jsjson"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// tsvRows splits a TSV, skipping blank lines, comments and the header line.
func tsvRows(text, header string) [][]string {
	var out [][]string
	for _, l := range strings.Split(text, "\n") {
		if jsTrim(l) == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, header) {
			continue
		}
		out = append(out, strings.Split(l, "\t"))
	}
	return out
}

func col(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

// ParseCompanyAliases reads company-aliases.tsv (old_slug, new_slug, ...)
// and checks it against the registry slugs, as gen-aliases did: an old slug
// must have left the registry and its target must be in it.
func ParseCompanyAliases(text string, slugs map[string]bool) (map[string]string, error) {
	var errs []string
	out := map[string]string{}
	for _, row := range tsvRows(text, "old_slug\t") {
		oldSlug, newSlug := col(row, 0), col(row, 1)
		if oldSlug == "" || newSlug == "" {
			continue
		}
		if slugs[oldSlug] {
			errs = append(errs, fmt.Sprintf("company alias %s is still a registry slug", oldSlug))
		}
		if !slugs[newSlug] {
			errs = append(errs, fmt.Sprintf("company alias target %s is not in the registry", newSlug))
		}
		out[oldSlug] = newSlug
	}
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

// ParseJobAliases reads job-aliases.tsv (old_job_id, new_job_id); both must
// be lowercase UUIDs.
func ParseJobAliases(text string) (map[string]string, error) {
	var errs []string
	out := map[string]string{}
	for _, row := range tsvRows(text, "old_job_id\t") {
		oldID, newID := col(row, 0), col(row, 1)
		if !uuidRe.MatchString(oldID) || !uuidRe.MatchString(newID) {
			errs = append(errs, fmt.Sprintf("bad job alias %s → %s", jsUndefined(row, 0), jsUndefined(row, 1)))
			continue
		}
		out[oldID] = newID
	}
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

// jsUndefined prints a missing column the way the TypeScript template did.
func jsUndefined(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return "undefined"
}

func (r *Registry) loadAliases(fsys fs.FS) error {
	slugs := map[string]bool{}
	for _, c := range r.Companies {
		slugs[c.Slug] = true
	}
	var errs []error
	if src, err := fs.ReadFile(fsys, CompanyAliasesPath); err != nil {
		errs = append(errs, fmt.Errorf("registry: %w", err))
	} else if r.companyAliases, err = ParseCompanyAliases(string(src), slugs); err != nil {
		errs = append(errs, fmt.Errorf("data/%s: %w", CompanyAliasesPath, err))
	}
	if src, err := fs.ReadFile(fsys, JobAliasesPath); err != nil {
		errs = append(errs, fmt.Errorf("registry: %w", err))
	} else if r.jobAliasMap, err = ParseJobAliases(string(src)); err != nil {
		errs = append(errs, fmt.Errorf("data/%s: %w", JobAliasesPath, err))
	}
	for o, n := range r.jobAliasMap {
		r.jobAliases = append(r.jobAliases, [2]string{o, n})
	}
	r.jobAliases = sortedAliases(r.jobAliases)
	return errors.Join(errs...)
}

// sortedAliases orders pairs by old id, the order of the generated module.
func sortedAliases(list [][2]string) [][2]string {
	sort.Slice(list, func(i, j int) bool { return jsjson.CompareCodeUnits(list[i][0], list[j][0]) < 0 })
	return list
}

// CompanyAliases is old slug → current slug.
func (r *Registry) CompanyAliases() map[string]string { return r.companyAliases }

// CanonicalCompanySlug is the current slug for a company, following merges
// and renames (one hop, as the TypeScript did).
func (r *Registry) CanonicalCompanySlug(slug string) string {
	if s, ok := r.companyAliases[slug]; ok {
		return s
	}
	return slug
}

// JobAlias is the job that replaced a retired job id, or "".
func (r *Registry) JobAlias(id string) string {
	return r.jobAliasMap[strings.ToLower(id)]
}

// JobAliasByPrefix is the replacement for a retired job whose id starts with
// this 8-hex prefix (job slugs carry only the prefix), or "". The first old
// id in sorted order wins.
func (r *Registry) JobAliasByPrefix(prefix8 string) string {
	p := strings.ToLower(prefix8)
	for _, a := range r.jobAliases {
		if strings.HasPrefix(strings.ReplaceAll(a[0], "-", ""), p) {
			return a[1]
		}
	}
	return ""
}
