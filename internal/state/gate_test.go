package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dnexdev/openintern/data"
	"github.com/dnexdev/openintern/internal/registry"
)

// testdata/gate/<case>/{cur,new} are scenarios and expected.json what
// scripts/state-gate.mjs printed for them (/tmp/openintern-pipeline/gate-cases.mjs),
// with the case directory replaced by <DIR>. The script read the real
// data/expected-delta.json, as this test does through the embedded copy.
func TestGateMatchesScript(t *testing.T) {
	delta, err := registry.LoadExpectedDelta(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC().Format("2006-01-02")
	cases, _ := filepath.Glob("testdata/gate/*")
	if len(cases) == 0 {
		t.Fatal("no gate cases")
	}
	for _, dir := range cases {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			var want struct {
				Stdout, Stderr string
				Code           int
			}
			b, err := os.ReadFile(filepath.Join(dir, "expected.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			res, err := Gate(filepath.Join(dir, "cur"), filepath.Join(dir, "new"), GateOptions{
				Approved: func(scope string) bool { return delta.Approved(scope, today) },
			})
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := res.Report()
			stderr = strings.ReplaceAll(stderr, dir, "<DIR>")
			if stdout != want.Stdout || stderr != want.Stderr || code != want.Code {
				t.Errorf("got\n%s%s(exit %d)\nwant\n%s%s(exit %d)", stdout, stderr, code, want.Stdout, want.Stderr, want.Code)
			}
		})
	}
}

func TestGateApproval(t *testing.T) {
	dir := "testdata/gate/growth"
	res, err := Gate(filepath.Join(dir, "cur"), filepath.Join(dir, "new"), GateOptions{
		Approved: func(scope string) bool { return scope == "company:alpha" },
	})
	if err != nil || len(res.Problems) != 0 {
		t.Errorf("approved growth: %v %v", res.Problems, err)
	}
}
