package ats

import (
	"context"
	"errors"
	"fmt"

	"github.com/dnexdev/openintern/internal/fetch"
)

// ProbeURL is the URL a probe (or a person) checks for a board.
func ProbeURL(ats, token string) (string, error) {
	enc := encodeURIComponent(token)
	switch ats {
	case "greenhouse":
		return "https://boards-api.greenhouse.io/v1/boards/" + enc + "/jobs", nil
	case "lever":
		return "https://api.lever.co/v0/postings/" + enc + "?mode=json", nil
	case "ashby":
		return "https://api.ashbyhq.com/posting-api/job-board/" + enc, nil
	case "workable":
		return "https://www.workable.com/api/accounts/" + enc, nil
	case "smartrecruiters":
		return "https://api.smartrecruiters.com/v1/companies/" + enc + "/postings?limit=1", nil
	case "recruitee":
		return "https://" + enc + ".recruitee.com/api/offers/", nil
	case "rippling":
		return "https://api.rippling.com/platform/api/ats/v1/board/" + enc + "/jobs", nil
	case "bamboohr":
		return "https://" + enc + ".bamboohr.com/careers/list", nil
	case "workday":
		b, ok := ParseWorkdayToken(token)
		if !ok {
			return "", fmt.Errorf("Invalid workday board_token (want tenant|wdN|site): %s", token)
		}
		return WorkdayJobsURL(b), nil
	case "citadel", "citadel_securities":
		return CitadelListingURL(ats, token, 1), nil
	case "tesla":
		return TeslaStateURL, nil
	case "bytedance":
		return BytedancePortal.APIBase + "/search/job/posts", nil
	case "tiktok":
		return TiktokPortal.APIBase + "/search/job/posts", nil
	case "phenom":
		return PhenomJobsAPIURL(token, 1)
	case "eightfold":
		return EightfoldSearchURL(ParseEightfoldToken(token), 0), nil
	case "avature":
		return AvatureSearchURL(token, "intern")
	case "icims":
		return IcimsSearchURL(token, "intern")
	case "google":
		return GoogleCareersSearchURL(token), nil
	case "ibm":
		t := token
		if t == "" {
			t = "ibmglobal"
		}
		return AvatureSearchURL(t, "Software Intern")
	case "deshaw":
		return DeshawInternshipsURL, nil
	case "shopify":
		return ShopifyCareersURL, nil
	case "x":
		return "https://careers.x.com/en", nil
	}
	return "", fmt.Errorf("Unknown ATS: %s", ats)
}

// PublicATS are the boards whose tokens can be guessed (recover-tokens);
// Workday and proprietary tokens cannot.
var PublicATS = []string{"greenhouse", "lever", "ashby", "workable", "smartrecruiters", "recruitee", "rippling", "bamboohr"}

// AllATS is every ingestible source: PublicATS, workday, then Proprietary.
func AllATS() []string {
	out := append(append([]string{}, PublicATS...), "workday")
	return append(out, Proprietary...)
}

// CountJobsFromProbeBody counts jobs in a probe response (a jsjson value).
func CountJobsFromProbeBody(ats string, data any) float64 {
	if a, ok := data.([]any); ok {
		return float64(len(a))
	}
	if !truthy(data) || !isObject(data) {
		return 0
	}
	if ats == "workday" {
		if n, ok := get(data, "total").(float64); ok {
			return n
		}
		if a, ok := get(data, "jobPostings").([]any); ok {
			return float64(len(a))
		}
	}
	if ats == "tesla" {
		return float64(len(ParseTeslaCareersState(data)))
	}
	if ats == "bytedance" || ats == "tiktok" {
		if n, ok := get(data, "data", "count").(float64); ok {
			return n
		}
		if a, ok := get(data, "data", "job_post_list").([]any); ok {
			return float64(len(a))
		}
	}
	for _, k := range []string{"jobs", "content", "offers", "result"} {
		if a, ok := get(data, k).([]any); ok {
			return float64(len(a))
		}
	}
	if tf := get(data, "totalFound"); truthy(tf) {
		n, ok := toNumber(tf)
		if !ok {
			return nan()
		}
		return n
	}
	return 1
}

func nan() float64 { var z float64; return z / z }

func probeBytedancePortal(ctx context.Context, portal Portal, token string) ProbeResult {
	websitePath := token
	if websitePath == "" {
		websitePath = portal.DefaultWebsitePath
	}
	url := portal.APIBase + "/search/job/posts"
	data, err := getJSON(ctx, url, fetch.Options{
		Method:   "POST",
		Headers:  map[string]string{"website-path": jsTrim(websitePath), "accept-language": "en"},
		Body:     string(jsonOf(bytedanceSearchBody(1, 0))),
		Attempts: 1,
	})
	if err != nil {
		return ProbeResult{Status: failureStatus(err), Error: err.Error()}
	}
	count := probeCount(CountJobsFromProbeBody(portal.Name, data))
	r := ProbeResult{OK: count > 0, Status: 200, Count: &count}
	if count <= 0 {
		r.Error = "empty listing"
	}
	return r
}

// ProbeATSBoard checks that a board answers and counts its jobs, with one
// attempt and no retries. A 429 counts as reachable.
func ProbeATSBoard(ctx context.Context, ats, token string) ProbeResult {
	switch ats {
	case "citadel", "citadel_securities", "tesla":
		return ManualProbe(ats)
	case "bytedance":
		return probeBytedancePortal(ctx, BytedancePortal, token)
	case "tiktok":
		return probeBytedancePortal(ctx, TiktokPortal, token)
	case "phenom":
		return probePhenom(ctx, token)
	case "eightfold":
		return probeEightfold(ctx, token)
	case "avature":
		return probeFromJobs(fetchAvature(ctx, token))
	case "icims":
		return probeFromJobs(fetchIcims(ctx, token))
	case "google":
		return probeFromJobs(fetchGoogle(ctx, token))
	case "ibm":
		return probeFromJobs(fetchIBM(ctx, token))
	case "deshaw":
		return probeFromJobs(fetchDeshaw(ctx, "internships"))
	case "shopify":
		return probeFromJobs(fetchShopify(ctx, "careers"))
	}
	url, err := ProbeURL(ats, token)
	if err != nil {
		return ProbeResult{Error: err.Error()}
	}
	opts := fetch.Options{Attempts: 1, Headers: map[string]string{"Accept": "application/json"}}
	if ats == "workday" {
		opts.Method = "POST"
		opts.Body = string(jsonOf(jsObject("appliedFacets", jsObject(), "limit", 1, "offset", 0, "searchText", "")))
	}
	res, err := fetch.Request(ctx, url, opts)
	if err != nil {
		var f *fetch.Failure
		if errors.As(err, &f) {
			if f.Status == 429 {
				return ProbeResult{OK: true, Status: 429}
			}
			return ProbeResult{Status: f.Status, Error: f.Message}
		}
		return ProbeResult{Error: err.Error()}
	}
	data, err := parseJSONBody(res.Body)
	if err != nil {
		return ProbeResult{Error: err.Error()}
	}
	count := probeCount(CountJobsFromProbeBody(ats, data))
	// Empty SmartRecruiters boards return HTTP 200 with totalFound=0: treat as failure.
	if ats == "smartrecruiters" && count == 0 {
		return ProbeResult{Status: res.Status, Count: &count, Error: "empty SmartRecruiters board"}
	}
	return ProbeResult{OK: true, Status: res.Status, Count: &count}
}
