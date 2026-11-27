package ats

// Citadel / Citadel Securities careers: manual capture only.
//
// These WordPress careers pages sit behind a CDN that refuses automated
// clients, and OpenIntern does not pretend to be a browser. The maintainer
// saves the listing pages (`/careers/open-opportunities/`, `/page/N/`) in a
// normal browser and runs `openintern manual-capture citadel <files…>`, which
// writes data/manual/<brand>.json.
//
// board_token: careers path segment, usually `open-opportunities`.

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/model"
)

// CitadelOrigins maps each brand to its site.
var CitadelOrigins = map[string]string{
	"citadel":            "https://www.citadel.com",
	"citadel_securities": "https://www.citadelsecurities.com",
}

var (
	citadelNumericEntity = regexp.MustCompile(`&#(\d+);`)
	citadelTag           = regexp.MustCompile(`<[^>]+>`)
	citadelCard          = jsRegexp(`(?i)<a\b[^>]*class="[^"]*careers-listing-card[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
	citadelH2            = jsRegexp(`(?i)<h2[^>]*>([\s\S]*?)</h2>`)
	citadelPosition      = regexp.MustCompile(`(?i)data-position="([^"]+)"`)
	citadelLocation      = jsRegexp(`(?i)careers-listing-card__location[^>]*>([\s\S]*?)</`)
	commaSpaces          = jsRegexp(`,\s*`)
)

// fromCharCode is String.fromCharCode(Number(digits)).
func fromCharCode(digits string) string {
	f, err := strconv.ParseFloat(digits, 64)
	if err != nil || math.IsInf(f, 0) {
		f = 0
	}
	u := uint16(math.Mod(math.Trunc(f), 65536))
	return unitsString([]uint16{u})
}

func citadelDecodeEntities(s string) string {
	s = strings.ReplaceAll(s, "&#8211;", "–")
	s = strings.ReplaceAll(s, "&#8212;", "—")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = citadelNumericEntity.ReplaceAllStringFunc(s, func(m string) string {
		return fromCharCode(m[2 : len(m)-1])
	})
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	// Last, so "&amp;lt;" decodes to "&lt;" rather than "<".
	return strings.ReplaceAll(s, "&amp;", "&")
}

func citadelStripTags(html string) string {
	return citadelDecodeEntities(jsTrim(collapseJSSpace(citadelTag.ReplaceAllString(html, " "))))
}

// ParseCitadelListingHTML reads the listing cards of a saved careers page.
func ParseCitadelListingHTML(html, origin string) []model.NormalizedJob {
	var jobs []model.NormalizedJob
	seen := map[string]bool{}
	base, err := parseURL(origin)
	if err != nil {
		return nil
	}
	for _, m := range citadelCard.FindAllStringSubmatch(html, -1) {
		hrefRaw, body := m[1], m[2]
		href := hrefRaw
		href = stripArchivePrefix(href)
		u, err := resolveURL(href, base)
		if err != nil {
			continue
		}
		applyURL, _, _ := strings.Cut(u.String(), "?")
		if !isCitadelDetails(applyURL) {
			continue
		}
		tm := citadelH2.FindStringSubmatch(body)
		if tm == nil {
			tm = citadelPosition.FindStringSubmatch(body)
		}
		title := ""
		if tm != nil {
			title = citadelStripTags(tm[1])
		}
		if title == "" {
			continue
		}
		var locations []string
		if lm := citadelLocation.FindStringSubmatch(body); lm != nil {
			if locText := citadelStripTags(lm[1]); locText != "" {
				for _, s := range commaSpaces.Split(locText, -1) {
					if s = jsTrim(s); s != "" {
						locations = append(locations, s)
					}
				}
			}
		}
		parts := strings.Split(strings.TrimSuffix(applyURL, "/"), "/")
		slug := parts[len(parts)-1]
		if seen[slug] {
			continue
		}
		seen[slug] = true
		jobs = append(jobs, model.NormalizedJob{
			ExternalID: slug,
			Title:      title,
			Locations:  locs(locations),
			ApplyURL:   applyURL,
		})
	}
	return jobs
}

var edgeSlashes = regexp.MustCompile(`^/+|/+$`)

// CitadelListingURL is the public listing page (page 1, or /page/N/).
func CitadelListingURL(brand, token string, page int) string {
	origin := CitadelOrigins[brand]
	t := edgeSlashes.ReplaceAllString(token, "")
	if t == "" {
		t = "open-opportunities"
	}
	if page <= 1 {
		return origin + "/careers/" + t + "/"
	}
	return origin + "/careers/" + t + "/page/" + strconv.Itoa(page) + "/"
}

func locs(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// isCitadelDetails reports whether applyURL is a Citadel job detail page,
// checking the parsed path rather than anywhere in the URL.
func isCitadelDetails(applyURL string) bool {
	u, err := url.Parse(applyURL)
	return err == nil && strings.Contains(strings.ToLower(u.Path), "/careers/details/")
}

// stripArchivePrefix removes the first "http(s)://web.archive.org/web/<digits>/"
// from s, so archived captures resolve to the live Citadel URL.
func stripArchivePrefix(s string) string {
	const mid = "://web.archive.org/web/"
	for from := 0; ; {
		i := strings.Index(s[from:], mid)
		if i < 0 {
			return s
		}
		i += from
		start := -1
		if i >= 5 && s[i-5:i] == "https" {
			start = i - 5
		} else if i >= 4 && s[i-4:i] == "http" {
			start = i - 4
		}
		j := i + len(mid)
		k := j
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		if start >= 0 && k > j && k < len(s) && s[k] == '/' {
			return s[:start] + s[k+1:]
		}
		from = i + 1
	}
}
