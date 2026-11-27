package proofgate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Claude Code hooks that keep edits inside the proofs (formerly
// .claude/hooks/bend-gate.mjs):
//
//	laws  PreToolUse: ask before anything edits LAWS.bend (the human-owned spec)
//	post  PostToolUse: recheck the proofs after a .bend edit, and the Go port's
//	      tests after an edit in internal/proven
//	stop  Stop: when proven code changed, refuse to finish until the full gate
//	      and the Go port's tests pass
//
// Exit codes follow Claude Code: 0 lets the action through, 2 blocks it and
// feeds stderr back.

type hookInput struct {
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"`
	} `json:"tool_input"`
	StopHookActive bool `json:"stop_hook_active"`
}

var (
	reLawsName  = regexp.MustCompile(`LAWS\.bend`)
	reLawsWrite = regexp.MustCompile(`(sed[` + jsSpace + `]+-i|>|\btee\b|\bmv\b|\bcp\b|\brm\b|python|perl|node[` + jsSpace + `]+-e)`)
)

// Paths whose changes the stop hook gates.
var provenPaths = []string{LawsFile, ProofFile, "packages/proven", ProvenPackage}

// Hook runs one hook mode and returns its exit code.
func Hook(ctx context.Context, g *Gate, mode string, stdin io.Reader, stdout, stderr io.Writer) int {
	var in hookInput
	if b, err := io.ReadAll(stdin); err == nil && len(bytes.TrimSpace(b)) > 0 {
		// Unreadable input is treated as empty, as before.
		_ = json.Unmarshal(b, &in)
	}
	edited := in.ToolInput.FilePath
	if edited == "" {
		edited = in.ToolInput.NotebookPath
	}

	switch mode {
	case "laws":
		touches := filepath.Base(edited) == LawsFile ||
			(reLawsName.MatchString(in.ToolInput.Command) && reLawsWrite.MatchString(in.ToolInput.Command))
		if touches {
			out, _ := json.Marshal(map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":            "PreToolUse",
					"permissionDecision":       "ask",
					"permissionDecisionReason": "LAWS.bend is the human-owned spec. Approve only if you meant to change a law, not to make a proof pass.",
				},
			})
			stdout.Write(out)
		}
		return 0

	case "post":
		switch {
		case strings.HasSuffix(edited, ".bend"):
			ok, out := g.capture(func(gg *Gate) error { return gg.Prove(ctx) })
			if ok {
				return 0
			}
			fmt.Fprintf(stderr, "Bend proofs no longer check after this edit:\n%s\n", out)
			return 2
		case inProvenPackage(g.Root, edited) && strings.HasSuffix(edited, ".go"):
			ok, out := goTestProven(ctx, g.Root)
			if ok {
				return 0
			}
			fmt.Fprintf(stderr, "go test ./%s fails after this edit:\n%s\n", ProvenPackage, out)
			return 2
		}
		return 0

	case "stop":
		args := append([]string{"status", "--porcelain", "--"}, provenPaths...)
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = g.Root
		status, _ := cmd.Output()
		if strings.TrimSpace(string(status)) == "" {
			return 0
		}
		ok, out := g.capture(func(gg *Gate) error { return gg.Check(ctx) })
		if ok {
			var tests string
			ok, tests = goTestProven(ctx, g.Root)
			out = strings.TrimSpace(out + "\n" + tests)
		}
		if ok {
			return 0
		}
		if in.StopHookActive {
			// Already blocked once this turn: tell the user instead of looping.
			msg, _ := json.Marshal(map[string]string{"systemMessage": "Bend proof gate is FAILING:\n" + out})
			stdout.Write(msg)
			return 0
		}
		fmt.Fprintf(stderr, "The Bend proof gate fails, so this work is not done. Fix the code or proofs (never LAWS.bend), then run openintern proofs build:\n%s\n", out)
		return 2
	}
	return 0
}

// capture runs a gate step with its output collected, as the hooks report it.
func (g *Gate) capture(step func(*Gate) error) (bool, string) {
	var buf bytes.Buffer
	gg := *g
	gg.Stdout, gg.Stderr = &buf, &buf
	err := step(&gg)
	if err != nil {
		if _, ok := err.(*Failure); ok {
			fmt.Fprintf(&buf, "proofs: %s\n", err)
		} else {
			fmt.Fprintf(&buf, "%v\n", err)
		}
	}
	return err == nil, strings.TrimSpace(buf.String())
}

func inProvenPackage(root, path string) bool {
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(filepath.Join(root, ProvenPackage), path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func goTestProven(ctx context.Context, root string) (bool, string) {
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./"+ProvenPackage)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}
