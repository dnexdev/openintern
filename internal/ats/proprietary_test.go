package ats

import (
	"regexp"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/internal/state/jsjson"
)

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	v, err := jsjson.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func mustArray(t *testing.T, s string) []any {
	t.Helper()
	return mustJSON(t, s).([]any)
}

// Port of proprietary.test.ts.
func TestCitadelListing(t *testing.T) {
	html := `
<div class="career-listing__container">
<a class="careers-listing-card js-career-card js-apply-now" href="https://www.citadel.com/careers/details/software-engineer-intern-us/" data-position="Software Engineer – Intern (US)">
  <div class="careers-listing-card__content">
    <div class="careers-listing-card__title">
      <h2>Software Engineer &#8211; Intern (US)</h2>
    </div>
    <span class="careers-listing-card__location">Greenwich, Houston, Miami, New York</span>
  </div>
</a>
<a class="careers-listing-card js-career-card" href="/careers/details/software-engineer/">
  <div class="careers-listing-card__title"><h2>Software Engineer</h2></div>
  <span class="careers-listing-card__location">New York</span>
</a>
</div>
`
	jobs := ParseCitadelListingHTML(html, "https://www.citadel.com")
	if len(jobs) != 2 {
		t.Fatalf("%d jobs", len(jobs))
	}
	j := jobs[0]
	if j.ExternalID != "software-engineer-intern-us" || j.Title != "Software Engineer – Intern (US)" ||
		strings.Join(j.Locations, "|") != "Greenwich|Houston|Miami|New York" ||
		j.ApplyURL != "https://www.citadel.com/careers/details/software-engineer-intern-us/" {
		t.Fatalf("%+v", j)
	}
	if jobs[1].ApplyURL != "https://www.citadel.com/careers/details/software-engineer/" {
		t.Fatalf("relative href: %+v", jobs[1])
	}
}

func TestTeslaState(t *testing.T) {
	state := mustJSON(t, `{
		"sites": {"1": "Fremont, California", "2": "Austin, Texas"},
		"listings": [
			{"id": 233415, "t": "Internship, Software Engineer", "sid": "1", "d": "Build cars"},
			{"id": "99", "title": "Staff Engineer", "l": "Remote"}
		]}`)
	jobs := ParseTeslaCareersState(state)
	if len(jobs) != 2 {
		t.Fatalf("%d jobs", len(jobs))
	}
	j := jobs[0]
	if j.ExternalID != "233415" || j.Title != "Internship, Software Engineer" ||
		strings.Join(j.Locations, "|") != "Fremont, California" ||
		j.ApplyURL != "https://www.tesla.com/careers/search/job/233415" {
		t.Fatalf("%+v", j)
	}
}

func TestBytedanceMapping(t *testing.T) {
	mapped := MapBytedancePosts(mustArray(t, `[{
		"id": "7657410043649804597",
		"title": "Platform Strategy Project Intern",
		"description": "Shop intern role",
		"city_info": {"en_name": "Singapore", "parent": {"en_name": "Singapore", "parent": {"en_name": "Singapore"}}},
		"recruit_type": {"id": "202", "en_name": "Intern"}
	}]`), TiktokPortal)
	if mapped[0].ExternalID != "7657410043649804597" || mapped[0].ApplyURL != "https://lifeattiktok.com/search/7657410043649804597" ||
		!strings.Contains(mapped[0].Locations[0], "Singapore") {
		t.Fatalf("%+v", mapped[0])
	}
	bd := MapBytedancePosts(mustArray(t, `[{"id": "1", "title": "SWE Intern", "description": "x"}]`), BytedancePortal)
	if bd[0].ApplyURL != "https://joinbytedance.com/search/1" {
		t.Fatalf("%+v", bd[0])
	}
}

func TestPhenomMapping(t *testing.T) {
	jobs := MapPhenomJobs(mustArray(t, `[{"data": {
		"req_id": "77690", "slug": "77690", "title": "FPGA Design Intern",
		"description": "<p>Build chips</p>", "full_location": "Austin, TX",
		"meta_data": {"canonical_url": "https://careers.amd.com/jobs/77690?lang=en-us"},
		"posted_date": "2026-08-01T00:00:00Z"}}]`), "https://careers.amd.com")
	j := jobs[0]
	if j.ExternalID != "77690" || j.Title != "FPGA Design Intern" || j.ApplyURL != "https://careers.amd.com/jobs/77690?lang=en-us" {
		t.Fatalf("%+v", j)
	}
	if j.PostedAt == nil || *j.PostedAt != "2026-08-01T00:00:00.000Z" || j.Description != "Build chips" {
		t.Fatalf("%+v", j)
	}
}

func TestEightfoldMapping(t *testing.T) {
	b := ParseEightfoldToken("microsoft|ai")
	if b.Domain != "microsoft.com" || b.TitleFilter == nil {
		t.Fatalf("%+v", b)
	}
	jobs := MapEightfoldPositions(mustArray(t, `[
		{"id": 1970393556951950, "name": "Software Engineer: AI/ML & LLM Intern Opportunities",
		 "locations": ["United States, Washington, Redmond"], "positionUrl": "/careers/job/1970393556951950",
		 "postedTs": 1785740400, "department": "Software Engineering"},
		{"id": 99, "name": "Account Management INTERN", "locations": ["Germany"], "positionUrl": "/careers/job/99"}
	]`), b)
	if len(jobs) != 1 || jobs[0].ExternalID != "1970393556951950" || !regexp.MustCompile(`1970393556951950`).MatchString(jobs[0].ApplyURL) {
		t.Fatalf("%+v", jobs)
	}
	if jobs[0].PostedAt == nil || *jobs[0].PostedAt != "2026-08-03T07:00:00.000Z" {
		t.Fatalf("postedTs is seconds: %v", jobs[0].PostedAt)
	}
}

func TestAvatureSearchHTML(t *testing.T) {
	html := `
<a class="link" href="https://bloomberg.avature.net/careers/JobDetail/Software-Engineer-Intern/21267">Software Engineer Intern</a>
<div>JobDetail/Software-Engineer-Intern/21267</div>
<span class="list-item-location">New York, NY</span>
`
	jobs, err := ParseAvatureSearchHTML(html, "https://bloomberg.avature.net/careers/SearchJobs/")
	if err != nil || len(jobs) != 1 || jobs[0].ExternalID != "21267" || jobs[0].Title != "Software Engineer Intern" {
		t.Fatalf("%+v %v", jobs, err)
	}
	if strings.Join(jobs[0].Locations, "|") != "New York, NY" {
		t.Fatalf("location: %+v", jobs[0].Locations)
	}
}

func TestIcimsSearchHTML(t *testing.T) {
	html := `
<a href="https://careers-sig.icims.com/jobs/11099/analytics-internship%3a-fall-2026/job?in_iframe=1"></a>
<h3>Analytics Internship: Fall 2026</h3>
`
	jobs, err := ParseIcimsSearchHTML(html, "https://careers-sig.icims.com/jobs/search")
	if err != nil || len(jobs) != 1 || jobs[0].ExternalID != "11099" || !regexp.MustCompile(`(?i)Analytics Internship`).MatchString(jobs[0].Title) {
		t.Fatalf("%+v %v", jobs, err)
	}
	if jobs[0].ApplyURL != "https://careers-sig.icims.com/jobs/11099/analytics-internship%3a-fall-2026/job" {
		t.Fatalf("in_iframe kept: %s", jobs[0].ApplyURL)
	}
}

func TestDeshawNextData(t *testing.T) {
	jobs := ParseDeshawNextData(mustJSON(t, `{"props": {"pageProps": {"internships": [{
		"office": [{"name": "New York"}],
		"data": {"id": 5894, "displayName": "Software Developer Intern (New York) – Summer 2027",
		         "jobUrl": "Software-Developer-Intern-New-York-Summer-2027-5894", "jobDescription": "<p>Write code</p>"}}]}}}`))
	if jobs[0].ExternalID != "5894" || jobs[0].ApplyURL != "https://www.deshaw.com/careers/Software-Developer-Intern-New-York-Summer-2027-5894" {
		t.Fatalf("%+v", jobs[0])
	}
}

func TestShopifyCareersHTML(t *testing.T) {
	jobs := ParseShopifyCareersHTML(`<a href="/careers/software-engineer-intern_a4b8357e-2849-4ef4-80da-459e232f9f16"> intern </a>`)
	if jobs[0].ExternalID != "a4b8357e-2849-4ef4-80da-459e232f9f16" || !regexp.MustCompile(`(?i)Software Engineer Intern`).MatchString(jobs[0].Title) {
		t.Fatalf("%+v", jobs[0])
	}
}

func TestGoogleCareersHTML(t *testing.T) {
	jobs := ParseGoogleCareersHTML(`ssk='17:12345'<div class="x">jobs/results/12345-software-engineering-intern</div><h3 class="QJPWVe">Software Engineering Intern</h3>`)
	if jobs[0].ExternalID != "12345" || jobs[0].Title != "Software Engineering Intern" ||
		jobs[0].ApplyURL != "https://www.google.com/about/careers/applications/jobs/results/12345-software-engineering-intern" {
		t.Fatalf("%+v", jobs[0])
	}
	fallback := ParseGoogleCareersHTML(`<a href="jobs/results/777-ml-research-intern-2nd">x</a>`)
	if len(fallback) != 1 || fallback[0].Title != "Ml Research Intern 2nd" {
		t.Fatalf("%+v", fallback)
	}
}

// Port of ats.test.ts (its inline mappers are exercised through the real
// adapters in fixtures_test.go).
func TestWorkdayLocationCount(t *testing.T) {
	if !WorkdayLocationCount("3 Locations") || !WorkdayLocationCount("2 locations") || WorkdayLocationCount("Santa Clara, CA") {
		t.Fatal("Workday location-count placeholder")
	}
}

func TestIsProprietary(t *testing.T) {
	if !IsProprietary("tesla") || !IsProprietary("x") || IsProprietary("greenhouse") || IsProprietary("workday") {
		t.Fatal("IsProprietary")
	}
}
