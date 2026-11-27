package dump

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnexdev/openintern/internal/registry"
)

// testdata/gate holds dump directories made by testdata/gen-gate.mjs and
// what scripts/dump-gate.mjs printed for each (results.json): exit code,
// stdout and stderr, with data/expected-delta.json as of the recorded day.
func TestGateMatchesScript(t *testing.T) {
	b, err := os.ReadFile("testdata/gate/results.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Today   string
		Results []struct {
			Name           string
			Prev           bool
			Env            map[string]string
			Code           int
			Stdout, Stderr string
		}
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	expected, err := registry.LoadExpectedDelta(os.DirFS("../../data"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range golden.Results {
		t.Run(c.Name, func(t *testing.T) {
			dir := filepath.Join("testdata/gate", c.Name)
			prev := ""
			if c.Prev {
				prev = filepath.Join(dir, "prev")
			}
			res, err := Gate(filepath.Join(dir, "new"), prev, GateOptions{
				// Same trim semantics as ApplyHostModeFromEnv / dump-gate CLI.
				Enforce:  strings.TrimSpace(c.Env["OPENINTERN_APPLY_HOSTS"]) == "enforce",
				Approved: func(scope string) bool { return expected.Approved(scope, golden.Today) },
			})
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := res.Report()
			if code != c.Code || stdout != c.Stdout || stderr != c.Stderr {
				t.Errorf("got %d %q %q\nwant %d %q %q", code, stdout, stderr, c.Code, c.Stdout, c.Stderr)
			}
		})
	}
}

func TestGateMissingPrevJobsIsNoBreaker(t *testing.T) {
	res, err := Gate("testdata/gate/breaker/new", t.TempDir(), GateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Holds) != 0 {
		t.Errorf("holds without a previous jobs.json: %v", res.Holds)
	}
}

// Padded "enforce" must enable allowlist Problems the same way
// ApplyHostModeFromEnv does for ingest/dump (issue #58).
func TestGateEnforceTrimsApplyHostsEnv(t *testing.T) {
	dir := filepath.Join("testdata/gate", "off-list-enforce")
	for _, env := range []string{"enforce", " enforce ", "enforce\n"} {
		enforce := strings.TrimSpace(env) == "enforce"
		if !enforce {
			t.Fatalf("trim of %q should be enforce", env)
		}
		res, err := Gate(filepath.Join(dir, "new"), "", GateOptions{Enforce: enforce})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Enforce || res.OffList == 0 || len(res.Problems) == 0 {
			t.Errorf("env %q: enforce=%v offList=%d problems=%v", env, res.Enforce, res.OffList, res.Problems)
		}
	}
}

func TestGateOnRealDump(t *testing.T) {
	// A dump written by FromState passes its own gate.
	st := loadGzState(t, "testdata/state-real")
	out := t.TempDir()
	if _, err := FromState(st, out, goldenNow, optsFor(registry.ApplyHostsEnforce)); err != nil {
		t.Fatal(err)
	}
	res, err := Gate(out, "", GateOptions{Enforce: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := res.Report(); code != 0 {
		t.Errorf("own dump held: %s", stderr)
	}
}
