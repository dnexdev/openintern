package ats

// Manual captures for career sites OpenIntern does not fetch automatically
// (Tesla, Citadel, Citadel Securities). The maintainer saves the public
// listing in a normal browser, `openintern manual-capture` reduces it to the
// fields below, and the result is committed as data/manual/<ats>.json.
//
// Only normalized fields are committed: no raw HTML and no full descriptions
// (excerpts are capped at 400 characters, like every other source). A capture
// older than ManualMaxAgeDays is not published.

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dnexdev/openintern/data"
	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/htmlx"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/safedate"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

const (
	ManualMaxAgeDays = 14
	ManualExcerptMax = 400
	dayMS            = 24 * 60 * 60 * 1000
)

// ManualJob is one committed listing.
type ManualJob struct {
	ExternalID string
	Title      string
	Locations  []string
	ApplyURL   string
	Excerpt    *string
	PostedAt   *string
}

// ManualCapture is data/manual/<ats>.json.
type ManualCapture struct {
	Source     string // the public page that was captured
	CapturedAt string
	CapturedBy string // a role, never a person's name (e.g. "maintainer")
	Jobs       []ManualJob
}

// ManualOptions overrides where captures are read from and the clock.
type ManualOptions struct {
	// Dir holds <ats>.json files. Nil means the captures compiled into the
	// binary (data/manual).
	Dir fs.FS
	// Now is the clock for the age limit. Zero means time.Now().
	Now time.Time
}

func (o ManualOptions) now() time.Time {
	if o.Now.IsZero() {
		return time.Now()
	}
	return o.Now
}

func (o ManualOptions) dir() (fs.FS, error) {
	if o.Dir != nil {
		return o.Dir, nil
	}
	return fs.Sub(data.FS, "manual")
}

var manualName = regexp.MustCompile(`^[a-z0-9_]+$`)

// ManualFile is the capture's file name, after checking the source name.
func ManualFile(ats string) (string, error) {
	if !manualName.MatchString(ats) {
		return "", fmt.Errorf("invalid manual source name: %s", ats)
	}
	return ats + ".json", nil
}

func isHTTPSURL(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	u, err := parseURL(s)
	return err == nil && u.Protocol == "https:"
}

func nowMS(t time.Time) float64 { return float64(t.UnixMilli()) }

// ValidateManualCapture checks a parsed capture (jsjson values) and returns
// it typed, or an error listing every problem.
func ValidateManualCapture(v any, now time.Time) (ManualCapture, error) {
	var errs []string
	obj := v
	if nullish(obj) {
		obj = jsjson.Object{}
	}
	if !isHTTPSURL(get(obj, "source")) {
		errs = append(errs, "source must be an https URL")
	}
	capturedAt := math.NaN()
	if s, ok := get(obj, "captured_at").(string); ok {
		if ms, ok := safedate.DateParse(s); ok {
			capturedAt = ms
		}
	}
	if math.IsNaN(capturedAt) {
		errs = append(errs, "captured_at must be an ISO date")
	} else if capturedAt > nowMS(now)+60*60*1000 {
		errs = append(errs, "captured_at is in the future")
	}
	if by, ok := get(obj, "captured_by").(string); !ok || jsTrim(by) == "" {
		errs = append(errs, "captured_by is required")
	}
	rawJobs, jobsOK := get(obj, "jobs").([]any)
	if !jobsOK {
		errs = append(errs, "jobs must be an array")
	}
	ids := map[string]bool{}
	var jobs []ManualJob
	for i, raw := range rawJobs {
		j := raw
		if nullish(j) {
			j = jsjson.Object{}
		}
		at := fmt.Sprintf("jobs[%d]", i)
		var job ManualJob
		id, ok := get(j, "external_id").(string)
		switch {
		case !ok || jsTrim(id) == "":
			errs = append(errs, at+".external_id is required")
		case ids[id]:
			errs = append(errs, fmt.Sprintf("%s.external_id %s is duplicated", at, id))
		default:
			ids[id] = true
		}
		job.ExternalID = id
		if title, ok := get(j, "title").(string); !ok || jsTrim(title) == "" {
			errs = append(errs, at+".title is required")
		} else {
			job.Title = title
		}
		locs, ok := get(j, "locations").([]any)
		allStrings := ok
		for _, l := range locs {
			s, isStr := l.(string)
			if !isStr {
				allStrings = false
				break
			}
			job.Locations = append(job.Locations, s)
		}
		if !allStrings {
			errs = append(errs, at+".locations must be a list of strings")
		}
		if !isHTTPSURL(get(j, "apply_url")) {
			errs = append(errs, at+".apply_url must be an https URL")
		} else {
			job.ApplyURL = get(j, "apply_url").(string)
		}
		ex := get(j, "excerpt")
		if !nullish(ex) {
			s, isStr := ex.(string)
			if !isStr || len16(s) > ManualExcerptMax {
				errs = append(errs, fmt.Sprintf("%s.excerpt must be a string of at most %d characters", at, ManualExcerptMax))
			} else {
				job.Excerpt = &s
			}
		}
		if p := get(j, "posted_at"); !nullish(p) {
			s, isStr := p.(string)
			if _, parsed := safedate.DateParse(s); !isStr || !parsed {
				errs = append(errs, at+".posted_at must be an ISO date or null")
			} else {
				job.PostedAt = &s
			}
		}
		jobs = append(jobs, job)
	}
	if len(errs) > 0 {
		return ManualCapture{}, errors.New("invalid manual capture: " + strings.Join(errs, "; "))
	}
	return ManualCapture{
		Source:     get(obj, "source").(string),
		CapturedAt: get(obj, "captured_at").(string),
		CapturedBy: get(obj, "captured_by").(string),
		Jobs:       jobs,
	}, nil
}

// ManualCaptureAgeDays is how old a capture is, in fractional days.
func ManualCaptureAgeDays(c ManualCapture, now time.Time) float64 {
	ms, _ := safedate.DateParse(c.CapturedAt)
	return (nowMS(now) - ms) / dayMS
}

// LoadManualJobs returns a capture's jobs, or a *fetch.Failure when it is
// missing (not-found) or too old to publish (manual-expired).
func LoadManualJobs(ats string, opts ManualOptions) ([]model.NormalizedJob, error) {
	now := opts.now()
	name, err := ManualFile(ats)
	if err != nil {
		return nil, err
	}
	dir, err := opts.dir()
	if err != nil {
		return nil, err
	}
	raw, err := fs.ReadFile(dir, name)
	if err != nil {
		return nil, fetch.NewFailure(fetch.KindNotFound, "no manual capture at data/manual/"+name, 0)
	}
	v, err := jsjson.Parse(decodeBody(raw))
	if err != nil {
		return nil, fmt.Errorf("data/manual/%s: %w", name, err)
	}
	capture, err := ValidateManualCapture(v, now)
	if err != nil {
		return nil, err
	}
	if age := ManualCaptureAgeDays(capture, now); age > ManualMaxAgeDays {
		return nil, fetch.NewFailure(fetch.KindManualExpired,
			fmt.Sprintf("data/manual/%s was captured %d days ago (limit %d)", name, int64(math.Floor(age)), ManualMaxAgeDays), 0)
	}
	out := make([]model.NormalizedJob, 0, len(capture.Jobs))
	for _, j := range capture.Jobs {
		var posted *string
		if j.PostedAt != nil && *j.PostedAt != "" {
			posted = safedate.ISOPtr(safedate.ParseString(*j.PostedAt))
		}
		desc := ""
		if j.Excerpt != nil {
			desc = *j.Excerpt
		}
		locs := j.Locations
		if locs == nil {
			locs = []string{}
		}
		out = append(out, model.NormalizedJob{
			ExternalID:  j.ExternalID,
			Title:       j.Title,
			Locations:   locs,
			ApplyURL:    j.ApplyURL,
			Excerpt:     j.Excerpt,
			PostedAt:    posted,
			Description: desc,
		})
	}
	return out, nil
}

// ToManualCapture reduces parsed listings to a committable capture, sorted
// by external_id for stable diffs. capturedBy "" means "maintainer".
func ToManualCapture(jobs []model.NormalizedJob, source string, now time.Time, capturedBy string) (ManualCapture, error) {
	if capturedBy == "" {
		capturedBy = "maintainer"
	}
	c := ManualCapture{Source: source, CapturedAt: safedate.FormatISO(now), CapturedBy: capturedBy}
	for _, j := range jobs {
		src := j.Description
		if j.Excerpt != nil {
			src = *j.Excerpt
		}
		ex := slice16(jsTrim(collapseJSSpace(src)), ManualExcerptMax)
		var exPtr *string
		if ex != "" {
			exPtr = &ex
		}
		locs := j.Locations
		if locs == nil {
			locs = []string{}
		}
		c.Jobs = append(c.Jobs, ManualJob{
			ExternalID: j.ExternalID,
			Title:      j.Title,
			Locations:  locs,
			ApplyURL:   j.ApplyURL,
			Excerpt:    exPtr,
			PostedAt:   j.PostedAt,
		})
	}
	sort.SliceStable(c.Jobs, func(a, b int) bool {
		return localeCompare(c.Jobs[a].ExternalID, c.Jobs[b].ExternalID) < 0
	})
	if _, err := ValidateManualCapture(c.JSValue(), now); err != nil {
		return ManualCapture{}, err
	}
	return c, nil
}

// JSValue is the capture as ordered JSON, keys as in the committed files.
func (c ManualCapture) JSValue() jsjson.Object {
	jobs := make([]any, 0, len(c.Jobs))
	for _, j := range c.Jobs {
		locs := make([]any, len(j.Locations))
		for i, l := range j.Locations {
			locs[i] = l
		}
		var ex, posted any
		if j.Excerpt != nil {
			ex = *j.Excerpt
		}
		if j.PostedAt != nil {
			posted = *j.PostedAt
		}
		jobs = append(jobs, jsObject(
			"external_id", j.ExternalID,
			"title", j.Title,
			"locations", locs,
			"apply_url", j.ApplyURL,
			"excerpt", ex,
			"posted_at", posted,
		))
	}
	return jsObject(
		"source", c.Source,
		"captured_at", c.CapturedAt,
		"captured_by", c.CapturedBy,
		"jobs", jobs,
	)
}

// MarshalFile is `${JSON.stringify(capture, null, 2)}\n`.
func (c ManualCapture) MarshalFile() []byte {
	return append(jsjson.MarshalIndent(c.JSValue(), "  "), '\n')
}

// ProbeResult is what the probe tools report for one board.
type ProbeResult struct {
	OK     bool
	Status int
	// Count is nil when the probe did not count jobs.
	Count *int
	Error string
}

// ManualProbe reports whether there is a publishable capture and its size.
func ManualProbe(ats string) ProbeResult {
	jobs, err := LoadManualJobs(ats, ManualOptions{})
	if err != nil {
		return ProbeResult{Error: err.Error()}
	}
	n := len(jobs)
	r := ProbeResult{OK: n > 0, Status: 200, Count: &n}
	if n == 0 {
		r.Error = "empty capture"
	}
	return r
}

// collapseJSSpace is `.replace(/\s+/g, " ")`. Bytes that are not
// whitespace are copied as they are, so WTF-8 lone surrogates survive.
func collapseJSSpace(s string) string {
	var b strings.Builder
	prev := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if htmlx.IsJSSpace(r) {
			if !prev {
				b.WriteByte(' ')
			}
			prev = true
		} else {
			prev = false
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
