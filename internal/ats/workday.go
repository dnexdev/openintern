package ats

import (
	"regexp"
	"strings"
)

// WorkdayBoard is a public Workday CXS board. The token form is
// tenant|wdN|site, e.g. nvidia|wd5|NVIDIAExternalCareerSite.
type WorkdayBoard struct {
	Tenant     string
	Datacenter string
	Site       string
}

var wdDatacenter = regexp.MustCompile(`(?i)^wd\d+$`)

// ParseWorkdayToken parses tenant|wdN|site.
func ParseWorkdayToken(token string) (WorkdayBoard, bool) {
	var parts []string
	for _, p := range strings.Split(token, "|") {
		if p = jsTrim(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) != 3 || !wdDatacenter.MatchString(parts[1]) {
		return WorkdayBoard{}, false
	}
	return WorkdayBoard{Tenant: parts[0], Datacenter: strings.ToLower(parts[1]), Site: parts[2]}, true
}

// EncodeWorkdayToken is the inverse of ParseWorkdayToken.
func EncodeWorkdayToken(b WorkdayBoard) string {
	return b.Tenant + "|" + strings.ToLower(b.Datacenter) + "|" + b.Site
}

func workdayOrigin(b WorkdayBoard) string {
	return "https://" + b.Tenant + "." + b.Datacenter + ".myworkdayjobs.com"
}

// WorkdayJobsURL is the CXS list endpoint (POST).
func WorkdayJobsURL(b WorkdayBoard) string {
	return workdayOrigin(b) + "/wday/cxs/" + encodeURIComponent(b.Tenant) + "/" + encodeURIComponent(b.Site) + "/jobs"
}

// WorkdayCareersURL is the public careers page.
func WorkdayCareersURL(b WorkdayBoard) string {
	return workdayOrigin(b) + "/" + b.Site
}

func slashPath(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

// WorkdayDetailURL is the CXS detail endpoint for a posting's externalPath.
func WorkdayDetailURL(b WorkdayBoard, externalPath string) string {
	return workdayOrigin(b) + "/wday/cxs/" + encodeURIComponent(b.Tenant) + "/" + encodeURIComponent(b.Site) + slashPath(externalPath)
}

// WorkdayApplyURL is the public page of a posting.
func WorkdayApplyURL(b WorkdayBoard, externalPath string) string {
	return workdayOrigin(b) + "/" + b.Site + slashPath(externalPath)
}

var (
	wdHost   = regexp.MustCompile(`(?i)^([a-z0-9-]+)\.(wd\d+)\.myworkdayjobs\.com$`)
	wdLocale = regexp.MustCompile(`(?i)^[a-z]{2}(-[A-Z]{2})?$`)
	wdJobSeg = regexp.MustCompile(`(?i)^(job|jobs)$`)
)

// ParseWorkdayURL reads a board from a public Workday careers or job URL:
//
//	https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite
//	https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/...
//	https://selinc.wd1.myworkdayjobs.com/SEL/job/...
func ParseWorkdayURL(rawURL string) (WorkdayBoard, bool) {
	u, err := parseURL(jsTrim(rawURL))
	if err != nil {
		return WorkdayBoard{}, false
	}
	m := wdHost.FindStringSubmatch(strings.ToLower(u.Hostname))
	if m == nil {
		return WorkdayBoard{}, false
	}
	var filtered []string
	for _, p := range strings.Split(u.Pathname, "/") {
		// Drop locale prefixes like en-US.
		if p != "" && !wdLocale.MatchString(p) {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 || strings.ToLower(filtered[0]) == "wday" {
		return WorkdayBoard{}, false
	}
	site := filtered[0]
	if wdJobSeg.MatchString(site) {
		return WorkdayBoard{}, false
	}
	return WorkdayBoard{Tenant: m[1], Datacenter: strings.ToLower(m[2]), Site: site}, true
}
