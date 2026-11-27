package ats

// Map public Apply / careers URLs to supported ATS board tokens. Greenhouse
// embed job ids and generic career pages are not boards.

import (
	"regexp"
	"strings"

	"github.com/dnexdev/openintern/internal/htmlx"
)

// ParsedBoard is a board found in an Apply URL.
type ParsedBoard struct {
	ATS        string
	BoardToken string
	CareersURL string
}

var skipPathTokens = map[string]bool{
	"embed": true, "job_app": true, "jobs": true, "o": true,
	"application": true, "posting": true, "careers": true, "www": true,
}

func boardRoot(ats, token string) string {
	switch ats {
	case "greenhouse":
		return "https://job-boards.greenhouse.io/" + token
	case "lever":
		return "https://jobs.lever.co/" + token
	case "ashby":
		return "https://jobs.ashbyhq.com/" + token
	case "workable":
		return "https://apply.workable.com/" + token
	case "smartrecruiters":
		return "https://jobs.smartrecruiters.com/" + token
	case "recruitee":
		return "https://" + token + ".recruitee.com"
	case "rippling":
		return "https://ats.rippling.com/" + token
	case "bamboohr":
		return "https://" + token + ".bamboohr.com/careers"
	}
	return ""
}

var trailingSlashes = regexp.MustCompile(`/+$`)

// cleanToken decodes and trims a path segment; skip words and segments that
// do not decode are not tokens (the TypeScript threw a URIError there).
func cleanToken(raw string) (string, bool) {
	t, err := decodeURIComponent(raw)
	if err != nil {
		return "", false
	}
	t = trailingSlashes.ReplaceAllString(jsTrim(t), "")
	if t == "" || skipPathTokens[strings.ToLower(t)] {
		return "", false
	}
	return t, true
}

var (
	recruiteeHost = regexp.MustCompile(`(?i)^([a-z0-9-]+)\.recruitee\.com$`)
	bambooHost    = regexp.MustCompile(`(?i)^([a-z0-9-]+)\.bamboohr\.com$`)
)

func pathParts(pathname string) []string {
	var parts []string
	for _, p := range strings.Split(pathname, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// ParseATSFromApplyURL reads a board from one Apply URL. ok is false for an
// unsupported host or a path that names a job rather than a board.
func ParseATSFromApplyURL(rawURL string) (ParsedBoard, bool) {
	u, err := parseURL(jsTrim(rawURL))
	if err != nil {
		return ParsedBoard{}, false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname), "www.")
	parts := pathParts(u.Pathname)
	first := ""
	if len(parts) > 0 {
		first = parts[0]
	}
	simple := func(ats string, lower bool) (ParsedBoard, bool) {
		token, ok := cleanToken(first)
		if !ok {
			return ParsedBoard{}, false
		}
		if lower {
			token = strings.ToLower(token)
		}
		return ParsedBoard{ATS: ats, BoardToken: token, CareersURL: boardRoot(ats, token)}, true
	}
	switch host {
	case "job-boards.greenhouse.io", "boards.greenhouse.io", "boards-api.greenhouse.io":
		// embed/job_app?token=JOB_ID — not a board token
		if strings.ToLower(first) == "embed" {
			return ParsedBoard{}, false
		}
		return simple("greenhouse", true)
	case "jobs.ashbyhq.com":
		return simple("ashby", true)
	case "jobs.lever.co":
		return simple("lever", true)
	case "apply.workable.com":
		return simple("workable", true)
	case "jobs.smartrecruiters.com":
		// SmartRecruiters tokens are often case-sensitive.
		return simple("smartrecruiters", false)
	case "ats.rippling.com":
		return simple("rippling", true)
	}
	for _, sub := range []struct {
		re  *regexp.Regexp
		ats string
	}{{recruiteeHost, "recruitee"}, {bambooHost, "bamboohr"}} {
		if m := sub.re.FindStringSubmatch(host); m != nil {
			token, ok := cleanToken(m[1])
			if !ok {
				return ParsedBoard{}, false
			}
			token = strings.ToLower(token)
			return ParsedBoard{ATS: sub.ats, BoardToken: token, CareersURL: boardRoot(sub.ats, token)}, true
		}
	}
	// Workday CXS: tenant.wdN.myworkdayjobs.com/{site}/...
	if b, ok := ParseWorkdayURL(rawURL); ok {
		return ParsedBoard{ATS: "workday", BoardToken: EncodeWorkdayToken(b), CareersURL: WorkdayCareersURL(b)}, true
	}
	return ParsedBoard{}, false
}

// TokenKey identifies a board for de-duplication. SmartRecruiters and
// Workday site paths are case-sensitive.
func TokenKey(ats, token string) string {
	if ats == "smartrecruiters" || ats == "workday" {
		return ats + ":" + token
	}
	return ats + ":" + strings.ToLower(token)
}

// ATSHit is a company row whose Apply link maps to a supported board.
type ATSHit struct {
	CompanyName string
	ApplyURL    string
	ATS         string
	BoardToken  string
	CareersURL  string
}

// HostCount is an unsupported Apply host and how often it appeared.
type HostCount struct {
	Host  string
	Count int
}

// ExtractedBoards is the result of ExtractATSBoardsFromMarkdown.
type ExtractedBoards struct {
	Hits                  []ATSHit
	ApplyLinkCount        int
	UnsupportedApplyCount int
	// UnsupportedHosts is in first-seen order ("(invalid)" for unparseable URLs).
	UnsupportedHosts []HostCount
}

var (
	mdBold       = regexp.MustCompile(`\*\*`)
	mdLink       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	headerWord   = regexp.MustCompile(`(?i)^(company|position|role|location|salary|age)$`)
	asciiLetter  = regexp.MustCompile(`[a-zA-Z]`)
	htmlRow      = jsRegexp(`(?i)<tr[^>]*>([\s\S]*?)</tr>`)
	htmlCell     = jsRegexp(`(?i)<td[^>]*>([\s\S]*?)</td>`)
	altApply     = regexp.MustCompile(`(?i)alt=["']Apply["']`)
	applyImgLink = jsRegexp(`(?i)<a\s+[^>]*href=["'](https?:[^"']+)["'][^>]*>\s*<img[^>]*alt=["']Apply["']`)
	anyLink      = jsRegexp(`(?i)<a\s+[^>]*href=["'](https?:[^"']+)["']`)
	simplifyJobs = regexp.MustCompile(`(?i)simplify\.jobs`)
	mdApply      = regexp.MustCompile(`(?i)\[Apply\]\(`)
	mdRule       = jsRegexp(`^\s*\|?\s*:?-{3,}`)
	tdTag        = jsRegexp(`(?i)<td[\s>]`)
	mdApplyLink  = jsRegexp(`(?i)\[Apply\]\((https?:[^)\s]+)\)`)
	ampEntity    = regexp.MustCompile(`&amp;`)
)

func companyFromCell(raw string) (string, bool) {
	cell := mdLink.ReplaceAllString(mdBold.ReplaceAllString(raw, ""), "$1")
	cell = htmlx.Strip(cell)
	if cell == "" || cell == "↳" {
		return "", false
	}
	if n := len16(cell); n < 2 || n > 80 {
		return "", false
	}
	if headerWord.MatchString(cell) || !asciiLetter.MatchString(cell) {
		return "", false
	}
	return cell, true
}

// ExtractATSBoardsFromMarkdown finds company + Apply URL pairs in markdown
// pipe tables and HTML tables (the SimplifyJobs README), keeping rows whose
// Apply link maps to a supported board.
func ExtractATSBoardsFromMarkdown(md string) ExtractedBoards {
	var out ExtractedBoards
	hostIdx := map[string]int{}
	record := func(company, applyRaw string) {
		out.ApplyLinkCount++
		applyURL := ampEntity.ReplaceAllString(applyRaw, "&")
		parsed, ok := ParseATSFromApplyURL(applyURL)
		if !ok {
			out.UnsupportedApplyCount++
			host := "(invalid)"
			if u, err := parseURL(applyURL); err == nil {
				host = strings.TrimPrefix(strings.ToLower(u.Hostname), "www.")
			}
			if i, seen := hostIdx[host]; seen {
				out.UnsupportedHosts[i].Count++
			} else {
				hostIdx[host] = len(out.UnsupportedHosts)
				out.UnsupportedHosts = append(out.UnsupportedHosts, HostCount{Host: host, Count: 1})
			}
			return
		}
		out.Hits = append(out.Hits, ATSHit{
			CompanyName: company, ApplyURL: applyURL,
			ATS: parsed.ATS, BoardToken: parsed.BoardToken, CareersURL: parsed.CareersURL,
		})
	}

	// HTML tables: company in the first <td>, Apply as
	// <a href="..."><img alt="Apply"></a>.
	lastHTML := ""
	for _, tr := range htmlRow.FindAllStringSubmatch(md, -1) {
		var tds []string
		for _, m := range htmlCell.FindAllStringSubmatch(tr[1], -1) {
			tds = append(tds, m[1])
		}
		if len(tds) < 2 {
			continue
		}
		company, ok := companyFromCell(tds[0])
		if ok {
			lastHTML = company
		} else {
			company = lastHTML
		}
		if company == "" {
			continue
		}
		applyCell := tds[len(tds)-2]
		for _, td := range tds {
			if altApply.MatchString(td) {
				applyCell = td
				break
			}
		}
		var hrefs []string
		for _, m := range applyImgLink.FindAllStringSubmatch(applyCell, -1) {
			hrefs = append(hrefs, m[1])
		}
		if len(hrefs) == 0 {
			for _, m := range anyLink.FindAllStringSubmatch(applyCell, -1) {
				if !simplifyJobs.MatchString(m[1]) {
					hrefs = append(hrefs, m[1])
				}
			}
		}
		for _, h := range hrefs {
			record(company, h)
		}
	}

	// Markdown pipe tables with [Apply](url).
	lastMD := ""
	for _, line := range strings.Split(md, "\n") {
		if !strings.Contains(line, "|") || !mdApply.MatchString(line) {
			continue
		}
		if mdRule.MatchString(line) || tdTag.MatchString(line) {
			continue
		}
		var cells []string
		for _, c := range strings.Split(line, "|") {
			if c = jsTrim(c); c != "" {
				cells = append(cells, c)
			}
		}
		if len(cells) < 2 {
			continue
		}
		company, ok := companyFromCell(cells[0])
		if ok {
			lastMD = company
		} else {
			company = lastMD
		}
		if company == "" {
			continue
		}
		for _, m := range mdApplyLink.FindAllStringSubmatch(line, -1) {
			record(company, m[1])
		}
	}
	return out
}
