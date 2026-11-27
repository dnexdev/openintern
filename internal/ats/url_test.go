package ats

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// testdata/url_node.json is the TypeScript output (Node 26) of
// parseAtsFromApplyUrl and parseWorkdayUrl on every careers/website URL in
// data/companies and every apply URL in the cached dump, the URL builders on
// odd tokens, probeUrl for every ATS, and extractAtsBoardsFromMarkdown on a
// mixed table (/tmp/openintern-sources/url-vectors.mts).
type urlVectors struct {
	Parsed   [][3]json.RawMessage `json:"parsed"`
	Builders []map[string]json.RawMessage
	ATSList  []string `json:"atsList"`
	Extract  struct {
		Hits                  []map[string]string `json:"hits"`
		ApplyLinkCount        int                 `json:"applyLinkCount"`
		UnsupportedApplyCount int                 `json:"unsupportedApplyCount"`
		UnsupportedHosts      [][2]any            `json:"unsupportedHosts"`
	} `json:"extracted"`
}

func loadURLVectors(t *testing.T) urlVectors {
	t.Helper()
	raw, err := os.ReadFile("testdata/url_node.json")
	if err != nil {
		t.Fatal(err)
	}
	var v urlVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestParseATSFromApplyURLMatchesTypeScript(t *testing.T) {
	v := loadURLVectors(t)
	for _, row := range v.Parsed {
		var in string
		_ = json.Unmarshal(row[0], &in)
		var want *struct{ ATS, BoardToken, CareersURL string }
		var raw map[string]string
		_ = json.Unmarshal(row[1], &raw)
		if raw != nil {
			want = &struct{ ATS, BoardToken, CareersURL string }{raw["ats"], raw["boardToken"], raw["careersUrl"]}
		}
		got, ok := ParseATSFromApplyURL(in)
		if (want != nil) != ok || (ok && (got.ATS != want.ATS || got.BoardToken != want.BoardToken || got.CareersURL != want.CareersURL)) {
			t.Errorf("ParseATSFromApplyURL(%q) = %+v,%v want %+v", in, got, ok, want)
		}
		var wd map[string]string
		_ = json.Unmarshal(row[2], &wd)
		b, ok := ParseWorkdayURL(in)
		if (wd != nil) != ok || (ok && (b.Tenant != wd["tenant"] || b.Datacenter != wd["datacenter"] || b.Site != wd["site"])) {
			t.Errorf("ParseWorkdayURL(%q) = %+v,%v want %v", in, b, ok, wd)
		}
	}
}

func stringOrError(s string, err error) any {
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return s
}

func TestURLBuildersMatchTypeScript(t *testing.T) {
	v := loadURLVectors(t)
	check := func(token, name string, got any, raw json.RawMessage) {
		t.Helper()
		var want any
		_ = json.Unmarshal(raw, &want)
		if wm, ok := want.(map[string]any); ok {
			if _, isErr := wm["error"]; isErr {
				if gm, ok := got.(map[string]any); ok && gm["error"] != nil {
					return // both threw; messages differ by design (Go wording)
				}
			}
		}
		gotJSON, _ := json.Marshal(got)
		var g any
		_ = json.Unmarshal(gotJSON, &g)
		if !reflect.DeepEqual(g, want) {
			t.Errorf("%s(%q) = %s, want %s", name, token, gotJSON, raw)
		}
	}
	for _, b := range v.Builders {
		var token string
		_ = json.Unmarshal(b["token"], &token)
		check(token, "avatureSearchUrl", stringOrError(AvatureSearchURL(token, "intern")), b["avature"])
		check(token, "avatureSearchUrl empty", stringOrError(AvatureSearchURL(token, "")), b["avatureEmpty"])
		check(token, "avatureSearchUrl SI", stringOrError(AvatureSearchURL(token, "Software Intern")), b["avatureSI"])
		check(token, "icimsSearchUrl", stringOrError(IcimsSearchURL(token, "intern")), b["icims"])
		check(token, "icimsHost", IcimsHost(token), b["icimsHost"])
		check(token, "phenomJobsApiUrl", stringOrError(PhenomJobsAPIURL(token, 2)), b["phenom"])
		check(token, "phenomCareersOrigin", PhenomCareersOrigin(token), b["phenomOrigin"])
		check(token, "eightfoldSearchUrl", EightfoldSearchURL(ParseEightfoldToken(token), 20), b["eightfold"])
		check(token, "eightfold applyOrigin", ParseEightfoldToken(token).ApplyOrigin, b["eightfoldOrigin"])
		check(token, "googleCareersSearchUrl", GoogleCareersSearchURL(token), b["google"])
		check(token, "citadelListingUrl", CitadelListingURL("citadel_securities", token, 3), b["citadel"])
		check(token, "citadelListingUrl 1", CitadelListingURL("citadel", token, 1), b["citadel1"])
		var wd any
		if board, ok := ParseWorkdayToken(token); ok {
			wd = map[string]string{"tenant": board.Tenant, "datacenter": board.Datacenter, "site": board.Site}
		}
		check(token, "parseWorkdayToken", wd, b["workday"])
		var probes []json.RawMessage
		_ = json.Unmarshal(b["probe"], &probes)
		for i, ats := range v.ATSList {
			check(token, "probeUrl "+ats, stringOrError(ProbeURL(ats, token)), probes[i])
		}
	}
}

func TestExtractATSBoardsMatchesTypeScript(t *testing.T) {
	v := loadURLVectors(t)
	md, err := os.ReadFile("testdata/extract.md")
	if err != nil {
		t.Fatal(err)
	}
	got := ExtractATSBoardsFromMarkdown(string(md))
	if got.ApplyLinkCount != v.Extract.ApplyLinkCount || got.UnsupportedApplyCount != v.Extract.UnsupportedApplyCount {
		t.Fatalf("counts %d/%d, want %d/%d", got.ApplyLinkCount, got.UnsupportedApplyCount, v.Extract.ApplyLinkCount, v.Extract.UnsupportedApplyCount)
	}
	if len(got.Hits) != len(v.Extract.Hits) {
		t.Fatalf("%d hits, want %d", len(got.Hits), len(v.Extract.Hits))
	}
	for i, h := range got.Hits {
		w := v.Extract.Hits[i]
		if h.CompanyName != w["companyName"] || h.ApplyURL != w["applyUrl"] || h.ATS != w["ats"] || h.BoardToken != w["boardToken"] || h.CareersURL != w["careersUrl"] {
			t.Errorf("hit %d = %+v, want %v", i, h, w)
		}
	}
	if len(got.UnsupportedHosts) != len(v.Extract.UnsupportedHosts) {
		t.Fatalf("hosts %+v, want %v", got.UnsupportedHosts, v.Extract.UnsupportedHosts)
	}
	for i, h := range got.UnsupportedHosts {
		w := v.Extract.UnsupportedHosts[i]
		if h.Host != w[0].(string) || float64(h.Count) != w[1].(float64) {
			t.Errorf("host %d = %+v, want %v", i, h, w)
		}
	}
}

// The gap-report.test.ts cases for parse-ats-url.
func TestParseATSFromApplyURLCases(t *testing.T) {
	cases := []struct {
		in   string
		want *ParsedBoard
	}{
		{"https://jobs.ashbyhq.com/bloxd/7ade559a/application", &ParsedBoard{"ashby", "bloxd", "https://jobs.ashbyhq.com/bloxd"}},
		{"https://job-boards.greenhouse.io/later/jobs/8621762002", &ParsedBoard{"greenhouse", "later", "https://job-boards.greenhouse.io/later"}},
		{"https://boards.greenhouse.io/embed/job_app?token=7975026&utm_source=Simplify", nil},
		{"https://selinc.wd1.myworkdayjobs.com/SEL/job/Idaho---Boise/Software-Engineer-Intern", &ParsedBoard{"workday", "selinc|wd1|SEL", "https://selinc.wd1.myworkdayjobs.com/SEL"}},
	}
	for _, c := range cases {
		got, ok := ParseATSFromApplyURL(c.in)
		if (c.want != nil) != ok || (ok && got != *c.want) {
			t.Errorf("%s: %+v %v", c.in, got, ok)
		}
	}
	if TokenKey("greenhouse", "Later") != "greenhouse:later" || TokenKey("ashby", "Bloxd") != "ashby:bloxd" || TokenKey("workday", "selinc|wd1|SEL") != "workday:selinc|wd1|SEL" {
		t.Fatal("TokenKey")
	}
}
