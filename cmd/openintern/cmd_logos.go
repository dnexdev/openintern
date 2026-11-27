package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dnexdev/openintern/internal/logos"
	"github.com/dnexdev/openintern/internal/registry"
)

func init() {
	register("logos", "fetch company logos into data/logos (run by a maintainer, then commit the files)", runLogos)
}

// logoSourcesFile records where each committed logo came from.
const logoSourcesFile = "SOURCES.tsv"

func runLogos(args []string) error {
	flags := flag.NewFlagSet("logos", flag.ContinueOnError)
	dataDir := flags.String("data", "data", "registry directory; logos are written to <data>/logos")
	only := flags.String("only", "", "comma-separated company slugs (default: every active company)")
	dumpPath := flags.String("dump", "", "jobs.json: only companies with listings in this dump")
	force := flags.Bool("force", false, "refetch logos that already exist (manual ones are kept)")
	workers := flags.Int("workers", 8, "parallel sites")
	fallback := flags.Bool("fallback", true, "ask Google and DuckDuckGo favicon services when a site gives no usable icon (maintainer machine only; visitors never reach them)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	reg, err := registry.Load(os.DirFS(*dataDir))
	if err != nil {
		return err
	}
	outDir := filepath.Join(*dataDir, "logos")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	sources, err := readLogoSources(filepath.Join(outDir, logoSourcesFile))
	if err != nil {
		return err
	}

	want := map[string]bool{}
	for _, s := range strings.Split(*only, ",") {
		if s = strings.TrimSpace(s); s != "" {
			want[s] = true
		}
	}
	if *dumpPath != "" {
		onBoard, err := dumpCompanySlugs(*dumpPath, reg)
		if err != nil {
			return err
		}
		if len(want) == 0 {
			want = onBoard
		} else {
			for s := range want {
				if !onBoard[s] {
					delete(want, s)
				}
			}
		}
	}

	type job struct{ slug, homepage string }
	var jobs []job
	var noSite, kept []string
	for _, c := range reg.Unique() {
		if len(want) > 0 && !want[c.Slug] {
			continue
		}
		if len(want) == 0 && !c.Active {
			continue
		}
		if src, ok := sources[c.Slug]; ok && (!*force || src.source == "manual") {
			if _, err := os.Stat(filepath.Join(outDir, logos.FileName(c.Slug))); err == nil {
				kept = append(kept, c.Slug)
				continue
			}
		}
		home := logos.Homepage(c.WebsiteURL, c.CareersURL)
		if home == "" {
			noSite = append(noSite, c.Slug)
			continue
		}
		jobs = append(jobs, job{c.Slug, home})
	}

	type outcome struct {
		slug string
		res  *logos.Result
		err  error
	}
	ctx := context.Background()
	work := make(chan job)
	results := make(chan outcome)
	var wg sync.WaitGroup
	for range max(1, *workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				res, err := logos.Fetch(ctx, j.homepage, logos.Options{Fallback: *fallback})
				results <- outcome{j.slug, res, err}
			}
		}()
	}
	go func() {
		for _, j := range jobs {
			work <- j
		}
		close(work)
		wg.Wait()
		close(results)
	}()

	today := time.Now().UTC().Format("2006-01-02")
	var fetched, failed []string
	for o := range results {
		if o.err != nil {
			failed = append(failed, o.slug+": "+o.err.Error())
			continue
		}
		if err := os.WriteFile(filepath.Join(outDir, logos.FileName(o.slug)), o.res.PNG, 0o644); err != nil {
			return err
		}
		sources[o.slug] = logoSource{source: o.res.Source, fetched: today}
		fetched = append(fetched, fmt.Sprintf("%s (%dpx from %s)", o.slug, o.res.Edge, o.res.Source))
	}
	if err := writeLogoSources(filepath.Join(outDir, logoSourcesFile), sources); err != nil {
		return err
	}
	sort.Strings(fetched)
	sort.Strings(failed)
	sort.Strings(noSite)
	for _, f := range fetched {
		fmt.Println("fetched", f)
	}
	for _, f := range failed {
		fmt.Println("failed ", f)
	}
	for _, s := range noSite {
		fmt.Println("no site", s, "(add website_url to its registry entry)")
	}
	fmt.Printf("\n%d fetched, %d kept, %d failed, %d without a website_url\n", len(fetched), len(kept), len(failed), len(noSite))
	return nil
}

type logoSource struct{ source, fetched string }

func readLogoSources(path string) (map[string]logoSource, error) {
	out := map[string]logoSource{}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "slug\t") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("%s: bad line %q", path, line)
		}
		out[f[0]] = logoSource{source: f[1], fetched: f[2]}
	}
	return out, nil
}

func writeLogoSources(path string, sources map[string]logoSource) error {
	slugs := make([]string, 0, len(sources))
	for s := range sources {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	var b strings.Builder
	b.WriteString("slug\tsource\tfetched\n")
	for _, s := range slugs {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", s, sources[s].source, sources[s].fetched)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// dumpCompanySlugs is the set of canonical company slugs with listings in a
// jobs.json dump.
func dumpCompanySlugs(path string, reg *registry.Registry) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d struct {
		Jobs []struct {
			Company struct {
				Slug string `json:"slug"`
			} `json:"company"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := map[string]bool{}
	for _, j := range d.Jobs {
		out[reg.CanonicalCompanySlug(j.Company.Slug)] = true
	}
	return out, nil
}
