// Package proofgate is the Bend proof gate (formerly scripts/proofs.mjs) and
// the checks that tie the Go port in internal/proven to the Bend model.
//
//	check    PROOF.bend proves every law in LAWS.bend, and the files derived
//	         from the Bend model (internal/proven/testdata/vectors.json, and
//	         packages/proven/generated/proven.mjs while it exists) are fresh
//	prove    only the proofs
//	build    check the proofs, then regenerate the derived files
//	mutants  every row of packages/proven/mutants.tsv must make the proofs
//	         fail, and its Go equivalent must make go test ./internal/proven fail
//
// bend runs with telemetry and crash reporting off. A pass needs exit status 0
// and exactly bend2's success output. BEND_VERDICT=1 adds --verdict, which
// rechecks non-Base defs with a kernel whose consistency is proven in Lean
// (needs Lean v4.34.0; not run in CI). The gate never runs bend login, update,
// link or --publish.
package proofgate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Paths relative to the repository root.
const (
	LawsFile      = "LAWS.bend"
	ProofFile     = "PROOF.bend"
	BendDir       = "packages/proven/bend"
	MutantsFile   = "packages/proven/mutants.tsv"
	GeneratedMJS  = "packages/proven/generated/proven.mjs"
	VectorsFile   = "internal/proven/testdata/vectors.json"
	ProvenPackage = "internal/proven"
)

// The exact output of a passing bend2 check, line by line (trimmed).
var passLines = []string{"ALL PROOFS CHECK", "Use --verdict for mathematical validity."}

// Gate runs the proof checks for one checkout.
type Gate struct {
	Root    string    // repository root (holds go.mod and LAWS.bend)
	Bend    string    // bend binary; empty = BendBinary()
	Verdict bool      // add --verdict to the proof check (BEND_VERDICT=1)
	Stdout  io.Writer // progress and bend's output; nil = os.Stdout
	Stderr  io.Writer // nil = os.Stderr
}

// Failure is a gate refusal. Its message is what the gate prints after
// "proofs: ".
type Failure struct{ Msg string }

func (f *Failure) Error() string { return f.Msg }

func fail(format string, args ...any) error { return &Failure{Msg: fmt.Sprintf(format, args...)} }

// New returns a gate for root with settings from the environment (BEND_BIN,
// BEND_VERDICT).
func New(root string) *Gate {
	return &Gate{Root: root, Bend: BendBinary(), Verdict: os.Getenv("BEND_VERDICT") == "1"}
}

// BendBinary is $BEND_BIN, else ~/.bend/bin/bend when it exists, else "bend"
// from PATH.
func BendBinary() string {
	if b := os.Getenv("BEND_BIN"); b != "" {
		return b
	}
	if home, err := os.UserHomeDir(); err == nil {
		local := filepath.Join(home, ".bend", "bin", "bend")
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	return "bend"
}

// FindRoot walks up from dir to the directory holding this module's go.mod.
func FindRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && regexp.MustCompile(`(?m)^module\s+github\.com/dnexdev/openintern\s*$`).Match(b) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside the openintern checkout (no go.mod for github.com/dnexdev/openintern)")
		}
		dir = parent
	}
}

func (g *Gate) out() io.Writer {
	if g.Stdout != nil {
		return g.Stdout
	}
	return os.Stdout
}

func (g *Gate) bendBin() string {
	if g.Bend != "" {
		return g.Bend
	}
	return BendBinary()
}

// bendEnv is the caller's environment with bend's telemetry (a daily ping to
// bend-lang.com) and its embedded Bun's crash reports (bun.report) switched off.
func bendEnv() []string {
	off := map[string]string{"BEND_NO_TELEMETRY": "1", "DO_NOT_TRACK": "1", "BUN_ENABLE_CRASH_REPORTING": "0"}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := off[k]; !ok {
			env = append(env, kv)
		}
	}
	for _, k := range []string{"BEND_NO_TELEMETRY", "DO_NOT_TRACK", "BUN_ENABLE_CRASH_REPORTING"} {
		env = append(env, k+"="+off[k])
	}
	return env
}

type bendResult struct {
	status         int
	stdout, stderr string
}

func (r bendResult) combined() string { return r.stdout + r.stderr }

// bend runs the bend binary in dir. Only a failure to start it is an error.
func (g *Gate) bend(ctx context.Context, dir string, args ...string) (bendResult, error) {
	cmd := exec.CommandContext(ctx, g.bendBin(), args...)
	cmd.Dir = dir
	cmd.Env = bendEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := bendResult{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit) && exit.ExitCode() >= 0:
		r.status = exit.ExitCode()
	default:
		return r, fail("could not run bend (%v); install it or set BEND_BIN", err)
	}
	return r, nil
}

// passed reports whether bend's output is exactly the success output.
func passed(status int, out string, verdict bool) bool {
	if status != 0 {
		return false
	}
	lines := strings.Split(jsTrim(out), "\n")
	for i := range lines {
		lines[i] = jsTrim(lines[i])
	}
	if verdict {
		return len(lines) == 1 && lines[0] == passLines[0]
	}
	if len(lines) != len(passLines) {
		return false
	}
	for i := range lines {
		if lines[i] != passLines[i] {
			return false
		}
	}
	return true
}

// proofTree lists LAWS.bend, PROOF.bend and every .bend file in the
// implementation directory, relative to root.
func proofTree(root string) ([]string, error) {
	files := []string{LawsFile, ProofFile}
	entries, err := os.ReadDir(filepath.Join(root, BendDir))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bend") {
			files = append(files, filepath.ToSlash(filepath.Join(BendDir, e.Name())))
		}
	}
	return files, nil
}

// JavaScript's \s, so the guard refuses exactly what the original refused.
const jsSpace = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var (
	reUnsafe      = regexp.MustCompile(`@unsafe|^[` + jsSpace + `]*def[` + jsSpace + `]+[\w.]+\?\(`)
	reForeign     = regexp.MustCompile(`^[` + jsSpace + `]*import[` + jsSpace + `]+"`)
	reHub         = regexp.MustCompile(`^[` + jsSpace + `]*import[` + jsSpace + `]+(0x|[\w-]+@)`)
	reMain        = regexp.MustCompile(`^[` + jsSpace + `]*def[` + jsSpace + `]+main\b`)
	reIO          = regexp.MustCompile(`\bIO[` + jsSpace + `]*[(<.]`)
	reLocalImport = regexp.MustCompile(`^[` + jsSpace + `]*import[` + jsSpace + `]+(\.\.?/[^` + jsSpace + `]+)`)
)

// Guard refuses every escape hatch from the proofs: @unsafe defs, foreign C/JS
// code, hub packages, imports from outside the proof tree, and any `main` or
// IO, which could run code that prints a fake pass line.
func Guard(root string) error {
	files, err := proofTree(root)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, f := range files {
		allowed[filepath.Join(root, f)] = true
	}
	var problems []string
	for _, rel := range files {
		abs := filepath.Join(root, rel)
		b, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			at := fmt.Sprintf("%s:%d", rel, i+1)
			if reUnsafe.MatchString(line) {
				problems = append(problems, at+" skips the termination check")
			}
			if reForeign.MatchString(line) {
				problems = append(problems, at+" imports foreign code")
			}
			if reHub.MatchString(line) {
				problems = append(problems, at+" imports a hub package")
			}
			if reMain.MatchString(line) {
				problems = append(problems, at+" defines main")
			}
			if reIO.MatchString(line) {
				problems = append(problems, at+" uses IO")
			}
			if m := reLocalImport.FindStringSubmatch(line); m != nil {
				if !allowed[filepath.Join(filepath.Dir(abs), m[1])] {
					problems = append(problems, fmt.Sprintf("%s imports %s, outside the proof tree", at, m[1]))
				}
			}
		}
	}
	if len(problems) > 0 {
		return fail("refusing unproven code:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

// Prove runs the guard and the proof check, printing bend's output.
func (g *Gate) Prove(ctx context.Context) error {
	if err := Guard(g.Root); err != nil {
		return err
	}
	args := []string{ProofFile}
	if g.Verdict {
		args = append(args, "--verdict")
	}
	r, err := g.bend(ctx, g.Root, args...)
	if err != nil {
		return err
	}
	io.WriteString(g.out(), r.combined())
	if !passed(r.status, r.combined(), g.Verdict) {
		return fail("proofs do not check (exit status %d)", r.status)
	}
	return nil
}

// Check proves the laws and requires every file derived from the Bend model to
// match a fresh rebuild.
func (g *Gate) Check(ctx context.Context) error {
	if err := g.Prove(ctx); err != nil {
		return err
	}
	if g.hasGeneratedMJS() {
		fresh, err := g.compileMJS(ctx)
		if err != nil {
			return err
		}
		old, err := os.ReadFile(filepath.Join(g.Root, GeneratedMJS))
		if err != nil {
			return err
		}
		if !bytes.Equal(fresh, old) {
			return fail("%s is stale; run openintern proofs build", GeneratedMJS)
		}
		fmt.Fprintln(g.out(), "proofs: proven.mjs is up to date")
	}
	fresh, err := g.Vectors(ctx)
	if err != nil {
		return err
	}
	old, err := os.ReadFile(filepath.Join(g.Root, VectorsFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !bytes.Equal(fresh, old) {
		return fail("%s is stale; run openintern proofs build", VectorsFile)
	}
	fmt.Fprintln(g.out(), "proofs: vectors.json is up to date")
	return nil
}

// Build proves the laws, then rewrites the derived files that changed.
func (g *Gate) Build(ctx context.Context) error {
	if err := g.Prove(ctx); err != nil {
		return err
	}
	if g.hasGeneratedMJS() {
		fresh, err := g.compileMJS(ctx)
		if err != nil {
			return err
		}
		if err := writeIfChanged(filepath.Join(g.Root, GeneratedMJS), fresh, g.out()); err != nil {
			return err
		}
	}
	fresh, err := g.Vectors(ctx)
	if err != nil {
		return err
	}
	return writeIfChanged(filepath.Join(g.Root, VectorsFile), fresh, g.out())
}

func writeIfChanged(path string, data []byte, out io.Writer) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		fmt.Fprintf(out, "proofs: %s is up to date\n", filepath.Base(path))
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "proofs: wrote %s\n", path)
	return nil
}

// The compiled module stays checked only while the TypeScript reference
// imports it; once packages/proven/generated is deleted this check goes too.
func (g *Gate) hasGeneratedMJS() bool {
	_, err := os.Stat(filepath.Join(g.Root, GeneratedMJS))
	return err == nil
}

func (g *Gate) compileMJS(ctx context.Context) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "proven-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	to := filepath.Join(tmp, "proven.mjs")
	r, err := g.bend(ctx, g.Root, ProofFile, "-o", to)
	if err != nil {
		return nil, err
	}
	if r.status != 0 {
		return nil, fail("build failed:\n%s", r.combined())
	}
	return os.ReadFile(to)
}

// copyTree copies files (relative to root) into a new temporary directory.
func copyTree(root string, files []string) (string, error) {
	tmp, err := os.MkdirTemp("", "mutant-")
	if err != nil {
		return "", err
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			os.RemoveAll(tmp)
			return "", err
		}
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			os.RemoveAll(tmp)
			return "", err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			os.RemoveAll(tmp)
			return "", err
		}
	}
	return tmp, nil
}

// jsTrim is String.prototype.trim.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
			return true
		}
		return r >= 0x2000 && r <= 0x200a
	})
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
