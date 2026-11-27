package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dnexdev/openintern/data"
	"github.com/dnexdev/openintern/internal/applycheck"
	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/dump"
	"github.com/dnexdev/openintern/internal/ingest"
	"github.com/dnexdev/openintern/internal/registry"
	"github.com/dnexdev/openintern/internal/state"
)

// boardFetcher reads one board for ingest; tests may swap it.
var boardFetcher ingest.Fetcher = ats.Fetch

// shadowResults reports the honest-request shadow tests the fetches ran
// (OPENINTERN_SHADOW_HONEST=1).
func shadowResults() []ingest.ShadowResult {
	var out []ingest.ShadowResult
	for _, r := range ats.ShadowResults() {
		res := ingest.ShadowResult{Source: r.Source, OK: r.OK, Status: r.Status, Count: r.Count, LegacyCount: r.LegacyCount}
		if r.Error != "" {
			e := r.Error
			res.Error = &e
		}
		out = append(out, res)
	}
	return out
}

func init() {
	register("ingest", "fetch every active board into a state directory (ingest <state-dir>)", runIngest)
	register("ingest-state", "same as ingest", runIngest)
	register("dump", "write the public dump from a state directory (dump <state-dir> <out-dir>)", runDump)
	register("dump-state", "same as dump", runDump)
	register("sync-companies", "sync data/companies into a state directory (sync-companies <state-dir>)", runSyncCompanies)
	register("import-state", "seed a state directory from an export-state.sh export (import-state <export-dir> <state-dir>)", runImportState)
	register("state-from-export", "same as import-state", runImportState)
	register("state-gate", "check a candidate state before publishing (state-gate <current> <candidate>)", runStateGate)
	register("dump-gate", "check a dump before publishing (dump-gate <new-dir> [<previous-dir>])", runDumpGate)
	register("validate-companies", "validate data/companies/*.yaml", runValidateCompanies)
	register("validate-curated", "validate data/curated/tier-1.yaml", runValidateCurated)
	register("apply-hosts", "print the effective apply-host allowlist as JSON", runApplyHosts)
	register("job-aliases", "append job aliases for merged companies to data/job-aliases.tsv (job-aliases <state-dir>)", runJobAliases)
}

// dataFlag adds -data. Empty means the registry compiled into the binary
// (the commit it was built from); a directory reads data/ from disk.
func dataFlag(flags *flag.FlagSet, def string) *string {
	return flags.String("data", def, "registry directory (data/); empty uses the copy built into the binary")
}

func dataFS(dir string) fs.FS {
	if dir == "" {
		return data.FS
	}
	return os.DirFS(dir)
}

func loadRegistry(dir string) (*registry.Registry, error) {
	if dir == "" {
		return registry.Default(), nil
	}
	return registry.Load(os.DirFS(dir))
}

// parse parses flags and checks the positional count; a wrong count prints
// usage and exits 2, as the scripts did.
func parse(flags *flag.FlagSet, args []string, usage string, min, max int) ([]string, error) {
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: openintern %s %s\n", flags.Name(), usage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if n := flags.NArg(); n < min || n > max {
		flags.Usage()
		os.Exit(2)
	}
	return flags.Args(), nil
}

// approvals reads data/expected-delta.json for the gates; today is UTC.
func approvals(dir string) (func(string) bool, error) {
	expected, err := registry.LoadExpectedDelta(dataFS(dir))
	if err != nil {
		return nil, err
	}
	today := time.Now().UTC().Format("2006-01-02")
	return func(scope string) bool { return expected.Approved(scope, today) }, nil
}

func runIngest(args []string) error {
	flags := flag.NewFlagSet("ingest", flag.ContinueOnError)
	dataDir := dataFlag(flags, "")
	pos, err := parse(flags, args, "[-data dir] <state-dir>", 1, 1)
	if err != nil {
		return err
	}
	if boardFetcher == nil {
		return fmt.Errorf("this build has no board fetcher")
	}
	reg, err := loadRegistry(*dataDir)
	if err != nil {
		return err
	}
	dir := pos[0]
	st, err := state.Load(dir)
	if err != nil {
		return err
	}
	opts := ingest.Options{
		Fetch:             boardFetcher,
		Registry:          reg,
		Only:              ingest.OnlySlugs(os.Getenv("OPENINTERN_ONLY_SLUGS")),
		EnforceApplyHosts: registry.ApplyHostModeFromEnv() == registry.ApplyHostsEnforce,
		Shadow:            shadowResults,
	}
	// OPENINTERN_APPLY_PROBE=1 checks apply links and retires jobs whose link
	// stays dead (internal/applycheck); the Ingest workflow sets it.
	if strings.TrimSpace(os.Getenv("OPENINTERN_APPLY_PROBE")) == "1" {
		approved, err := approvals(*dataDir)
		if err != nil {
			return err
		}
		opts.ApplyProbe = &ingest.ApplyProbeOptions{Probe: applycheck.Probe, Approved: approved}
	}
	summary, err := ingest.Run(context.Background(), st, opts)
	if err != nil {
		return err
	}
	if err := state.Write(dir, st); err != nil {
		return err
	}
	fmt.Println(string(summary.JSON()))
	// CI hands the digest to a separate job that may open issues.
	if path := strings.TrimSpace(os.Getenv("OPENINTERN_DIGEST_PATH")); path != "" {
		if digest := ingest.Digest(summary); digest != "" {
			if err := os.WriteFile(path, []byte(digest), 0o666); err != nil {
				return err
			}
		}
	}
	if path := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY")); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
		if err != nil {
			return err
		}
		_, werr := f.WriteString(ingest.StepSummary(summary))
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return werr
		}
	}
	if summary.Failed() {
		os.Exit(1)
	}
	return nil
}

func runDump(args []string) error {
	flags := flag.NewFlagSet("dump", flag.ContinueOnError)
	dataDir := dataFlag(flags, "")
	pos, err := parse(flags, args, "[-data dir] <state-dir> <out-dir>", 2, 2)
	if err != nil {
		return err
	}
	opts := dump.Options{}
	if *dataDir != "" {
		reg, err := loadRegistry(*dataDir)
		if err != nil {
			return err
		}
		mode := registry.ApplyHostModeFromEnv()
		opts.ApplyURLAllowed = func(slug, url string) bool { return reg.ApplyURLAllowed(slug, url, mode) }
	}
	st, err := state.Load(pos[0])
	if err != nil {
		return err
	}
	res, err := dump.FromState(st, pos[1], time.Now(), opts)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %d jobs to %s\n", res.Count, pos[1])
	return nil
}

func runSyncCompanies(args []string) error {
	flags := flag.NewFlagSet("sync-companies", flag.ContinueOnError)
	dataDir := dataFlag(flags, "data")
	pos, err := parse(flags, args, "[-data dir] <state-dir>", 1, 1)
	if err != nil {
		return err
	}
	reg, err := loadRegistry(*dataDir)
	if err != nil {
		return err
	}
	st, err := state.Load(pos[0])
	if err != nil {
		return err
	}
	orphans := ingest.SyncRegistry(st, reg, time.Now(), state.NewID)
	if err := state.Write(pos[0], st); err != nil {
		return err
	}
	fmt.Printf("Synced %d companies from YAML.\n", len(reg.Unique()))
	if len(orphans) > 0 {
		fmt.Printf("Deactivated (no longer in the registry): %s\n", strings.Join(orphans, ", "))
	}
	return nil
}

func runImportState(args []string) error {
	flags := flag.NewFlagSet("import-state", flag.ContinueOnError)
	pos, err := parse(flags, args, "<export-dir> <state-dir>", 2, 2)
	if err != nil {
		return err
	}
	st, err := state.FromExport(pos[0])
	if err != nil {
		return err
	}
	if err := state.Write(pos[1], st); err != nil {
		return err
	}
	fmt.Printf("state: %d companies, %d jobs, %d runs → %s\n", len(st.Companies), len(st.Jobs), len(st.Runs), pos[1])
	return nil
}

func report(stdout, stderr string, code int) error {
	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func runStateGate(args []string) error {
	flags := flag.NewFlagSet("state-gate", flag.ContinueOnError)
	dataDir := dataFlag(flags, "")
	pos, err := parse(flags, args, "[-data dir] <current-state-dir> <candidate-state-dir>", 2, 2)
	if err != nil {
		return err
	}
	approved, err := approvals(*dataDir)
	if err != nil {
		return err
	}
	res, err := state.Gate(pos[0], pos[1], state.GateOptions{Approved: approved})
	if err != nil {
		return err
	}
	return report(res.Report())
}

func runDumpGate(args []string) error {
	flags := flag.NewFlagSet("dump-gate", flag.ContinueOnError)
	dataDir := dataFlag(flags, "")
	pos, err := parse(flags, args, "[-data dir] <new-dir> [<previous-dir>]", 1, 2)
	if err != nil {
		return err
	}
	approved, err := approvals(*dataDir)
	if err != nil {
		return err
	}
	reg, err := loadRegistry(*dataDir)
	if err != nil {
		return err
	}
	prev := ""
	if len(pos) == 2 {
		prev = pos[1]
	}
	res, err := dump.Gate(pos[0], prev, dump.GateOptions{
		Table:    reg.ApplyHosts(),
		Enforce:  registry.ApplyHostModeFromEnv() == registry.ApplyHostsEnforce,
		Approved: approved,
	})
	if err != nil {
		return err
	}
	return report(res.Report())
}

func runValidateCompanies(args []string) error {
	flags := flag.NewFlagSet("validate-companies", flag.ContinueOnError)
	dataDir := dataFlag(flags, "data")
	if _, err := parse(flags, args, "[-data dir]", 0, 0); err != nil {
		return err
	}
	res, err := registry.ValidateCompanies(dataFS(*dataDir))
	if err != nil {
		return err
	}
	missing := 0
	for _, w := range res.Warnings {
		if strings.HasPrefix(w, "missing website_url") {
			missing++
			continue
		}
		fmt.Fprintf(os.Stderr, "warn: %s\n", w)
	}
	if missing > 0 {
		fmt.Fprintf(os.Stderr, "warn: %d companies are missing website_url (logos may use fallbacks)\n", missing)
	}
	for _, e := range res.Errors {
		fmt.Fprintf(os.Stderr, "error: %s\n", e)
	}
	if len(res.Errors) > 0 {
		os.Exit(1)
	}
	fmt.Printf("Company registry OK (%d entries).\n", res.Count)
	return nil
}

func runValidateCurated(args []string) error {
	flags := flag.NewFlagSet("validate-curated", flag.ContinueOnError)
	dataDir := dataFlag(flags, "data")
	if _, err := parse(flags, args, "[-data dir]", 0, 0); err != nil {
		return err
	}
	res, err := registry.ValidateTier1(dataFS(*dataDir))
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(os.Stderr, "warn: %s\n", w)
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		os.Exit(1)
	}
	fmt.Printf("Tier 1 curated list OK (%d slugs).\n", res.Count)
	return nil
}

func runApplyHosts(args []string) error {
	flags := flag.NewFlagSet("apply-hosts", flag.ContinueOnError)
	dataDir := dataFlag(flags, "data")
	if _, err := parse(flags, args, "[-data dir]", 0, 0); err != nil {
		return err
	}
	reg, err := loadRegistry(*dataDir)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(registry.TableJSON(reg.ApplyHosts()))
	return err
}

// runJobAliases is job-aliases.ts over flat-file state: after companies
// merge (data/company-aliases.tsv) and ingest has run, match each old
// company's jobs to the same postings under the surviving company and append
// them to data/job-aliases.tsv. Old job links then redirect and applied
// marks move over. Re-running is safe: existing pairs are kept.
func runJobAliases(args []string) error {
	flags := flag.NewFlagSet("job-aliases", flag.ContinueOnError)
	dataDir := flags.String("data", "data", "registry directory whose job-aliases.tsv is appended to")
	pos, err := parse(flags, args, "[-data dir] <state-dir>", 1, 1)
	if err != nil {
		return err
	}
	st, err := state.Load(pos[0])
	if err != nil {
		return err
	}
	aliasText, err := os.ReadFile(filepath.Join(*dataDir, registry.CompanyAliasesPath))
	if err != nil {
		return err
	}
	file := filepath.Join(*dataDir, registry.JobAliasesPath)
	existing, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, l := range strings.Split(string(existing), "\n") {
		known[strings.SplitN(l, "\t", 2)[0]] = true
	}
	var fresh [][2]string
	for _, pair := range FindJobAliases(st, parseCompanyAliases(string(aliasText))) {
		if !known[pair[0]] {
			fresh = append(fresh, pair)
		}
	}
	if len(fresh) > 0 {
		var b strings.Builder
		for _, p := range fresh {
			b.WriteString(p[0] + "\t" + p[1] + "\n")
		}
		f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o666)
		if err != nil {
			return err
		}
		_, werr := f.WriteString(b.String())
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return werr
		}
	}
	fmt.Printf("job-aliases: %d new pair(s); commit %s\n", len(fresh), file)
	return nil
}

// parseCompanyAliases reads company-aliases.tsv rows as [old, new] pairs,
// skipping blanks, comments and the header.
func parseCompanyAliases(text string) [][2]string {
	var out [][2]string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "old_slug\t") {
			continue
		}
		cols := strings.Split(l, "\t")
		pair := [2]string{cols[0], ""}
		if len(cols) > 1 {
			pair[1] = cols[1]
		}
		out = append(out, pair)
	}
	return out
}

// FindJobAliases pairs each job of an old (merged) company with the job of
// the surviving company that has the same external id, old jobs in id order.
func FindJobAliases(st *state.State, companyAliases [][2]string) [][2]string {
	bySlug := map[string]*state.Company{}
	for _, c := range st.Companies {
		bySlug[c.Slug] = c
	}
	jobsOf := map[string][]*state.Job{}
	for _, j := range st.Jobs {
		jobsOf[j.CompanyID] = append(jobsOf[j.CompanyID], j)
	}
	for _, list := range jobsOf {
		sort.Slice(list, func(a, b int) bool { return state.ByCodeUnit(list[a].ID, list[b].ID) < 0 })
	}
	var pairs [][2]string
	for _, alias := range companyAliases {
		oldCo, newCo := bySlug[alias[0]], bySlug[alias[1]]
		if oldCo == nil || newCo == nil {
			continue
		}
		byExt := map[string]string{}
		for _, j := range jobsOf[newCo.ID] {
			byExt[j.ExternalJobID] = j.ID
		}
		for _, j := range jobsOf[oldCo.ID] {
			if target, ok := byExt[j.ExternalJobID]; ok && target != j.ID {
				pairs = append(pairs, [2]string{j.ID, target})
			}
		}
	}
	return pairs
}
