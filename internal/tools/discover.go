package tools

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/registry/weburl"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

const ycHiringURL = "https://yc-oss.github.io/api/companies/hiring.json"

var a16zPortfolioURLs = []string{"https://a16z.com/portfolio-companies/", "https://a16z.com/portfolio/"}

var (
	techIndustry = regexp.MustCompile(`(?i)^(B2B|Consumer|Fintech|Healthcare|Education|Industrials|Unspecified)`)
	techTags     = regexp.MustCompile(`(?i)ai|artificial intelligence|machine learning|developer tools|saas|fintech|infra|security|crypto|devtools|api|data|robotics|hardware|semiconductor|climate|biotech|health tech|open source`)
	ycDead       = regexp.MustCompile(`(?i)inactive|dead`)
	notCompany   = regexp.MustCompile(`(?i)follow|subscribe|twitter|linkedin|careers|privacy|terms`)
	jsonLDPair   = jsRegexp(`"name"\s*:\s*"([^"]{2,80})"\s*,\s*"url"\s*:\s*"(https?://[^"]+)"`)
	leadingWWW   = regexp.MustCompile(`^www\.`)
)

// discoverATSOrder is the order boards are guessed in: common startup ATSes
// first.
var discoverATSOrder = []string{"ashby", "greenhouse", "lever", "rippling", "workable", "smartrecruiters", "recruitee", "bamboohr"}

// DiscoverUsage is the usage line the script printed.
const DiscoverUsage = "Usage: discover-accelerators.ts --yc | --a16z [--write] [--limit=N]"

// ycCompany is one entry of hiring.json, read as the script read it.
type ycCompany struct {
	name, slug, batch string // batch is "hiring" when absent or null
	website           *string
	industry          any
	tags              any
	status            any
}

// jsTruthy is JavaScript truthiness for a JSON value.
func jsTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case string:
		return x != ""
	}
	return true
}

// jsStr is String(v) for the JSON values these fields hold.
func jsStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return jsString(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if e != nil {
				parts[i] = jsStr(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

func isTechYC(c ycCompany) (bool, error) {
	if jsTruthy(c.status) && ycDead.MatchString(jsStr(c.status)) {
		return false, nil
	}
	if jsTruthy(c.industry) && techIndustry.MatchString(jsStr(c.industry)) {
		return true, nil
	}
	if c.tags != nil {
		tags, ok := c.tags.([]any)
		if !ok {
			return false, errors.New("TypeError: (c.tags ?? []).some is not a function")
		}
		for _, t := range tags {
			if techTags.MatchString(jsStr(t)) {
				return true, nil
			}
		}
	}
	// Default include hiring companies without industry: still a
	// tech-leaning YC set.
	return !jsTruthy(c.industry), nil
}

// tokenCandidatesFromName guesses board tokens for a company we have no
// token for: its slug, name spellings and website host, at most four.
func tokenCandidatesFromName(slug, name string, website *string) []string {
	compact := asciiAlnum(jsLower(name))
	dashed := Slugify(name)
	if dashed == "" {
		dashed = slug
	}
	nospace := strings.ReplaceAll(dashed, "-", "")
	first := strings.Split(dashed, "-")[0]
	set := newOrderedSet(dashed, nospace, compact, first, slug)
	if website != nil {
		if u, err := weburl.Parse(*website); err == nil {
			host := leadingWWW.ReplaceAllString(u.Hostname, "")
			if base := strings.Split(host, ".")[0]; len16(base) >= 2 {
				set.add(base)
			}
		}
	}
	var out []string
	for _, t := range set.list {
		if n := len16(t); n >= 2 && n <= 48 {
			out = append(out, t)
		}
	}
	return out[:min(4, len(out))]
}

// asciiAlnum is `.replace(/[^a-z0-9]/g, "")`.
func asciiAlnum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type orderedSet struct {
	list []string
	seen map[string]bool
}

func newOrderedSet(items ...string) *orderedSet {
	s := &orderedSet{seen: map[string]bool{}}
	for _, it := range items {
		s.add(it)
	}
	return s
}

func (s *orderedSet) add(it string) {
	if !s.seen[it] {
		s.seen[it] = true
		s.list = append(s.list, it)
	}
}

type discoverRegistry struct {
	bySlug, byToken, byNormName map[string]bool
}

type found struct {
	name, slug, ats, token, source string
	website                        *string
}

func probeCompany(ctx context.Context, name, slugHint string, website *string, reg *discoverRegistry, source string) *found {
	if reg.byNormName[normalizeName(name)] {
		return nil
	}
	tokens := tokenCandidatesFromName(slugHint, name, website)
	for _, atsName := range discoverATSOrder {
		for _, token := range tokens {
			if reg.byToken[ats.TokenKey(atsName, token)] {
				continue
			}
			if !probeOK(ctx, atsName, token) {
				continue
			}
			// An empty board that answers is accepted too.
			base := Slugify(name)
			if base == "" {
				base = slugHint
			}
			if base == "" {
				base = strings.ToLower(token)
			}
			slug := base
			for i := 2; reg.bySlug[slug]; i++ {
				slug = fmt.Sprintf("%s-%d", base, i)
			}
			return &found{name: name, slug: slug, ats: atsName, token: token, website: website, source: source}
		}
	}
	return nil
}

func fetchYCHiring(ctx context.Context, limit float64) ([]ycCompany, error) {
	text, err := fetchText(ctx, ycHiringURL, "application/json")
	if err != nil {
		var msg = err.Error()
		if strings.HasPrefix(msg, "HTTP ") {
			// "HTTP 503 for <url>" → the script's "YC hiring.json HTTP 503".
			msg = "YC hiring.json " + strings.SplitN(msg, " for ", 2)[0]
		}
		return nil, errors.New(msg)
	}
	v, err := jsjson.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("SyntaxError: %w", err)
	}
	all, ok := v.([]any)
	if !ok {
		return nil, errors.New("TypeError: all.filter is not a function")
	}
	var tech []ycCompany
	for _, it := range all {
		o, _ := it.(jsjson.Object)
		c := ycCompany{}
		get := func(k string) any { x, _ := o.Get(k); return x }
		c.name = jsStr(get("name"))
		if s, ok := get("slug").(string); ok {
			c.slug = s
		}
		c.batch = "hiring" // c.batch ?? "hiring"
		if b := get("batch"); b != nil {
			c.batch = jsStr(b)
		}
		if s, ok := get("website").(string); ok {
			c.website = &s
		}
		c.industry, c.tags, c.status = get("industry"), get("tags"), get("status")
		ok, err := isTechYC(c)
		if err != nil {
			return nil, err
		}
		if ok {
			tech = append(tech, c)
		}
	}
	return sliceTo(tech, limit), nil
}

type portfolioCompany struct {
	name    string
	website *string
}

// a16zSeed is used when the portfolio page is JavaScript-rendered or sparse.
var a16zSeed = [][2]string{
	{"OpenAI", "https://openai.com"}, {"Databricks", "https://databricks.com"},
	{"Stripe", "https://stripe.com"}, {"Ramp", "https://ramp.com"},
	{"Anduril", "https://www.anduril.com"}, {"Figma", "https://www.figma.com"},
	{"Notion", "https://www.notion.so"}, {"Roblox", "https://www.roblox.com"},
	{"Coinbase", "https://www.coinbase.com"}, {"Instacart", "https://www.instacart.com"},
	{"Pinterest", "https://www.pinterest.com"}, {"Airbnb", "https://www.airbnb.com"},
	{"Okta", "https://www.okta.com"}, {"Snowflake", "https://www.snowflake.com"},
	{"MongoDB", "https://www.mongodb.com"}, {"Samsara", "https://www.samsara.com"},
	{"Robinhood", "https://robinhood.com"}, {"Affirm", "https://www.affirm.com"},
	{"Chime", "https://www.chime.com"}, {"Scale AI", "https://scale.com"},
	{"Hugging Face", "https://huggingface.co"}, {"Character.AI", "https://character.ai"},
	{"Anysphere", "https://cursor.com"}, {"Retool", "https://retool.com"},
	{"Vercel", "https://vercel.com"}, {"Linear", "https://linear.app"},
	{"Mercury", "https://mercury.com"}, {"Rippling", "https://www.rippling.com"},
	{"Deel", "https://www.deel.com"}, {"Brex", "https://www.brex.com"},
	{"Plaid", "https://plaid.com"}, {"Faire", "https://www.faire.com"},
	{"Flexport", "https://www.flexport.com"}, {"Navan", "https://navan.com"},
	{"Asana", "https://asana.com"}, {"Box", "https://www.box.com"},
	{"Airtable", "https://www.airtable.com"}, {"Webflow", "https://webflow.com"},
	{"Fivetran", "https://www.fivetran.com"}, {"dbt Labs", "https://www.getdbt.com"},
	{"Harness", "https://www.harness.io"}, {"Snyk", "https://snyk.io"},
	{"Wiz", "https://www.wiz.io"}, {"Cribl", "https://cribl.io"},
	{"Gong", "https://www.gong.io"}, {"Ironclad", "https://ironcladapp.com"},
	{"Dialpad", "https://www.dialpad.com"}, {"Opendoor", "https://www.opendoor.com"},
	{"NerdWallet", "https://www.nerdwallet.com"}, {"Reddit", "https://www.reddit.com"},
}

// fetchA16zPortfolio is a best-effort read of the a16z portfolio pages for
// company names and websites, topped up with a curated seed.
func fetchA16zPortfolio(ctx context.Context, limit float64) []portfolioCompany {
	var list []portfolioCompany
	seen := map[string]bool{}
	add := func(name, website string) {
		key := normalizeName(name)
		if !seen[key] {
			seen[key] = true
			w := website
			list = append(list, portfolioCompany{name: name, website: &w})
		}
	}
	for _, url := range a16zPortfolioURLs {
		html, err := fetchText(ctx, url, "text/html,application/xhtml+xml")
		if err != nil {
			continue
		}
		// Portfolio cards often link out: <a href="https://company.com">Name</a>
		for _, m := range outboundAnchors(html) {
			website := m[0]
			name := htmlx.TrimJS(collapseJSSpaceRuns(m[1]))
			if !reASCIILetter.MatchString(name) || notCompany.MatchString(name) || hostIn(website, "linkedin.com", "twitter.com", "x.com", "facebook.com", "youtube.com") {
				continue
			}
			add(name, website)
		}
		// JSON-LD or embedded portfolio blobs.
		for _, m := range jsonLDPair.FindAllStringSubmatch(html, -1) {
			if hostIn(m[2], "a16z.com") {
				continue
			}
			add(m[1], m[2])
		}
	}
	for _, s := range a16zSeed {
		add(s[0], s[1])
	}
	var out []portfolioCompany
	for _, c := range list {
		if !reURLStart.MatchString(c.name) && reASCIILetter.MatchString(c.name) {
			out = append(out, c)
		}
	}
	return sliceTo(out, limit)
}

// collapseJSSpaceRuns is `.replace(/\s+/g, " ")`.
func collapseJSSpaceRuns(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range s {
		if htmlx.IsJSSpace(r) {
			if !prev {
				b.WriteByte(' ')
			}
			prev = true
			continue
		}
		prev = false
		b.WriteRune(r)
	}
	return b.String()
}

// outboundAnchors finds what the script matched with
//
//	/<a[^>]+href=["'](https?:\/\/(?!(?:www\.)?a16z\.com)[^"']+)["'][^>]*>([^<]{2,80})<\/a>/gi
//
// returning [url, text] pairs. Go's regexp has no lookahead, so this walks
// the same backtracking order by hand: at each "<a", the last href= in the
// tag first (the greedy [^>]+), falling back to earlier ones; the URL runs to
// the next quote, the tag to the next ">", and the text to the next "<",
// which must start "</a>".
func outboundAnchors(s string) [][2]string {
	var out [][2]string
	lower := asciiLower(s) // offsets unchanged
	for p := 0; p+2 <= len(s); {
		i := strings.Index(lower[p:], "<a")
		if i < 0 {
			break
		}
		start := p + i
		end, pair, ok := matchAnchorAt(s, lower, start)
		if !ok {
			p = start + 1
			continue
		}
		out = append(out, pair)
		p = end
	}
	return out
}

// asciiLower lowercases ASCII letters only, so byte offsets stay valid.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func matchAnchorAt(s, lower string, start int) (int, [2]string, bool) {
	tagEnd := strings.IndexByte(s[start+2:], '>') // [^>]+ stops before the first '>'
	limit := len(s)
	if tagEnd >= 0 {
		limit = start + 2 + tagEnd
	}
	// Candidate href= positions h with start+3 <= h and h+6 <= limit, latest first.
	for h := limit - 6; h >= start+3; h-- {
		if lower[h:h+5] != "href=" || (s[h+5] != '"' && s[h+5] != '\'') {
			continue
		}
		u := h + 6
		rest := lower[u:]
		var scheme int
		switch {
		case strings.HasPrefix(rest, "https://"):
			scheme = 8
		case strings.HasPrefix(rest, "http://"):
			scheme = 7
		default:
			continue
		}
		after := rest[scheme:]
		if strings.HasPrefix(after, "a16z.com") || strings.HasPrefix(after, "www.a16z.com") {
			continue
		}
		q := strings.IndexAny(s[u+scheme:], `"'`)
		if q < 0 {
			continue
		}
		urlEnd := u + scheme + q
		if urlEnd == u+scheme { // [^"']+ needs one character
			continue
		}
		gt := strings.IndexByte(s[urlEnd+1:], '>')
		if gt < 0 {
			continue
		}
		textStart := urlEnd + 1 + gt + 1
		lt := strings.IndexByte(s[textStart:], '<')
		if lt < 0 {
			continue
		}
		text := s[textStart : textStart+lt]
		if n := len16(text); n < 2 || n > 80 {
			continue
		}
		if !strings.HasPrefix(lower[textStart+lt:], "</a>") {
			continue
		}
		return textStart + lt + 4, [2]string{s[u:urlEnd], text}, true
	}
	return 0, [2]string{}, false
}

func yamlEscape(v string) string {
	if yamlSpecial.MatchString(v) || strings.Contains(v, "'") || strings.Contains(v, ".") {
		return jsonQuote(v)
	}
	return v
}

func appendAccelerators(env Env, additions []*found, marker string) error {
	if len(additions) == 0 {
		return nil
	}
	path := filepath.Join(env.companiesDir(), "tech-accelerators.yaml")
	raw := ""
	if b, err := os.ReadFile(path); err == nil {
		raw = string(b)
	}
	used := map[string]bool{}
	if htmlx.TrimJS(raw) != "" {
		existing, _, err := registry.ParseCompaniesFile("data/companies/tech-accelerators.yaml", []byte(raw))
		if err != nil {
			raw = "" // start fresh if corrupt or empty
		}
		for _, c := range existing {
			used[c.Slug] = true
		}
	}
	var block []string
	for _, f := range additions {
		slug := f.slug
		for i := 2; used[slug]; i++ {
			slug = fmt.Sprintf("%s-%d", f.slug, i)
		}
		used[slug] = true
		lines := []string{
			"  - name: " + yamlEscape(f.name),
			"    slug: " + slug,
			"    ats: " + f.ats,
			"    board_token: " + yamlEscape(f.token),
		}
		if f.website != nil && *f.website != "" {
			lines = append(lines, "    website_url: "+*f.website)
		}
		block = append(block, strings.Join(lines, "\n"))
	}
	if htmlx.TrimJS(raw) == "" {
		raw = "# YC / a16z accelerator ATS discoveries (probe-OK only)\ncompanies:\n" + strings.Join(block, "\n") + "\n"
	} else {
		if !strings.HasSuffix(raw, "\n") {
			raw += "\n"
		}
		raw += "\n# " + marker + "\n" + strings.Join(block, "\n") + "\n"
	}
	if _, _, err := registry.ParseCompaniesFile("data/companies/tech-accelerators.yaml", []byte(raw)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(raw), 0o644)
}

// DiscoverAccelerators is discover-accelerators.ts. It returns the exit
// code for a usage error (1) and an error for anything the script threw.
func DiscoverAccelerators(ctx context.Context, env Env, args []string) (int, error) {
	doYC := hasArg(args, "--yc")
	doA16z := hasArg(args, "--a16z")
	write := hasArg(args, "--write")
	limit := 200.0
	if n, ok := numberArg(args, "--limit="); ok {
		limit = math.Max(1, orDefault(n, 200))
	}
	if !doYC && !doA16z {
		fmt.Fprintln(env.Stderr, DiscoverUsage)
		return 1, nil
	}
	rows, err := env.loadCompanyRows()
	if err != nil {
		return 1, err
	}
	reg := &discoverRegistry{bySlug: map[string]bool{}, byToken: map[string]bool{}, byNormName: map[string]bool{}}
	for _, c := range rows {
		reg.bySlug[c.Slug] = true
		reg.byToken[ats.TokenKey(c.ATS, c.BoardToken)] = true
		reg.byNormName[normalizeName(c.Name)] = true
	}
	w := env.Stdout
	var all []*found
	keep := func(hit *found) {
		all = append(all, hit)
		reg.bySlug[hit.slug] = true
		reg.byToken[ats.TokenKey(hit.ats, hit.token)] = true
		reg.byNormName[normalizeName(hit.name)] = true
		fmt.Fprintf(w, "  + %s → %s/%s\n", hit.name, hit.ats, hit.token)
	}
	ycTried, a16zTried := 0, 0
	if doYC {
		fmt.Fprintf(w, "Fetching YC hiring.json (tech filter, limit %s)…\n", jsString(limit))
		companies, err := fetchYCHiring(ctx, limit)
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(w, "YC candidates: %d\n", len(companies))
		for _, c := range companies {
			ycTried++
			if ycTried%25 == 0 {
				fmt.Fprintf(w, "  probed %d/%d…\n", ycTried, len(companies))
			}
			hint := c.slug
			if hint == "" {
				hint = Slugify(c.name)
			}
			if hit := probeCompany(ctx, c.name, hint, c.website, reg, "yc:"+c.batch); hit != nil {
				keep(hit)
			}
		}
	}
	if doA16z {
		fmt.Fprintf(w, "Fetching a16z portfolio (limit %s)…\n", jsString(limit))
		companies := fetchA16zPortfolio(ctx, limit)
		fmt.Fprintf(w, "a16z candidates: %d\n", len(companies))
		for _, c := range companies {
			a16zTried++
			if hit := probeCompany(ctx, c.name, Slugify(c.name), c.website, reg, "a16z-portfolio"); hit != nil {
				keep(hit)
			}
		}
	}
	fmt.Fprint(w, "\n## Summary\n")
	fmt.Fprintf(w, "YC probed: %d\n", ycTried)
	fmt.Fprintf(w, "a16z probed: %d\n", a16zTried)
	fmt.Fprintf(w, "New boards found: %d\n", len(all))
	for _, f := range all {
		fmt.Fprintf(w, "- %s (%s) %s/%s [%s]\n", f.name, f.slug, f.ats, f.token, f.source)
	}
	if write {
		if err := appendAccelerators(env, all, "Added from accelerator ATS discovery ("+isoDate(env.now())+")"); err != nil {
			return 1, err
		}
		fmt.Fprintf(w, "\nWrote %d companies to data/companies/tech-accelerators.yaml\n", len(all))
	} else {
		fmt.Fprint(w, "\nDry run — pass --write to append probe-OK companies.\n")
	}
	return 0, nil
}

// hostIn reports whether rawURL's host is one of domains or a subdomain of
// one. Matching the parsed host (not a substring of the URL) keeps
// "fedex.com" from counting as x.com.
func hostIn(rawURL string, domains ...string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
