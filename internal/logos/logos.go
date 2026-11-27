// Package logos fetches company logos once, at maintenance time, so the
// website can serve them from its own origin. Visitors' browsers never ask a
// company site or a favicon service for anything.
//
// A logo is the company's own icon (apple-touch-icon, <link rel="icon">, its
// web app manifest's icons, or /apple-touch-icon.png and /favicon.ico on its
// website_url), decoded with the standard library (PNG, JPEG, GIF, ICO with
// PNG or BMP entries) and re-encoded as a Size×Size PNG.
// Re-encoding means the committed files are plain pixels we wrote: no SVG, no
// scripts, nothing a browser could run.
package logos

import (
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
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
// not the company's. The vendor's own homepage (the bare domain or www.) is
// not a job board: it is where Rippling's or Lever's logo comes from.
var atsHost = regexp.MustCompile(`(?i)(?:^|\.)(greenhouse\.io|lever\.co|ashbyhq\.com|workable\.com|smartrecruiters\.com|recruitee\.com|rippling\.com|bamboohr\.com|myworkdayjobs\.com|myworkdaysite\.com|workday\.com|jobvite\.com|icims\.com|breezy\.hr|teamtailor\.com|personio\.de|personio\.com)$`)

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
		if jobBoardHost(u.Hostname()) {
			continue
		}
		return u.Scheme + "://" + u.Host + "/"
	}
	return ""
}

// jobBoardHost reports whether host is a hosted job board: a subdomain of an
// ATS vendor's domain other than www (jobs.lever.co, ats.rippling.com).
func jobBoardHost(host string) bool {
	m := atsHost.FindStringSubmatch(host)
	if m == nil {
		return false
	}
	h := strings.TrimPrefix(strings.ToLower(host), "www.")
	return h != strings.ToLower(m[1])
}

// Candidate is one icon URL found for a site, best first after sorting.
type Candidate struct {
	URL string
	// Rel is "apple-touch-icon", "icon", "manifest" (a web app manifest
	// icon), or "guess" for /apple-touch-icon.png and /favicon.ico on the
	// page's host, which many sites serve without declaring them.
	Rel  string
	Size int // largest edge declared in sizes="", 0 when unknown
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

// IconLinks lists the raster icons a page declares, best first (see
// sortCandidates), then the /apple-touch-icon.png and /favicon.ico guesses on
// the page's host. SVG and mask icons are skipped (the standard library
// cannot rasterize them).
func IconLinks(page []byte, pageURL string) []Candidate {
	out, _ := pageIcons(page, pageURL)
	return out
}

// pageIcons is IconLinks plus the page's web app manifest URL ("" when it
// declares none). Sites whose <link rel="icon"> is SVG-only often list PNGs
// in their manifest.
func pageIcons(page []byte, pageURL string) (out []Candidate, manifest string) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, ""
	}
	head := page
	if i := indexFold(head, "</head>"); i >= 0 {
		head = head[:i]
	}
	hasTouch := false
	if m := baseTag.Find(head); m != nil {
		if href := attrs(string(m))["href"]; href != "" {
			if b, err := base.Parse(href); err == nil {
				base = b
			}
		}
	}
	seen := map[string]bool{}
	for _, tag := range linkTag.FindAll(head, -1) {
		a := attrs(string(tag))
		rel := strings.Fields(strings.ToLower(a["rel"]))
		if len(rel) == 1 && rel[0] == "manifest" {
			href := strings.TrimSpace(a["href"])
			if u, err := base.Parse(href); manifest == "" && href != "" && err == nil && (u.Scheme == "https" || u.Scheme == "http") {
				manifest = u.String()
			}
			continue
		}
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
		u := rasterURL(base, a["href"], a["type"])
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		size := declaredSize(a["sizes"])
		if size == 0 && kind == "apple-touch-icon" {
			size = 180 // the size iOS asks for when none is declared
		}
		if kind == "apple-touch-icon" {
			hasTouch = true
		}
		out = append(out, Candidate{URL: u, Rel: kind, Size: size})
	}
	sortCandidates(out)
	return append(out, guesses(base, seen, hasTouch)...), manifest
}

// rasterURL resolves an icon href against base. It is "" for data: URLs,
// non-HTTP schemes and SVG, which the standard library cannot rasterize.
func rasterURL(base *url.URL, href, typ string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "data:") {
		return ""
	}
	u, err := base.Parse(href)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	if strings.Contains(strings.ToLower(typ), "svg") || strings.HasSuffix(strings.ToLower(u.Path), ".svg") {
		return ""
	}
	return u.String()
}

// sortCandidates orders declared icons best first: larger declared sizes
// first, then apple-touch-icon before manifest icons before plain icons.
func sortCandidates(c []Candidate) {
	rank := map[string]int{"apple-touch-icon": 0, "manifest": 1, "icon": 2}
	sort.SliceStable(c, func(i, j int) bool {
		if c[i].Size != c[j].Size {
			return c[i].Size > c[j].Size
		}
		return rank[c[i].Rel] < rank[c[j].Rel]
	})
}

// guesses are the conventional icon paths on the page's host that it did not
// declare: /apple-touch-icon.png (iOS asks for it on every site, so many
// serve it unannounced) unless an apple-touch-icon was declared, then
// /favicon.ico.
func guesses(base *url.URL, seen map[string]bool, hasTouch bool) []Candidate {
	var out []Candidate
	paths := []string{"/apple-touch-icon.png", "/favicon.ico"}
	if hasTouch {
		paths = paths[1:]
	}
	for _, p := range paths {
		if u := base.ResolveReference(&url.URL{Path: p}).String(); !seen[u] {
			out = append(out, Candidate{URL: u, Rel: "guess"})
		}
	}
	return out
}

// ManifestIcons lists the raster icons in a web app manifest, resolved
// against the manifest's URL. Maskable icons (drawn with a wide safe-zone
// margin) are kept only when the manifest has no other kind; monochrome ones
// are skipped.
func ManifestIcons(body []byte, manifestURL string) []Candidate {
	base, err := url.Parse(manifestURL)
	if err != nil {
		return nil
	}
	var m struct {
		Icons []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if json.Unmarshal(body, &m) != nil {
		return nil
	}
	var plain, maskable []Candidate
	seen := map[string]bool{}
	for _, ic := range m.Icons {
		purpose := strings.Fields(strings.ToLower(ic.Purpose))
		if len(purpose) == 0 {
			purpose = []string{"any"}
		}
		u := rasterURL(base, ic.Src, ic.Type)
		if u == "" || seen[u] {
			continue
		}
		c := Candidate{URL: u, Rel: "manifest", Size: declaredSize(ic.Sizes)}
		switch {
		case slices.Contains(purpose, "any"):
			plain = append(plain, c)
		case slices.Contains(purpose, "maskable"):
			maskable = append(maskable, c)
		default: // monochrome only
			continue
		}
		seen[u] = true
	}
	if len(plain) == 0 {
		plain = maskable
	}
	sortCandidates(plain)
	return plain
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
