package board

import (
	"regexp"
	"strings"

	"github.com/dnexdev/openintern/internal/proven"
)

var campusInTitle = regexp.MustCompile(`(?i)(?:^|[,–—-]\s*)((?:IIT|BITS|NIT|IIIT)\s+[\p{L}\p{N}&'.-]+)`)

func ExtractCampusFromTitle(title string) *string {
	match := campusInTitle.FindStringSubmatch(title)
	if len(match) < 2 {
		return nil
	}
	value := strings.TrimSpace(match[1])
	if value == "" {
		return nil
	}
	return &value
}

type FamilyPosting struct {
	ID        string
	Title     string
	Location  string
	Locations []string
	ApplyURL  string
}

type ApplyRow struct {
	Key          string
	JobID        string
	Label        string
	ApplyURL     string
	AppliedTitle string
}

func PostingDisplayLabel(posting FamilyPosting, siblings []FamilyPosting) string {
	campus := ExtractCampusFromTitle(posting.Title)
	sameLocation := 0
	for _, sibling := range siblings {
		if sibling.Location == posting.Location {
			sameLocation++
		}
	}
	if campus != nil && sameLocation > 1 {
		return *campus
	}
	if campus != nil {
		return *campus + " · " + posting.Location
	}
	if sameLocation > 1 {
		return posting.Title
	}
	return posting.Location
}

func FormatPostingLocations(locations []string, remote bool) string {
	return formatPostingLocations(locations, remote)
}

func trimmedLocations(locations []string) []string {
	out := make([]string, 0, len(locations))
	for _, location := range locations {
		if location = strings.TrimSpace(location); location != "" {
			out = append(out, location)
		}
	}
	return out
}

func NeedsApplyPicker(postings []FamilyPosting) bool {
	locationCount := 0
	if len(postings) > 0 {
		locationCount = len(trimmedLocations(postings[0].Locations))
	}
	return proven.NeedsPicker(int64(len(postings)), int64(locationCount))
}

func FamilyApplyRows(familyTitle string, postings []FamilyPosting) []ApplyRow {
	if len(postings) == 0 {
		return []ApplyRow{}
	}
	if len(postings) == 1 {
		posting := postings[0]
		locations := trimmedLocations(posting.Locations)
		label := posting.Location
		if len(locations) > 0 {
			label = strings.Join(locations, " · ")
		}
		if label == "" {
			label = "Location n/a"
		}
		appliedTitle := familyTitle
		if proven.QualifySingleTitle(int64(len(locations))) {
			appliedTitle += " (" + label + ")"
		}
		return []ApplyRow{{
			Key: posting.ID, JobID: posting.ID, Label: label,
			ApplyURL: posting.ApplyURL, AppliedTitle: appliedTitle,
		}}
	}
	rows := make([]ApplyRow, 0, len(postings))
	for _, posting := range postings {
		label := PostingDisplayLabel(posting, postings)
		rows = append(rows, ApplyRow{
			Key: posting.ID, JobID: posting.ID, Label: label,
			ApplyURL: posting.ApplyURL, AppliedTitle: familyTitle + " (" + label + ")",
		})
	}
	return rows
}

func FamilyPostingsLabel(postingCount, uniqueLocationCount int) string {
	word := "location"
	if proven.SaysOpenings(int64(postingCount), int64(uniqueLocationCount)) {
		word = "opening"
	}
	if postingCount != 1 {
		word += "s"
	}
	return integerString(postingCount) + " " + word
}

func FamilyMetaLabel(familyTitle string, postings []FamilyPosting) string {
	if NeedsApplyPicker(postings) {
		if len(postings) > 1 {
			locations := make(map[string]struct{})
			for _, posting := range postings {
				locations[posting.Location] = struct{}{}
			}
			return FamilyPostingsLabel(len(postings), len(locations))
		}
		locations := trimmedLocations(postings[0].Locations)
		word := "locations"
		if len(locations) == 1 {
			word = "location"
		}
		return integerString(len(locations)) + " " + word
	}
	rows := FamilyApplyRows(familyTitle, postings)
	if len(rows) == 0 {
		return "Location n/a"
	}
	return rows[0].Label
}

func FamilyPickerButtonLabel(postings []FamilyPosting) string {
	if len(postings) > 1 {
		locations := make(map[string]struct{})
		for _, posting := range postings {
			locations[posting.Location] = struct{}{}
		}
		if proven.SaysOpenings(int64(len(postings)), int64(len(locations))) {
			return "Show openings"
		}
	}
	return "Show locations"
}

func integerString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [24]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
