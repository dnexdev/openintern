package registry

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dnexdev/openintern/internal/registry/weburl"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// HostPrefix is one allowed [host, path prefix] pair.
type HostPrefix struct {
	Host   string
	Prefix string
}

// Table maps a company slug to the hosts and path prefixes its apply links
// may use (TS: ApplyHostTable).
type Table map[string][]HostPrefix

var tokenHostRe = regexp.MustCompile(`(?i)^[a-z0-9-]+$`)

// DerivedHosts are the hosts a board token implies; empty when the ATS gives
// no safe pattern (those need an explicit, reviewed row in apply-hosts.tsv).
func DerivedHosts(ats, token string) []HostPrefix {
	t := jsTrim(token)
	seg := "/" + strings.ToLower(t) + "/"
	switch ats {
	case "greenhouse":
		return []HostPrefix{{"job-boards.greenhouse.io", seg}, {"boards.greenhouse.io", seg}, {"job-boards.eu.greenhouse.io", seg}}
	case "lever":
		return []HostPrefix{{"jobs.lever.co", seg}, {"jobs.eu.lever.co", seg}}
	case "ashby":
		return []HostPrefix{{"jobs.ashbyhq.com", seg}}
	case "smartrecruiters":
		return []HostPrefix{{"jobs.smartrecruiters.com", seg}}
	case "rippling":
		return []HostPrefix{{"ats.rippling.com", seg}}
	case "workable":
		return []HostPrefix{{"apply.workable.com", seg}}
	case "recruitee":
		if tokenHostRe.MatchString(t) {
			return []HostPrefix{{strings.ToLower(t) + ".recruitee.com", "/"}}
		}
		return []HostPrefix{}
	case "bamboohr":
		if tokenHostRe.MatchString(t) {
			return []HostPrefix{{strings.ToLower(t) + ".bamboohr.com", "/"}}
		}
		return []HostPrefix{}
	case "workday":
		if tenant, dc, ok := parseWorkdayToken(t); ok {
			return []HostPrefix{{strings.ToLower(tenant) + "." + dc + ".myworkdayjobs.com", "/"}}
		}
		return []HostPrefix{}
	case "tiktok":
		return []HostPrefix{{"lifeattiktok.com", "/search/"}}
	case "bytedance":
		return []HostPrefix{{"joinbytedance.com", "/search/"}}
	case "tesla":
		return []HostPrefix{{"www.tesla.com", "/careers/"}}
	case "citadel":
		return []HostPrefix{{"www.citadel.com", "/careers/"}}
	case "citadel_securities":
		return []HostPrefix{{"www.citadelsecurities.com", "/careers/"}}
	}
	return []HostPrefix{}
}

var workdayDCRe = regexp.MustCompile(`(?i)^wd\d+$`)

// parseWorkdayToken is parseWorkdayToken from workday.ts ("tenant|wd5|site"),
// returning the tenant and the lowercased datacenter.
func parseWorkdayToken(token string) (tenant, datacenter string, ok bool) {
	var parts []string
	for _, p := range strings.Split(token, "|") {
		if p = jsTrim(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) != 3 || !workdayDCRe.MatchString(parts[1]) {
		return "", "", false
	}
	return parts[0], strings.ToLower(parts[1]), true
}

// ApplyHostRow is one row of data/apply-hosts.tsv.
type ApplyHostRow struct {
	Company, Host, Prefix string
	Line                  int
}

var tsvHostRe = regexp.MustCompile(`^[a-z0-9.-]+$`)

// ParseApplyHostsTSV is parseApplyHostsTsv: rows of company, host,
// path_prefix (more columns are notes). Comments, blank lines and the header
// are skipped; every bad row is reported.
func ParseApplyHostsTSV(text string) ([]ApplyHostRow, error) {
	var rows []ApplyHostRow
	var errs []string
	for i, l := range strings.Split(text, "\n") {
		n := i + 1
		if jsTrim(l) == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "company\t") {
			continue
		}
		cols := strings.Split(l, "\t")
		col := func(k int) string {
			if k < len(cols) {
				return cols[k]
			}
			return ""
		}
		company, host, prefix := col(0), col(1), col(2)
		if company == "" || host == "" || prefix == "" {
			errs = append(errs, fmt.Sprintf("line %d: needs company, host, path_prefix", n))
			continue
		}
		if host != strings.ToLower(host) || !tsvHostRe.MatchString(host) {
			errs = append(errs, fmt.Sprintf("line %d: bad host %s", n, host))
		}
		if !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
			errs = append(errs, fmt.Sprintf("line %d: prefix must start and end with /", n))
		}
		rows = append(rows, ApplyHostRow{Company: company, Host: host, Prefix: prefix, Line: n})
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("data/apply-hosts.tsv: %s", strings.Join(errs, "; "))
	}
	return rows, nil
}

// buildApplyHosts is buildApplyHosts from gen-apply-hosts.ts: hosts derived
// from every token of every company, plus the TSV rows of companies still in
// the registry. Pairs are deduplicated and sorted.
func buildApplyHosts(fsys fs.FS, companies []CompanyYAML) (Table, error) {
	sets := map[string]map[HostPrefix]bool{}
	add := func(slug string, hp HostPrefix) {
		if sets[slug] == nil {
			sets[slug] = map[HostPrefix]bool{}
		}
		sets[slug][hp] = true
	}
	slugs := map[string]bool{}
	for _, c := range companies {
		slugs[c.Slug] = true
		for _, tok := range append([]string{c.BoardToken}, c.ExtraBoardTokens...) {
			for _, hp := range DerivedHosts(c.ATS, tok) {
				add(c.Slug, hp)
			}
		}
	}
	src, err := fs.ReadFile(fsys, ApplyHostsPath)
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}
	rows, err := ParseApplyHostsTSV(string(src))
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if !slugs[r.Company] {
			continue // company left the registry
		}
		add(r.Company, HostPrefix{r.Host, r.Prefix})
	}
	table := Table{}
	for slug, set := range sets {
		list := make([]HostPrefix, 0, len(set))
		for hp := range set {
			list = append(list, hp)
		}
		// The TypeScript sorted "host\tprefix" strings by code unit.
		sort.Slice(list, func(i, j int) bool {
			return jsjson.CompareCodeUnits(list[i].Host+"\t"+list[i].Prefix, list[j].Host+"\t"+list[j].Prefix) < 0
		})
		table[slug] = list
	}
	return table, nil
}

// ApplyHosts is the effective allowlist.
func (r *Registry) ApplyHosts() Table { return r.applyHosts }

// TableJSON renders a table exactly as gen-apply-hosts wrote
// packages/db/src/apply-hosts.generated.json (slugs sorted, one line).
func TableJSON(t Table) []byte {
	slugs := make([]string, 0, len(t))
	for s := range t {
		slugs = append(slugs, s)
	}
	sort.Slice(slugs, func(i, j int) bool { return jsjson.CompareCodeUnits(slugs[i], slugs[j]) < 0 })
	obj := jsjson.Object{}
	for _, s := range slugs {
		pairs := make([]any, len(t[s]))
		for i, hp := range t[s] {
			pairs[i] = []string{hp.Host, hp.Prefix}
		}
		obj = append(obj, jsjson.Member{Key: s, Value: pairs})
	}
	return append(jsjson.Marshal(obj), '\n')
}

// Reasons an apply URL is refused.
const (
	ReasonNotHTTPS       = "not-https"
	ReasonMalformed      = "malformed"
	ReasonCredentials    = "credentials-or-port"
	ReasonUnknownCompany = "unknown-company"
	ReasonOffList        = "off-list"
)

// ApplyCheck is the result of CheckApplyURL. Host is nil when the URL did
// not parse.
type ApplyCheck struct {
	OK     bool
	Reason string
	Host   *string
}

// CheckApplyURL is checkApplyUrl: https, no credentials or port, and a host
// plus path prefix on the company's allowlist. Hosts compare exactly;
// prefixes compare case-insensitively on whole path segments.
func CheckApplyURL(companySlug, rawURL string, table Table) ApplyCheck {
	u, err := weburl.Parse(rawURL)
	if err != nil {
		return ApplyCheck{Reason: ReasonMalformed}
	}
	host := strings.ToLower(u.Hostname)
	if u.Protocol != "https:" {
		return ApplyCheck{Reason: ReasonNotHTTPS, Host: &host}
	}
	if u.Username != "" || u.Password != "" || u.Port != "" {
		return ApplyCheck{Reason: ReasonCredentials, Host: &host}
	}
	allowed, ok := table[companySlug]
	if !ok {
		return ApplyCheck{Reason: ReasonUnknownCompany, Host: &host}
	}
	path := strings.TrimRight(strings.ToLower(u.Pathname), "/") + "/"
	for _, hp := range allowed {
		if hp.Host == host && strings.HasPrefix(path, strings.ToLower(hp.Prefix)) {
			return ApplyCheck{OK: true}
		}
	}
	return ApplyCheck{Reason: ReasonOffList, Host: &host}
}

// CheckApplyURL checks against this registry's allowlist.
func (r *Registry) CheckApplyURL(companySlug, rawURL string) ApplyCheck {
	return CheckApplyURL(companySlug, rawURL, r.applyHosts)
}

// ApplyHostMode selects what the publishing gate does with an off-list link.
// off: no checks. warn (default, shadow mode): violations are logged, nothing
// is hidden. enforce: violating links are not ingested, dumped or shown.
type ApplyHostMode string

const (
	ApplyHostsOff     ApplyHostMode = "off"
	ApplyHostsWarn    ApplyHostMode = "warn"
	ApplyHostsEnforce ApplyHostMode = "enforce"
)

// ApplyHostModeFromEnv reads OPENINTERN_APPLY_HOSTS; anything but "off" or
// "enforce" is warn.
func ApplyHostModeFromEnv() ApplyHostMode {
	switch v := ApplyHostMode(jsTrim(os.Getenv("OPENINTERN_APPLY_HOSTS"))); v {
	case ApplyHostsOff, ApplyHostsEnforce:
		return v
	}
	return ApplyHostsWarn
}

var (
	warnedMu sync.Mutex
	warned   = map[string]bool{}
)

// ApplyURLAllowed is applyUrlAllowed, the gate every publishing path calls.
// In warn mode it logs each violation once per process and returns true; in
// enforce mode it returns false.
func (r *Registry) ApplyURLAllowed(companySlug, rawURL string, mode ApplyHostMode) bool {
	if mode == ApplyHostsOff {
		return true
	}
	check := r.CheckApplyURL(companySlug, rawURL)
	if check.OK {
		return true
	}
	key := companySlug + " " + rawURL
	warnedMu.Lock()
	first := !warned[key]
	warned[key] = true
	warnedMu.Unlock()
	if first {
		verb := "would block"
		if mode == ApplyHostsEnforce {
			verb = "blocked"
		}
		fmt.Fprintf(os.Stderr, "[apply-hosts] %s %s: %s %s\n", verb, companySlug, check.Reason, rawURL)
	}
	return mode != ApplyHostsEnforce
}
