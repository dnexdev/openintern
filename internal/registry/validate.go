package registry

import (
	"fmt"
	"io/fs"
)

// Result is what a registry check found.
type Result struct {
	Count    int
	Errors   []string
	Warnings []string
}

// ValidateCompanies is validateCompanies: duplicate slugs and duplicate ATS
// tokens across files are errors, a missing website_url is a warning. Schema
// errors fail the load itself. Unknown keys (which the loader ignores) are
// reported as warnings so a typo such as "webiste_url" does not go unnoticed.
func ValidateCompanies(fsys fs.FS) (Result, error) {
	companies, unknown, err := LoadCompanies(fsys)
	if err != nil {
		return Result{}, err
	}
	var res Result
	res.Count = len(companies)
	slugSources := map[string]string{}
	tokenSources := map[string]CompanyYAML{}
	for _, c := range companies {
		if prior, ok := slugSources[c.Slug]; ok {
			res.Errors = append(res.Errors, fmt.Sprintf("duplicate slug %q in %s and %s", c.Slug, prior, c.File))
		} else {
			slugSources[c.Slug] = c.File
		}
		key := c.ATS + ":" + c.BoardToken
		if prior, ok := tokenSources[key]; ok && prior.Slug != c.Slug {
			res.Errors = append(res.Errors, fmt.Sprintf("duplicate ATS token %q for %s (%s) and %s (%s)", key, prior.Slug, prior.File, c.Slug, c.File))
		} else {
			tokenSources[key] = c
		}
		if c.WebsiteURL == nil || *c.WebsiteURL == "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("missing website_url for %s (%s)", c.Slug, c.File))
		}
	}
	res.Warnings = append(res.Warnings, unknown...)
	return res, nil
}

// ValidateTier1 is validateTier1Curated: every slug on the Tier 1 list must
// be in the registry once; an inactive company is a warning.
func ValidateTier1(fsys fs.FS) (Result, error) {
	slugs, err := LoadTier1(fsys)
	if err != nil {
		return Result{}, err
	}
	companies, _, err := LoadCompanies(fsys)
	if err != nil {
		return Result{}, err
	}
	bySlug := map[string]CompanyYAML{}
	for _, c := range companies {
		bySlug[c.Slug] = c // later files win
	}
	res := Result{Count: len(slugs)}
	seen := map[string]bool{}
	for _, s := range slugs {
		if seen[s] {
			res.Errors = append(res.Errors, "duplicate slug: "+s)
			continue
		}
		seen[s] = true
		c, ok := bySlug[s]
		if !ok {
			res.Errors = append(res.Errors, "unknown slug (not in data/companies): "+s)
			continue
		}
		if !c.Active {
			res.Warnings = append(res.Warnings, s+" is active: false — no jobs will be highlighted")
		}
	}
	return res, nil
}
