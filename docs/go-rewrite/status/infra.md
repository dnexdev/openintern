# Infra workstream status

Owns `.github/**`, `api/**`, `vercel.json`, `internal/proofgate/**`,
`cmd/openintern/cmd_proofs.go`, `internal/proven/mutants_test.go`,
`docs/go-rewrite/status/infra.md`. Throwaway files: `/tmp/openintern-infra/`
(Bend 2.0.39 unpacked at `/tmp/openintern-infra/bend/bin/bend`, sha256 of the
tarball checked against `ci.yml`; `~/.bend/bin/bend` on this machine is a
binary for another platform and fails with "exec format error", so set
`BEND_BIN`).

## Done

- `internal/proofgate` + `openintern proofs <mode>` (`cmd_proofs.go`): port of
  `scripts/proofs.mjs`. Modes `check`, `prove`, `build`, `mutants`
  (`-only bend|go`), `hook laws|post|stop`. Telemetry env forced off,
  exact-success-output check, `BEND_VERDICT=1`, `BEND_BIN` /
  `~/.bend/bin/bend` / PATH lookup, and the guard (`@unsafe`, `def f?(`,
  foreign `import "`, hub imports, out-of-tree imports, `def main`, `IO`),
  with JavaScript's `\s` class so it refuses exactly what the script refused.
- Vectors from Bend (`internal/proofgate/vectors.go`): `proofs build`
  regenerates `internal/proven/testdata/vectors.json` from the Bend model and
  `proofs check` fails when it is stale (design below). The regenerated file
  is byte-identical to the one the TypeScript generator wrote
  (`TestVectorsMatchBendModel`).
- Real Go mutants: `internal/proofgate/gomutants.tsv` holds the Go equivalent
  of each of the 19 rows of `packages/proven/mutants.tsv`, paired by the
  `why` column (the run refuses a missing or extra row). Per row, the runner
  copies `go.mod` plus every module directory `go test ./internal/proven`
  builds from (from `go list -deps -test`, testdata included) to a temp dir,
  applies the text mutation (text must occur exactly once), runs
  `go test -count=1 ./internal/proven` and requires failure. A mutant that
  does not compile is an error, not a kill; the unmutated copy must pass
  first. `internal/proven/mutants_test.go` is now
  `//go:build mutants` and calls the same runner
  (`go test -tags mutants -run TestGoMutants ./internal/proven`), so plain
  `go test` stays fast. The Bend mutant run also checks the unmutated copy
  first.
- Hooks: `openintern proofs hook laws|post|stop` (prepared, not installed;
  snippet below).
- CI (`ci.yml`), Ingest, Dump workflows on Go; Ops unchanged (it runs only
  `gh`); `export-state.yml` kept with a one-time-migration note; Dependabot
  `gomod` + `github-actions`; `CODEOWNERS` and the PR template updated.
  Behaviour kept/dropped line by line below.
- Vercel: `api/index.go` (`Handler` delegating to `web.Handler()`, built once
  per instance) and `vercel.json` (no Node install or build, every path
  rewritten to the function). Does not deploy as is: see Blocked on.

## Vectors design (why a Bend program)

Options were (a) a Bend program that prints the vectors, run outside the
guarded proof tree, or (b) `bend -o proven.mjs` plus a JavaScript wrapper and
generator run with Node in the proofs job. (a) won:

- Guarantees: every decision in the file comes straight from `bend` running
  the checked source (`packages/proven/bend/*.bend`), through the same tool
  that checks the proofs. (b) adds a compiled artifact and a JavaScript copy
  of the wrapper, neither of which anything checks.
- Moving parts: (a) needs only the pinned `bend` binary, which the proofs job
  already installs. `bend file.bend` runs an IO `main` on its embedded
  runtime; it needs no clang or Node (tested with an empty PATH). (b) needs
  Node in the job and a JS wrapper to maintain.

How it works: the input lists are the ones `gen-vectors.ts` used. The
generator first asks Bend for the constants and the `apply_by` table (the
inputs are built from them), then writes a throwaway `vectors.bend` (one
`IO.print` per def call, with small `show_*` helpers) next to a temporary copy
of `packages/proven/bend/`, never inside the guarded tree. The Go side holds
only the wrapper contract of `packages/proven/src/index.ts`, which decided
nothing: RangeError for a non-Nat or unknown tag (same messages, same
left-to-right order), the defaults, term name to constructor, and
`{years_back, month0, day}` to a UTC instant. Bend's runtime aborts the
program when a Nat passes 2^48-1 (stderr `bend: a Nat past the largest
immediate 2^48-1`); the generator records that call as the overflow error the
wrapper raised (`proven <module>:<def>: bend: ...`) and reruns from the next
call. Any other failure (a check failure, other stderr, missing lines) stops
the build. `check` takes ~8 s locally (about 20 reruns for overflow cases).

While `packages/proven/generated/proven.mjs` exists (the TypeScript still
imports it), `check` also compares it with a fresh `bend -o` and `build`
rewrites it, as the script did. Once phase 3 deletes it, those steps skip
themselves.

## Ready APIs

```go
// internal/proofgate
func New(root string) *Gate                  // BEND_BIN / BEND_VERDICT from the environment
func FindRoot(dir string) (string, error)    // directory of this module's go.mod
func BendBinary() string
type Gate struct { Root, Bend string; Verdict bool; Stdout, Stderr io.Writer }
func (g *Gate) Prove(ctx) error              // guard + bend PROOF.bend, exact output
func (g *Gate) Check(ctx) error              // Prove + proven.mjs (if present) + vectors fresh
func (g *Gate) Build(ctx) error              // Prove + rewrite derived files that changed
func (g *Gate) Vectors(ctx) ([]byte, error)  // vectors.json bytes from the Bend model
func (g *Gate) Mutants / BendMutants / GoMutants(ctx) error
func Guard(root string) error
func Hook(ctx, g *Gate, mode string, stdin io.Reader, stdout, stderr io.Writer) int
type Failure struct{ Msg string }            // a refusal; printed as "proofs: <Msg>", exit 1
```

## Workflows, line by line

`ci.yml`, job `check` (`contents: read`):
- kept: triggers (push to main, every PR), `fetch-depth: 0` (the PR token step
  diffs against the base), workflow-level `contents: read`.
- added: `persist-credentials: false` on checkout (nothing in the job pushes);
  checkout pinned by SHA like the other workflows (it used `@v7`);
  `actions/setup-go` pinned to v7.0.0 by SHA (v7.1.0 is 9 days old, inside
  Dependabot's 14-day cooldown), Go from `go.mod`.
- `pnpm install`, `pnpm typecheck`, `pnpm test` → `go.mod has no require`,
  `gofmt -l` over tracked `.go` files, `go vet ./...`, `go test ./...`.
- `node scripts/check-honest-requests.mjs` → `go test -run
  'TestHonestRequests$' ./internal/fetch` (the Go scanner from sources; also
  inside `go test ./...`, run by name so a failure is easy to spot).
- `pnpm validate-curated` / `validate-companies` → the same commands of the
  built binary.
- `pnpm gen-apply-hosts --check`, `pnpm gen-aliases --check` → dropped (the
  generated files are gone; the registry tests fail on the same problems, per
  pipeline).
- PR-only "Validate company tokens": same `git diff` selection, then
  `openintern validate-tokens <files>` (see Divergences and Requests).

`ci.yml`, job `bend` (isolated): kept `permissions: {}`, no secrets, no caches
(`setup-go` with `cache: false`), `persist-credentials: false`, the pinned
Bend 2.0.39 download with its sha256 check, the telemetry env. `node
scripts/proofs.mjs check` → `openintern proofs check`; `node scripts/proofs.mjs
mutants` → `openintern proofs mutants` (Bend and Go mutants).

`ingest.yml`:
- kept: schedule, concurrency group, workflow `contents: read`, 180-minute
  timeout, read-only state checkout (`STATE_READ_KEY`, no persisted
  credentials), recording the base state commit, `OPENINTERN_SHADOW_HONEST=1`,
  `OPENINTERN_DIGEST_PATH`, the digest output, state artifact (30 days,
  `if-no-files-found: error`) and digest artifact (7 days); `publish-state` as
  a separate job in the `state-publish` environment with `contents: read`, the
  only holder of `STATE_DEPLOY_KEY`, the "refuse if the state repo moved"
  check byte for byte, the gate before the push, the push step unchanged;
  `digest` and `notify` jobs unchanged apart from wording.
- dropped: Neon mode (`FILES_MODE`, `vars.OPENINTERN_STATE`, the
  `DATABASE_URL` check, `pnpm db:migrate`, `pnpm ingest`); every step that
  existed only for it now runs unconditionally. Input `files_dry_run` →
  `dry_run` (files is the only mode); `publish-state` runs unless `dry_run`.
- changed: `pnpm install` + `tsx src/cli.ts ingest-state` → `setup-go`,
  `go build`, `openintern ingest <state>`. `node scripts/state-gate.mjs` →
  `openintern state-gate state-repo state-new`, built in `publish-state` from
  its own checkout (stdlib only, no module downloads, `cache: false`). The
  binary is deliberately not handed over from the ingest job, which talks to
  job boards. Notify text lists the file-mode failure causes.

`dump.yml`:
- kept: `workflow_run` trigger and its `if`, `permissions: {}` at the top,
  concurrency, build job `contents: read` writing nothing, `OPENINTERN_APPLY_HOSTS`
  on both the export and the gate, artifact handoff (7 days), `publish` in the
  `dump-publish` environment with `contents: write`, `id-token: write`,
  `attestations: write`, the previous-release download, the gate before the
  attestation, `attest-build-provenance` on `new/*`, `dump-latest` in place,
  the dated prerelease, the 30-day prune, the notify job.
- dropped: Neon mode (`FILES_MODE`, `DATABASE_URL` check, `cli.ts dump`).
- changed: `pnpm install` + `tsx src/cli.ts dump-state` → `openintern dump
  state dumps`; `node scripts/dump-gate.mjs new prev` → `openintern dump-gate
  new prev`, built in `publish` from its own checkout (same reasoning as
  publish-state).

`ops.yml`: unchanged (it only calls `gh`). `export-state.yml`: unchanged
apart from the header note (one-time migration, `openintern import-state`).

## Hooks: `.claude/settings.json` snippet (prepared, not installed)

Replace the `hooks` object (keep `env` and `permissions` as they are). `go run`
caches the binary, so a hook costs about 0.1 s once built. If
`cmd/openintern` does not compile, `laws` falls back to asking whenever the
tool input mentions `LAWS.bend`, and `post`/`stop` report a hook error to the
user instead of blocking.

```json
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
        "hooks": [
          {
            "type": "command",
            "command": "cd \"$CLAUDE_PROJECT_DIR\" && in=$(cat); printf '%s' \"$in\" | go run ./cmd/openintern proofs hook laws 2>/dev/null || { printf '%s' \"$in\" | grep -q 'LAWS\\.bend' && printf '%s' '{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"ask\",\"permissionDecisionReason\":\"The LAWS.bend hook could not run (does cmd/openintern build?). LAWS.bend is the human-owned spec: approve only if you meant to change a law.\"}}'; }; exit 0",
            "timeout": 30
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "cd \"$CLAUDE_PROJECT_DIR\" && go run ./cmd/openintern proofs hook post",
            "timeout": 120,
            "statusMessage": "Checking Bend proofs..."
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "cd \"$CLAUDE_PROJECT_DIR\" && go run ./cmd/openintern proofs hook stop",
            "timeout": 180,
            "statusMessage": "Running Bend proof gate..."
          }
        ]
      }
    ]
  }
```

Timeouts are higher than before (10/60/120 s): the first `go run` after a
change compiles the binary, and `stop` now also regenerates the vectors
(~8 s) and runs `go test ./internal/proven`. Then delete
`.claude/hooks/bend-gate.mjs`. Note for this machine: `~/.bend/bin/bend` is a
binary for another platform, so the current hooks and the new ones both need
`BEND_BIN` (for example in the settings `env`) or a working `~/.bend`.

## In progress

- Nothing. The infra workstream is complete apart from what is blocked below.

## Blocked on

- **Vercel cannot build `api/index.go` as written.** The Go builder
  (`packages/go/src/index.ts` in vercel/vercel, read 2026-11-26) renames the
  module in `go.mod` to `handler`, adds `require github.com/dnexdev/openintern
  v0.0.0-unpublished` + `replace ... => ./`, and builds a generated
  `main__vc__go__.go`; the handler then sits outside our module, and Go
  refuses its import of `internal/web` ("use of internal package ... not
  allowed"; widely reported). Needs a public package (lead request below).
  The builder also runs `go mod tidy`, which pulls `github.com/vercel/go-bridge`
  at build time (Vercel's own code, never in our committed `go.mod`).
- Needs a deploy to confirm: the rewrite hands the handler the original path
  and query (Vercel documents this for functions; not verified for Go);
  an empty `outputDirectory` (`.vercel-static`, created by `buildCommand`) is
  accepted and keeps the repository from being served as static files ahead of
  the rewrite; `installCommand: ""` skips pnpm while the TypeScript is still
  in the tree; Go 1.26 from `go.mod` is available on Vercel; `maxDuration: 30`
  is within the plan's limit; cold-start time of loading the dump.
- Needs GitHub to confirm: every workflow end to end (actionlint is clean,
  shellcheck was not available); `setup-go` v7.0.0 inputs as used
  (`go-version-file`, `cache`, `cache-dependency-path`); environments
  `state-publish` / `dump-publish` and `vars.OPENINTERN_STATE_REPO` exist (the
  ingest and dump jobs now always read the state repo; with no
  `OPENINTERN_STATE_REPO` set they fail instead of falling back to Neon).

## Requests

- lead: add a public bridge so Vercel can build the function, for example
  `server/server.go`:

  ```go
  // Package server exposes the site handler to entry points that are built
  // outside this module's internal tree (Vercel compiles api/index.go as a
  // separate module).
  package server

  import (
  	"net/http"

  	"github.com/dnexdev/openintern/internal/web"
  )

  // Handler is web.Handler with production settings.
  func Handler() http.Handler { return web.Handler() }
  ```

  then change the import in `api/index.go` to `.../server` and call
  `server.Handler()` (or tell infra to). Alternative with fewer moving parts:
  Vercel's Go server preset (`framework: "go"`, a `cmd/server/main.go` that
  listens on `$PORT` with the same handler): no module rewrite, no go-bridge,
  no internal-package problem. Either needs a new path outside infra's lane.
- lead: `internal/third_party/golang.org/x/text/unicode/norm/tables17.0.0.go`
  is not gofmt-clean (one extra blank line after the header). CI's gofmt step
  checks every tracked `.go` file and will fail on it.
- lead: install the hooks snippet above when the agents are done.
- tools: CI's PR step calls `openintern validate-tokens <files>` (same
  semantics as `pnpm validate-tokens`: with `GITHUB_BASE_REF` set, probe only
  added or edited active boards; skip dump-gated proprietary boards when
  `CI=true`). Until it is registered, PRs that touch `data/companies/*.yaml`
  fail that step.
- phase 3 (docs): `docs/PROOFS.md` commands become `openintern proofs
  check|build|mutants`; mention vectors.json and the Go mutants. Keep
  `scripts/export-state.sh` until the Neon migration is done
  (`export-state.yml` calls it). Delete `scripts/proofs.mjs` and
  `.claude/hooks/bend-gate.mjs` with the TypeScript.

## Parity evidence

- `TestVectorsMatchBendModel`: `Gate.Vectors` output equals the committed
  `internal/proven/testdata/vectors.json` (written by the TypeScript
  `gen-vectors.ts` through `proven.mjs`) byte for byte: 675 cases including
  every RangeError and overflow error.
- `openintern proofs check` with pinned Bend 2.0.39: proofs check,
  `proven.mjs` up to date, vectors up to date (same verdicts as
  `node scripts/proofs.mjs check`).
- `openintern proofs mutants`: 19/19 Bend mutants killed (same as the
  script), 19/19 Go mutants killed by failing tests (checked by hand for two
  rows: `TestLaw_pg_page_max`, `TestLaw_js_suffix_skips_dashes`, vectors).
- Guard: unit tests for every refusal rule and for look-alikes that must pass
  (`def mainly`, `IOU` in a comment, self-import inside the tree).
- Hook `laws`: unit tests for the edit-path and shell-command rules; all
  three modes run by hand through `go run` (laws asks on a LAWS.bend edit,
  post ignores other files, stop runs the full gate and `go test`, ~12 s).
- Whole module: `go vet ./...` and `go test ./...` clean with Go 1.27.4 and
  with `GOTOOLCHAIN=go1.26.0` (the version CI and Vercel take from `go.mod`);
  `gofmt -l` clean on infra's files; `go.mod` has no `require`.
- Workflows: actionlint 1.7.7 clean on all five files. The commands the
  workflows call ran locally on test data: `validate-curated`,
  `validate-companies`, `dump` (pipeline's `constructed` state), `dump-gate`
  with a missing previous dump (exit 0), `state-gate` from an empty current
  state (exit 0).

## Divergences

- Error prefix: refusals print `proofs: <message>` as before, but other
  errors (bad flags, unreadable files) print `openintern proofs: <error>`.
- `check`'s freshness step covers `vectors.json` (new) and `proven.mjs` only
  while that file exists.
- `mutants` also runs the Go mutants (`-only bend` gives the old behaviour),
  and both runs first require the unmutated copy to pass.
- CI's PR token step runs `openintern validate-tokens` rather than a loop over
  `openintern probe`: it keeps what `pnpm validate-tokens` did (probe only
  boards the PR adds or edits, skip dump-gated boards in CI), which a probe
  loop in shell would lose.
- Ingest's dispatch input is `dry_run` (was `files_dry_run`).
- Hooks also watch `internal/proven`: `post` runs `go test ./internal/proven`
  after an edit to a `.go` file there; `stop` treats changes under
  `internal/proven` like changes to the Bend tree and also requires
  `go test ./internal/proven` to pass. Messages say `openintern proofs build`
  instead of `pnpm proofs:build`.
