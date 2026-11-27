package tools

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/applycheck"
)

const CheckApplyUsage = `usage: openintern check-apply [-all] <jobs.json>

Probe each unique apply_url in a dump with the honest client, the same way
the ingest apply-link probe does (internal/applycheck): HEAD, then GET when
HEAD is not allowed or says 404/410. Prints dead links (404/410, or a
redirect that drops the posting's id, e.g. to a careers index) and
inconclusive ones (401/403, 429, 5xx, timeouts, redirects to a login page).
-all also prints the links that work, with where their redirects ended.
One request at a time, about two a second. Exit 1 when any link is dead.
`

type dumpJobApply struct {
	ID       string `json:"id"`
	ApplyURL string `json:"apply_url"`
	Company  struct {
		Slug string `json:"slug"`
	} `json:"company"`
	Title string `json:"title"`
}

// checkApplyProbe is the probe check-apply uses; tests replace it.
var checkApplyProbe = applycheck.Probe

// CheckApply probes apply URLs from a dump file. Returns process exit code.
func CheckApply(ctx context.Context, args []string) (int, error) {
	return checkApply(ctx, args, os.Stdout, os.Stderr, 500*time.Millisecond)
}

func checkApply(ctx context.Context, args []string, stdout, stderr io.Writer, pace time.Duration) (int, error) {
	flags := flag.NewFlagSet("check-apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	all := flags.Bool("all", false, "also print working links")
	flags.Usage = func() { fmt.Fprint(stderr, CheckApplyUsage) }
	if err := flags.Parse(args); err != nil {
		return 2, nil
	}
	if flags.NArg() != 1 {
		fmt.Fprint(stderr, CheckApplyUsage)
		return 2, nil
	}
	raw, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return 1, err
	}
	var dump struct {
		Jobs []dumpJobApply `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &dump); err != nil {
		return 1, fmt.Errorf("parse dump: %w", err)
	}

	type sample struct{ id, slug, title string }
	byURL := map[string]sample{}
	for _, j := range dump.Jobs {
		url := strings.TrimSpace(j.ApplyURL)
		if url == "" {
			continue
		}
		if _, ok := byURL[url]; !ok {
			byURL[url] = sample{j.ID, j.Company.Slug, j.Title}
		}
	}
	urls := make([]string, 0, len(byURL))
	for u := range byURL {
		urls = append(urls, u)
	}
	sort.Strings(urls)

	counts := map[applycheck.Verdict]int{}
	for i, url := range urls {
		if i > 0 {
			if err := sleep(ctx, pace); err != nil {
				return 1, err
			}
		}
		res := checkApplyProbe(ctx, url)
		counts[res.Verdict]++
		if res.Verdict == applycheck.Alive && !*all {
			continue
		}
		s := byURL[url]
		fmt.Fprintln(stdout, strings.Join([]string{string(res.Verdict), s.slug, s.id, s.title, url,
			"HTTP " + strconv.Itoa(res.Status), res.Reason, res.FinalURL}, "\t"))
	}
	fmt.Fprintf(stderr, "check-apply: %d URLs, %d dead, %d inconclusive, %d alive\n",
		len(urls), counts[applycheck.Dead], counts[applycheck.Inconclusive], counts[applycheck.Alive])
	if counts[applycheck.Dead] > 0 {
		return 1, nil
	}
	return 0, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
