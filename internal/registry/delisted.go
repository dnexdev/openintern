package registry

import "strings"

// Delisted is one listing pulled by hand.
type Delisted struct {
	ID     string // lowercase job UUID
	Reason string // issue and reason, for the audit trail
}

// delisted are listings pulled by hand: reported closed, wrong, or
// unsafe. They are hidden from the board, API, sitemap and dumps on the next
// deploy, even while ingest can't run. Remove an entry once ingest has
// retired the job. Order is the order of denylist.json.
var delisted = []Delisted{
	{"cbb76a8d-0726-4a2a-b84d-8f359be23a90", "#24 Khan Academy SWE Intern Summer 2027: closed (Greenhouse 404), 2026-10-24"},
}

// DelistedJobs returns the hand-delisted jobs in denylist.json order.
func DelistedJobs() []Delisted { return append([]Delisted(nil), delisted...) }

var delistedSet = func() map[string]bool {
	m := make(map[string]bool, len(delisted))
	for _, d := range delisted {
		m[d.ID] = true
	}
	return m
}()

// IsDelisted reports whether a job id was pulled by hand.
func IsDelisted(jobID string) bool { return delistedSet[strings.ToLower(jobID)] }
