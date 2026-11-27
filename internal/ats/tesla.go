package ats

// Tesla careers: manual capture only.
//
// Tesla's careers API refuses automated clients, and OpenIntern does not
// pretend to be a browser. The maintainer saves the state JSON from
// TeslaStateURL in a normal browser and runs
// `openintern manual-capture tesla <file>`, which writes
// data/manual/tesla.json.
//
// board_token: unused; conventionally `careers`.

import (
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

const TeslaStateURL = "https://www.tesla.com/cua-api/apps/careers/state"

// teslaString is asString: a trimmed non-empty string, or a number as text.
func teslaString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		if t := jsTrim(x); t != "" {
			return t, true
		}
	case float64:
		return jsString(x), true
	}
	return "", false
}

func teslaFirst(row any, keys ...string) (string, bool) {
	for _, k := range keys {
		if s, ok := teslaString(get(row, k)); ok {
			return s, true
		}
	}
	return "", false
}

// isObject is `v && typeof v === "object"` (arrays included).
func isObject(v any) bool {
	switch v.(type) {
	case jsjson.Object, []any:
		return true
	}
	return false
}

// entries is Object.entries for an object or array.
func entries(v any) []jsjson.Member {
	switch x := v.(type) {
	case jsjson.Object:
		return jsjson.JSOrder(dedupe(x))
	case []any:
		out := make([]jsjson.Member, len(x))
		for i, e := range x {
			out[i] = jsjson.Member{Key: jsString(float64(i)), Value: e}
		}
		return out
	}
	return nil
}

func teslaSites(state any) map[string]string {
	out := map[string]string{}
	lookup := get(state, "lookup")
	var nested any = lookup
	if truthy(lookup) {
		nested = get(lookup, "sites")
	}
	for _, c := range []any{get(state, "sites"), nested} {
		if !truthy(c) || !isObject(c) {
			continue
		}
		for _, e := range entries(c) {
			switch v := e.Value.(type) {
			case string:
				out[e.Key] = v
			default:
				if truthy(v) && isObject(v) {
					if name, ok := teslaFirst(v, "name", "l"); ok {
						out[e.Key] = name
					}
				}
			}
		}
	}
	return out
}

func teslaListings(state any) []any {
	if !isObject(state) {
		return nil
	}
	for _, key := range []string{"listings", "jobs", "results", "data"} {
		if arr, ok := get(state, key).([]any); ok {
			var out []any
			for _, x := range arr {
				if truthy(x) && isObject(x) {
					out = append(out, x)
				}
			}
			return out
		}
	}
	if arr, ok := state.([]any); ok {
		return arr
	}
	return nil
}

func teslaLocations(row any, sites map[string]string) []string {
	if direct, ok := teslaFirst(row, "l", "location", "locations", "city"); ok {
		return []string{direct}
	}
	if siteID, ok := teslaFirst(row, "sid", "siteId", "locationId"); ok && sites[siteID] != "" {
		return []string{sites[siteID]}
	}
	ids := coalesce(get(row, "locationIds"), get(row, "siteIds"))
	if arr, ok := ids.([]any); ok {
		var names []string
		for _, id := range arr {
			if name := sites[jsString(id)]; name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	return []string{}
}

// ParseTeslaCareersState reads the saved careers state JSON (jsjson value).
func ParseTeslaCareersState(state any) []model.NormalizedJob {
	root := state
	if !isObject(root) {
		root = jsjson.Object{}
	}
	sites := teslaSites(root)
	var jobs []model.NormalizedJob
	for _, row := range teslaListings(state) {
		id, okID := teslaFirst(row, "id", "jobId", "job_id", "Id")
		title, okTitle := teslaFirst(row, "t", "title", "name", "jobTitle")
		if !okID || !okTitle {
			continue
		}
		description, _ := teslaFirst(row, "description", "d", "jobDescription")
		apply, ok := teslaFirst(row, "url", "applyUrl", "absolute_url")
		if !ok {
			apply = "https://www.tesla.com/careers/search/job/" + encodeURIComponent(id)
		}
		jobs = append(jobs, model.NormalizedJob{
			ExternalID:  id,
			Title:       title,
			Locations:   teslaLocations(row, sites),
			ApplyURL:    apply,
			Excerpt:     excerpt(description),
			Description: description,
		})
	}
	return jobs
}
