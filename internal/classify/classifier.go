// Package classify contains the internship classifier, metadata extractors,
// freshness helpers, and title canonicalization used by ingest and the site.
package classify

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/proven"
)

var (
	internshipTitleRE  = regexp.MustCompile(`(?i)\b(intern|internship|co-?op|coop|apprentice(ship)?|project\s+intern|campus\s+\w+\s+intern|year\s+at\s+\w+)\b`)
	excludeSeniorityRE = regexp.MustCompile(`(?i)\b(senior|staff|principal|director|lead|head of|vp\b|vice president|chief|architect)\b`)
	excludeManagerRE   = regexp.MustCompile(`(?i)\bmanager\b`)
	allowedManagerRE   = regexp.MustCompile(`(?i)\b(product|program|project|technical\s*program)\s*managers?\b`)
	negativeDomainRE   = regexp.MustCompile(`(?i)\b(marketing|sales|\bsdr\b|account\s*(executive|manager)|hr\b|human\s*resources|people\s*(ops|operations|team)?|talent|recruit(er|ing|ment)|finance|financial|accounting|accountant|audit|tax\b|legal|paralegal|counsel|customer\s*(success|support|service|experience)|technical\s*support|business\s*development|bd\b|partnerships?|community|content|copywrit|social\s*media|brand|communications?|pr\b|public\s*relations|government\s*affairs|policy|events?\s*(coordinator|manager|intern)|admin(istrative)?|office\s*(manager|coordinator)|executive\s*assistant|retail|store|warehouse|logistics|supply\s*chain|procurement|facilities|culinary|barista|driver|electrician|plumber|hvac|mechanic|weld(er|ing)?|machinist|assembler|janitor|custodian|landscap|nurse|clinical|pharmac|industrial\s*engineer(ing)?|process\s*engineer(ing)?|manufacturing\s*engineer(ing)?|mechanical\s*engineer(ing)?|civil\s*engineer(ing)?|chemical\s*engineer(ing)?|quality\s*engineer(ing)?|tires?\b|lab\s*(technician|tech|assistant)|production\s*engineer(ing)?|trade\s*compliance|network\s*strategy|business\s*operations|(?:platform|business|content|marketing|growth|commercial|sales|brand|product)\s+strategy|product\s*operations|program\s*management|project\s*management|user\s*research|ux\s*(research|design))\b`)
	genericTechTitleRE = regexp.MustCompile(`(?i)\b(engineer(?:ing)?|technical|technology|computer\s+science|computing|systems)\s+(?:intern(?:ship)?|co-?op|coop|apprentice)\b|\b(?:intern(?:ship)?|co-?op|coop|apprentice)[,\s-]+(?:engineer(?:ing)?|technical|technology|computer\s+science|computing|systems)\b`)
	remoteRE           = regexp.MustCompile(`(?i)\bremote\b|\bwfh\b|work from home|anywhere`)
	termRE             = regexp.MustCompile(`(?i)\b(winter|spring|summer|fall|autumn)\b`)
	termYearRE         = regexp.MustCompile(`(?i)\b(winter|spring|summer|fall|autumn)\s+(20\d{2})\b`)
	monthRangeRE       = regexp.MustCompile(`(?i)\b(\d{1,2})\s*(?:to|-|–|—)\s*(\d{1,2})\s*months?\b`)
	monthSingleRE      = regexp.MustCompile(`(?i)\b(\d{1,2})\s*[-–—]?\s*months?\b`)
	weekRangeRE        = regexp.MustCompile(`(?i)\b(\d{1,2})\s*(?:to|-|–|—)\s*(\d{1,2})\s*weeks?\b`)
	weekSingleRE       = regexp.MustCompile(`(?i)\b(\d{1,2})\s*[-–—]?\s*weeks?\b`)
	classOfRE          = regexp.MustCompile(`(?i)\bclass of\s+(20\d{2})\b`)
	startYearRE        = regexp.MustCompile(`(?i)\b(20\d{2})\s+start\b|\bstart(?:ing|s)?\s+(?:in\s+)?(20\d{2})\b`)
	nearInternRE       = regexp.MustCompile(`(?i)\b(20\d{2})\b[^.]{0,40}\b(?:intern|internship|co-?op|coop|apprentice)`)
	afterInternRE      = regexp.MustCompile(`(?i)\b(?:intern|internship|co-?op|coop|apprentice)[^.]{0,40}\b(20\d{2})\b`)
)

// IsTechInternship reports whether a title has an internship signal and maps
// to at least one supported technical role.
func IsTechInternship(title string, description ...string) bool {
	if !internshipTitleRE.MatchString(title) || excludeSeniorityRE.MatchString(title) ||
		(excludeManagerRE.MatchString(title) && !allowedManagerRE.MatchString(title)) ||
		negativeDomainRE.MatchString(title) {
		return false
	}
	if len(ExtractRoles(title)) > 0 {
		return true
	}
	descriptionText := ""
	if len(description) != 0 {
		descriptionText = description[0]
	}
	return genericTechTitleRE.MatchString(title) && len(ExtractRoles("", descriptionText)) > 0
}

// LooksRemote finds remote-work wording in locations or the title.
func LooksRemote(locations []string, title ...string) bool {
	jobTitle := ""
	if len(title) != 0 {
		jobTitle = title[0]
	}
	return remoteRE.MatchString(strings.Join(locations, " ") + " " + jobTitle)
}

// Exported aliases keep the names of the TypeScript classifier contracts.
type (
	InternshipTerm = string
	TermYear       = model.TermYear
	JobRole        = string
	JobRegion      = string
)

var (
	TermOrder       = []string{"summer", "fall", "winter", "spring"}
	RoleOptions     = []string{"software", "backend", "frontend", "fullstack", "data", "ml", "mobile", "security", "devops", "hardware", "quant", "product", "research"}
	RegionOptions   = []string{"remote", "us", "canada", "europe", "other"}
	DurationPresets = []int{3, 4, 6, 8, 12}

	TERM_ORDER       = TermOrder
	ROLE_OPTIONS     = RoleOptions
	REGION_OPTIONS   = RegionOptions
	DURATION_PRESETS = DurationPresets
)

const (
	MaxAgeDaysNoTerm  = proven.MaxAgeDaysNoTerm
	LastSeenStaleDays = proven.LastSeenStaleDays

	MAX_AGE_DAYS_NO_TERM = MaxAgeDaysNoTerm
	LAST_SEEN_STALE_DAYS = LastSeenStaleDays
)

func normalizeTermWord(raw string) string {
	switch strings.ToLower(raw) {
	case "autumn", "fall":
		return "fall"
	case "summer":
		return "summer"
	case "spring":
		return "spring"
	default:
		return "winter"
	}
}

// ExtractTerms returns canonical seasons in board filter order.
func ExtractTerms(text string) []string {
	found := make(map[string]bool)
	for _, match := range termRE.FindAllStringSubmatch(text, -1) {
		found[normalizeTermWord(match[1])] = true
	}
	out := make([]string, 0, len(found))
	for _, term := range TermOrder {
		if found[term] {
			out = append(out, term)
		}
	}
	return out
}

func optionalNow(now []time.Time) time.Time {
	if len(now) != 0 {
		return now[0]
	}
	return time.Now()
}

func clampCohortYear(year int, now time.Time) (int, bool) {
	current := now.UTC().Year()
	return year, year >= current-1 && year <= current+3
}

func parseSmallDecimal(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// ExtractTermYears returns explicit season-year pairs and backfills bare
// seasons from cohortYear. Results are sorted by term order, then year.
func ExtractTermYears(text string, cohortYear *int, now ...time.Time) []model.TermYear {
	clock := optionalNow(now)
	pairs := make(map[string]model.TermYear)
	add := func(term string, year int) {
		year, ok := clampCohortYear(year, clock)
		if ok {
			pairs[term+":"+strconv.Itoa(year)] = model.TermYear{Term: term, Year: year}
		}
	}
	for _, match := range termYearRE.FindAllStringSubmatch(text, -1) {
		add(normalizeTermWord(match[1]), parseSmallDecimal(match[2]))
	}
	if cohortYear != nil {
		for _, term := range ExtractTerms(text) {
			hasExplicit := false
			for _, pair := range pairs {
				if pair.Term == term {
					hasExplicit = true
					break
				}
			}
			if !hasExplicit {
				add(term, *cohortYear)
			}
		}
	}
	out := make([]model.TermYear, 0, len(pairs))
	for _, term := range TermOrder {
		termPairs := make([]model.TermYear, 0)
		for _, pair := range pairs {
			if pair.Term == term {
				termPairs = append(termPairs, pair)
			}
		}
		sort.Slice(termPairs, func(i, j int) bool { return termPairs[i].Year < termPairs[j].Year })
		out = append(out, termPairs...)
	}
	return out
}

// TermYearEndDate returns the final instant at which the term is open for
// applications. The name is retained for compatibility with the TS export.
func TermYearEndDate(ty model.TermYear) time.Time {
	return proven.ApplyByDate(proven.TermYearLike(ty))
}

// IsStaleByTermYears reports whether every known term has passed.
func IsStaleByTermYears(termYears []model.TermYear, now ...time.Time) bool {
	if len(termYears) == 0 {
		return false
	}
	clock := optionalNow(now)
	converted := make([]proven.TermYearLike, len(termYears))
	for i, termYear := range termYears {
		converted[i] = proven.TermYearLike(termYear)
	}
	return proven.IsStale(proven.FreshnessFacts{TermYears: converted, Anchor: clock, Now: clock})
}

// StaleByAgeOptions contains the age-based fallback inputs for termless jobs.
type StaleByAgeOptions struct {
	TermYears   []model.TermYear
	PostedAt    *time.Time
	FirstSeenAt time.Time
	Now         *time.Time
	MaxAgeDays  *int64
}

// IsStaleByAge applies the age guard only to jobs without term-year tags.
func IsStaleByAge(opts StaleByAgeOptions) bool {
	if len(opts.TermYears) != 0 {
		return false
	}
	anchor := opts.FirstSeenAt
	if opts.PostedAt != nil {
		anchor = *opts.PostedAt
	}
	now := time.Now()
	if opts.Now != nil {
		now = *opts.Now
	}
	return proven.IsStale(proven.FreshnessFacts{Anchor: anchor, Now: now, MaxAgeDays: opts.MaxAgeDays})
}

// IsStaleByLastSeen applies the ingest last-seen guard. Omit maxDays to use
// LastSeenStaleDays.
func IsStaleByLastSeen(lastSeenAt, now time.Time, maxDays ...int64) bool {
	return proven.IsStaleByLastSeen(lastSeenAt, now, maxDays...)
}

func parseFirstYear(match []string, now time.Time) *int {
	if match == nil {
		return nil
	}
	for _, value := range match[1:] {
		if len(value) != 4 || value[0] != '2' || value[1] != '0' {
			continue
		}
		year, ok := clampCohortYear(parseSmallDecimal(value), now)
		if ok {
			return &year
		}
		return nil
	}
	return nil
}

// ExtractCohortYear extracts a plausible program year near internship wording.
func ExtractCohortYear(text string, now ...time.Time) *int {
	clock := optionalNow(now)
	for _, re := range []*regexp.Regexp{termYearRE, classOfRE, startYearRE, nearInternRE, afterInternRE} {
		if year := parseFirstYear(re.FindStringSubmatch(text), clock); year != nil {
			return year
		}
	}
	return nil
}

func addDurationRange(found map[int]bool, a, b int) {
	if a > b {
		a, b = b, a
	}
	for n := a; n <= b; n++ {
		if n >= 1 && n <= 24 {
			found[n] = true
		}
	}
}

// ExtractDurationMonthsList returns duration values in months. Month ranges
// expand inclusively; week values are used only when no month value exists.
func ExtractDurationMonthsList(text string) []int {
	found := make(map[int]bool)
	covered := make([][2]int, 0)
	for _, loc := range monthRangeRE.FindAllStringSubmatchIndex(text, -1) {
		covered = append(covered, [2]int{loc[0], loc[1]})
		addDurationRange(found, parseSmallDecimal(text[loc[2]:loc[3]]), parseSmallDecimal(text[loc[4]:loc[5]]))
	}
	for _, loc := range monthSingleRE.FindAllStringSubmatchIndex(text, -1) {
		inside := false
		for _, span := range covered {
			if loc[0] >= span[0] && loc[0] < span[1] {
				inside = true
				break
			}
		}
		if !inside {
			n := parseSmallDecimal(text[loc[2]:loc[3]])
			if n >= 1 && n <= 24 {
				found[n] = true
			}
		}
	}
	if len(found) == 0 {
		for _, loc := range weekRangeRE.FindAllStringSubmatchIndex(text, -1) {
			a := int(math.Round(float64(parseSmallDecimal(text[loc[2]:loc[3]])) / 4.345))
			b := int(math.Round(float64(parseSmallDecimal(text[loc[4]:loc[5]])) / 4.345))
			addDurationRange(found, a, b)
		}
		if len(found) == 0 {
			for _, loc := range weekSingleRE.FindAllStringSubmatchIndex(text, -1) {
				months := int(math.Round(float64(parseSmallDecimal(text[loc[2]:loc[3]])) / 4.345))
				if months >= 1 && months <= 24 {
					found[months] = true
				}
			}
		}
	}
	out := make([]int, 0, len(found))
	for n := range found {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// ExtractDurationMonths returns the smallest extracted duration, or nil.
func ExtractDurationMonths(text string) *int {
	values := ExtractDurationMonthsList(text)
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}

type rolePattern struct {
	role string
	re   *regexp.Regexp
}

var rolePatterns = []rolePattern{
	{"fullstack", regexp.MustCompile(`(?i)\b(full[\s-]?stack)\b`)},
	{"frontend", regexp.MustCompile(`(?i)\b(frontend|front[\s-]?end|ui\s+engineer|web\s+(developer|engineer)|react|next\.?js)\b`)},
	{"backend", regexp.MustCompile(`(?i)\b(backend|back[\s-]?end|server[\s-]?side|api\s+engineer|distributed\s+systems)\b`)},
	{"ml", regexp.MustCompile(`(?i)\b(machine learning|\bml\b|deep learning|\bai\b|artificial intelligence|llm|nlp|natural language|computer vision|applied scientist)\b`)},
	{"data", regexp.MustCompile(`(?i)\b(data\s*(science|scientist|engineer|analytics|analyst)|analytics|business intelligence)\b`)},
	{"mobile", regexp.MustCompile(`(?i)\b(mobile|\bios\b|android|react native|flutter)\b`)},
	{"security", regexp.MustCompile(`(?i)\b(security|cyber|infosec|appsec|privacy\s+engineer)\b`)},
	{"devops", regexp.MustCompile(`(?i)\b(devops|sre|site reliability|infrastructure|platform\s+engineer|cloud\s+engineer|network\s+engineer)\b`)},
	{"hardware", regexp.MustCompile(`(?i)\b(hardware|firmware|embedded|fpga|asic|robotics|silicon|semiconductor|rtl|vlsi|chip\s+design|electrical\s+engineer)\b`)},
	{"quant", regexp.MustCompile(`(?i)\b(quant|quantitative)\b`)},
	{"product", regexp.MustCompile(`(?i)\b(product\s*manager|technical\s+product|\bpm\b|product\s+intern)\b`)},
	{"research", regexp.MustCompile(`(?i)\b(research\s+(engineer|scientist)|(?:ai|ml|machine learning|computer vision|quantitative|software)\s+research)\b`)},
	{"software", regexp.MustCompile(`(?i)\b(software|swe|sde|developer|programming|algorithm\s+engineer|full[\s-]?stack|forward\s+deployed\s+engineer)\b`)},
}

var researchRE = regexp.MustCompile(`(?i)\bresearch\b`)

func rolesIn(text string) map[string]bool {
	found := make(map[string]bool)
	for _, pattern := range rolePatterns {
		if pattern.re.MatchString(text) {
			found[pattern.role] = true
		}
	}
	if researchRE.MatchString(text) && (found["software"] || found["data"] || found["ml"] || found["hardware"] || found["quant"]) {
		found["research"] = true
	}
	return found
}

// ExtractRoles returns title roles, or description roles only when the title
// has none. It never unions incidental description disciplines into a title.
func ExtractRoles(title string, description ...string) []string {
	found := rolesIn(title)
	if len(found) == 0 && len(description) != 0 {
		found = rolesIn(description[0])
	}
	out := make([]string, 0, len(found))
	for _, role := range RoleOptions {
		if found[role] {
			out = append(out, role)
		}
	}
	return out
}

var (
	usRE             = regexp.MustCompile(`(?i)\b(united states|\bUSA\b|\bU\.S\.A\.?\b|,?\s*U\.?S\.?A?\.?\b|california|new york|texas|washington|massachusetts|illinois|georgia|florida|colorado|arizona|oregon|virginia|north carolina|new jersey|pennsylvania|ohio|michigan|seattle|san francisco|foster city|bay area|nyc|los angeles|austin|boston|chicago|denver|atlanta|sf\b|palo alto|mountain view|sunnyvale|menlo park|cupertino|redmond|bellevue|spacex\s*site|hawthorne|boca chica|cape canaveral|kennedy space|starbase)\b|, (AL|AK|AZ|AR|CA|CO|CT|DE|FL|GA|HI|IA|ID|IL|IN|KS|KY|LA|MA|MD|ME|MI|MN|MO|MS|MT|NC|ND|NE|NH|NJ|NM|NV|NY|OH|OK|OR|PA|RI|SC|SD|TN|TX|UT|VA|VT|WA|WI|WV)\b`)
	canadaRE         = regexp.MustCompile(`(?i)\b(canada|toronto|vancouver|montreal|ottawa|calgary|waterloo|ontario|quebec|british columbia|\bB\.?C\.?\b|, ON\b|, QC\b|, AB\b|, BC\b|, MB\b|, SK\b|, NS\b|, NB\b)\b`)
	europeRE         = regexp.MustCompile(`(?i)\b(united kingdom|\bUK\b|england|london|ireland|dublin|europe|european|germany|berlin|munich|france|paris|netherlands|amsterdam|sweden|stockholm|switzerland|zurich|spain|madrid|barcelona|italy|milan|denmark|copenhagen|finland|helsinki|norway|oslo|portugal|lisbon|austria|vienna|belgium|brussels|poland|warsaw)\b`)
	locationRemoteRE = regexp.MustCompile(`(?i)\bremote\b`)
)

// ExtractRegions maps locations to stable board region filters.
func ExtractRegions(locations []string, isRemote bool) []string {
	found := make(map[string]bool)
	blob := strings.Join(locations, " ")
	if isRemote || locationRemoteRE.MatchString(blob) {
		found["remote"] = true
	}
	if usRE.MatchString(blob) {
		found["us"] = true
	}
	if canadaRE.MatchString(blob) {
		found["canada"] = true
	}
	if europeRE.MatchString(blob) {
		found["europe"] = true
	}
	if len(locations) > 0 && len(found) == boolInt(found["remote"]) {
		if !found["remote"] || anyNonRemoteLocation(locations) {
			found["other"] = true
		}
	} else if len(locations) > 0 {
		nonRemote := make([]string, 0, len(locations))
		for _, location := range locations {
			if !locationRemoteRE.MatchString(location) {
				nonRemote = append(nonRemote, location)
			}
		}
		unknown := strings.Join(nonRemote, " ")
		if len(nonRemote) > 0 && !usRE.MatchString(unknown) && !canadaRE.MatchString(unknown) && !europeRE.MatchString(unknown) {
			found["other"] = true
		}
	}
	out := make([]string, 0, len(found))
	for _, region := range RegionOptions {
		if found[region] {
			out = append(out, region)
		}
	}
	return out
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func anyNonRemoteLocation(locations []string) bool {
	for _, location := range locations {
		if !locationRemoteRE.MatchString(location) {
			return true
		}
	}
	return false
}

var (
	nbspRE          = regexp.MustCompile(`(?i)&nbsp;`)
	quotRE          = regexp.MustCompile(`(?i)&quot;`)
	aposNumericRE   = regexp.MustCompile(`&#0?39;`)
	aposRE          = regexp.MustCompile(`(?i)&apos;`)
	ltRE            = regexp.MustCompile(`(?i)&lt;`)
	gtRE            = regexp.MustCompile(`(?i)&gt;`)
	ampRE           = regexp.MustCompile(`(?i)&amp;`)
	numericEntityRE = regexp.MustCompile(`(?i)&#(x[0-9a-f]+|\d+);`)
	scriptRE        = regexp.MustCompile(`(?is)<script\b.*?</script[^>]*>`)
	styleRE         = regexp.MustCompile(`(?is)<style\b.*?</style[^>]*>`)
	tagRE           = regexp.MustCompile(`<[^>]+>`)
	markdownImageRE = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markdownDataRE  = regexp.MustCompile(`(?i)\[[^\]]*\]\(data:[^)]*\)`)
	orphanDataRE    = regexp.MustCompile(`(?i)\[?data:[a-z0-9.+/-]+;base64,[a-z0-9+/=]*`)
)

func decodeNumericEntity(m string) string {
	inner := m[2 : len(m)-1] // strip &# and ;
	base := 10
	if len(inner) > 0 && (inner[0] == 'x' || inner[0] == 'X') {
		base = 16
		inner = inner[1:]
	}
	n, err := strconv.ParseUint(inner, base, 32)
	if err != nil || n == 0 || n > unicode.MaxRune {
		return m
	}
	return string(rune(n))
}

func decodeEntities(text string) string {
	text = nbspRE.ReplaceAllString(text, " ")
	text = quotRE.ReplaceAllString(text, `"`)
	text = aposNumericRE.ReplaceAllString(text, "'")
	text = aposRE.ReplaceAllString(text, "'")
	text = ltRE.ReplaceAllString(text, "<")
	text = gtRE.ReplaceAllString(text, ">")
	text = numericEntityRE.ReplaceAllStringFunc(text, decodeNumericEntity)
	return ampRE.ReplaceAllString(text, "&")
}

func stripTags(text string) string {
	text = scriptRE.ReplaceAllString(text, " ")
	text = styleRE.ReplaceAllString(text, " ")
	return tagRE.ReplaceAllString(text, " ")
}

func stripEmbeddedMedia(text string) string {
	text = markdownImageRE.ReplaceAllString(text, " ")
	text = markdownDataRE.ReplaceAllString(text, " ")
	return orphanDataRE.ReplaceAllString(text, " ")
}

func isJSWhitespace(r rune) bool {
	return unicode.IsSpace(r) || r == '\uFEFF'
}

func trimJSWhitespace(s string) string {
	return strings.TrimFunc(s, isJSWhitespace)
}

func collapseJSWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSpace := false
	for _, r := range s {
		if isJSWhitespace(r) {
			inSpace = true
			continue
		}
		if inSpace && b.Len() != 0 {
			b.WriteByte(' ')
		}
		inSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

func utf16Length(s string) int { return len(utf16.Encode([]rune(s))) }

func sliceUTF16End(s string, end int) string {
	total := utf16Length(s)
	if end < 0 {
		end = total + end
	}
	if end <= 0 {
		return ""
	}
	if end >= total {
		return s
	}
	units := 0
	for i, r := range s {
		width := 1
		if r > 0xffff {
			width = 2
		}
		if units+width > end {
			return s[:i]
		}
		units += width
		if units == end {
			return s[:i+len(string(r))]
		}
	}
	return s
}

// ExcerptFromHtml strips unsafe markup and embedded media, decodes the small
// entity set used by the TS implementation, and truncates to max UTF-16 units.
func ExcerptFromHtml(html string, maximum ...int) *string {
	if html == "" {
		return nil
	}
	max := 400
	if len(maximum) != 0 {
		max = maximum[0]
	}
	text := stripTags(html)
	text = decodeEntities(text)
	text = stripTags(text)
	text = decodeEntities(text)
	text = stripEmbeddedMedia(text)
	text = trimJSWhitespace(collapseJSWhitespace(text))
	if text == "" {
		return nil
	}
	if utf16Length(text) > max {
		text = sliceUTF16End(text, max-1) + "…"
	}
	return &text
}
