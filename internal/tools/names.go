package tools

import (
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/unicodex"
)

// jsLower is String.prototype.toLowerCase for what the callers keep:
// Go's simple case mapping differs from JavaScript's full mapping only for
// characters that lower to non-ASCII (İ, final sigma), which every caller
// then drops as non-ASCII anyway.
func jsLower(s string) string { return strings.ToLower(s) }

func isASCIIWord(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// Slugify is gap-report's slugify: lowercase, NFKD, drop everything but
// ASCII word characters, whitespace and hyphens, then hyphenate.
func Slugify(name string) string {
	s := unicodex.NFKD(jsLower(name))
	var b strings.Builder
	for _, r := range s { // .replace(/[^\w\s-]/g, "")
		if isASCIIWord(r) || r == '-' || htmlx.IsJSSpace(r) {
			b.WriteRune(r)
		}
	}
	s = htmlx.TrimJS(b.String())
	b.Reset()
	run := false // .replace(/[\s_]+/g, "-")
	for _, r := range s {
		if r == '_' || htmlx.IsJSSpace(r) {
			if !run {
				b.WriteByte('-')
			}
			run = true
			continue
		}
		run = false
		b.WriteRune(r)
	}
	return collapseHyphens(b.String())
}

// collapseHyphens is `.replace(/-+/g, "-")`.
func collapseHyphens(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// normalizeName is the name key both reports match registry entries by:
// lowercase, NFKD, & as "and", only ASCII letters and digits kept.
func normalizeName(name string) string {
	s := unicodex.NFKD(jsLower(name))
	s = strings.ReplaceAll(s, "&", " and ")
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var (
	reHTMLFirstCell = jsRegexp(`(?i)<tr[^>]*>\s*<td[^>]*>([\s\S]*?)</td>`)
	reAnchorText    = jsRegexp(`(?i)<a\s+href=["'][^"']+["'][^>]*>\s*(?:<strong>)?([^<]+?)(?:</strong>)?\s*</a>`)
	reTableRule     = jsRegexp(`^\s*\|?\s*:?-{3,}`)
	reMDBold        = regexp.MustCompile(`\*\*`)
	reMDLink        = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	reCompanyHeader = regexp.MustCompile(`(?i)^company$`)
	reNotCompany    = regexp.MustCompile(`(?i)^(company|position|role|location|salary|posting|age|application|faang)`)
	reURLStart      = regexp.MustCompile(`(?i)^https?:`)
	reAgeCell       = regexp.MustCompile(`(?i)^\d+[dhms]?$`)
	reASCIILetter   = regexp.MustCompile(`[a-zA-Z]`)
)

// jsRegexp compiles a JavaScript pattern: \s becomes JavaScript's Unicode
// whitespace class (Go's \s is ASCII only).
func jsRegexp(src string) *regexp.Regexp {
	cls := htmlx.JSSpace
	var b strings.Builder
	inClass := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src):
			switch {
			case src[i+1] == 's' && inClass:
				b.WriteString(cls)
			case src[i+1] == 's':
				b.WriteString("[" + cls + "]")
			case src[i+1] == 'S' && !inClass:
				b.WriteString("[^" + cls + "]")
			default:
				b.WriteByte(c)
				b.WriteByte(src[i+1])
			}
			i++
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return regexp.MustCompile(b.String())
}

// ExtractCompaniesFromMarkdown returns the company names in markdown and
// HTML table rows of a community list, first-seen order, without repeats.
func ExtractCompaniesFromMarkdown(md string) []string {
	var found []string
	seen := map[string]bool{}
	add := func(cell string) {
		if isPlausibleCompany(cell) && !seen[cell] {
			seen[cell] = true
			found = append(found, cell)
		}
	}
	for _, m := range reHTMLFirstCell.FindAllStringSubmatch(md, -1) {
		add(htmlx.Strip(m[1]))
	}
	for _, m := range reAnchorText.FindAllStringSubmatch(md, -1) {
		add(htmlx.Strip(m[1]))
	}
	for _, line := range strings.Split(md, "\n") {
		if !strings.Contains(line, "|") || reTableRule.MatchString(line) {
			continue
		}
		var cells []string
		for _, c := range strings.Split(line, "|") {
			c = htmlx.Strip(reMDLink.ReplaceAllString(reMDBold.ReplaceAllString(c, ""), "$1"))
			if c != "" {
				cells = append(cells, c)
			}
		}
		if len(cells) < 2 || reCompanyHeader.MatchString(cells[0]) {
			continue
		}
		add(cells[0])
	}
	return found
}

func isPlausibleCompany(name string) bool {
	if n := len16(name); n < 2 || n > 80 {
		return false
	}
	if reNotCompany.MatchString(name) || reURLStart.MatchString(name) || reAgeCell.MatchString(name) {
		return false
	}
	return reASCIILetter.MatchString(name)
}

// loadCompanyRows reads every *.yaml/*.yml file of DataDir/companies in
// name order (Node's readdir order). An invalid file fails the command, as
// companiesFileSchema.parse threw.
func (e Env) loadCompanyRows() ([]registry.CompanyYAML, error) {
	list, _, err := registry.LoadCompanies(os.DirFS(e.DataDir))
	return list, err
}

// registryIndex is gap-report's loadRegistry.
type registryIndex struct {
	byNorm  map[string]registry.CompanyYAML
	bySlug  map[string]registry.CompanyYAML
	byToken map[string]registry.CompanyYAML
}

func (e Env) loadRegistryIndex() (*registryIndex, error) {
	list, err := e.loadCompanyRows()
	if err != nil {
		return nil, err
	}
	r := &registryIndex{byNorm: map[string]registry.CompanyYAML{}, bySlug: map[string]registry.CompanyYAML{}, byToken: map[string]registry.CompanyYAML{}}
	for _, c := range list {
		r.bySlug[c.Slug] = c
		r.byNorm[normalizeName(c.Name)] = c
		r.byNorm[normalizeName(strings.ReplaceAll(c.Slug, "-", " "))] = c
		r.byToken[ats.TokenKey(c.ATS, c.BoardToken)] = c
	}
	return r, nil
}

// firstRuneUpper is `s.charAt(0).toUpperCase() + s.slice(1)` for the ASCII
// slugs it is used on.
func firstRuneUpper(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[n:]
}
