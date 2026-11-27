package registry

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dnexdev/openintern/data"
)

func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	for _, name := range []string{ApplyHostsPath, CompanyAliasesPath, JobAliasesPath} {
		if _, ok := fsys[name]; !ok {
			fsys[name] = &fstest.MapFile{Data: []byte("")}
		}
	}
	return fsys
}

func containsSub(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// validate-companies.test.ts
func TestValidateCompanies(t *testing.T) {
	fsys := mapFS(map[string]string{
		"companies/a.yaml": `companies:
  - name: Alpha
    slug: alpha
    ats: greenhouse
    board_token: shared
`,
		"companies/b.yaml": `companies:
  - name: Alpha duplicate
    slug: alpha
    ats: lever
    board_token: alpha
  - name: Beta
    slug: beta
    ats: greenhouse
    board_token: shared
    website_url: https://beta.example
`,
	})
	res, err := ValidateCompanies(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSub(res.Errors, `duplicate slug "alpha"`) {
		t.Errorf("duplicate slug: %v", res.Errors)
	}
	if !containsSub(res.Errors, `duplicate ATS token "greenhouse:shared"`) {
		t.Errorf("duplicate ATS token error: %v", res.Errors)
	}
	if !containsSub(res.Warnings, "missing website_url for alpha") {
		t.Errorf("missing website warning: %v", res.Warnings)
	}
	if want := `duplicate slug "alpha" in a.yaml and b.yaml`; res.Errors[0] != want {
		t.Errorf("got %q, want %q", res.Errors[0], want)
	}
	if want := `duplicate ATS token "greenhouse:shared" for alpha (a.yaml) and beta (b.yaml)`; res.Errors[1] != want {
		t.Errorf("got %q, want %q", res.Errors[1], want)
	}
}

// validate-curated.test.ts
func TestValidateTier1(t *testing.T) {
	company := `companies:
  - name: OpenAI
    slug: openai
    ats: greenhouse
    board_token: openai
`
	ok, err := ValidateTier1(mapFS(map[string]string{"companies/a.yaml": company, Tier1Path: "slugs:\n  - openai\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(ok.Errors) != 0 {
		t.Errorf("valid slug passes: %v", ok.Errors)
	}
	bad, err := ValidateTier1(mapFS(map[string]string{"companies/a.yaml": company, Tier1Path: "slugs:\n  - not-a-real-slug\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if !containsSub(bad.Errors, "not-a-real-slug") {
		t.Errorf("unknown slug fails: %v", bad.Errors)
	}
	dup, _ := ValidateTier1(mapFS(map[string]string{
		"companies/a.yaml": company + "    active: false\n",
		Tier1Path:          "slugs:\n  - openai\n  - openai\n",
	}))
	if len(dup.Errors) != 1 || dup.Errors[0] != "duplicate slug: openai" || len(dup.Warnings) != 1 ||
		dup.Warnings[0] != "openai is active: false — no jobs will be highlighted" {
		t.Errorf("got %+v", dup)
	}
}

// The zod messages of schema.ts, located by file and line.
func TestSchemaMessages(t *testing.T) {
	ats := `Invalid option: expected one of "greenhouse"|"lever"|"ashby"|"workable"|"smartrecruiters"|"recruitee"|"rippling"|"bamboohr"|"workday"|"citadel"|"citadel_securities"|"tesla"|"bytedance"|"tiktok"|"phenom"|"avature"|"icims"|"eightfold"|"google"|"ibm"|"deshaw"|"shopify"|"x"`
	cases := []struct{ name, src, want string }{
		{"missing list", "other: 1\n", "f.yaml:1: companies: Invalid input: expected array, received undefined"},
		{"empty file", "# nothing\n", "f.yaml:1: (root): Invalid input: expected object, received null"},
		{"not a list", "companies: x\n", "f.yaml:1: companies: Invalid input: expected array, received string"},
		{"empty list", "companies:\n", "f.yaml:1: companies: Invalid input: expected array, received null"},
		{"bad slug", "companies:\n  - name: A\n    slug: A_B\n    ats: greenhouse\n    board_token: a\n", "f.yaml:3: companies[0].slug: slug must be lowercase alphanumeric with hyphens"},
		{"empty everything", "companies:\n  - name: ''\n    slug: ''\n    ats: ''\n    board_token: ''\n",
			"f.yaml:2: companies[0].name: Too small: expected string to have >=1 characters\n" +
				"f.yaml:3: companies[0].slug: Too small: expected string to have >=1 characters\n" +
				"f.yaml:3: companies[0].slug: slug must be lowercase alphanumeric with hyphens\n" +
				"f.yaml:4: companies[0].ats: " + ats + "\n" +
				"f.yaml:5: companies[0].board_token: Too small: expected string to have >=1 characters"},
		{"numeric token", "companies:\n  - name: A\n    slug: a\n    ats: greenhouse\n    board_token: 123\n", "f.yaml:5: companies[0].board_token: Invalid input: expected string, received number"},
		{"missing name", "companies:\n  - slug: a\n    ats: greenhouse\n    board_token: a\n", "f.yaml:2: companies[0].name: Invalid input: expected string, received undefined"},
		{"bad url", "companies:\n  - name: A\n    slug: a\n    ats: greenhouse\n    board_token: a\n    careers_url: stripe.com/jobs\n", "f.yaml:6: companies[0].careers_url: Invalid URL"},
		{"null url", "companies:\n  - name: A\n    slug: a\n    ats: greenhouse\n    board_token: a\n    careers_url:\n", "f.yaml:6: companies[0].careers_url: Invalid input: expected string, received null"},
		{"bad active", "companies:\n  - name: A\n    slug: a\n    ats: greenhouse\n    board_token: a\n    active: yes\n", "f.yaml:6: companies[0].active: Invalid input: expected boolean, received string"},
		{"bad extras", "companies:\n  - name: A\n    slug: a\n    ats: greenhouse\n    board_token: a\n    extra_board_tokens:\n      - ''\n      - 3\n",
			"f.yaml:7: companies[0].extra_board_tokens[0]: Too small: expected string to have >=1 characters\n" +
				"f.yaml:8: companies[0].extra_board_tokens[1]: Invalid input: expected string, received number"},
		{"item not a map", "companies:\n  - x\n", "f.yaml:2: companies[0]: Invalid input: expected object, received string"},
		{"yaml error", "companies: [a]\n", "f.yaml:1: flow collections"},
	}
	for _, c := range cases {
		_, _, err := ParseCompaniesFile("f.yaml", []byte(c.src))
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("%s:\n got %v\nwant %s", c.name, err, c.want)
		}
	}
	var se *SchemaError
	_, _, err := ParseCompaniesFile("f.yaml", []byte("companies: x\n"))
	if !errors.As(err, &se) || len(se.Issues) != 1 {
		t.Errorf("want *SchemaError, got %T", err)
	}

	list, unknown, err := ParseCompaniesFile("f.yaml", []byte("companies:\n  - name: A\n    slug: a\n    ats: x\n    board_token: \"1\"\n    website_url: '  https://a.com/\\tx  '\n    webiste_url: https://a.com\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c := list[0]; c.ATS != "x" || c.BoardToken != "1" || !c.Active || c.WebsiteURL == nil || *c.WebsiteURL != `https://a.com/\tx` || c.CareersURL != nil || c.ExtraBoardTokens != nil {
		t.Errorf("got %+v", c)
	}
	if len(unknown) != 1 || unknown[0] != `f.yaml:7: unknown key "webiste_url" (ignored)` {
		t.Errorf("unknown keys: %v", unknown)
	}

	cases1 := []struct{ src, want string }{
		{"slugs:\n", "f.yaml:1: slugs: Invalid input: expected array, received null"},
		{"slugs:\n  - ''\n  - A\n", "f.yaml:2: slugs[0]: Too small: expected string to have >=1 characters\nf.yaml:2: slugs[0]: slug must be lowercase alphanumeric with hyphens\nf.yaml:3: slugs[1]: slug must be lowercase alphanumeric with hyphens"},
		{"slugs:\n  - 1\n", "f.yaml:2: slugs[0]: Invalid input: expected string, received number"},
	}
	for _, c := range cases1 {
		if _, err := ParseTier1("f.yaml", []byte(c.src)); err == nil || err.Error() != c.want {
			t.Errorf("%q:\n got %v\nwant %s", c.src, err, c.want)
		}
	}
}

// apply-hosts.test.ts
func TestApplyHosts(t *testing.T) {
	table := Table{"acme": {{"job-boards.greenhouse.io", "/acme/"}, {"acme.wd5.myworkdayjobs.com", "/"}}}
	reason := func(slug, url string) string {
		c := CheckApplyURL(slug, url, table)
		if c.OK {
			return "ok"
		}
		return c.Reason
	}
	cases := []struct{ slug, url, want, msg string }{
		{"acme", "https://job-boards.greenhouse.io/acme/jobs/1", "ok", ""},
		{"acme", "https://job-boards.greenhouse.io/ACME/jobs/1", "ok", "prefix ignores case"},
		{"acme", "https://job-boards.greenhouse.io/acme", "ok", "bare board path"},
		{"acme", "https://job-boards.greenhouse.io/acmescam/jobs/1", "off-list", "whole segments only"},
		{"acme", "https://job-boards.greenhouse.io/evil/jobs/1", "off-list", "another tenant on a shared host"},
		{"acme", "http://job-boards.greenhouse.io/acme/jobs/1", "not-https", ""},
		{"acme", "javascript:alert(1)", "not-https", ""},
		{"acme", "https://user:pw@job-boards.greenhouse.io/acme/", "credentials-or-port", ""},
		{"acme", "https://job-boards.greenhouse.io:8443/acme/", "credentials-or-port", ""},
		{"acme", "https://acme.wd5.myworkdayjobs.com.evil.com/", "off-list", "suffix tricks"},
		{"acme", "https://acme.wd5.myworkdayjobs.com/Careers/job/x", "ok", ""},
		{"nobody", "https://job-boards.greenhouse.io/acme/", "unknown-company", ""},
		{"acme", "not a url", "malformed", ""},
		// Beyond the TypeScript test: what a browser normalizes away.
		{"acme", "https://job-boards.greenhouse.io/acme/../evil/jobs/1", "off-list", "dot segments"},
		{"acme", "https://job-boards.greenhouse.io/acme/%2e%2e/evil/", "off-list", "encoded dot segments"},
		{"acme", "https://job-boards.greenhouse.io:443/acme/", "ok", "default port is no port"},
		{"acme", "https://JOB-BOARDS.greenhouse.io/acme/", "ok", "host case"},
		{"acme", "https:\\\\job-boards.greenhouse.io\\acme\\jobs", "ok", "backslashes"},
	}
	for _, c := range cases {
		if got := reason(c.slug, c.url); got != c.want {
			t.Errorf("%s %s: got %s, want %s (%s)", c.slug, c.url, got, c.want, c.msg)
		}
	}
	if c := CheckApplyURL("acme", "not a url", table); c.Host != nil {
		t.Errorf("malformed has no host")
	}
	if c := CheckApplyURL("acme", "https://evil.example/x", table); c.Host == nil || *c.Host != "evil.example" {
		t.Errorf("off-list host: %+v", c)
	}

	// warn mode never hides; enforce does.
	r := &Registry{applyHosts: table}
	if !r.ApplyURLAllowed("nobody", "https://evil.example/", ApplyHostsWarn) {
		t.Error("warn hides")
	}
	if r.ApplyURLAllowed("nobody", "https://evil.example/", ApplyHostsEnforce) {
		t.Error("enforce shows")
	}
	if !r.ApplyURLAllowed("nobody", "https://evil.example/", ApplyHostsOff) {
		t.Error("off hides")
	}

	eq := func(got, want []HostPrefix, msg string) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", msg, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", msg, got, want)
			}
		}
	}
	eq(DerivedHosts("lever", "Acme"), []HostPrefix{{"jobs.lever.co", "/acme/"}, {"jobs.eu.lever.co", "/acme/"}}, "lever")
	eq(DerivedHosts("workday", "nvidia|wd5|NVIDIAExternalCareerSite"), []HostPrefix{{"nvidia.wd5.myworkdayjobs.com", "/"}}, "workday")
	eq(DerivedHosts("recruitee", "evil.com/x"), nil, "no hosts from a malformed token")
	eq(DerivedHosts("phenom", "amd"), nil, "no safe pattern: needs an explicit row")
	if _, err := ParseApplyHostsTSV("acme\tEVIL.com\t/\n"); err == nil || !regexp.MustCompile(`bad host`).MatchString(err.Error()) {
		t.Errorf("bad host: %v", err)
	}
	if _, err := ParseApplyHostsTSV("acme\tx.com\tnoslash\n"); err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Errorf("prefix: %v", err)
	}
	if _, err := ParseApplyHostsTSV("acme\tx.com\n"); err == nil || err.Error() != "data/apply-hosts.tsv: line 1: needs company, host, path_prefix" {
		t.Errorf("short row: %v", err)
	}

	// The compiled table accepts the live corpus it was seeded from.
	d := Default()
	if !d.CheckApplyURL("khan-academy", "https://job-boards.greenhouse.io/khanacademy/jobs/8250259").OK {
		t.Error("khan-academy")
	}
	if !d.CheckApplyURL("lyft", "https://app.careerpuck.com/job-board/lyft/job/8797819002").OK {
		t.Error("lyft")
	}
	if d.CheckApplyURL("lyft", "https://app.careerpuck.com/job-board/evil/job/1").OK {
		t.Error("lyft evil")
	}
	if len(d.ApplyHosts()) <= 500 {
		t.Errorf("table has %d companies", len(d.ApplyHosts()))
	}
}

func TestApplyHostModeFromEnv(t *testing.T) {
	for env, want := range map[string]ApplyHostMode{"": ApplyHostsWarn, "off": ApplyHostsOff, " enforce ": ApplyHostsEnforce, "ENFORCE": ApplyHostsWarn, "x": ApplyHostsWarn} {
		t.Setenv("OPENINTERN_APPLY_HOSTS", env)
		if got := ApplyHostModeFromEnv(); got != want {
			t.Errorf("%q: got %s, want %s", env, got, want)
		}
	}
}

// The embedded registry is what CI validated: it loads, and its derived
// tables equal the TypeScript generated modules (apply-hosts.generated.json
// and aliases.generated.ts), which `gen-* --check` kept in sync with data/.
func TestAliases(t *testing.T) {
	slugs := map[string]bool{"new": true, "kept": true}
	if _, err := ParseCompanyAliases("old_slug\tnew_slug\nkept\tnew\nold\tgone\n", slugs); err == nil ||
		err.Error() != "company alias kept is still a registry slug; company alias target gone is not in the registry" {
		t.Errorf("got %v", err)
	}
	if _, err := ParseJobAliases("old_job_id\tnew_job_id\nnot-a-uuid\n"); err == nil || err.Error() != "bad job alias not-a-uuid → undefined" {
		t.Errorf("got %v", err)
	}
	a := "aaaaaaaa-0000-4000-8000-000000000001"
	b := "aaaaaaaa-0000-4000-8000-000000000002"
	c := "bbbbbbbb-0000-4000-8000-000000000003"
	jobs, err := ParseJobAliases("# x\n" + b + "\t" + c + "\n" + a + "\t" + c + "\n")
	if err != nil {
		t.Fatal(err)
	}
	r := &Registry{jobAliasMap: jobs}
	for o, n := range jobs {
		r.jobAliases = append(r.jobAliases, [2]string{o, n})
	}
	r.jobAliases = sortedAliases(r.jobAliases)
	if r.JobAlias(strings.ToUpper(a)) != c || r.JobAlias(c) != "" {
		t.Error("JobAlias")
	}
	if r.JobAliasByPrefix("AAAAAAAA") != c || r.JobAliasByPrefix("cccccccc") != "" {
		t.Error("JobAliasByPrefix")
	}
	if !IsDelisted("CBB76A8D-0726-4A2A-B84D-8F359BE23A90") || IsDelisted(a) || len(DelistedJobs()) != 1 {
		t.Error("IsDelisted")
	}
}

func TestEmbeddedValid(t *testing.T) {
	res, err := ValidateCompanies(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		t.Errorf("validate-companies: %v", res.Errors)
	}
	cur, err := ValidateTier1(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(cur.Errors) > 0 {
		t.Errorf("validate-curated: %v", cur.Errors)
	}
	d := Default()
	if len(d.Unique()) == 0 || len(d.Unique()) > len(d.Companies) || !d.IsTier1("openai") {
		t.Errorf("unique %d of %d", len(d.Unique()), len(d.Companies))
	}
}
