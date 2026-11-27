package classify

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

type classifierGoldenInput struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Locations   []string `json:"locations"`
	ID          string   `json:"id"`
	CompanySlug string   `json:"companySlug"`
	Source      string   `json:"source"`
	PostedAt    *string  `json:"postedAt"`
	FirstSeenAt string   `json:"firstSeenAt"`
	LastSeenAt  string   `json:"lastSeenAt"`
}

type classifierGoldenOutput struct {
	Tech               bool             `json:"tech"`
	Remote             bool             `json:"remote"`
	Terms              []string         `json:"terms"`
	CohortYear         *int             `json:"cohortYear"`
	TermYears          []model.TermYear `json:"termYears"`
	DurationMonths     []int            `json:"durationMonths"`
	FirstDurationMonth *int             `json:"firstDurationMonth"`
	Roles              []string         `json:"roles"`
	Regions            []string         `json:"regions"`
	Excerpt            *string          `json:"excerpt"`
	Normalized         string           `json:"normalized"`
	ShortHash          string           `json:"shortHash"`
	Fingerprint        string           `json:"fingerprint"`
	RoleFamilyID       string           `json:"roleFamilyId"`
	StaleByTermYears   bool             `json:"staleByTermYears"`
	StaleByAge         bool             `json:"staleByAge"`
	StaleByLastSeen    bool             `json:"staleByLastSeen"`
}

func TestClassifierTypeScriptGoldens(t *testing.T) {
	file, err := os.Open("testdata/goldens.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	var golden struct {
		Now            string `json:"now"`
		RealCount      int    `json:"realCount"`
		SyntheticCount int    `json:"syntheticCount"`
		Cases          []struct {
			Input  classifierGoldenInput  `json:"input"`
			Output classifierGoldenOutput `json:"output"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(compressed).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	if golden.RealCount != 1434 || golden.SyntheticCount < 4_000 || len(golden.Cases) != golden.RealCount+golden.SyntheticCount {
		t.Fatalf("unexpected corpus counts: real=%d synthetic=%d total=%d", golden.RealCount, golden.SyntheticCount, len(golden.Cases))
	}
	now := mustTime(golden.Now)
	for _, tc := range golden.Cases {
		t.Run(tc.Input.Key, func(t *testing.T) {
			text := tc.Input.Title + " " + tc.Input.Description
			assertEqual(t, IsTechInternship(tc.Input.Title, tc.Input.Description), tc.Output.Tech)
			remote := LooksRemote(tc.Input.Locations, tc.Input.Title)
			assertEqual(t, remote, tc.Output.Remote)
			assertSlice(t, ExtractTerms(text), tc.Output.Terms)
			cohort := ExtractCohortYear(text, now)
			assertOptionalInt(t, cohort, tc.Output.CohortYear)
			termYears := ExtractTermYears(text, cohort, now)
			if !reflect.DeepEqual(termYears, tc.Output.TermYears) {
				t.Fatalf("term years: want %#v, got %#v", tc.Output.TermYears, termYears)
			}
			assertSlice(t, ExtractDurationMonthsList(text), tc.Output.DurationMonths)
			assertOptionalInt(t, ExtractDurationMonths(text), tc.Output.FirstDurationMonth)
			assertSlice(t, ExtractRoles(tc.Input.Title, tc.Input.Description), tc.Output.Roles)
			assertSlice(t, ExtractRegions(tc.Input.Locations, remote), tc.Output.Regions)
			if got := ExcerptFromHtml(tc.Input.Description); !reflect.DeepEqual(got, tc.Output.Excerpt) {
				t.Fatalf("excerpt: want %#v, got %#v", tc.Output.Excerpt, got)
			}
			normalized := NormalizeTitle(tc.Input.Title)
			assertEqual(t, normalized, tc.Output.Normalized)
			assertEqual(t, ShortHash(normalized), tc.Output.ShortHash)
			assertEqual(t, JobFingerprint(tc.Input.CompanySlug, tc.Input.Source, tc.Input.ID), tc.Output.Fingerprint)
			assertEqual(t, RoleFamilyId(tc.Input.CompanySlug, normalized), tc.Output.RoleFamilyID)
			assertEqual(t, IsStaleByTermYears(termYears, now), tc.Output.StaleByTermYears)
			firstSeen := mustTime(tc.Input.FirstSeenAt)
			var posted *time.Time
			if tc.Input.PostedAt != nil {
				parsed := mustTime(*tc.Input.PostedAt)
				posted = &parsed
			}
			assertEqual(t, IsStaleByAge(StaleByAgeOptions{TermYears: termYears, PostedAt: posted, FirstSeenAt: firstSeen, Now: &now}), tc.Output.StaleByAge)
			assertEqual(t, IsStaleByLastSeen(mustTime(tc.Input.LastSeenAt), now), tc.Output.StaleByLastSeen)
		})
	}
}
