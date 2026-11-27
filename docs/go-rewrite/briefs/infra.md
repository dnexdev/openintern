# Brief: infra (proof gate, Go mutants, CI and workflows, Vercel, Dependabot, hooks)

Owns: `.github/**`, `api/**`, `vercel.json`, `internal/proofgate/**`,
`cmd/openintern/cmd_proofs.go`, `internal/proven/mutants_test.go`,
`docs/go-rewrite/status/infra.md`. Do NOT edit `.claude/**` (running agents
depend on the hooks): prepare replacements and put the settings snippet in the
status file. Never edit `LAWS.bend`, `PROOF.bend`, or anything under
`packages/`, `apps/`, `scripts/` (read and run them freely). Throwaway files:
`/tmp/openintern-infra/` (a previous run downloaded Bend 2.0.39 there; verify
its sha256 against `.github/workflows/ci.yml` before use).

Read first: `docs/PROOFS.md`, `scripts/proofs.mjs`, `.claude/settings.json`,
`.claude/hooks/bend-gate.mjs`, `.github/workflows/*.yml`, every status file.

1. Proof gate in Go (`internal/proofgate`, command `proofs` with modes
   `check`, `prove`, `build`, `mutants`): faithful port of
   `scripts/proofs.mjs` (telemetry env off, exact-success-output check, guard
   against `@unsafe`, foreign imports, hub packages, out-of-tree imports, any
   `main`/IO in proof files, `BEND_VERDICT=1`, the Bend mutants in
   `packages/proven/mutants.tsv`). Never run `bend login`, `bend update`,
   `bend link` or `--publish`. After the TypeScript is deleted,
   `packages/proven/generated/proven.mjs` and the TS wrapper used by
   `internal/proven/testdata/gen-vectors.ts` are gone: design how `build` keeps
   `internal/proven/testdata/vectors.json` derived from the current Bend model
   (a Bend program that prints the vectors if bend can run one outside the
   guarded proof tree, or the compiled JS plus a tiny generator run only in the
   proofs CI job), pick the option with the most guarantees and fewest moving
   parts, explain why, implement it. `check` fails when vectors are stale.
2. Real Go mutants: `internal/proven/mutants_test.go` today checks hand-written
   lambdas, never mutating `internal/proven` or running the law tests. Replace
   it with a runner that, per row of `mutants.tsv`, copies the needed module
   files to a temp dir, applies the equivalent text mutation to the real Go
   source, runs `go test ./internal/proven` there, and requires failure. An
   anchor not found fails the run. Behind a build tag or `proofs mutants`, so
   plain `go test` stays fast.
3. CI (`ci.yml`): Go jobs replacing pnpm: `gofmt -l`, `go vet ./...`,
   `go test ./...`, no-`require` check on `go.mod`, registry validators and
   generators in check mode (command names from `cmd/openintern/cmd_pipeline.go`),
   the honest-request test, the isolated Bend job running `openintern proofs
   check` and `proofs mutants`. Keep the security posture: actions pinned by
   SHA where they are today, minimal `permissions`, no persisted credentials,
   the PR-only probe step ported to the Go `probe` command.
4. Ingest, Dump, Ops workflows on Go commands (names/flags from
   `cmd_pipeline.go` and `status/pipeline.md`). Files mode only (no Neon path;
   `OPENINTERN_STATE` no longer a switch). Keep separate publish jobs,
   environments, gates, attestations, artifact handoff, and the "refuse if the
   state repo moved" check exactly. `export-state.yml` stays (psql, one-time
   Neon migration) with a note saying so.
5. Vercel: `api/index.go` exposing `Handler(w http.ResponseWriter, r
   *http.Request)` delegating to `web.Handler()` (see `internal/web`), and
   `vercel.json` routing every path to it with no Node build. Check Vercel's
   current Go runtime docs (Go version, go.mod and internal packages, static
   assets). What can't be checked without a deploy goes under "Blocked on".
6. Dependabot: `gomod` and `github-actions` only.
7. Hook replacement (prepared, not installed): Go equivalents of the three
   `bend-gate.mjs` modes (`laws`, `post`, `stop`) as `openintern proofs hook
   <mode>`, same decisions and exit codes, also watching `internal/proven`.
   Put the exact `.claude/settings.json` snippet in the status file.

Done: `go vet`, `gofmt -l`, `go test` clean; proof gate `check` and `mutants`
verified locally with pinned Bend; workflows reviewed line by line against
the originals (list every behavior kept or dropped); status file complete
(PLAN.md rule 10). Final message: what landed, how verified, what needs GitHub
or Vercel to verify, divergences, requests.
