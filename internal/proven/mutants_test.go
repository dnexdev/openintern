//go:build mutants

package proven

import (
	"context"
	"testing"

	"github.com/dnexdev/openintern/internal/proofgate"
)

// TestGoMutants applies the Go equivalent of every row of
// packages/proven/mutants.tsv to a copy of this package and requires go test
// to fail there (a mutant that does not compile is an error, not a kill). It
// runs go test once per row, so it sits behind a build tag:
//
//	go test -tags mutants -run TestGoMutants ./internal/proven
//
// `openintern proofs mutants` runs the same check after the Bend mutants.
func TestGoMutants(t *testing.T) {
	root, err := proofgate.FindRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	g := proofgate.New(root)
	g.Stdout = testWriter{t}
	if err := g.GoMutants(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}
