# Tools workstream status

Owns `internal/tools/**`, `cmd/openintern/cmd_tools.go`, this file.
Throwaway files: `/tmp/openintern-tools/`.

## Done

- `internal/tools`: ports of `gap-report.ts`, `discover-accelerators.ts`,
  `recover-tokens.ts`, `validate-tokens.ts`, built on `internal/ats`
  (`ProbeATSBoard`, `ProbeURL`, `CountJobsFromProbeBody`,
  `ExtractATSBoardsFromMarkdown`, `TokenKey`, `PublicATS`, `MapConcurrent`),
  `internal/fetch` (every request), `internal/registry` (`LoadCompanies`,
  `ParseCompaniesFile`), `internal/yamlite`, `internal/registry/weburl`,
  `internal/state/jsjson`, `internal/htmlx`, `internal/unicodex`.
- Company names sort with a root-collation `localeCompare` (`collate.go`):
  identical to Node's on all 2,106 names in the recorded lists plus crafted
  accents, case, typographic punctuation, symbols, emoji and other scripts.
- `cmd/openintern/cmd_tools.go`: `gap-report`, `discover-accelerators`,
  `recover-tokens`, `validate-tokens`.
- `gen-aliases.ts` was not ported: pipeline replaced it (aliases are parsed
  from the embedded TSVs at startup; `openintern job-aliases` writes new job
  aliases), so it no longer has a job.
- `gap-report.test.ts` ported (`TestExtractCompaniesFromMarkdown`, with its
  stripHtml asserts); its parse-ats-url cases were ported by sources.
- `go vet`, `gofmt -l`, `go test -race ./internal/tools/` clean;
  `cmd/openintern` builds.

The workstream is complete.

## Commands

Flags are the scripts' own and are read the same way (anywhere on the
line, unknown ones ignored, `--x=` values through JavaScript's `Number`).
`-data dir` (also `--data`, `=dir`) picks the registry directory; the
default is `data` in the working directory, so run from the repository root
as CI does. Commands read and write `<data>/companies`.

| Command | Flags | Exit codes |
|---|---|---|
| `gap-report` | `--json`, `--top=N`, `--ats` / `--from-apply`, `--write` | 0; 1 on a thrown error (bad registry file, unreadable `tech-c.yaml`, invalid result) |
| `discover-accelerators` | `--yc`, `--a16z`, `--write`, `--limit=N` | 1 with the usage line when neither `--yc` nor `--a16z`; 1 on a thrown error |
| `recover-tokens` | `--limit=N`, `--slug=a,b`, `--write`, a bare `--` ignored | 0; 1 on a thrown error |
| `validate-tokens` | `[--all] [file.yaml ...]`; env `GITHUB_BASE_REF`, `CI` | 1 when a board fails; 0 otherwise |

`validate-tokens` resolves a relative file against the working directory,
then against the repository root (the parent of `-data`), and on a pull
request reads the base version with `git show origin/$GITHUB_BASE_REF:<path>`
from that root. CI's existing step (`openintern validate-tokens
"${CHANGED[@]}"` from the checkout root) works unchanged.

## Ready APIs

```go
type Env struct {
    Stdout, Stderr io.Writer
    DataDir, WorkDir string
    Now    func() time.Time
    Getenv func(string) string
    Git    func(ctx context.Context, dir string, args ...string) ([]byte, error) // nil: git binary
}
func DefaultEnv(dataDir string) Env
func GapReport(ctx, env Env, args []string) error
func DiscoverAccelerators(ctx, env Env, args []string) (code int, err error)
func RecoverTokens(ctx, env Env, args []string) error
func ValidateTokens(ctx, env Env, args []string) (code int, err error)
func Slugify(name string) string                       // gap-report's slugify
func ExtractCompaniesFromMarkdown(md string) []string
const GapReportUsage, DiscoverUsage string
```

## In progress

Nothing.

## Blocked on

Nothing.

## Requests

- sources (minor): `fetch.Request` sleeps one backoff (400 ms, or the
  Retry-After) after the last attempt fails on a 429, 5xx or network error
  before returning. With `Attempts: 1` (every probe) that is a pure delay;
  returning right after the final attempt would speed up probe-heavy runs
  (`recover-tokens`, `discover-accelerators`).
- docs (phase 3): CONTRIBUTING.md still shows `pnpm recover-tokens`,
  `pnpm validate-tokens`, `pnpm discover-accelerators -- --yc ...`,
  `pnpm gap-report -- --ats --write`; the Go forms are
  `openintern recover-tokens`, `openintern validate-tokens <files>|--all`,
  `openintern discover-accelerators --yc --write --limit=120`,
  `openintern gap-report --ats --write` (no `--` needed). Also
  `docs/reviews/tier1-coverage-2026-10.md` mentions `pnpm recover-tokens`.
- infra: nothing required. Optional: `git diff --diff-filter=d` in the
  "Validate company tokens" step would stop a PR that deletes a company file
  from failing on the missing file (the TypeScript failed the same way).

## Regenerating goldens

From the repository root (TypeScript reference, delete with it in phase 3):

    node internal/tools/testdata/ts/gen.mjs [case...]      # golden/*.json from cases.json
    cd packages/ingest && ./node_modules/.bin/tsx ../../internal/tools/testdata/ts/vectors.mts \
      $PWD/src ../../internal/tools/testdata/collate_node.json | gzip -n -9 > ../../internal/tools/testdata/vectors_node.json.gz

`collate_node.json` came from `testdata/ts/collate.mts` over the four full
community lists as fetched on 2026-11-26 (not kept; the recordings hold
trimmed copies). Re-running `gen.mjs` reproduces every golden byte for byte.

## Parity evidence

- Golden runs (`TestScriptsMatchTypeScript`, 23 cases): `testdata/ts/gen.mjs`
  runs each TypeScript script through `testdata/ts/run.mts` in a scratch
  tree whose `data/companies` is the case's test registry, with `fetch`
  replaced by the case's recording (first entry with the same method, URL
  and body answers; anything else is a 404), the clock fixed at
  2026-11-26T12:00:00Z, and records stdout, stderr, exit code, every request
  and every changed registry file. The Go test replays the same recordings
  through `fetch.WithRoundTripper` and compares all five. All 23 match byte
  for byte, request lists included (the only normalization is the
  next-step hint wording below; `recover-write` compares the written files
  by parsed entries, see Divergences). Cases: name report text, `--json`
  with a source down (503), `--top=2.5`/`--top=abc`/repeated `--top`; ATS
  report text, JSON, `--write` into `tech-c.yaml` (slug disambiguation,
  quoted names and tokens), a source 404 during `--write --json`;
  discover usage error, YC dry runs (`--limit=40`, `--limit=0x3`), YC + a16z
  `--write` onto an existing `tech-accelerators.yaml`, a16z `--write`
  creating it; recover by `--slug` (spaces, unknown slug), default targets
  with `--`, `--limit=x` (no targets), `--write`; validate files with mixed
  results, all-OK, `--all`, pull request against a git base (edited token,
  re-activated board, new board), pull request with nothing changed, pull
  request outside a git repository with `CI=true`.
- Recordings: real responses fetched 2026-11-26 with the OpenIntern
  User-Agent, one request each: the four community lists (trimmed to the
  first rows of every table, `testdata/bodies/*.md.gz`), YC `hiring.json`
  (36 companies, fields the script reads, plus 4 synthetic edge entries),
  a16z `/portfolio-companies/` (live 404) and `/portfolio/` (its plain-text
  anchors, plus synthetic regex edge cases), and three board probes
  (Greenhouse neuralink, Ashby etched, Lever exowatt, trimmed to 3 jobs).
  Other probe answers are synthetic (counts, empty SmartRecruiters, 429,
  500, 403, non-JSON, network failure).
- Helpers (`TestHelpersMatchNode`, `testdata/ts/vectors.mts` copies the
  scripts with their private functions exported): `slugify`,
  `normalizeName`, discover's and recover's `tokenCandidates` on 2,122 names
  with varied websites and ATSes, the a16z anchor regex (which uses a
  lookahead, so Go walks the backtracking order by hand) on 909 generated
  HTML snippets, and `isTechYc` on 13 shaped entries. All equal.
- Collation: `TestLocaleCompareMatchesNode` (`testdata/ts/collate.mts`).
- Live check 2026-11-26 (sequential, honest client, 7 requests):
  `validate-tokens` on one-company files for Stripe (Greenhouse), Notion
  (Ashby), Quartzy (Lever): all `ok`, exit 0; `gap-report --json --top=3`
  on the real registry: 719 companies, 2,582 mentions, exit 0.

## Divergences

- User-Agent: the scripts sent `OpenIntern/0.1 (+https://github.com/dnexdev/openintern; <script>)`
  from raw `fetch`. Every request now goes through `internal/fetch`, whose
  callers cannot set the User-Agent, so the tools send the standard
  `OpenIntern/0.1 (+https://github.com/dnexdev/openintern)`.
- `recover-tokens --write` edits the matching entries in place (`ats`,
  `board_token`, `active`; `active: true` inserted after `board_token` when
  absent, trailing comments kept) and re-validates the file. The script
  parsed the file and wrote it back with `YAML.stringify`, which dropped
  every comment and added `active:` to every entry. The resulting company
  data is identical (golden case `recover-write`).
- Next-step hints in `gap-report` text output name the Go commands
  (`openintern validate-companies && openintern validate-tokens
  data/companies/tech-c.yaml`, `openintern sync-companies`, `verify with
  openintern validate-tokens`) instead of `pnpm ...`.
- Registry directory: the scripts found `data/companies` relative to their
  source file; the commands use `-data` (default `data` in the working
  directory).
- Error text only: a thrown error prints `openintern <cmd>: <message>`
  instead of a Node stack; registry files fail with yamlite/registry
  messages (file:line) instead of a ZodError dump; a network failure reads
  `fetch failed for <url>` instead of `fetch failed` (validate-tokens prints
  it in the failure list).
- Inherited from sources: probes of `citadel`, `citadel_securities` and
  `tesla` check the manual capture compiled into the binary instead of the
  live site, so `validate-tokens` on those boards reports the capture
  (`empty capture`, `manual-expired: ...`); the other probe paths are the
  same requests.
- Regex `i` flag (from Go's case folding): `ſ` and the Kelvin sign match `s`
  and `k` in the few case-insensitive patterns (header words, YC tags).
  No real list or YC entry contains them.
