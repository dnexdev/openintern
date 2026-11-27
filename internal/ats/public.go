package ats

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/safedate"
)

// jsDate is `v ? new Date(v) : null` followed by the safeDate range check
// that plan.ts applies to every postedAt, as an ISO string.
func jsDate(v any) *string {
	if !truthy(v) {
		return nil
	}
	switch x := v.(type) {
	case string:
		return safedate.ISOPtr(safedate.ParseString(x))
	case float64:
		return safedate.ISOPtr(safedate.ParseMillis(x))
	case bool:
		// new Date(true) is 1 ms after the epoch.
		return safedate.ISOPtr(safedate.ParseMillis(1))
	}
	return nil
}

func strPtr(s string) *string { return &s }

// sliceExcerpt is `v?.slice(0, 400) ?? null` for a field that may be absent.
func sliceExcerpt(v any) *string {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return strPtr(slice16(s, 400))
}

func fetchGreenhouse(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	data, err := getJSON(ctx, "https://boards-api.greenhouse.io/v1/boards/"+encodeURIComponent(token)+"/jobs?content=true", fetch.Options{})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for _, j := range array(get(data, "jobs")) {
		var locations []string
		if name := get(j, "location", "name"); truthy(name) {
			locations = []string{text(name)}
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  jsString(get(j, "id")),
			Title:       text(get(j, "title")),
			Locations:   locations,
			ApplyURL:    text(get(j, "absolute_url")),
			PostedAt:    jsDate(get(j, "updated_at")),
			Description: textOr(get(j, "content"), ""),
		})
	}
	return out, nil
}

func fetchLever(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	data, err := getJSON(ctx, "https://api.lever.co/v0/postings/"+encodeURIComponent(token)+"?mode=json", fetch.Options{})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for _, j := range array(data) {
		var locations []string
		if loc := get(j, "categories", "location"); truthy(loc) {
			locations = []string{text(loc)}
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  text(get(j, "id")),
			Title:       text(get(j, "text")),
			Locations:   locations,
			ApplyURL:    text(get(j, "hostedUrl")),
			Excerpt:     sliceExcerpt(get(j, "descriptionPlain")),
			PostedAt:    jsDate(get(j, "createdAt")),
			Description: text(coalesce(get(j, "descriptionPlain"), get(j, "description"), "")),
		})
	}
	return out, nil
}

func fetchAshby(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	data, err := getJSON(ctx, "https://api.ashbyhq.com/posting-api/job-board/"+encodeURIComponent(token)+"?includeCompensation=true", fetch.Options{})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for _, j := range array(get(data, "jobs")) {
		var locations []string
		if loc := get(j, "location"); truthy(loc) {
			locations = append(locations, text(loc))
		}
		if truthy(get(j, "isRemote")) && !someRemote(locations) {
			locations = append(locations, "Remote")
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  text(get(j, "id")),
			Title:       text(get(j, "title")),
			Locations:   locations,
			ApplyURL:    text(get(j, "jobUrl")),
			Excerpt:     sliceExcerpt(get(j, "descriptionPlain")),
			PostedAt:    jsDate(get(j, "publishedAt")),
			Description: text(coalesce(get(j, "descriptionPlain"), get(j, "descriptionHtml"), "")),
		})
	}
	return out, nil
}

func workableLocation(loc any) string {
	if !truthy(loc) {
		return ""
	}
	return filterJoin(", ", get(loc, "city"), get(loc, "region"), get(loc, "country"))
}

func fetchWorkable(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	data, err := getJSON(ctx, "https://www.workable.com/api/accounts/"+encodeURIComponent(token)+"?details=true", fetch.Options{})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for _, j := range array(get(data, "jobs")) {
		var locations []string
		if primary := workableLocation(get(j, "location")); primary != "" {
			locations = append(locations, primary)
		}
		for _, extra := range array(get(j, "locations")) {
			if s := workableLocation(extra); s != "" && !containsStr(locations, s) {
				locations = append(locations, s)
			}
		}
		if (truthy(get(j, "telecommuting")) || truthy(get(j, "location", "telecommuting"))) && !someRemote(locations) {
			locations = append(locations, "Remote")
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  text(coalesce(get(j, "shortcode"), get(j, "id"), get(j, "url"), get(j, "title"))),
			Title:       text(get(j, "title")),
			Locations:   locations,
			ApplyURL:    text(coalesce(get(j, "application_url"), get(j, "url"), get(j, "shortlink"), "")),
			PostedAt:    jsDate(coalesce(get(j, "published_on"), get(j, "created_at"))),
			Description: text(coalesce(get(j, "full_description"), get(j, "description"), "")),
		})
	}
	return out, nil
}

func fetchSmartRecruiters(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	const limit = 100
	var all []any
	for offset := 0; ; offset += limit {
		data, err := getJSON(ctx, "https://api.smartrecruiters.com/v1/companies/"+encodeURIComponent(token)+"/postings?limit="+strconv.Itoa(limit)+"&offset="+strconv.Itoa(offset), fetch.Options{})
		if err != nil {
			return nil, err
		}
		batch := array(get(data, "content"))
		all = append(all, batch...)
		if len(batch) < limit {
			break
		}
	}

	// List responses frequently omit jobAd.sections. Fetch public details so card
	// excerpts and description-based classification are based on the real posting.
	withDetails, err := MapConcurrent(ctx, all, 8, func(ctx context.Context, posting any) (any, error) {
		detail, err := getJSON(ctx, "https://api.smartrecruiters.com/v1/companies/"+encodeURIComponent(token)+"/postings/"+encodeURIComponent(jsString(get(posting, "id"))), fetch.Options{})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return posting, nil
		}
		return merge(posting, detail), nil
	})
	if err != nil {
		return nil, err
	}

	var out []model.NormalizedJob
	for _, j := range withDetails {
		var locations []string
		if s := filterJoin(", ", get(j, "location", "city"), get(j, "location", "region"), get(j, "location", "country")); s != "" {
			locations = append(locations, s)
		}
		if truthy(get(j, "location", "remote")) && !someRemote(locations) {
			locations = append(locations, "Remote")
		}
		var texts []any
		for _, s := range values(coalesce(get(j, "jobAd", "sections"), nil)) {
			texts = append(texts, coalesce(get(s, "text"), ""))
		}
		out = append(out, model.NormalizedJob{
			ExternalID: text(coalesce(get(j, "uuid"), get(j, "id"))),
			Title:      text(get(j, "name")),
			Locations:  locations,
			ApplyURL: text(coalesce(get(j, "applyUrl"), get(j, "postingUrl"),
				"https://jobs.smartrecruiters.com/"+encodeURIComponent(token)+"/"+jsString(get(j, "id")))),
			PostedAt:    jsDate(get(j, "releasedDate")),
			Description: filterJoin("\n\n", texts...),
		})
	}
	return out, nil
}

func fetchRecruitee(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	data, err := getJSON(ctx, "https://"+encodeURIComponent(token)+".recruitee.com/api/offers/", fetch.Options{})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for _, j := range array(get(data, "offers")) {
		var locations []string
		if loc := get(j, "location"); truthy(loc) {
			locations = append(locations, text(loc))
		}
		if cc := filterJoin(", ", get(j, "city"), get(j, "country")); cc != "" && !containsStr(locations, cc) {
			locations = append(locations, cc)
		}
		for _, loc := range array(get(j, "locations")) {
			v := get(loc, "name")
			if nullish(v) {
				v = filterJoin(", ", get(loc, "city"), get(loc, "country"))
			}
			if s := text(v); truthy(v) && !containsStr(locations, s) {
				locations = append(locations, s)
			}
		}
		if truthy(get(j, "remote")) && !someRemote(locations) {
			locations = append(locations, "Remote")
		}
		posted := get(j, "published_at")
		if !truthy(posted) {
			posted = get(j, "created_at")
		}
		out = append(out, model.NormalizedJob{
			ExternalID: jsString(get(j, "id")),
			Title:      textOr(get(j, "title"), "Untitled"),
			Locations:  locations,
			ApplyURL: text(coalesce(get(j, "careers_url"),
				"https://"+encodeURIComponent(token)+".recruitee.com/o/"+jsString(coalesce(get(j, "slug"), get(j, "id"))))),
			PostedAt:    jsDate(posted),
			Description: filterJoin("\n\n", get(j, "description"), get(j, "requirements")),
		})
	}
	return out, nil
}

func fetchRippling(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	list, err := getJSON(ctx, "https://api.rippling.com/platform/api/ats/v1/board/"+encodeURIComponent(token)+"/jobs", fetch.Options{})
	if err != nil {
		return nil, err
	}
	withDesc, err := MapConcurrent(ctx, array(list), 8, func(ctx context.Context, j any) (any, error) {
		detail, err := getJSON(ctx, "https://api.rippling.com/platform/api/ats/v1/board/"+encodeURIComponent(token)+"/jobs/"+jsString(get(j, "uuid")), fetch.Options{})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return j, nil
		}
		return merge(j, detail), nil
	})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for _, j := range withDesc {
		var locations []string
		if label := get(j, "workLocation", "label"); truthy(label) {
			locations = append(locations, text(label))
		}
		for _, loc := range array(get(j, "workLocations")) {
			if label := get(loc, "label"); truthy(label) && !containsStr(locations, text(label)) {
				locations = append(locations, text(label))
			}
		}
		description := ""
		switch d := get(j, "description").(type) {
		case string:
			description = d
		case nil, undefinedT, bool, float64:
		default:
			description = filterJoin("\n\n", get(d, "company"), get(d, "role"))
		}
		out = append(out, model.NormalizedJob{
			ExternalID: text(get(j, "uuid")),
			Title:      text(get(j, "name")),
			Locations:  locations,
			ApplyURL: text(coalesce(get(j, "url"),
				"https://ats.rippling.com/"+encodeURIComponent(token)+"/jobs/"+jsString(get(j, "uuid")))),
			PostedAt:    jsDate(get(j, "createdOn")),
			Description: description,
		})
	}
	return out, nil
}

type bambooDetail struct {
	description string
	postedAt    *string
}

func fetchBambooHRDetail(ctx context.Context, token, jobID string) (bambooDetail, error) {
	data, err := getJSON(ctx, "https://"+encodeURIComponent(token)+".bamboohr.com/careers/"+encodeURIComponent(jobID)+"/detail", fetch.Options{})
	if err != nil {
		if ctx.Err() != nil {
			return bambooDetail{}, ctx.Err()
		}
		return bambooDetail{}, nil
	}
	opening := get(data, "result", "jobOpening")
	var d bambooDetail
	if desc := get(opening, "description"); truthy(desc) {
		s, ok := desc.(string)
		if !ok {
			// stripHtmlTags threw inside the try: the detail counts as missing.
			return bambooDetail{}, nil
		}
		d.description = htmlx.StripTags(s)
	}
	d.postedAt = jsDate(get(opening, "datePosted"))
	return d, nil
}

func fetchBambooHR(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	data, err := getJSON(ctx, "https://"+encodeURIComponent(token)+".bamboohr.com/careers/list", fetch.Options{})
	if err != nil {
		return nil, err
	}
	openings := array(get(data, "result"))
	details, err := MapConcurrent(ctx, openings, 8, func(ctx context.Context, j any) (bambooDetail, error) {
		return fetchBambooHRDetail(ctx, token, jsString(get(j, "id")))
	})
	if err != nil {
		return nil, err
	}
	var out []model.NormalizedJob
	for i, j := range openings {
		var locations []string
		loc := coalesce(get(j, "atsLocation"), get(j, "location"))
		if truthy(loc) {
			if s := filterJoin(", ", get(loc, "city"), get(loc, "state"), get(loc, "country")); s != "" {
				locations = append(locations, s)
			}
		}
		if truthy(get(j, "isRemote")) && !someRemote(locations) {
			locations = append(locations, "Remote")
		}
		id := jsString(get(j, "id"))
		out = append(out, model.NormalizedJob{
			ExternalID:  id,
			Title:       text(get(j, "jobOpeningName")),
			Locations:   locations,
			ApplyURL:    "https://" + encodeURIComponent(token) + ".bamboohr.com/careers/" + id,
			Excerpt:     excerpt(details[i].description),
			PostedAt:    details[i].postedAt,
			Description: details[i].description,
		})
	}
	return out, nil
}

// trimmedStrings is `[...].map((l) => l?.trim()).filter(Boolean)`; ok is
// false where JavaScript would throw (a value that is not a string).
func trimmedStrings(vs []any) ([]string, bool) {
	var out []string
	for _, v := range vs {
		if nullish(v) {
			continue
		}
		s, isStr := v.(string)
		if !isStr {
			return nil, false
		}
		if t := jsTrim(s); t != "" {
			out = append(out, t)
		}
	}
	return out, true
}

var workdayLocationCount = jsRegexp(`(?i)^\d+\s+locations?$`)

// WorkdayLocationCount reports Workday's placeholder for postings at several
// sites ("3 Locations"); the places come from the detail instead.
func WorkdayLocationCount(s string) bool { return workdayLocationCount.MatchString(s) }

type workdayDetail struct {
	posting     any
	description string
	postedAt    *string
	id          string
	locations   []string
}

func fetchWorkday(ctx context.Context, token string) ([]model.NormalizedJob, error) {
	board, ok := ParseWorkdayToken(token)
	if !ok {
		return nil, errorf("Invalid workday board_token (want tenant|wdN|site): %s", token)
	}
	listURL := WorkdayJobsURL(board)
	const limit = 20
	var postings []any
	total := -1
	for offset := 0; total < 0 || offset < total; offset += limit {
		page, err := postJSON(ctx, listURL, jsObject("appliedFacets", jsObject(), "limit", limit, "offset", offset, "searchText", ""))
		if err != nil {
			return nil, err
		}
		batch := array(get(page, "jobPostings"))
		if n, ok := get(page, "total").(float64); ok {
			total = jsTotal(n)
		}
		postings = append(postings, batch...)
		if len(batch) == 0 || len(batch) < limit {
			break
		}
		// polite pacing — Workday throttles aggressive paging
		if err := pause(ctx, 400*time.Millisecond); err != nil {
			return nil, err
		}
	}

	details, err := MapConcurrent(ctx, postings, 4, func(ctx context.Context, p any) (workdayDetail, error) {
		path := get(p, "externalPath")
		if !truthy(path) {
			return workdayDetail{posting: p, id: text(coalesce(path, get(p, "title"), ""))}, nil
		}
		fallback := workdayDetail{posting: p, id: text(path)}
		pathStr, isStr := path.(string)
		if !isStr {
			return fallback, nil
		}
		detail, err := getJSON(ctx, WorkdayDetailURL(board, pathStr), fetch.Options{})
		if err != nil {
			if ctx.Err() != nil {
				return workdayDetail{}, ctx.Err()
			}
			return fallback, nil
		}
		info := get(detail, "jobPostingInfo")
		d := workdayDetail{posting: p}
		if desc := get(info, "jobDescription"); truthy(desc) {
			s, ok := desc.(string)
			if !ok {
				return fallback, nil
			}
			d.description = htmlx.StripTags(s)
		}
		if posted := get(info, "postedOn"); truthy(posted) {
			d.postedAt = jsDate(posted)
		} else {
			d.postedAt = jsDate(get(info, "startDate"))
		}
		d.id = jsString(coalesce(get(info, "id"), get(info, "jobReqId"), path))
		raw := append([]any{get(info, "location")}, array(coalesce(get(info, "additionalLocations"), nil))...)
		if extra := get(info, "additionalLocations"); !nullish(extra) {
			if _, isArr := extra.([]any); !isArr {
				// Spreading a non-array threw inside the try.
				return fallback, nil
			}
		}
		locs, ok := trimmedStrings(raw)
		if !ok {
			return fallback, nil
		}
		d.locations = locs
		return d, nil
	})
	if err != nil {
		return nil, err
	}

	var out []model.NormalizedJob
	for _, d := range details {
		p := d.posting
		var locations []string
		for _, loc := range append([]string{text(get(p, "locationsText"))}, d.locations...) {
			value := jsTrim(loc)
			if value != "" && WorkdayLocationCount(value) {
				continue
			}
			if value != "" && !containsStr(locations, value) {
				locations = append(locations, value)
			}
		}
		applyURL := WorkdayJobsURL(board)
		if path := get(p, "externalPath"); truthy(path) {
			applyURL = WorkdayApplyURL(board, text(path))
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  d.id,
			Title:       textOr(get(p, "title"), "Untitled"),
			Locations:   locations,
			ApplyURL:    applyURL,
			Excerpt:     excerpt(d.description),
			PostedAt:    d.postedAt,
			Description: d.description,
		})
	}
	return out, nil
}

// jsTotal turns a JSON total into an integer bound for `offset < total`.
func jsTotal(n float64) int {
	if n != n || n <= 0 {
		return 0
	}
	if n > 1e9 {
		return 1e9
	}
	return int(math.Ceil(n))
}
