// Package ats reads job boards: the public ATS APIs (Greenhouse, Lever,
// Ashby, Workable, SmartRecruiters, Recruitee, Rippling, BambooHR, Workday),
// the proprietary careers sites, and committed manual captures. Every request
// goes through internal/fetch.
package ats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
	"github.com/dnexdev/openintern/internal/model"
	"github.com/dnexdev/openintern/internal/state/jsjson"
)

// Proprietary lists the careers adapters that are not Greenhouse-style
// public boards, in the TypeScript PROPRIETARY_ATS order.
var Proprietary = []string{
	"citadel",
	"citadel_securities",
	"tesla",
	"bytedance",
	"tiktok",
	"phenom",
	"avature",
	"icims",
	"eightfold",
	"google",
	"ibm",
	"deshaw",
	"shopify",
	"x",
}

// IsProprietary reports whether name is one of Proprietary.
func IsProprietary(name string) bool {
	for _, p := range Proprietary {
		if p == name {
			return true
		}
	}
	return false
}

// Fetch reads one board. An error means the board was not read: HTTP and
// manual-capture failures are a *fetch.Failure (use errors.As; the
// manual-expired kind retires the company's jobs), and anything else
// (unsupported ATS, bad token, a page that parsed to nothing where that is
// impossible) is a plain error. A nil error with no jobs is a board that was
// read and is empty.
func Fetch(ctx context.Context, atsName, boardToken string) ([]model.NormalizedJob, error) {
	var jobs []model.NormalizedJob
	var err error
	switch atsName {
	case "greenhouse":
		jobs, err = fetchGreenhouse(ctx, boardToken)
	case "lever":
		jobs, err = fetchLever(ctx, boardToken)
	case "ashby":
		jobs, err = fetchAshby(ctx, boardToken)
	case "workable":
		jobs, err = fetchWorkable(ctx, boardToken)
	case "smartrecruiters":
		jobs, err = fetchSmartRecruiters(ctx, boardToken)
	case "recruitee":
		jobs, err = fetchRecruitee(ctx, boardToken)
	case "rippling":
		jobs, err = fetchRippling(ctx, boardToken)
	case "bamboohr":
		jobs, err = fetchBambooHR(ctx, boardToken)
	case "workday":
		jobs, err = fetchWorkday(ctx, boardToken)
	case "citadel":
		jobs, err = LoadManualJobs("citadel", ManualOptions{})
	case "citadel_securities":
		jobs, err = LoadManualJobs("citadel_securities", ManualOptions{})
	case "tesla":
		jobs, err = LoadManualJobs("tesla", ManualOptions{})
	case "bytedance":
		jobs, err = fetchBytedancePortal(ctx, BytedancePortal, boardToken)
	case "tiktok":
		jobs, err = fetchBytedancePortal(ctx, TiktokPortal, boardToken)
	case "phenom":
		jobs, err = fetchPhenom(ctx, boardToken)
	case "avature":
		jobs, err = fetchAvature(ctx, boardToken)
	case "icims":
		jobs, err = fetchIcims(ctx, boardToken)
	case "eightfold":
		jobs, err = fetchEightfold(ctx, boardToken)
	case "google":
		jobs, err = fetchGoogle(ctx, boardToken)
	case "ibm":
		jobs, err = fetchIBM(ctx, boardToken)
	case "deshaw":
		jobs, err = fetchDeshaw(ctx, boardToken)
	case "shopify":
		jobs, err = fetchShopify(ctx, boardToken)
	case "x":
		// No public board; X's terms forbid scraping. Dropped 2026-10-24.
		return nil, fmt.Errorf("X is not ingested")
	default:
		return nil, fmt.Errorf("Unsupported ATS: %s", atsName)
	}
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		if jobs[i].Locations == nil {
			jobs[i].Locations = []string{}
		}
	}
	if jobs == nil {
		jobs = []model.NormalizedJob{}
	}
	return jobs, nil
}

// MapConcurrent is mapWithConcurrency: fn runs on every item with at most
// limit calls in flight, and results keep the input order. The first error
// (in input order) is returned after all calls finish; the adapters catch
// their own per-item errors, as the TypeScript did.
func MapConcurrent[T, R any](ctx context.Context, items []T, limit int, fn func(context.Context, T) (R, error)) ([]R, error) {
	out := make([]R, len(items))
	errs := make([]error, len(items))
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range items {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			out[i], errs[i] = fn(ctx, items[i])
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// getJSON is httpJson(url) with default options.
func getJSON(ctx context.Context, rawURL string, opts fetch.Options) (any, error) {
	body, err := fetch.JSONBody(ctx, rawURL, opts)
	if err != nil {
		return nil, err
	}
	v, err := parseJSONBody(body)
	if err != nil {
		return nil, fetch.InvalidJSON(rawURL, err)
	}
	return v, nil
}

// postJSON is httpJson(url, { method: "POST", body }). The body is written
// by jsjson so keys keep JSON.stringify's order.
func postJSON(ctx context.Context, rawURL string, body jsjson.Object) (any, error) {
	return getJSON(ctx, rawURL, fetch.Options{Method: "POST", Body: string(jsjson.Marshal(body))})
}

// getText is httpText(url).
func getText(ctx context.Context, rawURL string, opts fetch.Options) (string, error) {
	body, err := fetch.TextBody(ctx, rawURL, opts)
	if err != nil {
		return "", err
	}
	return decodeBody(body), nil
}

type noPacingKey struct{}

// withoutPacing turns off the polite pauses between pages (fixture tests).
func withoutPacing(ctx context.Context) context.Context {
	return context.WithValue(ctx, noPacingKey{}, true)
}

// pause is the TypeScript sleep(ms) between pages.
func pause(ctx context.Context, d time.Duration) error {
	if off, _ := ctx.Value(noPacingKey{}).(bool); off {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
