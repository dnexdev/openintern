package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/classify"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state"
)

// import-dump seeds a state directory from a public jobs.json when no
// database export is available. The dump has job ids and dates but no
// external job ids, which ingest needs for fingerprints, so each company's
// board is fetched once and dump jobs are matched to live postings by apply
// URL. Unmatched dump jobs are kept as inactive so their ids stay reserved.
func init() {
	register("import-dump", "seed a state dir from a public jobs.json (run sync-companies first)", runImportDump)
}

func runImportDump(args []string) error {
	fs := flag.NewFlagSet("import-dump", flag.ContinueOnError)
	conc := fs.Int("concurrency", 6, "boards fetched at once")
	timeout := fs.Duration("timeout", 3*time.Minute, "per-board fetch timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: openintern import-dump <jobs.json> <state-dir>")
	}
	raw, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	var dump model.JobsDump
	if err := json.Unmarshal(raw, &dump); err != nil {
		return fmt.Errorf("parse dump: %w", err)
	}
	st, err := state.Load(fs.Arg(1))
	if err != nil {
		return err
	}
	if len(st.Companies) == 0 {
		return fmt.Errorf("no companies in state; run `openintern sync-companies %s` first", fs.Arg(1))
	}
	bySlug := map[string]*state.Company{}
	for _, c := range st.Companies {
		bySlug[c.Slug] = c
	}

	need := map[string]bool{}
	for _, j := range dump.Jobs {
		if bySlug[j.Company.Slug] == nil {
			continue
		}
		if prev, ok := st.Jobs[j.ID]; ok && !strings.HasPrefix(prev.ExternalJobID, "dump:") {
			continue // matched on an earlier run
		}
		need[j.Company.Slug] = true
	}
	// apply URL -> external id, per company
	ext := map[string]map[string]string{}
	var mu sync.Mutex
	sem := make(chan struct{}, max(1, *conc))
	var wg sync.WaitGroup
	for slug := range need {
		c := bySlug[slug]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()
			jobs, err := ats.Fetch(ctx, c.ATS, c.BoardToken)
			m := map[string]string{}
			for _, j := range jobs {
				m[urlKey(j.ApplyURL)] = j.ExternalID
			}
			mu.Lock()
			ext[slug] = m
			mu.Unlock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "fetch %s (%s): %v\n", slug, c.ATS, err)
			}
		}()
	}
	wg.Wait()

	var matched, inactive, skipped int
	for _, d := range dump.Jobs {
		c := bySlug[d.Company.Slug]
		if c == nil {
			skipped++
			continue
		}
		if prev, ok := st.Jobs[d.ID]; ok && !strings.HasPrefix(prev.ExternalJobID, "dump:") {
			continue // matched on an earlier run
		}
		externalID, ok := ext[d.Company.Slug][urlKey(d.ApplyURL)]
		if ok {
			matched++
		} else {
			// Never matches a live posting; the id stays reserved.
			externalID = "dump:" + d.ID
			inactive++
		}
		norm := d.NormalizedTitle
		if norm == "" {
			norm = classify.NormalizeTitle(d.Title)
		}
		family := d.RoleFamilyID
		if family == "" {
			family = classify.RoleFamilyId(c.Slug, norm)
		}
		first, err1 := pgTime(d.FirstSeenAt)
		last, err2 := pgTime(d.LastSeenAt)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("job %s: bad timestamp", d.ID)
		}
		var posted *string
		if d.PostedAt != nil {
			if p, err := pgTime(*d.PostedAt); err == nil {
				posted = &p
			}
		}
		j := &state.Job{Job: model.Job{
			ID:              d.ID,
			CompanyID:       c.ID,
			ExternalJobID:   externalID,
			Title:           d.Title,
			Locations:       orEmpty(d.Locations),
			ApplyURL:        d.ApplyURL,
			Excerpt:         d.Excerpt,
			IsRemote:        d.IsRemote,
			IsActive:        ok,
			Source:          d.Source,
			PostedAt:        posted,
			FirstSeenAt:     first,
			LastSeenAt:      last,
			CreatedAt:       first,
			UpdatedAt:       last,
			Terms:           orEmpty(d.Terms),
			CohortYear:      d.CohortYear,
			DurationMonths:  orEmptyInts(d.DurationMonths),
			Roles:           orEmpty(d.Roles),
			Regions:         orEmpty(d.Regions),
			TermYears:       orEmptyTY(d.TermYears),
			Fingerprint:     classify.JobFingerprint(c.ID, c.ATS, externalID),
			NormalizedTitle: norm,
			RoleFamilyID:    family,
		}}
		st.Jobs[j.ID] = j
	}
	if problems := state.Validate(st); len(problems) > 0 {
		return fmt.Errorf("refusing to write state:\n  %s", strings.Join(problems, "\n  "))
	}
	if err := state.Write(fs.Arg(1), st); err != nil {
		return err
	}
	fmt.Printf("dump %s: %d jobs matched to live boards (active), %d kept inactive, %d skipped (company not in registry)\n",
		dump.GeneratedAt, matched, inactive, skipped)
	return nil
}

func urlKey(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return strings.TrimSuffix(u.String(), "/")
}

func pgTime(iso string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return "", err
	}
	return state.PgTimestamp(t.UnixMilli()), nil
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func orEmptyInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

func orEmptyTY(s []model.TermYear) []model.TermYear {
	if s == nil {
		return []model.TermYear{}
	}
	return s
}
