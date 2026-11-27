package ats

// ByteDance / TikTok careers portals (atsx supplier API).
//
// Public JSON (no CSRF when hitting the supplier host):
//
//	POST {apiBase}/search/job/posts
//
// board_token is the `website-path` header value: `en` for ByteDance
// (joinbytedance.com overseas), `tiktok` for TikTok (lifeattiktok.com).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// Portal describes one supplier portal.
type Portal struct {
	Name               string // ATS name, for errors
	APIBase            string
	Origin             string
	DefaultWebsitePath string // used when board_token is empty
	ApplyBase          string
}

var (
	BytedancePortal = Portal{
		Name:               "bytedance",
		APIBase:            "https://jobs.bytedance.com/api/v1/public/supplier",
		Origin:             "https://joinbytedance.com",
		DefaultWebsitePath: "en",
		ApplyBase:          "https://joinbytedance.com/search/",
	}
	TiktokPortal = Portal{
		Name:               "tiktok",
		APIBase:            "https://api.lifeattiktok.com/api/v1/public/supplier",
		Origin:             "https://lifeattiktok.com",
		DefaultWebsitePath: "tiktok",
		ApplyBase:          "https://lifeattiktok.com/search/",
	}
)

func bytedanceDisplayName(n any) any {
	if !truthy(n) {
		return nil
	}
	if v := or(get(n, "en_name"), get(n, "i18n_name"), get(n, "name")); truthy(v) {
		return v
	}
	return nil
}

func bytedanceCityLabel(city any) string {
	if !truthy(city) {
		return ""
	}
	var parts []string
	for cur := city; truthy(cur); cur = get(cur, "parent") {
		if name := bytedanceDisplayName(cur); name != nil {
			parts = append(parts, jsString(name))
		}
	}
	// city, region, country — keep city + country when deep
	if len(parts) >= 3 {
		return parts[0] + ", " + parts[len(parts)-1]
	}
	return strings.Join(parts, ", ")
}

// MapBytedancePosts turns search results into jobs.
func MapBytedancePosts(posts []any, portal Portal) []model.NormalizedJob {
	out := make([]model.NormalizedJob, 0, len(posts))
	for _, p := range posts {
		id := jsString(coalesce(get(p, "id"), get(p, "code"), ""))
		description := filterJoin("\n\n", get(p, "description"), get(p, "requirement"))
		var locations []string
		if loc := bytedanceCityLabel(get(p, "city_info")); loc != "" {
			locations = []string{loc}
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  id,
			Title:       textOr(get(p, "title"), "Untitled"),
			Locations:   locations,
			ApplyURL:    portal.ApplyBase + encodeURIComponent(id),
			Excerpt:     excerpt(description),
			Description: description,
		})
	}
	return out
}

func bytedanceSearchBody(limit, offset int) jsjson.Object {
	return jsObject(
		"job_category_id_list", []any{},
		"keyword", "",
		"limit", limit,
		"offset", offset,
		"location_code_list", []any{},
		// Campus facet — Experienced=1, Campus=2
		"recruitment_id_list", []any{"2"},
		"subject_id_list", []any{},
		"portal_type", 2,
		"portal_entrance", 1,
	)
}

// ShadowResult is one honest request made beside the legacy one
// (OPENINTERN_SHADOW_HONEST=1), reported in the ingest summary.
type ShadowResult struct {
	Source      string   `json:"source"`
	OK          bool     `json:"ok"`
	Status      *int     `json:"status"`
	Count       *float64 `json:"count"`
	LegacyCount int      `json:"legacyCount"`
	Error       string   `json:"error,omitempty"`
}

var shadow struct {
	sync.Mutex
	results []ShadowResult
}

// ShadowResults returns the shadow-test results recorded by this process.
func ShadowResults() []ShadowResult {
	shadow.Lock()
	defer shadow.Unlock()
	return append([]ShadowResult{}, shadow.results...)
}

// shadowHonestProbe sends one honest request (no origin/Referer) beside the
// legacy fetch and logs the outcome; it never changes what is ingested.
func shadowHonestProbe(ctx context.Context, portal Portal, websitePath string, legacyCount int) {
	res := ShadowResult{Source: portal.Name, LegacyCount: legacyCount}
	body, err := fetch.JSONBody(ctx, portal.APIBase+"/search/job/posts", fetch.Options{
		Method:   "POST",
		Body:     string(jsjson.Marshal(bytedanceSearchBody(50, 0))),
		Headers:  map[string]string{"website-path": websitePath, "accept-language": "en"},
		Attempts: 1,
	})
	var page any
	if err == nil {
		page, err = parseJSONBody(body)
		if err != nil {
			err = fetch.InvalidJSON(portal.APIBase+"/search/job/posts", err)
		}
	}
	if err != nil {
		var f *fetch.Failure
		if errors.As(err, &f) && f.Status != 0 {
			s := f.Status
			res.Status = &s
		}
		res.Error = err.Error()
	} else {
		ok := 200
		res.Status = &ok
		code := get(page, "code")
		res.OK = nullish(code) || code == float64(0)
		var count float64
		if n, isNum := get(page, "data", "count").(float64); isNum {
			count = n
		} else if list, isArr := get(page, "data", "job_post_list").([]any); isArr {
			count = float64(len(list))
		}
		res.Count = &count
		if !res.OK {
			res.Error = jsTrim(fmt.Sprintf("code=%s %s", jsString(code), textOr(get(page, "message"), "")))
		}
	}
	shadow.Lock()
	shadow.results = append(shadow.results, res)
	shadow.Unlock()
	line, _ := json.Marshal(res)
	fmt.Fprintf(os.Stdout, "[shadow] honest %s: %s\n", portal.Name, line)
}

// fetchBytedancePortal reads campus postings (recruitment_id=2), paginated.
// The internship classifier narrows titles on ingest.
//
// PENDING OWNER DECISION (plan: "ByteDance/TikTok: test first"). This is the
// only request in ingest that still sends a portal's own origin/Referer
// (fetch.LegacyPortalPostJSON). It stays until the shadow test shows whether
// honest requests work; then it is removed (honest) or the source moves to a
// manual capture.
func fetchBytedancePortal(ctx context.Context, portal Portal, token string) ([]model.NormalizedJob, error) {
	websitePath := token
	if websitePath == "" {
		websitePath = portal.DefaultWebsitePath
	}
	websitePath = jsTrim(websitePath)
	headers := map[string]string{"website-path": websitePath, "accept-language": "en"}
	url := portal.APIBase + "/search/job/posts"

	const limit = 50
	var all []any
	total := -1
	for offset := 0; (total < 0 || offset < total) && offset < 5000; offset += limit {
		body, err := fetch.LegacyPortalPostJSON(ctx, url, portal.Origin, string(jsjson.Marshal(bytedanceSearchBody(limit, offset))), headers)
		if err != nil {
			return nil, err
		}
		page, err := parseJSONBody(body)
		if err != nil {
			return nil, fetch.InvalidJSON(url, err)
		}
		if code := get(page, "code"); !nullish(code) && code != float64(0) {
			return nil, errors.New(jsTrim(fmt.Sprintf("%s search failed: code=%s %s", portal.Name, jsString(code), textOr(get(page, "message"), ""))))
		}
		batch := array(get(page, "data", "job_post_list"))
		if n, ok := get(page, "data", "count").(float64); ok {
			total = jsTotal(n)
		}
		all = append(all, batch...)
		if len(batch) == 0 || len(batch) < limit {
			break
		}
	}

	if os.Getenv("OPENINTERN_SHADOW_HONEST") == "1" {
		shadowHonestProbe(ctx, portal, websitePath, len(all))
	}
	return MapBytedancePosts(all, portal), nil
}
