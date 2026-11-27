package board

import (
	"net/url"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/dnexdev/openintern/internal/proven"
)

type BoardSearch struct {
	Query     string
	Company   string
	Roles     []string
	Regions   []string
	Terms     []string
	Durations []int
	Sort      FamilySort
	Page      int64
}

func firstValue(values url.Values, key string) string {
	if all, ok := values[key]; ok && len(all) > 0 {
		return all[0]
	}
	return ""
}

func truncateUTF16(value string, maximum int) string {
	units := utf16.Encode([]rune(value))
	if len(units) <= maximum {
		return value
	}
	units = units[:maximum]
	if units[len(units)-1] >= 0xd800 && units[len(units)-1] <= 0xdbff {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units))
}

func allowedStrings(values []string, allowed []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, option := range allowed {
			if value == option {
				out = append(out, value)
				break
			}
		}
	}
	return out
}

// ParseBoardSearchParams applies the page form's allowlists and defaults. API
// parsing differs slightly because the API also accepts comma-separated lists.
func (s *Store) ParseBoardSearchParams(values url.Values) BoardSearch {
	durations := make([]int, 0, len(values["duration"]))
	for _, raw := range values["duration"] {
		n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			continue
		}
		for _, option := range DurationOptions {
			if n == float64(option) {
				durations = append(durations, option)
				break
			}
		}
	}
	familySort := SortFirstSeen
	switch firstValue(values, "sort") {
	case string(SortPosted):
		familySort = SortPosted
	case string(SortPrestige):
		familySort = SortPrestige
	}
	pageValues := values["page"]
	return BoardSearch{
		Query:     truncateUTF16(strings.TrimSpace(firstValue(values, "q")), 100),
		Company:   s.CanonicalCompanySlug(strings.TrimSpace(firstValue(values, "company"))),
		Roles:     allowedStrings(values["role"], RoleOptions),
		Regions:   allowedStrings(values["region"], RegionOptions),
		Terms:     allowedStrings(values["term"], TermOptions),
		Durations: durations,
		Sort:      familySort,
		Page:      proven.ParsePage(pageValues...),
	}
}
