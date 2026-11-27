package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/dnexdev/openintern/internal/proofgate"
)

func init() {
	register("proofs", "Bend proof gate: proofs check|prove|build|mutants, proofs hook laws|post|stop", runProofs)
}

func runProofs(args []string) error {
	flags := flag.NewFlagSet("proofs", flag.ContinueOnError)
	root := flags.String("root", "", "repository root (default: the checkout holding the working directory)")
	only := flags.String("only", "", "mutants: run only the bend or the go mutants")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `usage: openintern proofs [flags] <mode>

modes:
  check    PROOF.bend proves every law in LAWS.bend, and the files derived
           from the Bend model are up to date (CI runs this)
  prove    only the proofs
  build    check the proofs, then regenerate the derived files
           (internal/proven/testdata/vectors.json, and
           packages/proven/generated/proven.mjs while it exists)
  mutants  every row of packages/proven/mutants.tsv must make the proofs
           fail, and its Go equivalent must make go test ./internal/proven fail
  hook laws|post|stop
           Claude Code hooks (JSON on stdin; see docs/go-rewrite/status/infra.md)

bend comes from $BEND_BIN, ~/.bend/bin/bend or PATH, and runs with telemetry
and crash reports off. BEND_VERDICT=1 adds --verdict to the proof check.

flags:
`)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	mode := "check"
	if flags.NArg() > 0 {
		mode = flags.Arg(0)
	}

	dir := *root
	if dir == "" {
		if mode == "hook" && os.Getenv("CLAUDE_PROJECT_DIR") != "" {
			dir = os.Getenv("CLAUDE_PROJECT_DIR")
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			dir = wd
		}
	}
	r, err := proofgate.FindRoot(dir)
	if err != nil {
		return err
	}
	gate := proofgate.New(r)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if mode == "hook" {
		if flags.NArg() != 2 {
			flags.Usage()
			return flag.ErrHelp
		}
		os.Exit(proofgate.Hook(ctx, gate, flags.Arg(1), os.Stdin, os.Stdout, os.Stderr))
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return flag.ErrHelp
	}

	switch mode {
	case "check":
		err = gate.Check(ctx)
	case "prove":
		err = gate.Prove(ctx)
	case "build":
		err = gate.Build(ctx)
	case "mutants":
		switch *only {
		case "":
			err = gate.Mutants(ctx)
		case "bend":
			err = gate.BendMutants(ctx)
		case "go":
			err = gate.GoMutants(ctx)
		default:
			return fmt.Errorf("-only must be bend or go, not %q", *only)
		}
	default:
		return fmt.Errorf("unknown mode %s; use check, prove, build, mutants or hook", mode)
	}
	var refusal *proofgate.Failure
	if errors.As(err, &refusal) {
		// The gate's own wording, as scripts/proofs.mjs printed it.
		fmt.Fprintf(os.Stderr, "proofs: %s\n", refusal.Msg)
		os.Exit(1)
	}
	return err
}
