package board

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/model"
)

func TestParseKillSwitch(t *testing.T) {
	killSwitch, err := ParseKillSwitch([]byte(`{
		"jobs":[" A ",""],"companies":["Example"],"hosts":["EVIL.EXAMPLE"],
		"maintenance":true,"message":"  planned work  "
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := killSwitch.Jobs["a"]; !ok {
		t.Fatal("job ID was not normalized")
	}
	if _, ok := killSwitch.Companies["example"]; !ok {
		t.Fatal("company was not normalized")
	}
	if _, ok := killSwitch.Hosts["evil.example"]; !ok {
		t.Fatal("host was not normalized")
	}
	if !killSwitch.Maintenance || killSwitch.Message == nil || *killSwitch.Message != "planned work" {
		t.Fatalf("maintenance fields = %+v", killSwitch)
	}

	invalid := []string{
		`{"jobs":"no"}`,
		`{"companies":[1]}`,
		`{"hosts":false}`,
		`{"maintenance":"yes"}`,
	}
	for _, input := range invalid {
		if _, err := ParseKillSwitch([]byte(input)); err == nil {
			t.Errorf("ParseKillSwitch(%s) succeeded", input)
		}
	}
}

func TestKilled(t *testing.T) {
	killSwitch, err := ParseKillSwitch([]byte(`{"jobs":["job"],"companies":["company"],"hosts":["bad.example"]}`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		company string
		job     string
		url     string
		want    bool
	}{
		{"ok", "JOB", "https://good.example/apply", true},
		{"COMPANY", "ok", "https://good.example/apply", true},
		{"ok", "ok", "https://bad.example/apply", true},
		{"ok", "ok", "not a url", true},
		{"ok", "ok", "https://good.example/apply", false},
	}
	for _, test := range tests {
		if got := killed(test.company, test.job, test.url, killSwitch); got != test.want {
			t.Errorf("killed(%q, %q, %q) = %v, want %v", test.company, test.job, test.url, got, test.want)
		}
	}
}

func TestDumpRefreshKeepsLastGoodCopy(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "jobs.json")
	body := `{"generatedAt":"2026-11-25T00:00:00.000Z","count":1,"jobs":[{"id":"id","title":"Intern","company":{"name":"Example","slug":"example","ats":"lever"},"locations":[],"apply_url":"https://example.test/jobs/1","excerpt":null,"terms":[],"term_years":[],"duration_months":[],"cohort_year":null,"roles":["software"],"regions":[],"is_remote":true,"source":"lever","posted_at":null,"first_seen_at":"2026-11-25T00:00:00.000Z","last_seen_at":"2026-11-25T00:00:00.000Z"}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 25, 1, 0, 0, 0, time.UTC)
	store := NewStore(Options{
		DumpSource: path,
		Now:        func() time.Time { return now },
		IsPublic:   func(model.DumpJob, time.Time) bool { return true },
	})
	first, err := store.DumpStatus(context.Background())
	if err != nil || first.ActiveJobs != 1 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	if err := os.WriteFile(path, []byte(`{"jobs":`), 0o600); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	second, err := store.DumpStatus(context.Background())
	if err != nil || second != first {
		t.Fatalf("fallback = %+v, %v; want %+v", second, err, first)
	}
}

func TestFamiliesGroupAndDiversify(t *testing.T) {
	jobs := []model.DumpJob{
		familyTestJob("a1", "a", "family-a1", "Role Long", "2026-11-25T03:00:00.000Z"),
		familyTestJob("a2", "a", "family-a2", "Other", "2026-11-25T02:00:00.000Z"),
		familyTestJob("b1", "b", "family-b1", "Third", "2026-11-25T01:00:00.000Z"),
		familyTestJob("a3", "a", "family-a1", "Role", "2026-11-25T00:00:00.000Z"),
	}
	families := orderFamilies(familiesFromJobs(filterJobs(jobs, FamilyQuery{Sort: SortFirstSeen}), FamilyQuery{Sort: SortFirstSeen}), SortFirstSeen)
	if got := []string{families[0].roleFamilyID, families[1].roleFamilyID, families[2].roleFamilyID}; !reflect.DeepEqual(got, []string{"family-a1", "family-b1", "family-a2"}) {
		t.Fatalf("order = %v", got)
	}
	if len(families[0].postings) != 2 || displayTitle(families[0].titles, "") != "Role" {
		t.Fatalf("grouped family = %+v", families[0])
	}
}

func familyTestJob(id, company, family, title, seen string) model.DumpJob {
	return model.DumpJob{
		ID: id, Title: title, Company: model.DumpCompanyRef{Name: company, Slug: company},
		ApplyURL: "https://example.test/" + id, Roles: []string{"software"},
		FirstSeenAt: seen, LastSeenAt: seen, RoleFamilyID: family,
	}
}

func TestPostingLabels(t *testing.T) {
	campus := ExtractCampusFromTitle("Software Intern — IIT Bombay")
	if campus == nil || *campus != "IIT Bombay" {
		t.Fatalf("campus = %v", campus)
	}
	postings := []FamilyPosting{
		{ID: "1", Title: "Intern - IIT Bombay", Location: "India", Locations: []string{"India"}, ApplyURL: "https://example/1"},
		{ID: "2", Title: "Intern - IIT Delhi", Location: "India", Locations: []string{"India"}, ApplyURL: "https://example/2"},
	}
	rows := FamilyApplyRows("Intern", postings)
	if rows[0].Label != "IIT Bombay" || rows[1].AppliedTitle != "Intern (IIT Delhi)" {
		t.Fatalf("rows = %+v", rows)
	}
	if FamilyMetaLabel("Intern", postings) != "2 openings" || FamilyPickerButtonLabel(postings) != "Show openings" {
		t.Fatalf("labels = %q, %q", FamilyMetaLabel("Intern", postings), FamilyPickerButtonLabel(postings))
	}
}

func TestReportIssueURL(t *testing.T) {
	got := ReportIssueURL("id", "Software Intern", "Example", "https://example.test/apply?a=1", "https://openintern.dev/jobs/example/job")
	want := "https://github.com/dnexdev/openintern/issues/new?template=listing.yml&title=Listing%3A+Example+%E2%80%94+Software+Intern&url=https%3A%2F%2Fopenintern.dev%2Fjobs%2Fexample%2Fjob+%28job+id%29&details=Apply+URL%3A+https%3A%2F%2Fexample.test%2Fapply%3Fa%3D1%0A%0A"
	if got != want {
		t.Fatalf("ReportIssueURL = %q\nwant %q", got, want)
	}
}
