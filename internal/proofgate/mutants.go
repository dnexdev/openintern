package proofgate

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Mutant is one plausible bug: replace Find (which must occur exactly once in
// File) with Replace. Why names the bug and pairs a Bend row with its Go
// equivalent.
type Mutant struct {
	File, Find, Replace, Why string
}

// ParseMutants reads a mutants table: one row per line, tab-separated
// columns, \n and \t escaped in cells, blank lines and #-comments skipped.
// cols maps the row's columns to file, find, replace, why.
func ParseMutants(text string, cols [4]int) ([]Mutant, error) {
	var out []Mutant
	for n, line := range strings.Split(text, "\n") {
		if jsTrim(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cells := strings.Split(line, "\t")
		for i, c := range cells {
			cells[i] = strings.ReplaceAll(strings.ReplaceAll(c, `\n`, "\n"), `\t`, "\t")
		}
		need := max(cols[0], cols[1], cols[2], cols[3]) + 1
		if len(cells) < need {
			return nil, fmt.Errorf("line %d: %d columns, want %d", n+1, len(cells), need)
		}
		out = append(out, Mutant{File: cells[cols[0]], Find: cells[cols[1]], Replace: cells[cols[2]], Why: cells[cols[3]]})
	}
	return out, nil
}

// LoadBendMutants reads packages/proven/mutants.tsv (file, text, replacement, why).
func LoadBendMutants(root string) ([]Mutant, error) {
	b, err := os.ReadFile(filepath.Join(root, MutantsFile))
	if err != nil {
		return nil, err
	}
	return ParseMutants(string(b), [4]int{0, 1, 2, 3})
}

// gomutants.tsv holds the Go equivalent of every Bend mutant, keyed by its
// why column, in the same layout as mutants.tsv: file, text, replacement, why.
//
//go:embed gomutants.tsv
var goMutantsTSV string

// LoadGoMutants returns the Go rows, checked to pair one to one with the Bend rows.
func LoadGoMutants(root string) ([]Mutant, error) {
	goRows, err := ParseMutants(goMutantsTSV, [4]int{0, 1, 2, 3})
	if err != nil {
		return nil, fmt.Errorf("gomutants.tsv: %w", err)
	}
	bendRows, err := LoadBendMutants(root)
	if err != nil {
		return nil, err
	}
	bendWhy := map[string]bool{}
	for _, m := range bendRows {
		bendWhy[m.Why] = true
	}
	goWhy := map[string]bool{}
	for _, m := range goRows {
		if goWhy[m.Why] {
			return nil, fail("gomutants.tsv lists %q twice", m.Why)
		}
		goWhy[m.Why] = true
	}
	var missing, extra []string
	for _, w := range sortedKeys(bendWhy) {
		if !goWhy[w] {
			missing = append(missing, w)
		}
	}
	for _, w := range sortedKeys(goWhy) {
		if !bendWhy[w] {
			extra = append(extra, w)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		return nil, fail("Go mutants do not match %s: no Go equivalent for %q; no Bend row for %q", MutantsFile, missing, extra)
	}
	return goRows, nil
}

// apply replaces the one occurrence of m.Find in the copy at dir.
func (m Mutant) apply(dir string) error {
	target := filepath.Join(dir, m.File)
	b, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	text := string(b)
	if count := strings.Count(text, m.Find); count != 1 {
		return fail("mutant %q: text occurs %d times in %s", m.Why, count, m.File)
	}
	return os.WriteFile(target, []byte(strings.Replace(text, m.Find, m.Replace, 1)), 0o644)
}

// BendMutants checks that every Bend mutant makes the proof tree fail to check.
func (g *Gate) BendMutants(ctx context.Context) error {
	if err := Guard(g.Root); err != nil {
		return err
	}
	rows, err := LoadBendMutants(g.Root)
	if err != nil {
		return err
	}
	tree, err := proofTree(g.Root)
	if err != nil {
		return err
	}
	// The unmutated copy must pass, or every mutant would count as killed.
	if base, err := copyTree(g.Root, tree); err != nil {
		return err
	} else {
		r, err := g.bend(ctx, base, ProofFile)
		os.RemoveAll(base)
		if err != nil {
			return err
		}
		if !passed(r.status, r.combined(), false) {
			return fail("the unmutated proof tree does not check:\n%s", r.combined())
		}
	}
	var survivors []string
	for _, m := range rows {
		tmp, err := copyTree(g.Root, tree)
		if err != nil {
			return err
		}
		if err := m.apply(tmp); err != nil {
			os.RemoveAll(tmp)
			return err
		}
		r, err := g.bend(ctx, tmp, ProofFile)
		os.RemoveAll(tmp)
		if err != nil {
			return err
		}
		ok := passed(r.status, r.combined(), false)
		g.report(ok, "bend", m.Why)
		if ok {
			survivors = append(survivors, m.Why)
		}
	}
	if len(survivors) > 0 {
		return fail("%d Bend mutant(s) survived; strengthen LAWS.bend", len(survivors))
	}
	fmt.Fprintf(g.out(), "proofs: all %d Bend mutants killed\n", len(rows))
	return nil
}

func (g *Gate) report(survived bool, kind, why string) {
	verdict := "killed  "
	if survived {
		verdict = "SURVIVED"
	}
	fmt.Fprintf(g.out(), "%s  %-4s  %s\n", verdict, kind, why)
}

// GoMutants applies each Go mutant to a copy of the module files that
// internal/proven's tests build from, runs go test there, and requires a test
// failure. A mutant that does not compile is an error, not a kill: it would
// prove nothing about the tests.
func (g *Gate) GoMutants(ctx context.Context) error {
	rows, err := LoadGoMutants(g.Root)
	if err != nil {
		return err
	}
	files, err := testFiles(ctx, g.Root)
	if err != nil {
		return err
	}
	if base, err := copyTree(g.Root, files); err != nil {
		return err
	} else {
		out, status, err := goTest(ctx, base)
		os.RemoveAll(base)
		if err != nil {
			return err
		}
		if status != 0 {
			return fail("go test ./%s fails without any mutant:\n%s", ProvenPackage, out)
		}
	}
	var survivors []string
	for _, m := range rows {
		tmp, err := copyTree(g.Root, files)
		if err != nil {
			return err
		}
		if err := m.apply(tmp); err != nil {
			os.RemoveAll(tmp)
			return err
		}
		out, status, err := goTest(ctx, tmp)
		os.RemoveAll(tmp)
		if err != nil {
			return err
		}
		if status != 0 && isBuildFailure(out) {
			return fail("Go mutant %q does not compile:\n%s", m.Why, out)
		}
		g.report(status == 0, "go", m.Why)
		if status == 0 {
			survivors = append(survivors, m.Why)
		}
	}
	if len(survivors) > 0 {
		return fail("%d Go mutant(s) survived; strengthen the tests in %s", len(survivors), ProvenPackage)
	}
	fmt.Fprintf(g.out(), "proofs: all %d Go mutants killed\n", len(rows))
	return nil
}

// Mutants runs the Bend mutants, then the Go mutants.
func (g *Gate) Mutants(ctx context.Context) error {
	if err := g.BendMutants(ctx); err != nil {
		return err
	}
	return g.GoMutants(ctx)
}

// testFiles lists go.mod and every file in the module directories that
// go test ./internal/proven compiles or reads (testdata included).
func testFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-test", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", "./"+ProvenPackage)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v\n%s", err, stderr.String())
	}
	files := []string{"go.mod"}
	seen := map[string]bool{}
	for _, dir := range strings.Fields(string(out)) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || seen[rel] {
			continue
		}
		seen[rel] = true
		err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Subpackages are listed on their own when they are dependencies.
				if path != dir && d.Name() != "testdata" && !strings.Contains(path, string(filepath.Separator)+"testdata"+string(filepath.Separator)) {
					return filepath.SkipDir
				}
				return nil
			}
			r, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, r)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func goTest(ctx context.Context, dir string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./"+ProvenPackage)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() > 0 {
			return string(out), exit.ExitCode(), nil
		}
		return string(out), 0, fmt.Errorf("go test: %v\n%s", err, out)
	}
	return string(out), 0, nil
}

func isBuildFailure(out string) bool {
	return strings.Contains(out, "[build failed]") || strings.Contains(out, "[setup failed]")
}
