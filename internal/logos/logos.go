// Package logos fetches company logos once, at maintenance time, so the
// website can serve them from its own origin. Visitors' browsers never ask a
// company site or a favicon service for anything.
//
// A logo is the company's own icon (apple-touch-icon, <link rel="icon">, or
// /favicon.ico on its website_url), decoded with the standard library (PNG,
// JPEG, GIF, ICO with PNG or BMP entries) and re-encoded as a Size×Size PNG.
// Re-encoding means the committed files are plain pixels we wrote: no SVG, no
// scripts, nothing a browser could run.
package logos

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Size is the edge of every committed logo, in pixels. The site draws marks at
// 20-24 CSS px, so 64 stays sharp at 2x and 3x.
const Size = 64

// FileName is the committed file for a company slug.
func FileName(slug string) string { return slug + ".png" }

// atsHost matches hosted job-board domains: their icon is the ATS vendor's,
// not the company's.
var atsHost = regexp.MustCompile(`(?i)(?:^|\.)(?:greenhouse\.io|lever\.co|ashbyhq\.com|workable\.com|smartrecruiters\.com|recruitee\.com|rippling\.com|bamboohr\.com|myworkdayjobs\.com|myworkdaysite\.com|workday\.com|jobvite\.com|icims\.com|breezy\.hr|teamtailor\.com|personio\.de|personio\.com)$`)

// Homepage is the page to read a company's icon from: website_url, or
// careers_url when that is on the company's own domain. It is "" when
// neither gives the company's site (the logo then stays an initials mark).
func Homepage(websiteURL, careersURL *string) string {
	for _, raw := range []*string{websiteURL, careersURL} {
		if raw == nil {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(*raw))
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
			continue
		}
		if atsHost.MatchString(u.Hostname()) {
			continue
		}
		return u.Scheme + "://" + u.Host + "/"
	}
	return ""
}

// Candidate is one icon URL found for a site, best first after sorting.
type Candidate struct {
	URL  string
	Rel  string // "apple-touch-icon", "icon", or "favicon" for the /favicon.ico guess
	Size int    // largest edge declared in sizes="", 0 when unknown
}

var (
	linkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	attr    = regexp.MustCompile(`(?is)([a-z][a-z0-9:_-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	sizeTok = regexp.MustCompile(`(?i)^(\d+)x(\d+)$`)
	baseTag = regexp.MustCompile(`(?is)<base\b[^>]*>`)
)

func attrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attr.FindAllStringSubmatch(tag, -1) {
		k := strings.ToLower(m[1])
		if _, seen := out[k]; seen {
			continue
		}
		out[k] = htmlUnescape(m[2] + m[3] + m[4])
	}
	return out
}

// htmlUnescape decodes the entities that show up in href attributes.
func htmlUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return strings.NewReplacer("&amp;", "&", "&#38;", "&", "&#x26;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(s)
}

// IconLinks lists the raster icons a page declares, best first: larger
// declared sizes first, apple-touch-icon before icon when sizes tie, then
// /favicon.ico on the page's host. SVG and mask icons are skipped (the
// standard library cannot rasterize them).
func IconLinks(page []byte, pageURL string) []Candidate {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	head := page
	if i := indexFold(head, "</head>"); i >= 0 {
		head = head[:i]
	}
	if m := baseTag.Find(head); m != nil {
		if href := attrs(string(m))["href"]; href != "" {
			if b, err := base.Parse(href); err == nil {
				base = b
			}
		}
	}
	var out []Candidate
	seen := map[string]bool{}
	for _, tag := range linkTag.FindAll(head, -1) {
		a := attrs(string(tag))
		rel := strings.Fields(strings.ToLower(a["rel"]))
		kind := ""
		for _, r := range rel {
			switch r {
			case "apple-touch-icon", "apple-touch-icon-precomposed":
				kind = "apple-touch-icon"
			case "icon":
				if kind == "" {
					kind = "icon"
				}
			case "mask-icon":
				kind = "skip"
			}
		}
		if kind == "" || kind == "skip" {
			continue
		}
		href := strings.TrimSpace(a["href"])
		if href == "" || strings.HasPrefix(href, "data:") {
			continue
		}
		typ := strings.ToLower(a["type"])
		u, err := base.Parse(href)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		if strings.Contains(typ, "svg") || strings.HasSuffix(strings.ToLower(u.Path), ".svg") {
			continue
		}
		s := u.String()
		if seen[s] {
			continue
		}
		seen[s] = true
		size := declaredSize(a["sizes"])
		if size == 0 && kind == "apple-touch-icon" {
			size = 180 // the size iOS asks for when none is declared
		}
		out = append(out, Candidate{URL: s, Rel: kind, Size: size})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Rel == "apple-touch-icon" && out[j].Rel != "apple-touch-icon"
	})
	fav := base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()
	if !seen[fav] {
		out = append(out, Candidate{URL: fav, Rel: "favicon"})
	}
	return out
}

func declaredSize(sizes string) int {
	best := 0
	for _, tok := range strings.Fields(sizes) {
		m := sizeTok.FindStringSubmatch(tok)
		if m == nil {
			continue
		}
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		best = max(best, w, h)
	}
	return best
}

func indexFold(b []byte, sub string) int {
	return strings.Index(strings.ToLower(string(b)), sub)
}
