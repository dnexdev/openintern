package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
)

const CheckApplyUsage = `usage: openintern check-apply <jobs.json>

Probe each unique apply_url in a dump with the honest client (HEAD, then GET
if HEAD is not allowed). Prints dead (404/410), refused (401/403), and other
failures. Exit 1 when any URL is dead. About two requests per second.
`

type dumpJobApply struct {
	ID       string `json:"id"`
	ApplyURL string `json:"apply_url"`
	Company  struct {
		Slug string `json:"slug"`
	} `json:"company"`
	Title string `json:"title"`
}

// CheckApply probes apply URLs from a dump file. Returns process exit code.
func CheckApply(ctx context.Context, args []string) (int, error) {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, CheckApplyUsage)
		return 2, nil
	}
	raw, err := os.ReadFile(args[0])
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

	var dead, refused, other int
	for i, url := range urls {
		if i > 0 {
			if err := sleep(ctx, 500*time.Millisecond); err != nil {
				return 1, err
			}
		}
		label, status := probeApply(ctx, url)
		s := byURL[url]
		line := fmt.Sprintf("%s\t%s\t%s\t%s\tHTTP %d", s.slug, s.id, s.title, url, status)
		switch label {
		case "dead":
			dead++
			fmt.Fprintln(os.Stdout, "dead\t"+line)
		case "refused":
			refused++
			fmt.Fprintln(os.Stdout, "refused\t"+line)
		case "ok":
			// quiet
		default:
			other++
			fmt.Fprintln(os.Stdout, "fail\t"+line+"\t"+label)
		}
	}
	fmt.Fprintf(os.Stderr, "check-apply: %d URLs, %d dead, %d refused, %d other failures\n",
		len(urls), dead, refused, other)
	if dead > 0 {
		return 1, nil
	}
	return 0, nil
}

func probeApply(ctx context.Context, rawURL string) (label string, status int) {
	res, err := fetch.Request(ctx, rawURL, fetch.Options{Method: "HEAD", Attempts: 1, Timeout: 15 * time.Second})
	if err == nil {
		return "ok", res.Status
	}
	var f *fetch.Failure
	if !errors.As(err, &f) {
		return "network", 0
	}
	if f.Kind == fetch.KindHTTP && (f.Status == 405 || f.Status == 501) {
		res, err = fetch.Request(ctx, rawURL, fetch.Options{
			Method: "GET", Attempts: 1, Timeout: 15 * time.Second,
			Headers: map[string]string{"Range": "bytes=0-0"},
		})
		if err == nil {
			return "ok", res.Status
		}
		if !errors.As(err, &f) {
			return "network", 0
		}
	}
	switch f.Kind {
	case fetch.KindNotFound:
		return "dead", f.Status
	case fetch.KindRefused:
		return "refused", f.Status
	default:
		return string(f.Kind), f.Status
	}
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
