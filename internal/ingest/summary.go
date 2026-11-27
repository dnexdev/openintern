package ingest

import (
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/applycheck"
	"github.com/dnexdev/openintern/internal/proven"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

func obj(kv ...any) jsjson.Object {
	o := make(jsjson.Object, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		o = append(o, jsjson.Member{Key: kv[i].(string), Value: kv[i+1]})
	}
	return o
}

func strs(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func optStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func optInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func optNum(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// Value is the summary as the TypeScript returned it, keys in its order.
func (s Summary) Value() jsjson.Object {
	shadow := make([]any, len(s.Shadow))
	for i, r := range s.Shadow {
		o := obj("source", r.Source, "ok", r.OK, "status", optInt(r.Status), "count", optNum(r.Count), "legacyCount", r.LegacyCount)
		if r.Error != nil {
			o = append(o, jsjson.Member{Key: "error", Value: *r.Error})
		}
		shadow[i] = o
	}
	violations := make([]any, len(s.ApplyHostViolations))
	for i, v := range s.ApplyHostViolations {
		violations[i] = obj("company", v.Company, "host", optStr(v.Host), "reason", v.Reason, "example", v.Example)
	}
	failures := make([]any, len(s.Failures))
	for i, f := range s.Failures {
		failures[i] = obj("slug", f.Slug, "error", f.Error)
	}
	funnel := make([]any, len(s.Funnel))
	for i, f := range s.Funnel {
		funnel[i] = obj("slug", f.Slug, "fetched", f.Fetched, "internshipTitle", f.InternshipTitle,
			"techPass", f.TechPass, "upserted", f.Upserted, "stale", f.Stale)
	}
	t := s.FunnelTotals
	out := obj(
		"companies", s.Companies,
		"jobsUpserted", s.JobsUpserted,
		"jobsDeactivated", s.JobsDeactivated,
		"staleDeactivated", s.StaleDeactivated,
		"lastSeenDeactivated", s.LastSeenDeactivated,
		"lastSeenSweepSkipped", s.LastSeenSweepSkipped,
		"zeroMatchCompanies", strs(s.ZeroMatchCompanies),
		"autoDisabled", strs(s.AutoDisabled),
		"autoDisabledSkipped", strs(s.AutoDisabledSkipped),
		"suspectedBlocks", strs(s.SuspectedBlocks),
		"shadow", shadow,
		"applyHostViolations", violations,
		"failures", failures,
		"funnel", funnel,
		"funnelTotals", obj("fetched", t.Fetched, "internshipTitle", t.InternshipTitle, "techPass", t.TechPass, "upserted", t.Upserted),
	)
	// Only runs with the apply-link probe on carry it, so the summary of a
	// run without it reads exactly as before.
	if p := s.ApplyProbe; p != nil {
		retired := make([]any, len(p.RetiredJobs))
		for i, r := range p.RetiredJobs {
			retired[i] = obj("company", r.Company, "id", r.ID, "url", r.URL, "reason", r.Reason)
		}
		out = append(out, jsjson.Member{Key: "applyProbe", Value: obj(
			"probed", p.Probed, "alive", p.Alive, "dead", p.Dead, "inconclusive", p.Inconclusive,
			"strikes", p.Strikes, "retired", p.Retired, "cleared", p.Cleared, "deferred", p.Deferred,
			"hostsBacked", strs(p.HostsBacked), "heldCompanies", strs(p.HeldCompanies), "retiredJobs", retired,
		)})
	}
	return out
}

// JSON is JSON.stringify(summary, null, 2), as the CLI prints it.
func (s Summary) JSON() []byte { return jsjson.MarshalIndent(s.Value(), "  ") }

// Failed is the CLI's exit condition: companies were fetched, nothing was
// upserted and something failed.
func (s Summary) Failed() bool {
	return s.Companies > 0 && s.JobsUpserted == 0 && len(s.Failures) > 0
}

// StepSummary is ingestStepSummary: a short Markdown report for the Actions
// run page.
func StepSummary(s Summary) string {
	lines := []string{
		"### Ingest",
		strconv.Itoa(s.Companies) + " companies fetched, " + strconv.Itoa(s.JobsUpserted) + " jobs upserted, " +
			strconv.Itoa(s.JobsDeactivated) + " retired, " + strconv.Itoa(len(s.Failures)) + " failed.",
	}
	if s.LastSeenSweepSkipped {
		lines = append(lines, "", "Last-seen sweep skipped: the previous successful run was more than 2 days ago.")
	}
	if len(s.AutoDisabledSkipped) > 0 {
		lines = append(lines, "", "Auto-disabled, not fetched: "+strings.Join(s.AutoDisabledSkipped, ", "))
	}
	if p := s.ApplyProbe; p != nil {
		lines = append(lines, "", "Apply links: "+strconv.Itoa(p.Probed)+" checked, "+strconv.Itoa(p.Dead)+" dead, "+
			strconv.Itoa(p.Inconclusive)+" inconclusive, "+strconv.Itoa(p.Strikes)+" new strikes, "+
			strconv.Itoa(p.Retired)+" retired, "+strconv.Itoa(p.Cleared)+" working again, "+strconv.Itoa(p.Deferred)+" deferred.")
		if len(p.HostsBacked) > 0 {
			lines = append(lines, "", "Stopped probing (refused or throttled): "+strings.Join(p.HostsBacked, ", "))
		}
		if len(p.HeldCompanies) > 0 {
			lines = append(lines, "", "Held (every apply link dead): "+strings.Join(p.HeldCompanies, ", "))
		}
		for i, r := range p.RetiredJobs {
			if i == probeRetiredListedLimit {
				lines = append(lines, "- and "+strconv.Itoa(len(p.RetiredJobs)-i)+" more")
				break
			}
			lines = append(lines, "- retired `"+r.Company+"` "+r.URL+" ("+strings.ReplaceAll(r.Reason, "|", "/")+")")
		}
	}
	if len(s.Shadow) > 0 {
		lines = append(lines, "", "#### Honest-request shadow test", "",
			"| source | honest ok | status | honest count | legacy count | error |", "|---|---|---|---|---|---|")
		for _, r := range s.Shadow {
			status, count, errText := "", "", ""
			if r.Status != nil {
				status = strconv.Itoa(*r.Status)
			}
			if r.Count != nil {
				count = string(jsjson.AppendNumber(nil, *r.Count))
			}
			if r.Error != nil {
				errText = Slice16(strings.ReplaceAll(*r.Error, "|", "/"), 120)
			}
			lines = append(lines, "| "+r.Source+" | "+strconv.FormatBool(r.OK)+" | "+status+" | "+count+" | "+
				strconv.Itoa(r.LegacyCount)+" | "+errText+" |")
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// Digest is ingestDigest: Markdown for things a person must look at (new
// auto-disables, off-allowlist apply links, suspected blocks), or "" when
// there are none.
func Digest(s Summary) string {
	var lines []string
	if len(s.AutoDisabled) > 0 {
		lines = append(lines, "**Auto-disabled** after "+strconv.Itoa(ConsecutiveFailureLimit)+
			" failed runs in a row (retried every "+strconv.Itoa(DisabledRecheckDays)+
			" days). Fix the board token, set `active: false` in YAML, or close this as confirmed:")
		for _, slug := range s.AutoDisabled {
			lines = append(lines, "- `"+slug+"`")
		}
		lines = append(lines, "")
	}
	if len(s.ApplyHostViolations) > 0 {
		lines = append(lines, "**Apply links off the allowlist** (`data/apply-hosts.tsv`). Verify the employer owns each before adding a row:")
		for _, v := range s.ApplyHostViolations {
			host := "?"
			if v.Host != nil {
				host = *v.Host
			}
			lines = append(lines, "- `"+v.Company+"`: "+v.Reason+" `"+host+"` e.g. "+v.Example)
		}
		lines = append(lines, "")
	}
	if p := s.ApplyProbe; p != nil && len(p.HeldCompanies) > 0 {
		lines = append(lines, "**Every apply link dead** for these companies (at least "+strconv.Itoa(SuspectedBlockMinJobs)+
			" listed jobs, every link failing, at least one "+strconv.Itoa(applycheck.StrikesToRetire)+" days in a row) while their board still lists them."+
			" Jobs stay up until a person looks: fix the apply links (a moved careers site?), set `active: false` in YAML,"+
			" or approve `company:<slug>` in `data/expected-delta.json` to let them retire:")
		for _, slug := range p.HeldCompanies {
			lines = append(lines, "- `"+slug+"`")
		}
		lines = append(lines, "")
	}
	if len(s.SuspectedBlocks) > 0 {
		lines = append(lines, "**Suspected blocks**: had at least "+strconv.Itoa(SuspectedBlockMinJobs)+
			" active jobs, board now empty. Jobs are kept until "+strconv.FormatInt(proven.RetireAfterEmptyRuns, 10)+
			" empty runs in a row:")
		for _, slug := range s.SuspectedBlocks {
			lines = append(lines, "- `"+slug+"`")
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
