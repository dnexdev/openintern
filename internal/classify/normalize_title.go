package classify

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"github.com/dnexdev/openintern/internal/unicodex"
)

var (
	leadingCohortRE         = regexp.MustCompile(`(?i)^\s*20\d{2}\s*[-–—]\s*`)
	trailingParentheticalRE = regexp.MustCompile(`\s*\(([^)]*)\)\s*$`)
	trailingDashSegmentRE   = regexp.MustCompile(`^(.*?)(?:\s+[-–—]\s+)([^–—-]+)$`)
	trailingCommaSegmentRE  = regexp.MustCompile(`^(.*?)(?:,\s*)([^,]+)$`)
	campusPrefixRE          = regexp.MustCompile(`(?i)^campus\s+`)
	inlineCampusRE          = regexp.MustCompile(`(?i)\b(?:iit|bits|nit|iiit)\s+[\p{L}\p{N}&'.-]+`)
	inlineSeasonRE          = regexp.MustCompile(`(?i)\b(fall|winter|summer|spring|autumn)\s*(20\d{2})?\b`)
	inlineYearRE            = regexp.MustCompile(`\b20\d{2}\b`)
	internshipsRE           = regexp.MustCompile(`(?i)\binternships?\b`)
	coopsRE                 = regexp.MustCompile(`(?i)\bco-?ops?\b`)
	campusInitialRE         = regexp.MustCompile(`(?i)^(iit|bits|nit|iiit)\b`)
	universityRE            = regexp.MustCompile(`(?i)\buniversity\b`)
	campusProgramRE         = regexp.MustCompile(`(?i)^campus\s+(recruiting|program|hire|hiring)\b`)
	degreeSingleRE          = regexp.MustCompile(`(?i)^(phd/postdoc|phd|postdoc|ug/ms|ug|ms|bs|mba|m\.?s\.?|b\.?s\.?|bs/ms|bs/ms/phd|undergraduate|graduate|masters?)$`)
	degreeListRE            = regexp.MustCompile(`(?i)^(bs|ms|phd|ug|mba|postdoc)(\s*/\s*(bs|ms|phd|ug|mba|postdoc))+$`)
	seasonCohortRE          = regexp.MustCompile(`^(fall|winter|summer|spring|autumn)\s+20\d{2}$`)
	parentheticalInternRE   = regexp.MustCompile(`(?i)^internships?$`)
	parentheticalSeasonRE   = regexp.MustCompile(`(?i)^(fall|winter|summer|spring|autumn)(\s+20\d{2})?$`)
	explicitNoiseRE         = regexp.MustCompile(`(?i)^(commercial|usg|intel|government|aus|uk|eu|emea|apac|americas|federal|cleared|secret|ts/?sci|public\s+sector|enterprise|startup|early\s+career|aus\s+government|us\s+government)$`)
	noiseWordsSplitRE       = regexp.MustCompile(`[\s/]+`)
	cityCountryRE           = regexp.MustCompile(`^[A-Za-z .'-]+,\s*[A-Za-z .'-]+$`)
	locationNoiseRE         = regexp.MustCompile(`(?i)^(remote|hybrid|onsite|on-site|united states|usa|canada|uk|united kingdom|australia|germany|france|india|singapore|japan|korea|netherlands|ireland|sweden|switzerland)$`)
	shortCodeRE             = regexp.MustCompile(`^[A-Z0-9]{2,6}$`)
	bareYearRE              = regexp.MustCompile(`^20\d{2}$`)
)

var noiseWords = map[string]bool{
	"commercial": true, "usg": true, "intel": true, "government": true,
	"aus": true, "uk": true, "eu": true, "emea": true, "apac": true,
	"americas": true, "federal": true, "cleared": true, "secret": true,
	"ts": true, "sci": true, "tssci": true, "public": true, "sector": true,
	"enterprise": true, "startup": true, "early": true, "career": true,
	"remote": true, "hybrid": true, "onsite": true, "on-site": true,
	"united": true, "states": true, "usa": true, "canada": true,
	"kingdom": true, "australia": true, "germany": true, "france": true,
	"india": true, "singapore": true, "japan": true, "korea": true,
	"netherlands": true, "ireland": true, "sweden": true, "switzerland": true,
}

var officeCities = map[string]bool{
	"amsterdam": true, "austin": true, "boston": true, "chicago": true,
	"dublin": true, "hong kong": true, "london": true, "miami": true,
	"montreal": true, "new york": true, "paris": true, "san francisco": true,
	"seattle": true, "singapore": true, "sydney": true, "toronto": true,
	"zurich": true,
}

func isCampusSuffix(tail string) bool {
	t := strings.ToLower(trimJSWhitespace(tail))
	if t == "" {
		return false
	}
	return campusInitialRE.MatchString(t) || t == "campus" || universityRE.MatchString(t) || campusProgramRE.MatchString(t)
}

func isDegreeLabel(text string) bool {
	t := strings.ToLower(trimJSWhitespace(text))
	return t != "" && (degreeSingleRE.MatchString(t) || degreeListRE.MatchString(t))
}

func isSeasonCohortSuffix(tail string) bool {
	t := strings.ToLower(trimJSWhitespace(tail))
	return bareYearRE.MatchString(t) || seasonCohortRE.MatchString(t)
}

func shouldStripParenthetical(inner string) bool {
	if inner == "" || isDegreeLabel(inner) {
		return false
	}
	return parentheticalInternRE.MatchString(inner) || isSeasonCohortSuffix(inner) ||
		parentheticalSeasonRE.MatchString(inner) || isNoiseSuffix(inner)
}

func isNoiseSuffix(tail string) bool {
	t := strings.ToLower(trimJSWhitespace(tail))
	if t == "" || utf16Length(t) > 48 {
		return false
	}
	if explicitNoiseRE.MatchString(t) {
		return true
	}
	words := noiseWordsSplitRE.Split(t, -1)
	filtered := words[:0]
	for _, word := range words {
		if word != "" {
			filtered = append(filtered, word)
		}
	}
	if len(filtered) >= 1 && len(filtered) <= 4 {
		allNoise := true
		for _, word := range filtered {
			if !noiseWords[word] {
				allNoise = false
				break
			}
		}
		if allNoise {
			return true
		}
	}
	if cityCountryRE.MatchString(tail) || locationNoiseRE.MatchString(t) {
		return true
	}
	if shortCodeRE.MatchString(tail) && tail == strings.ToUpper(tail) {
		return true
	}
	return isSeasonCohortSuffix(tail) || officeCities[t] || isCampusSuffix(tail)
}

func replaceNonTitleRunes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || isJSWhitespace(r) || r == '+' || r == '/' || r == '&' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// NormalizeTitle removes location, campus, season, cohort, and program noise
// while retaining degree eligibility and role specialties.
func NormalizeTitle(raw string) string {
	s := trimJSWhitespace(unicodex.NFKC(raw))
	s = trimJSWhitespace(leadingCohortRE.ReplaceAllString(s, ""))

	for {
		match := trailingParentheticalRE.FindStringSubmatchIndex(s)
		if match == nil {
			break
		}
		inner := trimJSWhitespace(s[match[2]:match[3]])
		if !shouldStripParenthetical(inner) {
			break
		}
		s = trimJSWhitespace(s[:match[0]])
	}

	for {
		match := trailingDashSegmentRE.FindStringSubmatch(s)
		if match == nil {
			break
		}
		head, tail := trimJSWhitespace(match[1]), trimJSWhitespace(match[2])
		if !isNoiseSuffix(tail) && !isCampusSuffix(tail) {
			break
		}
		s = head
	}

	for {
		match := trailingCommaSegmentRE.FindStringSubmatch(s)
		if match == nil {
			break
		}
		head, tail := trimJSWhitespace(match[1]), trimJSWhitespace(match[2])
		if !isNoiseSuffix(tail) && !isCampusSuffix(tail) {
			break
		}
		s = head
	}

	s = trimJSWhitespace(campusPrefixRE.ReplaceAllString(s, ""))
	s = inlineCampusRE.ReplaceAllString(s, " ")
	s = inlineSeasonRE.ReplaceAllString(s, " ")
	s = inlineYearRE.ReplaceAllString(s, " ")
	s = internshipsRE.ReplaceAllString(s, "intern")
	s = coopsRE.ReplaceAllString(s, "coop")
	s = strings.ToLower(s)
	s = replaceNonTitleRunes(s)
	s = trimJSWhitespace(collapseJSWhitespace(s))
	return s
}

// ShortHash returns the requested leading hexadecimal characters of SHA-256.
func ShortHash(input string, length ...int) string {
	n := 10
	if len(length) != 0 {
		n = length[0]
	}
	digest := sha256.Sum256([]byte(input))
	encoded := hex.EncodeToString(digest[:])
	return sliceUTF16End(encoded, n)
}

// JobFingerprint is the stable identity hash for an ATS posting.
func JobFingerprint(companyID, source, externalJobID string) string {
	digest := sha256.Sum256([]byte(companyID + "|" + source + "|" + externalJobID))
	return hex.EncodeToString(digest[:])
}

// RoleFamilyId combines a company slug with the normalized-title hash.
func RoleFamilyId(companySlug, normalizedTitle string) string {
	return companySlug + ":" + ShortHash(normalizedTitle)
}
