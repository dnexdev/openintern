package classify

import (
	"reflect"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

func mustTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("want %#v, got %#v", want, got)
	}
}

func assertSlice[T comparable](t *testing.T, got, want []T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %#v, got %#v", want, got)
	}
}

func TestIsTechInternshipCases(t *testing.T) {
	cases := []struct {
		title       string
		description string
		want        bool
	}{
		{"Software Engineer Intern", "", true},
		{"Machine Learning Co-op", "", true},
		{"Product Manager Intern", "", true},
		{"Technical Support Engineer Intern", "", false},
		{"Engineering Internship", "Build software systems in TypeScript", true},
		{"Engineering Internship", "Support manufacturing process improvements", false},
		{"Platform Strategy Project Intern", "Partner with software engineering and data teams", false},
		{"Product Operations Intern", "Work with engineers on platform tooling", false},
		{"Research Intern", "Interview users and partner with product teams", false},
		{"Research Intern - Machine Learning", "", true},
		{"Robotics Software Intern", "", true},
		{"Quantitative Strategy Intern", "", true},
		{"Hardware Design Intern", "", true},
		{"Full Stack Software Engineer Intern - Winter 2027", "", true},
		{"New Grad Software Engineer", "", false},
		{"Marketing Intern", "", false},
		{"University Recruiter, Contract", "", false},
		{"Industrial Engineering Internship", "", false},
		{"Intern Process Engineer", "", false},
		{"Mechanical Engineering Co-op", "", false},
		{"Quality Engineer Intern", "", false},
		{"Sales Project Manager Intern", "", false},
		{"Engineering Internship (8h) - Tires", "", false},
		{"Global Trade Compliance Intern", "", false},
		{"Fall 2026 Business Operations Internship/Co-op", "", false},
		{"SDR Intern - Munich", "", false},
		{"Government Affairs Intern", "", false},
		{"Software Engineer Intern (Fall / Winter 2026)", "", true},
		{"Campus Software Engineer Intern", "", true},
		{"Forward Deployed Software Engineer - Internship - Commercial", "", true},
		{"Year at Palantir - Forward Deployed Software Engineer - Internship", "", true},
		{"Software Engineer Project Intern - Recommendation Infrastructure", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			if got := IsTechInternship(tc.title, tc.description); got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestLooksRemoteCases(t *testing.T) {
	if !LooksRemote([]string{"Remote - US"}, "Engineer Intern") {
		t.Fatal("remote location not recognized")
	}
	if LooksRemote([]string{"New York, NY"}, "Engineer Intern") {
		t.Fatal("New York marked remote")
	}
}

func TestExtractTermsCases(t *testing.T) {
	assertSlice(t, ExtractTerms("Software Intern (Spring 2026)"), []string{"spring"})
	assertSlice(t, ExtractTerms("Software Intern (Summer 2026)"), []string{"summer"})
	assertSlice(t, ExtractTerms("Fall or Winter co-op, autumn start"), []string{"fall", "winter"})
	assertSlice(t, ExtractTerms("Software Engineer Intern"), []string{})
}

func TestExtractTermYearsCases(t *testing.T) {
	now := mustTime("2026-10-24T00:00:00Z")
	assertSlice(t, ExtractTermYears("Software Intern (Spring 2027)", nil, now), []model.TermYear{{Term: "spring", Year: 2027}})
	year := 2026
	assertSlice(t, ExtractTermYears("Winter 2026 Software Intern", &year, now), []model.TermYear{{Term: "winter", Year: 2026}})
	year = 2027
	assertSlice(t, ExtractTermYears("Summer Intern", &year, now), []model.TermYear{{Term: "summer", Year: 2027}})
}

func TestTermYearFreshnessCases(t *testing.T) {
	if IsStaleByTermYears([]model.TermYear{{Term: "summer", Year: 2027}}, mustTime("2026-07-09T12:00:00Z")) {
		t.Fatal("summer 2027 future marked stale")
	}
	if !IsStaleByTermYears([]model.TermYear{{Term: "fall", Year: 2026}}, mustTime("2026-10-24T12:00:00Z")) {
		t.Fatal("fall 2026 not stale after start")
	}
	if IsStaleByTermYears([]model.TermYear{{Term: "winter", Year: 2027}}, mustTime("2026-10-24T12:00:00Z")) {
		t.Fatal("winter 2027 stale in October")
	}
	if !IsStaleByTermYears([]model.TermYear{{Term: "winter", Year: 2026}}, mustTime("2026-07-09T12:00:00Z")) {
		t.Fatal("winter 2026 not stale in July")
	}
	if IsStaleByTermYears([]model.TermYear{{Term: "fall", Year: 2026}}, mustTime("2026-07-09T12:00:00Z")) {
		t.Fatal("fall 2026 stale in July")
	}
	if IsStaleByTermYears(nil, mustTime("2026-07-09T12:00:00Z")) {
		t.Fatal("empty term years marked stale")
	}
}

func TestExtractDurationMonthsListCases(t *testing.T) {
	assertSlice(t, ExtractDurationMonthsList("4 to 6 months"), []int{4, 5, 6})
	assertSlice(t, ExtractDurationMonthsList("a 4-month co-op"), []int{4})
	assertSlice(t, ExtractDurationMonthsList("16-week summer program"), []int{4})
	assertSlice(t, ExtractDurationMonthsList("Software Intern"), []int{})
}

func TestExtractCohortYearCases(t *testing.T) {
	now := mustTime("2026-10-24T00:00:00Z")
	cases := []struct {
		text string
		want *int
	}{
		{"New Grad Software Engineer - 2027 Start", intPtr(2027)},
		{"Engineer, starting in 2027", intPtr(2027)},
		{"Software Intern (Summer 2026)", intPtr(2026)},
		{"Software Engineer Intern", nil},
	}
	for _, tc := range cases {
		assertOptionalInt(t, ExtractCohortYear(tc.text, now), tc.want)
	}
}

func intPtr(n int) *int { return &n }

func assertOptionalInt(t *testing.T, got, want *int) {
	t.Helper()
	if got == nil || want == nil {
		if got != nil || want != nil {
			t.Fatalf("want %#v, got %#v", want, got)
		}
		return
	}
	assertEqual(t, *got, *want)
}

func TestIsStaleByAgeCases(t *testing.T) {
	now := mustTime("2026-07-09T12:00:00Z")
	posted := mustTime("2024-09-30T00:00:00Z")
	first := mustTime("2026-07-01T00:00:00Z")
	if !IsStaleByAge(StaleByAgeOptions{PostedAt: &posted, FirstSeenAt: first, Now: &now}) {
		t.Fatal("old postedAt with no term years not stale")
	}
	if IsStaleByAge(StaleByAgeOptions{TermYears: []model.TermYear{{Term: "fall", Year: 2026}}, PostedAt: &posted, FirstSeenAt: first, Now: &now}) {
		t.Fatal("age guard applied despite term years")
	}
}

func TestFreshnessReferenceCases(t *testing.T) {
	now := mustTime("2027-01-15T12:00:00Z")
	old := mustTime("2025-01-01T00:00:00Z")
	if IsStaleByTermYears([]model.TermYear{{Term: "summer", Year: 2027}}, now) {
		t.Fatal("future term does not remain fresh")
	}
	if !IsStaleByTermYears([]model.TermYear{{Term: "fall", Year: 2026}}, now) {
		t.Fatal("past term remains fresh")
	}
	recent := now.Add(-time.Duration(MaxAgeDaysNoTerm-1) * 24 * time.Hour)
	if IsStaleByAge(StaleByAgeOptions{PostedAt: &recent, FirstSeenAt: old, Now: &now}) {
		t.Fatal("recent termless posting is stale")
	}
	oldFirst := now.Add(-time.Duration(MaxAgeDaysNoTerm+1) * 24 * time.Hour)
	if !IsStaleByAge(StaleByAgeOptions{FirstSeenAt: oldFirst, Now: &now}) {
		t.Fatal("old termless posting is fresh")
	}
}

func TestExtractRolesCases(t *testing.T) {
	cases := []struct {
		title       string
		description string
		want        []string
	}{
		{"Software Engineer Intern", "", []string{"software"}},
		{"ML Backend Intern", "", []string{"backend", "ml"}},
		{"Data Science Co-op", "", []string{"data"}},
		{"Research Intern - Computer Vision", "", []string{"ml", "research"}},
		{"Algorithm Engineer Intern", "", []string{"software"}},
		{"Firmware Intern - ASIC", "", []string{"hardware"}},
		{"Industrial Engineering Internship", "", []string{}},
		{"Engineering Internship", "", []string{}},
		{"Intern", "Looking for a software developer to join our team", []string{"software"}},
	}
	for _, tc := range cases {
		assertSlice(t, ExtractRoles(tc.title, tc.description), tc.want)
	}
}

func TestExtractRegionsCases(t *testing.T) {
	cases := []struct {
		locations []string
		remote    bool
		want      []string
	}{
		{[]string{"Flexible - Any SpaceX Site"}, false, []string{"us"}},
		{[]string{"San Francisco, CA"}, false, []string{"us"}},
		{[]string{"Foster City, CA", "Remote"}, true, []string{"remote", "us"}},
		{[]string{"Toronto, ON", "Remote"}, true, []string{"remote", "canada"}},
		{[]string{"London, UK"}, false, []string{"europe"}},
	}
	for _, tc := range cases {
		assertSlice(t, ExtractRegions(tc.locations, tc.remote), tc.want)
	}
}

func TestExcerptFromHtmlCases(t *testing.T) {
	cases := []struct {
		input string
		want  *string
	}{
		{"<p>Hello &amp; welcome</p>", stringPtr("Hello & welcome")},
		{"![banner](data:image/png;base64,iVBORw0KGgoAAAANSUhEUg) About the role: build robots.", stringPtr("About the role: build robots.")},
		{"[data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAABaAAAAEsCAYAAAA1nRR4AAAQAElEQVR4AezdW3bTSvb48V2mT+JzzkP7vJFw1mozgoYRdDKChhGQjIBkBIQREEZAGAH0CJIeAfQIUK/1J8lb5/dwwA4H1b+2hMmF2FHJupRKXy07F1uXqk/JsrS1VRoIAwIIIIAAAggggAACCCCAAAIIxC5A/RBAAAEEEGhFgAB0K+wsFAEEEEAAAQT6K0DNEUAAAQQQQAABBBBAAIH+CBCA7k9bU9PrAvyPAAIIIIAAAggggAACCCCAAALxC1BDBBoVYAAdKv8LBwBBBBAAAEEEEAAgf4IUFMEEEAAAQQQQACB/gkQgO5fm1NjBBBAAAEEEEAAA", nil},
	}
	for _, tc := range cases {
		got := ExcerptFromHtml(tc.input)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("want %#v, got %#v", tc.want, got)
		}
	}
}

func stringPtr(s string) *string { return &s }
