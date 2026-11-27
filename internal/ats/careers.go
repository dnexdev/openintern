package ats

// Proprietary careers sites read over HTTP: Phenom, Eightfold, Avature (and
// IBM on it), iCIMS, Google, D. E. Shaw and Shopify.

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/safedate"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// lowerHead8 is raw.slice(0, 8).toLowerCase().
func lowerHead8(raw string) string { return strings.ToLower(slice16(raw, 8)) }

func hasHTTPScheme(raw string) bool {
	h := lowerHead8(raw)
	return strings.HasPrefix(h, "https://") || strings.HasPrefix(h, "http://")
}

// stripScheme drops a leading http:// or https:// (any case).
func stripScheme(raw string) string {
	h := lowerHead8(raw)
	switch {
	case strings.HasPrefix(h, "https://"):
		return raw[8:]
	case strings.HasPrefix(h, "http://"):
		return raw[7:]
	}
	return raw
}

func probeFromJobs(jobs []model.NormalizedJob, err error) ProbeResult {
	if err != nil {
		return ProbeResult{Error: err.Error()}
	}
	n := len(jobs)
	return ProbeResult{OK: n > 0, Status: 200, Count: &n}
}

func failureStatus(err error) int {
	var f *fetch.Failure
	if errors.As(err, &f) {
		return f.Status
	}
	return 0
}

// toNumber is JavaScript ToNumber for JSON values.
func toNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case nil:
		return 0, true
	case string:
		t := jsTrim(x)
		if t == "" {
			return 0, true
		}
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}

// ---- Phenom People (careers.<company>.com/api/jobs) ----

const (
	phenomPageLimit = 100
	phenomMaxPages  = 30
)

// PhenomCareersOrigin maps a token (`amd`, `careers.amd.com` or a full
// origin) to the careers origin.
func PhenomCareersOrigin(token string) string {
	raw := strings.TrimRight(jsTrim(token), "/")
	if hasHTTPScheme(raw) {
		return raw
	}
	if strings.Contains(raw, ".") {
		return "https://" + raw
	}
	return "https://careers." + raw + ".com"
}

// PhenomJobsAPIURL is the jobs API page URL (pages start at 1).
func PhenomJobsAPIURL(token string, page int) (string, error) {
	u, err := parseURL(PhenomCareersOrigin(token) + "/api/jobs")
	if err != nil {
		return "", err
	}
	u.setParam("keywords", "")
	u.setParam("limit", strconv.Itoa(phenomPageLimit))
	u.setParam("page", strconv.Itoa(page))
	u.setParam("sortBy", "relevance")
	u.setParam("descending", "false")
	u.setParam("internal", "false")
	return u.String(), nil
}

func phenomLocations(d any) []string {
	var out []string
	push := func(v any) {
		s, ok := v.(string)
		if !ok {
			return
		}
		if t := jsTrim(s); t != "" && !containsStr(out, t) {
			out = append(out, t)
		}
	}
	push(get(d, "full_location"))
	push(get(d, "short_location"))
	if truthy(get(d, "city")) || truthy(get(d, "state")) || truthy(get(d, "country")) {
		push(filterJoin(", ", get(d, "city"), get(d, "state"), get(d, "country")))
	}
	push(get(d, "location_name"))
	for _, loc := range array(get(d, "multipleLocations")) {
		push(or(get(loc, "name"), filterJoin(", ", get(loc, "city"), get(loc, "state"), get(loc, "country"))))
	}
	return out
}

func trimmedText(v any) string {
	if s, ok := v.(string); ok {
		return jsTrim(s)
	}
	return ""
}

// MapPhenomJobs turns API rows into jobs.
func MapPhenomJobs(rows []any, origin string) []model.NormalizedJob {
	var jobs []model.NormalizedJob
	for _, row := range rows {
		d := get(row, "data")
		if !truthy(get(d, "title")) {
			continue
		}
		id := jsTrim(jsString(or(get(d, "req_id"), get(d, "slug"), "")))
		if id == "" {
			continue
		}
		applyURL := trimmedText(get(d, "meta_data", "canonical_url"))
		if applyURL == "" {
			applyURL = trimmedText(get(d, "apply_url"))
		}
		if applyURL == "" {
			applyURL = origin + "/jobs/" + encodeURIComponent(jsString(or(get(d, "slug"), id)))
		}
		description := ""
		if desc, ok := get(d, "description").(string); ok && desc != "" {
			description = htmlx.StripTags(desc)
		}
		posted := or(get(d, "posted_date"), get(d, "create_date"))
		jobs = append(jobs, model.NormalizedJob{
			ExternalID:  id,
			Title:       text(get(d, "title")),
			Locations:   locs(phenomLocations(d)),
			ApplyURL:    applyURL,
			Excerpt:     excerpt(description),
			PostedAt:    safeDateAny(posted),
			Description: description,
		})
	}
	return jobs
}

// safeDateAny is safeDate(v) on a JSON value, as an ISO string.
func safeDateAny(v any) *string {
	switch x := v.(type) {
	case string:
		return safedate.ISOPtr(safedate.Parse(x))
	case float64:
		return safedate.ISOPtr(safedate.Parse(x))
	}
	return nil
}

func fetchPhenom(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	origin := PhenomCareersOrigin(token)
	var all []model.NormalizedJob
	seen := map[string]bool{}
	for page := 1; page <= phenomMaxPages; page++ {
		url, err := PhenomJobsAPIURL(token, page)
		if err != nil {
			return nil, err
		}
		data, err := getJSON(ctx, url, fetch.Options{})
		if err != nil {
			return nil, err
		}
		rows := array(get(data, "jobs"))
		if len(rows) == 0 {
			break
		}
		for _, job := range MapPhenomJobs(rows, origin) {
			if !seen[job.ExternalID] {
				seen[job.ExternalID] = true
				all = append(all, job)
			}
		}
		total, ok := toNumber(coalesce(get(data, "totalCount"), get(data, "count"), float64(len(all))))
		if (ok && float64(len(all)) >= total) || len(rows) < phenomPageLimit {
			break
		}
	}
	return all, nil
}

func probePhenom(ctx context.Context, token string) ProbeResult {
	url, err := PhenomJobsAPIURL(token, 1)
	if err != nil {
		return ProbeResult{Error: err.Error()}
	}
	data, err := getJSON(ctx, url, fetch.Options{Attempts: 1})
	if err != nil {
		return ProbeResult{Status: failureStatus(err), Error: err.Error()}
	}
	count := probeCount(coalesce(get(data, "totalCount"), get(data, "count"), float64(len(array(get(data, "jobs"))))))
	r := ProbeResult{OK: count > 0, Status: 200, Count: &count}
	if count <= 0 {
		r.Error = "empty"
	}
	return r
}

func probeCount(v any) int {
	n, _ := toNumber(v)
	if n != n {
		return 0
	}
	return int(n)
}

// ---- Eightfold PCSX (Microsoft careers and similar tenants) ----

const (
	eightfoldPageSize = 10
	eightfoldMaxPages = 40
)

// EightfoldBoard is a parsed eightfold board token:
//
//	microsoft            domain=microsoft.com, query=intern
//	microsoft|ai         same domain, AI/Research query and title filter
//	domain.com|query     custom domain and search query
type EightfoldBoard struct {
	Domain      string
	Query       string
	TitleFilter *regexp.Regexp // extra title/department filter (microsoft|ai)
	ApplyOrigin string
}

var eightfoldAITitle = regexp.MustCompile(`(?i)\b(ai|a\.i\.|ml|machine learning|llm|deep learning|research|coreai|mai|openai|generative)\b`)
var dotComEnd = regexp.MustCompile(`(?i)\.com$`)

func ParseEightfoldToken(token string) EightfoldBoard {
	var parts []string
	for _, p := range strings.Split(token, "|") {
		if p = jsTrim(p); p != "" {
			parts = append(parts, p)
		}
	}
	part := func(i string, n int) string {
		if n < len(parts) {
			return parts[n]
		}
		return i
	}
	head := strings.ToLower(part("microsoft", 0))
	filterKey := strings.ToLower(part("", 1))
	if head == "microsoft" || head == "microsoft.com" {
		applyOrigin := "https://apply.careers.microsoft.com"
		if filterKey == "ai" || filterKey == "research" {
			return EightfoldBoard{Domain: "microsoft.com", Query: "AI Intern", TitleFilter: eightfoldAITitle, ApplyOrigin: applyOrigin}
		}
		return EightfoldBoard{Domain: "microsoft.com", Query: part("intern", 1), ApplyOrigin: applyOrigin}
	}
	domain := head
	if !strings.Contains(head, ".") {
		domain = head + ".com"
	}
	return EightfoldBoard{
		Domain:      domain,
		Query:       part("intern", 1),
		ApplyOrigin: "https://apply.careers." + dotComEnd.ReplaceAllString(domain, ".com"),
	}
}

// EightfoldSearchURL is the search API URL for results from start.
func EightfoldSearchURL(b EightfoldBoard, start int) string {
	u, _ := parseURL("https://apply.careers.microsoft.com/api/pcsx/search")
	// Microsoft hosts PCSX; other domains may need a different host later.
	if b.Domain != "microsoft.com" {
		u.setHost("apply.careers." + b.Domain)
		u.setPathname("/api/pcsx/search")
	}
	u.setParam("domain", b.Domain)
	u.setParam("query", b.Query)
	u.setParam("num", strconv.Itoa(eightfoldPageSize))
	u.setParam("start", strconv.Itoa(start))
	return u.String()
}

// MapEightfoldPositions turns search positions into jobs.
func MapEightfoldPositions(positions []any, b EightfoldBoard) []model.NormalizedJob {
	var jobs []model.NormalizedJob
	for _, p := range positions {
		title := jsTrim(text(or(get(p, "name"), get(p, "title"), "")))
		if title == "" {
			continue
		}
		dept := get(p, "department")
		if b.TitleFilter != nil && !b.TitleFilter.MatchString(title) && !b.TitleFilter.MatchString(text(or(dept, ""))) {
			continue
		}
		id := jsTrim(jsString(coalesce(get(p, "id"), get(p, "displayJobId"), "")))
		if id == "" {
			continue
		}
		path := "/careers/job/" + id
		if pu, ok := get(p, "positionUrl").(string); ok && strings.HasPrefix(pu, "/") {
			path = pu
		}
		var locations []string
		var seen []any
		for _, l := range append(append([]any{}, array(get(p, "standardizedLocations"))...), array(get(p, "locations"))...) {
			dup := false
			for _, s := range seen {
				if strictEqual(s, l) {
					dup = true
					break
				}
			}
			if !dup {
				seen = append(seen, l)
			}
			if truthy(l) && !dup {
				locations = append(locations, text(l))
			}
		}
		var ex *string
		description := ""
		if truthy(dept) {
			description = text(dept)
			ex = strPtr(slice16(description, 400))
		}
		var posted *string
		if ts := or(get(p, "postedTs"), get(p, "creationTs")); truthy(ts) {
			posted = safeDateAny(ts)
		}
		jobs = append(jobs, model.NormalizedJob{
			ExternalID:  id,
			Title:       title,
			Locations:   locs(locations),
			ApplyURL:    b.ApplyOrigin + path,
			Excerpt:     ex,
			PostedAt:    posted,
			Description: description,
		})
	}
	return jobs
}

// eightfoldPagePause spaces page requests: since 2026-11 Microsoft's
// careers API answers 429 to back-to-back pages, which failed the whole
// board. Tests set it to zero.
var eightfoldPagePause = 2 * time.Second

func fetchEightfold(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	b := ParseEightfoldToken(token)
	var all []model.NormalizedJob
	seen := map[string]bool{}
	total := -1.0
	for page := 0; page < eightfoldMaxPages; page++ {
		start := page * eightfoldPageSize
		if total >= 0 && float64(start) >= total {
			break
		}
		if page > 0 && eightfoldPagePause > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(eightfoldPagePause):
			}
		}
		body, err := getJSON(ctx, EightfoldSearchURL(b, start), fetch.Options{})
		if err != nil {
			return nil, err
		}
		positions := array(get(body, "data", "positions"))
		if n, ok := get(body, "data", "count").(float64); ok {
			total = n
		}
		if len(positions) == 0 {
			break
		}
		for _, job := range MapEightfoldPositions(positions, b) {
			if !seen[job.ExternalID] {
				seen[job.ExternalID] = true
				all = append(all, job)
			}
		}
		if len(positions) < eightfoldPageSize {
			break
		}
	}
	return all, nil
}

func probeEightfold(ctx context.Context, token string) ProbeResult {
	body, err := getJSON(ctx, EightfoldSearchURL(ParseEightfoldToken(token), 0), fetch.Options{Attempts: 1})
	if err != nil {
		// A 429 does not say the token is wrong. The generic probe treats it the same way.
		var f *fetch.Failure
		if errors.As(err, &f) && f.Kind == fetch.KindThrottled {
			status := f.Status
			if status == 0 {
				status = 429
			}
			return ProbeResult{OK: true, Status: status}
		}
		return ProbeResult{Status: failureStatus(err), Error: err.Error()}
	}
	count := probeCount(coalesce(get(body, "data", "count"), lengthOf(get(body, "data", "positions")), float64(0)))
	return ProbeResult{OK: count > 0, Status: 200, Count: &count}
}

// lengthOf is `v?.length` for arrays (undefined otherwise).
func lengthOf(v any) any {
	if a, ok := v.([]any); ok {
		return float64(len(a))
	}
	if s, ok := v.(string); ok {
		return float64(len16(s))
	}
	return undefined
}

// ---- Avature (SearchJobs HTML; IBM runs on it) ----

const avatureDumpEnv = "OPENINTERN_AVATURE_HTML_PATH"

// AvatureSearchURL builds the SearchJobs URL. token is `bloomberg`
// (bloomberg.avature.net), `ibmglobal`, a host, or a full SearchJobs URL.
func AvatureSearchURL(token, keywords string) (string, error) {
	raw := jsTrim(token)
	if hasHTTPScheme(raw) {
		u, err := parseURL(raw)
		if err != nil {
			return "", err
		}
		if !u.hasParam("keywords") {
			u.setParam("keywords", keywords)
		}
		if !u.hasParam("jobRecordsPerPage") {
			u.setParam("jobRecordsPerPage", "100")
		}
		return u.String(), nil
	}
	host := raw
	if strings.Contains(raw, ".") {
		host = stripScheme(raw)
		if slash := strings.IndexByte(host, '/'); slash >= 0 {
			host = host[:slash]
		}
	} else {
		host = raw + ".avature.net"
	}
	path := "/careers/SearchJobs/"
	if strings.HasPrefix(host, "ibm") {
		path = "/en_US/careers/SearchJobs/"
	}
	u, err := parseURL("https://" + host + path)
	if err != nil {
		return "", err
	}
	u.setParam("listFilterMode", "1")
	u.setParam("jobRecordsPerPage", "100")
	u.setParam("keywords", keywords)
	return u.String(), nil
}

var (
	avatureLink = jsRegexp(`(?i)<a[^>]+href="([^"]*/(?:careers/)?JobDetail/([^"/]+)/(\d+))"\s*[^>]*>([\s\S]*?)</a>`)
	hyphens     = regexp.MustCompile(`-`)
)

// ParseAvatureSearchHTML reads job links (and nearby locations) from a
// SearchJobs page.
func ParseAvatureSearchHTML(html, baseURL string) ([]model.NormalizedJob, error) {
	base, err := parseURL(baseURL)
	if err != nil {
		return nil, err
	}
	origin := base.origin()
	var jobs []model.NormalizedJob
	seen := map[string]bool{}
	for _, m := range avatureLink.FindAllStringSubmatch(html, -1) {
		href, slug, id, inner := m[1], m[2], m[3], m[4]
		src := inner
		if src == "" {
			src = hyphens.ReplaceAllString(slug, " ")
		}
		title := htmlx.Strip(src)
		if title == "" || seen[id] {
			continue
		}
		seen[id] = true
		applyURL := href
		if !strings.HasPrefix(href, "http") {
			applyURL = origin + href
		}
		jobs = append(jobs, model.NormalizedJob{ExternalID: id, Title: title, Locations: []string{}, ApplyURL: applyURL})
	}
	// Attach nearby locations when present after each JobDetail block.
	if len(jobs) > 0 {
		locMap := avatureLocations(html)
		for i := range jobs {
			if loc, ok := locMap[jobs[i].ExternalID]; ok && loc != "" {
				jobs[i].Locations = []string{loc}
			}
		}
	}
	return jobs, nil
}

// avatureLocations runs
//
//	/JobDetail\/[^"/]+\/(\d+)[\s\S]{0,1200}?class="[^"]*list-item-location[^"]*"[^>]*>([^<]+)/gi
//
// by hand: RE2 caps counted repeats at 1000, and the gap is measured in
// UTF-16 code units. Matches do not overlap, so a JobDetail inside a
// consumed match is skipped, as with exec in a loop; later ids overwrite
// earlier ones.
func avatureLocations(html string) map[string]string {
	out := map[string]string{}
	lower := asciiLower(html)
	pos := 0
	for {
		i := strings.Index(lower[pos:], "jobdetail/")
		if i < 0 {
			return out
		}
		start := pos + i
		end, id, val, ok := avatureLocAt(html, lower, start)
		if !ok {
			pos = start + 1
			continue
		}
		out[id] = htmlx.Strip(val)
		pos = end
	}
}

func avatureLocAt(html, lower string, start int) (end int, id, val string, ok bool) {
	p := start + len("jobdetail/")
	q := p
	for q < len(html) && html[q] != '"' && html[q] != '/' {
		q++
	}
	if q == p || q >= len(html) || html[q] != '/' {
		return 0, "", "", false
	}
	q++
	d := q
	for d < len(html) && html[d] >= '0' && html[d] <= '9' {
		d++
	}
	if d == q {
		return 0, "", "", false
	}
	id = html[q:d]
	units16 := 0
	for g := d; g <= len(html); {
		if e, v, ok := avatureClassAt(html, lower, g); ok {
			return e, id, v, true
		}
		if g == len(html) {
			break
		}
		r, size := utf8.DecodeRuneInString(html[g:])
		if r >= 0x10000 {
			units16 += 2
		} else {
			units16++
		}
		if units16 > 1200 {
			break
		}
		g += size
	}
	return 0, "", "", false
}

// avatureClassAt matches class="[^"]*list-item-location[^"]*"[^>]*>([^<]+) at g.
func avatureClassAt(html, lower string, g int) (end int, val string, ok bool) {
	if !strings.HasPrefix(lower[g:], `class="`) {
		return 0, "", false
	}
	a := g + len(`class="`)
	close := strings.IndexByte(html[a:], '"')
	if close < 0 || !strings.Contains(lower[a:a+close], "list-item-location") {
		return 0, "", false
	}
	t := a + close + 1
	gt := strings.IndexByte(html[t:], '>')
	if gt < 0 {
		return 0, "", false
	}
	v := t + gt + 1
	e := v
	for e < len(html) && html[e] != '<' {
		e++
	}
	if e == v {
		return 0, "", false
	}
	return e, html[v:e], true
}

// asciiLower lowercases A-Z only, keeping byte offsets (the `i` flag on
// ASCII patterns).
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// readDump returns an HTML dump named by an env var, or "" when unset/empty.
func readDump(env string) (string, error) {
	path := jsTrim(os.Getenv(env))
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return decodeBody(b), nil
}

func fetchAvature(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	dump, err := readDump(avatureDumpEnv)
	if err != nil {
		return nil, err
	}
	if dump != "" {
		url, err := AvatureSearchURL(token, "intern")
		if err != nil {
			return nil, err
		}
		return ParseAvatureSearchHTML(dump, url)
	}
	var all []model.NormalizedJob
	seen := map[string]bool{}
	for _, kw := range []string{"intern", "Intern", "Software Engineer Intern", ""} {
		url, err := AvatureSearchURL(token, kw)
		if err != nil {
			return nil, err
		}
		html, err := getText(ctx, url, fetch.Options{})
		if err != nil {
			return nil, err
		}
		jobs, err := ParseAvatureSearchHTML(html, url)
		if err != nil {
			return nil, err
		}
		for _, job := range jobs {
			if !seen[job.ExternalID] {
				seen[job.ExternalID] = true
				all = append(all, job)
			}
		}
	}
	if len(all) == 0 {
		return nil, errorf("Avature empty response for %s. Optional dump: %s", token, avatureDumpEnv)
	}
	return all, nil
}

// fetchIBM reads IBM's Avature tenant (`ibmglobal` by default), preferring
// a "Software Intern" search and falling back to the generic Avature fetch.
func fetchIBM(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	t := jsTrim(token)
	if t == "" {
		t = "ibmglobal"
	}
	url, err := AvatureSearchURL(t, "Software Intern")
	if err != nil {
		return nil, err
	}
	html, err := getText(ctx, url, fetch.Options{})
	if err != nil {
		return nil, err
	}
	jobs, err := ParseAvatureSearchHTML(html, url)
	if err != nil {
		return nil, err
	}
	if len(jobs) > 0 {
		return jobs, nil
	}
	return fetchAvature(ctx, t)
}

// ---- iCIMS (public job search HTML) ----

const icimsDumpEnv = "OPENINTERN_ICIMS_HTML_PATH"

// IcimsHost maps `sig`, `careers-sig` or a host to the iCIMS host.
func IcimsHost(token string) string {
	raw := stripScheme(jsTrim(token))
	if slash := strings.IndexByte(raw, '/'); slash >= 0 {
		raw = raw[:slash]
	}
	if strings.Contains(raw, ".") {
		return raw
	}
	if strings.HasPrefix(raw, "careers-") {
		return raw + ".icims.com"
	}
	return "careers-" + raw + ".icims.com"
}

// IcimsSearchURL is the job search page for keyword.
func IcimsSearchURL(token, keyword string) (string, error) {
	u, err := parseURL("https://" + IcimsHost(token) + "/jobs/search")
	if err != nil {
		return "", err
	}
	u.setParam("ss", "1")
	u.setParam("searchKeyword", keyword)
	u.setParam("searchRelation", "keyword_all")
	u.setParam("in_iframe", "1")
	return u.String(), nil
}

var (
	icimsLink   = regexp.MustCompile(`(?i)href="([^"]*/jobs/(\d+)/([^"?]+)/job[^"]*)"`)
	icimsH3     = jsRegexp(`(?i)<h3[^>]*>([\s\S]*?)</h3>`)
	icimsColon  = regexp.MustCompile(`(?i)%3a`)
	icimsPlus   = regexp.MustCompile(`(?i)%2b`)
	icimsIframe = regexp.MustCompile(`[?&]in_iframe=1`)
	jsSpaceRun  = jsRegexp(`\s+`)
	plusSign    = regexp.MustCompile(`\+`)
)

func replaceFirst(re *regexp.Regexp, s, repl string) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]] + repl + s[loc[1]:]
}

// ParseIcimsSearchHTML pairs job links with the page's <h3> titles by
// position, falling back to the URL slug.
func ParseIcimsSearchHTML(html, baseURL string) ([]model.NormalizedJob, error) {
	base, err := parseURL(baseURL)
	if err != nil {
		return nil, err
	}
	origin := base.origin()
	var h3s []string
	for _, m := range icimsH3.FindAllStringSubmatch(html, -1) {
		if t := htmlx.Strip(m[1]); t != "" {
			h3s = append(h3s, t)
		}
	}
	type link struct{ id, href, slug string }
	var links []link
	seen := map[string]bool{}
	for _, m := range icimsLink.FindAllStringSubmatch(html, -1) {
		decoded, err := decodeURIComponent(m[3])
		if err != nil {
			return nil, err
		}
		slug := plusSign.ReplaceAllString(decoded, " ")
		if seen[m[2]] {
			continue
		}
		seen[m[2]] = true
		links = append(links, link{id: m[2], href: m[1], slug: slug})
	}
	jobs := make([]model.NormalizedJob, 0, len(links))
	for i, l := range links {
		title := ""
		if i < len(h3s) {
			title = h3s[i]
		}
		if title == "" {
			t := icimsColon.ReplaceAllString(l.slug, ":")
			t = icimsPlus.ReplaceAllString(t, "+")
			t = hyphens.ReplaceAllString(t, " ")
			title = jsTrim(jsSpaceRun.ReplaceAllString(t, " "))
		}
		applyURL := l.href
		if !strings.HasPrefix(l.href, "http") {
			applyURL = origin + l.href
		}
		jobs = append(jobs, model.NormalizedJob{
			ExternalID: l.id,
			Title:      title,
			Locations:  []string{},
			ApplyURL:   replaceFirst(icimsIframe, applyURL, ""),
		})
	}
	return jobs, nil
}

func fetchIcims(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	url, err := IcimsSearchURL(token, "intern")
	if err != nil {
		return nil, err
	}
	dump, err := readDump(icimsDumpEnv)
	if err != nil {
		return nil, err
	}
	if dump != "" {
		return ParseIcimsSearchHTML(dump, url)
	}
	html, err := getText(ctx, url, fetch.Options{})
	if err != nil {
		return nil, err
	}
	return ParseIcimsSearchHTML(html, url)
}

// ---- Google / DeepMind (careers results HTML) ----

const googleApplyBase = "https://www.google.com/about/careers/applications/jobs/results"

// GoogleCareersSearchURL is the first results page for `google` or
// `deepmind`.
func GoogleCareersSearchURL(token string) string {
	t := strings.ToLower(jsTrim(token))
	u, _ := parseURL(googleApplyBase)
	u.setParam("pageSize", "100")
	if t == "deepmind" || t == "google-deepmind" {
		u.setParam("company", "DeepMind")
		return u.String()
	}
	u.setParam("q", "Software Engineering Intern")
	u.setParam("target_level", "INTERN")
	return u.String()
}

var (
	googlePath     = regexp.MustCompile(`jobs/results/(\d+)-([a-z0-9-]+)`)
	googleWordHead = regexp.MustCompile(`\b\w`)
)

// ParseGoogleCareersHTML reads job cards (h3.QJPWVe titles; filter sidebars
// also use <h3>, so the class is required), falling back to
// jobs/results/ID-slug paths with a title made from the slug.
func ParseGoogleCareersHTML(html string) []model.NormalizedJob {
	var jobs []model.NormalizedJob
	seen := map[string]bool{}
	for _, c := range googleCards(html) {
		title := htmlx.Strip(c.title)
		if title == "" || seen[c.id] {
			continue
		}
		seen[c.id] = true
		slug := c.id
		if m := regexp.MustCompile(`jobs/results/` + c.id + `-([a-z0-9-]+)`).FindStringSubmatch(html); m != nil && m[1] != "" {
			slug = c.id + "-" + m[1]
		}
		jobs = append(jobs, model.NormalizedJob{ExternalID: c.id, Title: title, Locations: []string{}, ApplyURL: googleApplyBase + "/" + slug})
	}
	if len(jobs) == 0 {
		for _, m := range googlePath.FindAllStringSubmatch(html, -1) {
			id, tail := m[1], m[2]
			if seen[id] {
				continue
			}
			seen[id] = true
			title := googleWordHead.ReplaceAllStringFunc(hyphens.ReplaceAllString(tail, " "), strings.ToUpper)
			jobs = append(jobs, model.NormalizedJob{ExternalID: id, Title: title, Locations: []string{}, ApplyURL: googleApplyBase + "/" + id + "-" + tail})
		}
	}
	return jobs
}

type googleCard struct{ id, title string }

// googleCards runs
//
//	/ssk='17:(\d+)'([\s\S]{0,20000}?)<h3 class="QJPWVe">([\s\S]*?)<\/h3>/gi
//
// by hand (RE2 caps counted repeats at 1000; the gap counts UTF-16 units).
func googleCards(html string) []googleCard {
	lower := asciiLower(html)
	const open = `<h3 class="qjpwve">`
	var out []googleCard
	pos := 0
	for {
		i := strings.Index(lower[pos:], "ssk='17:")
		if i < 0 {
			return out
		}
		start := pos + i
		p := start + len("ssk='17:")
		d := p
		for d < len(html) && html[d] >= '0' && html[d] <= '9' {
			d++
		}
		if d == p || d >= len(html) || html[d] != '\'' {
			pos = start + 1
			continue
		}
		id := html[p:d]
		gapStart := d + 1
		matched := false
		units16 := 0
		for g := gapStart; g <= len(html); {
			if strings.HasPrefix(lower[g:], open) {
				t := g + len(open)
				if c := strings.Index(lower[t:], "</h3>"); c >= 0 {
					out = append(out, googleCard{id: id, title: html[t : t+c]})
					pos = t + c + len("</h3>")
					matched = true
				}
				break
			}
			if g == len(html) {
				break
			}
			r, size := utf8.DecodeRuneInString(html[g:])
			if r >= 0x10000 {
				units16 += 2
			} else {
				units16++
			}
			if units16 > 20000 {
				break
			}
			g += size
		}
		if !matched {
			pos = start + 1
		}
	}
}

func fetchGoogle(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	t := strings.ToLower(jsTrim(token))
	urls := []string{GoogleCareersSearchURL(token)}
	if t == "google" || t == "" {
		urls = append(urls,
			googleApplyBase+"?q=intern&pageSize=100",
			googleApplyBase+"?target_level=INTERN&pageSize=100",
			googleApplyBase+"?q=Software%20Engineering%20Intern&pageSize=100",
		)
	}
	if t == "deepmind" || t == "google-deepmind" {
		urls = append(urls, googleApplyBase+"?company=DeepMind&q=Research&pageSize=100")
	}
	var all []model.NormalizedJob
	seen := map[string]bool{}
	for _, url := range urls {
		// Any refusal fails the run: a skipped page must not look like an empty board.
		html, err := getText(ctx, url, fetch.Options{Headers: map[string]string{"Accept-Language": "en-US,en;q=0.9"}})
		if err != nil {
			return nil, err
		}
		for _, job := range ParseGoogleCareersHTML(html) {
			if !seen[job.ExternalID] {
				seen[job.ExternalID] = true
				all = append(all, job)
			}
		}
	}
	return all, nil
}

// ---- D. E. Shaw (Next.js __NEXT_DATA__ on the internships page) ----

const DeshawInternshipsURL = "https://www.deshaw.com/careers/internships"

var (
	deshawNextData = jsRegexp(`<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)
	leadingSlash   = regexp.MustCompile(`^/`)
)

// ParseDeshawNextData reads internships from the parsed __NEXT_DATA__.
func ParseDeshawNextData(raw any) []model.NormalizedJob {
	var jobs []model.NormalizedJob
	for _, row := range array(get(raw, "props", "pageProps", "internships")) {
		d := get(row, "data")
		if nullish(d) {
			d = jsObject("id", get(row, "id"), "displayName", get(row, "displayName"))
		}
		id := jsTrim(jsString(coalesce(get(d, "id"), get(row, "id"), "")))
		title := jsTrim(text(or(get(d, "displayName"), get(row, "displayName"), "")))
		if id == "" || title == "" {
			continue
		}
		jobURL := text(or(get(d, "jobUrl"), id))
		applyURL := jobURL
		if !strings.HasPrefix(jobURL, "http") {
			applyURL = "https://www.deshaw.com/careers/" + leadingSlash.ReplaceAllString(jobURL, "")
		}
		var locations []string
		for _, o := range array(get(row, "office")) {
			if n := get(o, "name"); truthy(n) && !containsStr(locations, text(n)) {
				locations = append(locations, text(n))
			}
		}
		for _, l := range array(get(d, "jobMetadata", "jobLocations")) {
			if n := get(l, "name"); truthy(n) && !containsStr(locations, text(n)) {
				locations = append(locations, text(n))
			}
		}
		description := ""
		if s, ok := get(d, "jobDescription").(string); ok {
			description = htmlx.StripTags(s)
		}
		jobs = append(jobs, model.NormalizedJob{
			ExternalID:  id,
			Title:       title,
			Locations:   locs(locations),
			ApplyURL:    applyURL,
			Excerpt:     excerpt(description),
			Description: description,
		})
	}
	return jobs
}

// ParseDeshawHTML finds __NEXT_DATA__ in the page; a page without it has no
// jobs, and data that is not JSON is an error.
func ParseDeshawHTML(html string) ([]model.NormalizedJob, error) {
	m := deshawNextData.FindStringSubmatch(html)
	if m == nil || m[1] == "" {
		return nil, nil
	}
	v, err := jsjson.Parse(m[1])
	if err != nil {
		return nil, err
	}
	return ParseDeshawNextData(v), nil
}

func fetchDeshaw(ctx context.Context, _ string) ([]model.NormalizedJob, error) {
	html, err := getText(ctx, DeshawInternshipsURL, fetch.Options{})
	if err != nil {
		return nil, err
	}
	jobs, err := ParseDeshawHTML(html)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, errors.New("D.E. Shaw internships page parsed zero roles")
	}
	return jobs, nil
}

// ---- Shopify (public job cards on shopify.com/careers) ----
// Intern roles are often password-gated; publicly listed roles are ingested.

const ShopifyCareersURL = "https://www.shopify.com/careers"

var shopifyCard = regexp.MustCompile(`(?i)href="(/careers/([a-z0-9-]+)_([a-f0-9-]{36}))"`)

func shopifyTitle(slug string) string {
	var words []string
	for _, w := range strings.Split(strings.ReplaceAll(slug, "_", "-"), "-") {
		if w != "" {
			words = append(words, strings.ToUpper(w[:1])+w[1:])
		}
	}
	return strings.Join(words, " ")
}

// ParseShopifyCareersHTML reads /careers/role-name_uuid links.
func ParseShopifyCareersHTML(html string) []model.NormalizedJob {
	var jobs []model.NormalizedJob
	seen := map[string]bool{}
	for _, m := range shopifyCard.FindAllStringSubmatch(html, -1) {
		path, slug, uuid := m[1], m[2], m[3]
		if seen[uuid] {
			continue
		}
		seen[uuid] = true
		jobs = append(jobs, model.NormalizedJob{ExternalID: uuid, Title: shopifyTitle(slug), Locations: []string{}, ApplyURL: "https://www.shopify.com" + path})
	}
	return jobs
}

func fetchShopify(ctx context.Context, _ string) ([]model.NormalizedJob, error) {
	var all []model.NormalizedJob
	seen := map[string]bool{}
	for _, url := range []string{ShopifyCareersURL, "https://www.shopify.com/careers/search"} {
		html, err := getText(ctx, url, fetch.Options{})
		if err != nil {
			return nil, err
		}
		for _, job := range ParseShopifyCareersHTML(html) {
			if !seen[job.ExternalID] {
				seen[job.ExternalID] = true
				all = append(all, job)
			}
		}
	}
	return all, nil
}
