package health

import (
	"strings"
	"testing"
	"time"

	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state"
)

// Byte parity of the whole snapshot with the TypeScript is checked by the
// dump tests (health.json in internal/dump/testdata).

func ptr[T any](v T) *T { return &v }

func TestParseFunnel(t *testing.T) {
	f := ParseFunnel(ptr("funnel fetched=12 title=4 tech=2 upserted=2 description_missing=3"))
	if *f.Fetched != 12 || *f.Title != 4 || *f.Tech != 2 || *f.Upserted != 2 || *f.DescriptionMissing != 3 {
		t.Errorf("full funnel: %+v", f)
	}
	f = ParseFunnel(ptr("zero_match: no tech internships classified; funnel fetched=0 title=0 tech=0 upserted=0"))
	if *f.Fetched != 0 || f.DescriptionMissing != nil {
		t.Errorf("zero-match funnel without description_missing: %+v", f)
	}
	for _, s := range []*string{nil, ptr(""), ptr("HTTP 500"), ptr("funnel fetched=1 title=2")} {
		if f := ParseFunnel(s); f != (Funnel{}) {
			t.Errorf("ParseFunnel(%v) = %+v, want all nil", s, f)
		}
	}
	// Number(digits) < NAT_LIMIT: leading zeros are fine, huge counts are not real.
	f = ParseFunnel(ptr("funnel fetched=000000000000000000007 title=281474976710655 tech=281474976710656 upserted=99999999999999999999"))
	if *f.Fetched != 7 || *f.Title != 281474976710655 || f.Tech != nil || f.Upserted != nil {
		t.Errorf("limits: %+v", f)
	}
}

func TestZeroMatch(t *testing.T) {
	if IsZeroMatchRun(nil) || IsZeroMatchRun(ptr("")) || IsZeroMatchRun(ptr("x zero_match:")) || !IsZeroMatchRun(ptr("zero_match: none")) {
		t.Error("zero_match is a prefix of a non-empty error")
	}
}

func TestSnapshot(t *testing.T) {
	st := state.New()
	add := func(id, slug, ats string, active bool) {
		st.Companies[id] = &state.Company{Company: model.Company{ID: id, Slug: slug, Name: strings.ToUpper(slug), ATS: ats, Active: active}}
	}
	add("c1", "acme", "greenhouse", true)
	add("c2", "tiktok", "tiktok", true)
	add("c3", "gone", "lever", false)
	st.Jobs["j1"] = &state.Job{Job: model.Job{ID: "j1", IsActive: true}}
	st.Jobs["j2"] = &state.Job{Job: model.Job{ID: "j2"}}
	run := func(id, cid, status string, n int, err *string, at string) {
		st.Runs = append(st.Runs, &state.Run{Run: model.Run{ID: id, CompanyID: cid, Status: status, JobCount: n, Error: err, RanAt: at}})
	}
	run("r1", "c1", "ok", 3, ptr("funnel fetched=10 title=5 tech=3 upserted=3"), "2026-11-12T06:00:00+00:00")
	run("r2", "c1", "ok", 0, ptr("zero_match: none; funnel fetched=4 title=0 tech=0 upserted=0"), "2026-11-13T06:00:00+00:00")
	run("r3", "c2", "error", 0, ptr("refused: HTTP 403"), "2026-11-13T06:00:01+00:00")
	run("r4", "c2", "ok", 0, nil, "2026-11-12T06:00:01+00:00")
	run("r5", "missing", "error", 0, ptr("boom"), "2026-11-13T05:00:00.5+00:00")

	snap, err := FromState(st, time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	s := snap.Stats
	if s.ActiveJobs != 1 || s.TotalJobs != 2 || s.ActiveCompanies != 2 || s.TotalCompanies != 3 {
		t.Errorf("stats %+v", s)
	}
	if *s.LastSuccessfulIngest != "2026-11-13T06:00:00.000Z" {
		t.Errorf("last ok %v", *s.LastSuccessfulIngest)
	}
	if strings.Join(snap.ZeroMatchCompanies, ",") != "acme" || s.ZeroMatchCompanies != 1 {
		t.Errorf("zero match %v", snap.ZeroMatchCompanies)
	}
	ids := []string{}
	for _, r := range snap.RecentIngestRuns {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "r3,r2,r5,r4,r1" {
		t.Errorf("runs newest first: %v", ids)
	}
	if len(snap.RecentFailures) != 2 || snap.RecentFailures[1].CompanySlug != nil {
		t.Errorf("failures %+v", snap.RecentFailures)
	}
	if len(snap.ProprietaryFailures) != 1 || *snap.ProprietaryFailures[0].CompanySlug != "tiktok" || len(snap.ProprietaryZeroJobs) != 0 {
		t.Errorf("proprietary %+v %+v", snap.ProprietaryFailures, snap.ProprietaryZeroJobs)
	}
	if ft := snap.FunnelTotals; ft.Fetched != 4 || ft.Title != 0 {
		t.Errorf("funnel totals use the latest run per company: %+v", ft)
	}
	if !strings.HasPrefix(string(snap.JSON()), "{\n  \"generatedAt\": \"2026-11-14T00:00:00.000Z\",\n  \"stats\": {") {
		t.Errorf("json %s", snap.JSON()[:80])
	}
}

func TestBadRanAtFails(t *testing.T) {
	st := state.New()
	st.Runs = []*state.Run{{Run: model.Run{ID: "r", Status: "ok", RanAt: "yesterday"}}}
	if _, err := FromState(st, time.Now()); err == nil {
		t.Error("an unparseable ran_at that is rendered must fail, as toISOString threw")
	}
}
