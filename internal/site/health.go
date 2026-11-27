package site

import (
	"net/http"
	"strconv"
	"strings"
)

type funnelStep struct {
	Label   string
	Count   int
	Percent int // width of the bar, relative to fetched
}

type runRow struct {
	When        string
	Company     string
	CompanySlug string
	Status      string
	OK          bool
	Fetched     string
	Title       string
	Tech        string
	Upserted    string
	Note        string
}

type healthView struct {
	Error           bool
	HasSnapshot     bool
	ActiveJobs      int
	TotalJobs       int
	ActiveCompanies int
	TotalCompanies  int
	Inactive        int
	LastOK          string
	SnapshotAt      string
	Funnel          []funnelStep
	Runs            []runRow
}

func intOrDash(p *int) string {
	if p == nil {
		return "—"
	}
	return formatInt(*p)
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	// Reads the dump job's published snapshot; falls back to counting the
	// public dump when no snapshot is published.
	ctx := r.Context()
	var v healthView
	snapshot, err := h.store.HealthSnapshot(ctx)
	if err == nil && snapshot != nil {
		v.HasSnapshot = true
		s := snapshot.Stats
		v.ActiveJobs, v.TotalJobs = s.ActiveJobs, s.TotalJobs
		v.ActiveCompanies, v.TotalCompanies = s.ActiveCompanies, s.TotalCompanies
		v.Inactive = s.TotalCompanies - s.ActiveCompanies
		v.LastOK = "never"
		if s.LastSuccessfulIngest != nil {
			v.LastOK = *s.LastSuccessfulIngest
		}
		gen := snapshot.GeneratedAt
		if len(gen) >= 16 {
			gen = gen[:16]
		}
		v.SnapshotAt = strings.Replace(gen, "T", " ", 1)
		f := snapshot.FunnelTotalsLatestPerCompany
		for _, step := range []funnelStep{
			{Label: "Fetched from boards", Count: f.Fetched},
			{Label: "Internship title", Count: f.Title},
			{Label: "Tech role", Count: f.Tech},
			{Label: "Upserted", Count: f.Upserted},
		} {
			if f.Fetched > 0 {
				step.Percent = max(1, step.Count*100/f.Fetched)
				if step.Count == 0 {
					step.Percent = 0
				}
			}
			v.Funnel = append(v.Funnel, step)
		}
		for _, run := range snapshot.RecentIngestRuns {
			row := runRow{
				Status:   run.Status,
				OK:       run.Status == "ok",
				Fetched:  intOrDash(run.Funnel.Fetched),
				Title:    intOrDash(run.Funnel.Title),
				Upserted: intOrDash(run.Funnel.Upserted),
				Company:  "—",
			}
			when := run.RanAt
			if len(when) > 19 {
				when = when[:19]
			}
			row.When = strings.Replace(when, "T", " ", 1) + "Z"
			if run.CompanyName != nil {
				row.Company = *run.CompanyName
			}
			if run.CompanySlug != nil {
				row.CompanySlug = *run.CompanySlug
			}
			if run.Funnel.Tech != nil {
				row.Tech = formatInt(*run.Funnel.Tech)
			} else {
				row.Tech = strconv.Itoa(run.JobCount)
			}
			errText := ""
			if run.Error != nil {
				errText = *run.Error
			}
			switch {
			case strings.HasPrefix(errText, "zero_match"):
				row.Note = "zero_match"
			case run.Status == "error":
				row.Note = truncateUTF16(errText, 80)
			}
			v.Runs = append(v.Runs, row)
		}
	} else {
		status, derr := h.store.DumpStatus(ctx)
		if derr != nil {
			v.Error = true
		} else {
			v.ActiveJobs, v.ActiveCompanies = status.ActiveJobs, status.ActiveCompanies
		}
	}
	h.render(w, http.StatusOK, cachePublic, "health", view{
		Meta: meta{Title: "Ingest health · OpenIntern", Description: defaultDescription},
		Data: v,
	})
}
