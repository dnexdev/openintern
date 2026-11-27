package ats

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// Port of manual.test.ts.
func TestManualCaptureRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	posted := "2026-10-30T00:00:00.000Z"
	ex := "Embedded C"
	capture, err := ToManualCapture([]model.NormalizedJob{
		{ExternalID: "2", Title: "Software Engineering Intern", Locations: []string{"Palo Alto, CA"},
			ApplyURL: "https://www.tesla.com/careers/search/job/2", Description: strings.Repeat("x ", 600)},
		{ExternalID: "1", Title: "Firmware Intern", Locations: []string{},
			ApplyURL: "https://www.tesla.com/careers/search/job/1", Excerpt: &ex, PostedAt: &posted},
	}, "https://www.tesla.com/cua-api/apps/careers/state", now, "")
	if err != nil {
		t.Fatal(err)
	}
	if capture.CapturedBy != "maintainer" {
		t.Fatal(capture.CapturedBy)
	}
	if capture.Jobs[0].ExternalID != "1" || capture.Jobs[1].ExternalID != "2" {
		t.Fatal("sorted for stable diffs")
	}
	if capture.Jobs[1].Excerpt == nil || len16(*capture.Jobs[1].Excerpt) > 400 {
		t.Fatal("excerpt capped")
	}

	dir := fstest.MapFS{"tesla.json": {Data: jsjson.Marshal(capture.JSValue())}}
	jobs, err := LoadManualJobs("tesla", ManualOptions{Dir: dir, Now: now.Add(3 * day)})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].PostedAt == nil || *jobs[0].PostedAt != "2026-10-30T00:00:00.000Z" {
		t.Fatalf("%+v", jobs)
	}
	if jobs[0].Description != "Embedded C" {
		t.Fatalf("description is the excerpt: %q", jobs[0].Description)
	}

	var f *fetch.Failure
	_, err = LoadManualJobs("tesla", ManualOptions{Dir: dir, Now: now.Add((ManualMaxAgeDays + 1) * day)})
	if !errors.As(err, &f) || f.Kind != fetch.KindManualExpired {
		t.Fatalf("expired: %v", err)
	}
	if err.Error() != "manual-expired: data/manual/tesla.json was captured 15 days ago (limit 14)" {
		t.Fatalf("message: %v", err)
	}
	_, err = LoadManualJobs("citadel", ManualOptions{Dir: dir, Now: now})
	if !errors.As(err, &f) || f.Kind != fetch.KindNotFound {
		t.Fatalf("missing: %v", err)
	}
	if _, err := LoadManualJobs("../etc", ManualOptions{Dir: dir, Now: now}); err == nil || !strings.Contains(err.Error(), "invalid manual source") {
		t.Fatalf("bad name: %v", err)
	}

	base := capture.JSValue()
	withJobs := func(jobs ...any) jsjson.Object {
		o := append(jsjson.Object{}, base...)
		o[3] = jsjson.Member{Key: "jobs", Value: jobs}
		return o
	}
	firstJob := base[3].Value.([]any)[0].(jsjson.Object)
	bad := append(jsjson.Object{}, firstJob...)
	bad[3] = jsjson.Member{Key: "apply_url", Value: "javascript:alert(1)"}
	bad[4] = jsjson.Member{Key: "excerpt", Value: strings.Repeat("y", 401)}
	if _, err := ValidateManualCapture(withJobs(bad), now); err == nil || !regexp.MustCompile(`apply_url must be an https URL.*excerpt`).MatchString(err.Error()) {
		t.Fatalf("bad job: %v", err)
	}
	future := append(jsjson.Object{}, base...)
	future[1] = jsjson.Member{Key: "captured_at", Value: "2027-01-01T00:00:00Z"}
	if _, err := ValidateManualCapture(future, now); err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("future: %v", err)
	}
	if _, err := ValidateManualCapture(withJobs(firstJob, firstJob), now); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicated: %v", err)
	}
}

func TestManualCaptureFileBytes(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	c, err := ToManualCapture([]model.NormalizedJob{{ExternalID: "a", Title: "T", Locations: []string{"X"}, ApplyURL: "https://e.com/a", Description: " d\n e "}}, "https://e.com/", now, "")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "source": "https://e.com/",
  "captured_at": "2026-11-01T12:00:00.000Z",
  "captured_by": "maintainer",
  "jobs": [
    {
      "external_id": "a",
      "title": "T",
      "locations": [
        "X"
      ],
      "apply_url": "https://e.com/a",
      "excerpt": "d e",
      "posted_at": null
    }
  ]
}
`
	if got := string(c.MarshalFile()); got != want {
		t.Fatalf("got\n%s", got)
	}
}

func TestEmbeddedManualCaptures(t *testing.T) {
	// data/manual has no captures committed today: the manual sources fail
	// as not-found (a failed run), never as an empty board.
	for _, ats := range []string{"tesla", "citadel", "citadel_securities"} {
		if _, err := os.Stat(filepath.Join("..", "..", "data", "manual", ats+".json")); err == nil {
			continue
		}
		_, err := Fetch(context.Background(), ats, "careers")
		var f *fetch.Failure
		if !errors.As(err, &f) || f.Kind != fetch.KindNotFound {
			t.Errorf("%s: %v", ats, err)
		}
	}
}

func TestFetchUnsupported(t *testing.T) {
	if _, err := Fetch(context.Background(), "x", ""); err == nil || err.Error() != "X is not ingested" {
		t.Fatalf("x: %v", err)
	}
	if _, err := Fetch(context.Background(), "nope", ""); err == nil || err.Error() != "Unsupported ATS: nope" {
		t.Fatalf("nope: %v", err)
	}
	if _, err := Fetch(context.Background(), "workday", "bad"); err == nil || err.Error() != "Invalid workday board_token (want tenant|wdN|site): bad" {
		t.Fatalf("workday: %v", err)
	}
}

func TestJSHelpers(t *testing.T) {
	// "a😀" sliced inside the emoji keeps the high surrogate, as JavaScript does.
	if got := slice16("a😀", 2); got != "a\xed\xa0\xbd" {
		t.Fatalf("slice16 = %q", got)
	}
	if s := string(jsjson.Marshal(slice16("a😀", 2))); s != `"a\ud83d"` {
		t.Fatalf("lone surrogate encodes as %s", s)
	}
	if got, err := decodeURIComponent("a%3a%E2%9C%93+"); err != nil || got != "a:✓+" {
		t.Fatalf("%q %v", got, err)
	}
	for _, bad := range []string{"%", "%zz", "%E2%9C", "%C0%80", "%ED%A0%80"} {
		if _, err := decodeURIComponent(bad); err == nil {
			t.Errorf("decodeURIComponent(%q) should fail", bad)
		}
	}
	if encodeURIComponent("a b/c?d=é!'()*~") != "a%20b%2Fc%3Fd%3D%C3%A9!'()*~" {
		t.Fatal(encodeURIComponent("a b/c?d=é!'()*~"))
	}
	// Pairs Node's localeCompare orders first-before-second.
	for _, pair := range [][2]string{{"10", "9"}, {"a-b", "ab"}, {"a", "B"}, {"b", "B"}, {"B", "c"}, {"1", "a"}, {"_x", "1"}, {"é", "f"}, {"e", "é"}, {"a b", "a-b"}, {"a", "Z"}} {
		if localeCompare(pair[0], pair[1]) >= 0 {
			t.Errorf("localeCompare(%q, %q) should be < 0", pair[0], pair[1])
		}
	}
}
