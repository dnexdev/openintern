package proofgate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := FindRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// bendGate returns a gate for the checkout, skipping the test when no bend
// binary can run (plain go test needs no Bend install; CI's proofs job has one).
func bendGate(t *testing.T) *Gate {
	t.Helper()
	g := &Gate{Root: repoRoot(t), Bend: BendBinary(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	cmd := exec.Command(g.Bend, "version")
	cmd.Env = bendEnv()
	if err := cmd.Run(); err != nil {
		t.Skipf("bend not runnable (%v); set BEND_BIN to run this test", err)
	}
	return g
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGuardAcceptsCheckout(t *testing.T) {
	if err := Guard(repoRoot(t)); err != nil {
		t.Fatal(err)
	}
}

func TestGuardRefusesEscapeHatches(t *testing.T) {
	cases := []struct {
		line, want string
	}{
		{"@unsafe def f(x: Nat) -> Nat:", "skips the termination check"},
		{"def loop?(x: Nat) -> Nat:", "skips the termination check"},
		{`  import "./ffi.js"`, "imports foreign code"},
		{"import 0xabc/main.bend as P", "imports a hub package"},
		{"import pkg-name@1.0/main.bend as P", "imports a hub package"},
		{"def main() -> Nat:", "defines main"},
		{" def main() -> Nat:", "defines main"},
		{"def f() -> IO(Unit):", "uses IO"},
		{"  x = IO.print(\"ALL PROOFS CHECK\")", "uses IO"},
		{"import ../outside.bend as O", "imports ../outside.bend, outside the proof tree"},
		{"import ./packages/proven/bend/missing.bend as M", "outside the proof tree"},
	}
	for _, c := range cases {
		root := writeTree(t, map[string]string{
			"LAWS.bend":                       "import Base\nimport ./packages/proven/bend/a.bend as A\n",
			"PROOF.bend":                      "import Base\nimport ./LAWS.bend as Laws\n" + c.line + "\n",
			"packages/proven/bend/a.bend":     "import Base\n",
			"packages/proven/bend/notes.txt":  "def main\n",
			"packages/proven/bend/b.bend.bak": "def main\n",
		})
		err := Guard(root)
		if err == nil || !strings.Contains(err.Error(), "PROOF.bend:3 ") || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got %v, want PROOF.bend:3 %s", c.line, err, c.want)
		}
	}
	root := writeTree(t, map[string]string{
		"LAWS.bend":                   "import Base\nimport ./packages/proven/bend/a.bend as A\n",
		"PROOF.bend":                  "import Base\nimport ./LAWS.bend as Laws\n# IOU and def mainly are fine\ndef mainly() -> Nat:\n",
		"packages/proven/bend/a.bend": "import Base\nimport ./a.bend as Self\n",
	})
	if err := Guard(root); err != nil {
		t.Errorf("clean tree refused: %v", err)
	}
}

func TestPassedNeedsExactOutput(t *testing.T) {
	ok := "ALL PROOFS CHECK\nUse --verdict for mathematical validity.\n"
	cases := []struct {
		status  int
		out     string
		verdict bool
		want    bool
	}{
		{0, ok, false, true},
		{0, "  ALL PROOFS CHECK  \n\tUse --verdict for mathematical validity.\n\n", false, true},
		{1, ok, false, false},
		{0, "ALL PROOFS CHECK\n", false, false},
		{0, "warning\n" + ok, false, false},
		{0, ok + "extra\n", false, false},
		{0, "ALL PROOFS CHECK\n", true, true},
		{0, ok, true, false},
		{0, "SOME PROOFS FAIL\n", false, false},
	}
	for _, c := range cases {
		if got := passed(c.status, c.out, c.verdict); got != c.want {
			t.Errorf("passed(%d, %q, %v) = %v", c.status, c.out, c.verdict, got)
		}
	}
}

func TestParseMutants(t *testing.T) {
	rows, err := ParseMutants("# comment\n\na.bend\tx\\n  y\ty\\tz\twhy one\n  \nb.bend\tp\tq\twhy two", [4]int{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	want := []Mutant{{"a.bend", "x\n  y", "y\tz", "why one"}, {"b.bend", "p", "q", "why two"}}
	if len(rows) != len(want) || rows[0] != want[0] || rows[1] != want[1] {
		t.Fatalf("got %#v", rows)
	}
	if _, err := ParseMutants("a\tb\n", [4]int{0, 1, 2, 3}); err == nil {
		t.Fatal("short row accepted")
	}
}

// Every Bend mutant has a Go equivalent whose text occurs exactly once in the
// current Go source (the mutants run itself is `openintern proofs mutants`).
func TestGoMutantsPairWithBendRows(t *testing.T) {
	root := repoRoot(t)
	rows, err := LoadGoMutants(root)
	if err != nil {
		t.Fatal(err)
	}
	bend, err := LoadBendMutants(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(bend) {
		t.Fatalf("%d Go rows for %d Bend rows", len(rows), len(bend))
	}
	for _, m := range rows {
		b, err := os.ReadFile(filepath.Join(root, m.File))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(b), m.Find); n != 1 {
			t.Errorf("%q: text occurs %d times in %s", m.Why, n, m.File)
		}
	}
	for _, m := range bend {
		b, err := os.ReadFile(filepath.Join(root, m.File))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(b), m.Find); n != 1 {
			t.Errorf("bend %q: text occurs %d times in %s", m.Why, n, m.File)
		}
	}
}

func TestNatExpr(t *testing.T) {
	cases := map[int64]string{
		0:             "0n",
		4294967295:    "4294967295n",
		4294967296:    "Nat.add(Nat.mul(1n, Nat.mul(65536n, 65536n)), 0n)",
		natLimit - 1:  "Nat.add(Nat.mul(65535n, Nat.mul(65536n, 65536n)), 4294967295n)",
		1_700_000_000: "1700000000n",
	}
	for n, want := range cases {
		if got := natExpr(n); got != want {
			t.Errorf("natExpr(%d) = %s, want %s", n, got, want)
		}
	}
}

func TestHookLaws(t *testing.T) {
	g := &Gate{Root: t.TempDir()}
	cases := []struct {
		in  string
		ask bool
	}{
		{`{"tool_input":{"file_path":"/x/LAWS.bend"}}`, true},
		{`{"tool_input":{"notebook_path":"LAWS.bend"}}`, true},
		{`{"tool_input":{"file_path":"/x/PROOF.bend"}}`, false},
		{`{"tool_input":{"command":"sed -i s/a/b/ LAWS.bend"}}`, true},
		{`{"tool_input":{"command":"echo x > LAWS.bend"}}`, true},
		{`{"tool_input":{"command":"cp /tmp/x LAWS.bend"}}`, true},
		{`{"tool_input":{"command":"python3 fix.py LAWS.bend"}}`, true},
		{`{"tool_input":{"command":"cat LAWS.bend"}}`, false},
		{`{"tool_input":{"command":"grep law LAWS.bend"}}`, false},
		{`not json`, false},
		{``, false},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := Hook(context.Background(), g, "laws", strings.NewReader(c.in), &out, &errb)
		if code != 0 {
			t.Errorf("%s: exit %d", c.in, code)
		}
		if got := strings.Contains(out.String(), `"permissionDecision":"ask"`); got != c.ask {
			t.Errorf("%s: ask=%v, output %q", c.in, got, out.String())
		}
	}
}

func TestHookPostIgnoresOtherFiles(t *testing.T) {
	g := &Gate{Root: t.TempDir(), Bend: "/nonexistent/bend"}
	for _, in := range []string{`{"tool_input":{"file_path":"/x/README.md"}}`, `{}`, `{"tool_input":{"file_path":"internal/provenance/x.go"}}`} {
		var out, errb bytes.Buffer
		if code := Hook(context.Background(), g, "post", strings.NewReader(in), &out, &errb); code != 0 {
			t.Errorf("%s: exit %d: %s", in, code, errb.String())
		}
	}
	var out, errb bytes.Buffer
	if code := Hook(context.Background(), g, "post", strings.NewReader(`{"tool_input":{"file_path":"x.bend"}}`), &out, &errb); code != 2 {
		t.Errorf(".bend edit with no bend: exit %d, want 2", code)
	}
}

func TestInProvenPackage(t *testing.T) {
	root := "/repo"
	for path, want := range map[string]bool{
		"/repo/internal/proven/proven.go":       true,
		"internal/proven/laws_test.go":          true,
		"/repo/internal/provenance/x.go":        false,
		"/repo/internal/proofgate/gate.go":      false,
		"/elsewhere/internal/proven/x.go":       false,
		"":                                      false,
		"/repo/internal/proven/testdata/v.json": true,
	} {
		if got := inProvenPackage(root, path); got != want {
			t.Errorf("inProvenPackage(%q) = %v", path, got)
		}
	}
}

// The committed vectors are exactly what the current Bend model computes.
func TestVectorsMatchBendModel(t *testing.T) {
	g := bendGate(t)
	got, err := g.Vectors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(g.Root, VectorsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		tmp := filepath.Join(t.TempDir(), "vectors.json")
		os.WriteFile(tmp, got, 0o644)
		t.Fatalf("%s differs from the Bend model; regenerated copy at %s (run openintern proofs build)", VectorsFile, tmp)
	}
}
