package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dnexdev/openintern/internal/ats"
	"github.com/dnexdev/openintern/internal/registry"
)

// dumpGatedATS are proprietary boards that may refuse a CI runner without a
// manual capture or a browser.
var dumpGatedATS = map[string]bool{
	"citadel": true, "citadel_securities": true, "tesla": true, "x": true, "avature": true, "icims": true,
}

var dumpGateMessage = regexp.MustCompile(`(?i)blocked|HTTP 403|OPENINTERN_`)

func isDumpGateBlock(atsName, err string) bool {
	return dumpGatedATS[atsName] && dumpGateMessage.MatchString(err)
}

// resolveCompanyFile finds a file argument: absolute, relative to the
// working directory, or relative to the repository root (CI passes
// repository paths such as data/companies/tech-e.yaml).
func (e Env) resolveCompanyFile(file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	fromCwd := e.abs(file)
	if _, err := os.Stat(fromCwd); err == nil {
		return fromCwd
	}
	return filepath.Join(e.repoRoot(), file)
}

// repoRoot is the directory holding DataDir (DataDir is <repo>/data).
func (e Env) repoRoot() string { return filepath.Dir(e.abs(e.DataDir)) }

func parseCompanyFile(path string) ([]registry.CompanyYAML, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	list, _, err := registry.ParseCompaniesFile(path, src)
	return list, err
}

// boardKey identifies the board a company points at.
func boardKey(c registry.CompanyYAML) string {
	return c.ATS + "\x00" + c.BoardToken + "\x00" + strconv.FormatBool(c.Active)
}

// companiesChangedSince keeps the active companies whose board differs
// from revision rev. A PR that touches a company file re-checks every board
// in it otherwise; tokens that already fail on the base, and rate limits for
// boards the PR did not change, are not this PR's failure.
func (e Env) companiesChangedSince(ctx context.Context, files []string, rev string) ([]registry.CompanyYAML, error) {
	root := e.repoRoot()
	var changed []registry.CompanyYAML
	for _, f := range files {
		abs := e.resolveCompanyFile(f)
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			rel = abs
		}
		head, err := parseCompanyFile(abs)
		if err != nil {
			return nil, err
		}
		var base []registry.CompanyYAML
		if out, err := e.git(ctx, root, "show", rev+":"+filepath.ToSlash(rel)); err == nil {
			if list, _, err := registry.ParseCompaniesFile(rel, out); err == nil {
				base = list
			}
		}
		previous := map[string]string{}
		for _, c := range base {
			previous[c.Slug] = boardKey(c)
		}
		for _, c := range head {
			if !c.Active {
				continue
			}
			if prev, ok := previous[c.Slug]; !ok || prev != boardKey(c) {
				changed = append(changed, c)
			}
		}
	}
	return changed, nil
}

// ValidateTokens is validate-tokens.ts: probe the boards of the given
// company files (all files with --all or no arguments). On a pull request
// (GITHUB_BASE_REF set) only added or edited active boards are probed. It
// returns exit code 1 when a board fails.
func ValidateTokens(ctx context.Context, env Env, args []string) (int, error) {
	all := hasArg(args, "--all")
	var targets []string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			targets = append(targets, a)
		}
	}
	if all || len(targets) == 0 {
		dir := env.abs(env.companiesDir())
		entries, err := os.ReadDir(dir)
		if err != nil {
			return 1, err
		}
		targets = targets[:0]
		for _, en := range entries {
			if n := en.Name(); strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml") {
				targets = append(targets, filepath.Join(dir, n))
			}
		}
	}
	w, ew := env.Stdout, env.Stderr
	if len(targets) == 0 {
		fmt.Fprintln(w, "No company YAML files to validate.")
		return 0, nil
	}

	baseRef := env.getenv("GITHUB_BASE_REF")
	var companies []registry.CompanyYAML
	if baseRef != "" {
		var err error
		if companies, err = env.companiesChangedSince(ctx, targets, "origin/"+baseRef); err != nil {
			return 1, err
		}
	} else {
		for _, f := range targets {
			list, err := parseCompanyFile(env.resolveCompanyFile(f))
			if err != nil {
				return 1, err
			}
			for _, c := range list {
				if c.Active {
					companies = append(companies, c)
				}
			}
		}
	}
	if len(companies) == 0 {
		if baseRef != "" {
			fmt.Fprintln(w, "No added or edited company tokens to validate.")
		} else {
			fmt.Fprintln(w, "No active companies to validate.")
		}
		return 0, nil
	}

	ci := env.getenv("CI")
	inCI := ci == "true" || ci == "1"
	type result struct{ slug, ats, err string }
	var failures, skipped []string
	inChunks(ctx, companies, 10, func(ctx context.Context, c registry.CompanyYAML) result {
		r := ats.ProbeATSBoard(ctx, c.ATS, c.BoardToken)
		res := result{slug: c.Slug, ats: c.ATS}
		if !r.OK {
			res.err = r.Error
			if res.err == "" {
				res.err = fmt.Sprintf("HTTP %d", r.Status)
			}
		}
		return res
	}, func(rs []result) {
		for _, r := range rs {
			if r.err == "" {
				fmt.Fprintf(w, "ok %s\n", r.slug)
				continue
			}
			// CI runners often cannot reach Akamai-gated careers sites.
			if inCI && isDumpGateBlock(r.ats, r.err) {
				skipped = append(skipped, r.slug+": "+r.err)
				fmt.Fprintf(ew, "skip %s (dump-gated proprietary in CI)\n", r.slug)
				continue
			}
			failures = append(failures, r.slug+": "+r.err)
		}
	})
	if len(failures) > 0 {
		fmt.Fprintln(ew, "\nToken validation failures:")
		for _, f := range failures {
			fmt.Fprintf(ew, "  - %s\n", f)
		}
		return 1, nil
	}
	msg := fmt.Sprintf("\nValidated %d companies", len(companies)-len(skipped))
	if len(skipped) > 0 {
		msg += fmt.Sprintf(" (%d dump-gated skipped in CI)", len(skipped))
	}
	fmt.Fprintln(w, msg+".")
	return 0, nil
}
