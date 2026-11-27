package logos

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
)

// maxCandidates bounds how many declared icon URLs one site costs (the two
// conventional-path guesses come on top).
const maxCandidates = 6

// Options tunes Fetch.
type Options struct {
	// Fallback asks favicon services (Google, then DuckDuckGo) for the site's
	// icon when the site itself gives none or only a tiny one: many company
	// sites refuse a non-browser User-Agent. This runs once, on a
	// maintainer's machine; visitors never reach these services.
	Fallback bool
}

// Result is a fetched, normalized logo.
type Result struct {
	PNG    []byte
	Source string // the icon URL it came from
	Edge   int    // smaller side of the source icon, in pixels
}

// Fetch reads a company homepage, tries the icons it declares (and
// /favicon.ico), and returns the sharpest one that decodes and is not blank.
// Requests go through internal/fetch, so they carry OpenIntern's honest
// User-Agent.
func Fetch(ctx context.Context, homepage string, opts Options) (*Result, error) {
	var candidates []Candidate
	page, err := fetch.Request(ctx, homepage, fetch.Options{
		Attempts: 2,
		Timeout:  20 * time.Second,
		Headers:  map[string]string{"Accept": "text/html,application/xhtml+xml"},
	})
	if err == nil && genericHost(page.URL) {
		return nil, fmt.Errorf("logos: %s lands on %s, a blog or parking host, not the company's own site", homepage, page.URL)
	}
	if err == nil && parkedPage.Match(page.Body) {
		return nil, fmt.Errorf("logos: %s is a parked domain (redirects to /lander)", homepage)
	}
	if err == nil {
		candidates = siteCandidates(ctx, page.Body, page.URL)
	} else {
		u, perr := url.Parse(homepage)
		if perr != nil {
			return nil, perr
		}
		candidates = guesses(u, nil, false)
	}
	var errs []string
	if err != nil {
		errs = append(errs, "homepage: "+err.Error())
	}
	best, errs, cerr := tryCandidates(ctx, candidates, errs, Size)
	if cerr != nil {
		return nil, cerr
	}
	if opts.Fallback && (best == nil || best.Edge < fallbackBelow) {
		var alt *Result
		alt, errs, cerr = tryCandidates(ctx, ServiceCandidates(homepage), errs, 1)
		if cerr != nil {
			return nil, cerr
		}
		if alt != nil && (best == nil || alt.Edge > best.Edge) {
			best = alt
		}
	}
	if best == nil {
		if len(errs) == 0 {
			return nil, errors.New("logos: no icon candidates")
		}
		return nil, fmt.Errorf("logos: no usable icon (%s)", strings.Join(errs, "; "))
	}
	return best, nil
}

// siteCandidates are the icons a fetched homepage declares, merged with its
// web app manifest's icons, best first and capped at maxCandidates, then the
// conventional-path guesses.
func siteCandidates(ctx context.Context, page []byte, pageURL string) []Candidate {
	all, manifest := pageIcons(page, pageURL)
	var declared, guessed []Candidate
	for _, c := range all {
		if c.Rel == "guess" {
			guessed = append(guessed, c)
		} else {
			declared = append(declared, c)
		}
	}
	if manifest != "" && !genericHost(manifest) {
		res, err := fetch.Request(ctx, manifest, fetch.Options{
			Attempts: 1,
			Timeout:  20 * time.Second,
			Headers:  map[string]string{"Accept": "application/manifest+json,application/json"},
		})
		if err == nil {
			seen := map[string]bool{}
			for _, c := range declared {
				seen[c.URL] = true
			}
			for _, c := range ManifestIcons(res.Body, res.URL) {
				if !seen[c.URL] {
					declared = append(declared, c)
				}
			}
			sortCandidates(declared)
		}
	}
	if len(declared) > maxCandidates {
		declared = declared[:maxCandidates]
	}
	kept := map[string]bool{}
	for _, c := range declared {
		kept[c.URL] = true
	}
	for _, c := range guessed {
		if !kept[c.URL] {
			declared = append(declared, c)
		}
	}
	return declared
}

// generic matches hosts whose icon is the platform's, not the company's: a
// website_url that redirects to a free blog or a parked domain.
var generic = regexp.MustCompile(`(?i)(?:^|\.)(?:wordpress\.com|wp\.com|blogspot\.com|medium\.com|hugedomains\.com|godaddy\.com|sedo\.com|dan\.com|afternic\.com|parkingcrew\.net|bodis\.com)$`)

// parkedPage matches the stub a parked domain serves before its for-sale
// lander: the whole page is a script that jumps to /lander.
var parkedPage = regexp.MustCompile(`^\s*(?:<!DOCTYPE html>)?\s*<html>\s*<head>\s*<script>\s*window\.onload\s*=\s*function\s*\(\)\s*\{\s*window\.location\.href\s*=\s*"/lander"`)

func genericHost(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && generic.MatchString(u.Hostname())
}

// fallbackBelow: a site icon smaller than this is worth a second opinion from
// the favicon services.
const fallbackBelow = 32

// ServiceCandidates are the favicon-service URLs for a homepage's domain.
func ServiceCandidates(homepage string) []Candidate {
	u, err := url.Parse(homepage)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	q := url.QueryEscape(host)
	return []Candidate{
		{URL: "https://www.google.com/s2/favicons?domain=" + q + "&sz=128", Rel: "service"},
		{URL: "https://icons.duckduckgo.com/ip3/" + host + ".ico", Rel: "service"},
	}
}

// tryCandidates fetches candidates in order and keeps the sharpest usable
// icon, stopping once one has an edge of at least enough. The error is only
// for a cancelled context; per-candidate failures are appended to errs.
func tryCandidates(ctx context.Context, candidates []Candidate, errs []string, enough int) (*Result, []string, error) {
	var best *Result
	for _, c := range candidates {
		if genericHost(c.URL) {
			errs = append(errs, c.URL+": platform icon, not the company's")
			continue
		}
		res, ierr := fetch.Request(ctx, c.URL, fetch.Options{
			Attempts: 1,
			Timeout:  20 * time.Second,
			Headers:  map[string]string{"Accept": "image/png,image/x-icon,image/*;q=0.8"},
		})
		if ierr != nil {
			if ctx.Err() != nil {
				return nil, errs, ctx.Err()
			}
			errs = append(errs, c.URL+": "+ierr.Error())
			continue
		}
		out, edge, perr := Prepare(res.Body)
		if perr != nil {
			errs = append(errs, c.URL+": "+perr.Error())
			continue
		}
		if best == nil || edge > best.Edge {
			best = &Result{PNG: out, Source: c.URL, Edge: edge}
		}
		if best.Edge >= enough {
			break
		}
	}
	return best, errs, nil
}
