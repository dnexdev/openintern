# Pipeline workstream status

Owns `data/embed.go`, `internal/{yamlite,registry,state,health,dump,ingest}`,
`cmd/openintern/cmd_pipeline.go`.

## Done

- `internal/yamlite`: strict parser for the registry YAML subset; errors name
  file:line. Identical to the TypeScript `yaml` package on every file in
  `data/` and on a feature fixture.
- `data/embed.go`: `data.FS` embeds `companies/`, `curated/`, `manual/`, the
  three TSVs and `expected-delta.json`.
- `internal/registry`: schema (zod messages), loaders, validate-companies,
  validate-curated, apply-host allowlist, aliases, delisted. TS tests ported.
- `internal/registry/weburl`: WHATWG URL parsing for allowlist decisions
  (89 Node vectors).
- `internal/state/jsjson`: JSON.parse / JSON.stringify-exact codec (lone
  surrogates, U+2028/9, `<>&`, key order, numbers), code-unit compare.
- `internal/state/jstime`: Date.parse / toISOString / pgTimestamp on ms.
- `internal/state`: load, validate, write, `FromExport` (stateFromExport),
  `PgTimestamp`, `ByCodeUnit`, `NewID`; `Gate` (scripts/state-gate.mjs).
- `registry.LoadExpectedDelta` / `ExpectedDelta.Approved` for both gates.
- `internal/health`: health.json builder (healthSnapshotFromRecords +
  ingest-health.ts: ParseFunnel, SumFunnels via proven, IsZeroMatchRun).
- `internal/dump`: writeDumpsFromState / writeDumpFiles (every dump file,
  MANIFEST.json) and `Gate` (scripts/dump-gate.mjs).
- `internal/ingest`: runIngestFiles (registry sync, auto-disable, fetch in
  chunks of 8, planCompanyRun, retirement, last-seen sweep with the outage
  guard, run retention), ingestDigest, ingestStepSummary, the summary JSON.
  Fetcher, clock and id generator injected. `ingest.test.ts` and the ingest
  half of `state.test.ts` ported.
- `internal/state` additions: `LocaleCompare` (Node's localeCompare for
  ASCII), `Company/Job/Run.Value(key)` (the JS value of a field, raw shapes
  included), `jsjson.EncodeUTF8` (Node's lone-surrogate → U+FFFD on write).
- `cmd/openintern/cmd_pipeline.go`: every pipeline command (below), ingest
  wired to `ats.Fetch` and `ats.ShadowResults`.

All pipeline packages build, vet, gofmt and test clean (2026-11-25). Every
TS test in scope is ported: validate-companies, validate-curated,
apply-hosts, state, ingest (the scenario half of state.test.ts with a fake
fetcher in place of the mocked Greenhouse adapter).

## Commands (`openintern <command>`)

| Command | Replaces | Exit codes |
|---|---|---|
| `ingest [-data dir] <state-dir>` (alias `ingest-state`) | `cli.ts ingest-state` | 1 when companies were fetched, nothing upserted and something failed; summary JSON on stdout; `OPENINTERN_ONLY_SLUGS`, `OPENINTERN_APPLY_HOSTS`, `OPENINTERN_DIGEST_PATH`, `GITHUB_STEP_SUMMARY`, `OPENINTERN_SHADOW_HONEST` as before |
| `dump [-data dir] <state-dir> <out-dir>` (alias `dump-state`) | `cli.ts dump-state` | 1 on error |
| `state-gate [-data dir] <current> <candidate>` | `scripts/state-gate.mjs` | 0 ok, 1 refused, 2 usage |
| `dump-gate [-data dir] <new-dir> [<previous-dir>]` | `scripts/dump-gate.mjs` | 0 publish, 1 hold, 2 usage |
| `import-state <export-dir> <state-dir>` (alias `state-from-export`) | `cli.ts state-from-export` | 1 on error |
| `sync-companies [-data dir] <state-dir>` | `cli.ts sync-companies` (DB) | 1 on error |
| `validate-companies [-data dir]` | `validate-companies.ts` | 0 / 1, same output |
| `validate-curated [-data dir]` | `validate-curated.ts` | 0 / 1, same output |
| `apply-hosts [-data dir]` | `gen-apply-hosts` | prints `apply-hosts.generated.json`'s JSON |
| `job-aliases [-data dir] <state-dir>` | `job-aliases.ts` (DB) | 1 on error |

`-data` is the registry directory. Ingest, dump and the two gates default to
the registry built into the binary (the commit CI built); the validators,
`apply-hosts`, `sync-companies` and `job-aliases` default to `data` in the
working directory, since they check or edit the working copy.

## Ready APIs

### `internal/registry` (embedded data, for webapi/site/tools)

```go
// Package-level, on the registry compiled into the binary:
func ApplyURLAllowed(companySlug, url string) bool   // mode from OPENINTERN_APPLY_HOSTS (warn logs once, enforce hides)
func CanonicalCompanySlug(slug string) string        // one hop through company-aliases.tsv
func JobAlias(id string) string                       // "" when none (case-insensitive id)
func JobAliasByPrefix(prefix8 string) string          // "" when none
func IsDelisted(jobID string) bool
func DelistedJobs() []Delisted                        // {ID, Reason}, denylist.json order
func Tier1Slugs() []string                            // data/curated/tier-1.yaml order
func Default() *Registry                              // panics if embedded data invalid (tests guard it)

// Explicit:
func Load(fsys fs.FS) (*Registry, error)              // data/-shaped FS (data.FS or os.DirFS("data"))
func (r *Registry) Unique() []CompanyYAML             // by slug, later files win, first-seen order
func (r *Registry) Company(slug string) (CompanyYAML, bool)
func (r *Registry) IsTier1(slug string) bool
func (r *Registry) ApplyHosts() Table
func (r *Registry) CheckApplyURL(companySlug, url string) ApplyCheck   // {OK, Reason, Host *string}
func (r *Registry) ApplyURLAllowed(companySlug, url string, mode ApplyHostMode) bool
func (r *Registry) CanonicalCompanySlug / JobAlias / JobAliasByPrefix (same as above)
func CheckApplyURL(companySlug, url string, table Table) ApplyCheck
func ApplyHostModeFromEnv() ApplyHostMode             // ApplyHostsOff | ApplyHostsWarn | ApplyHostsEnforce
func DerivedHosts(ats, token string) []HostPrefix
func LoadCompanies(fsys fs.FS) ([]CompanyYAML, []string, error)  // all entries, warnings
func LoadTier1(fsys fs.FS) ([]string, error)
func ValidateCompanies(fsys fs.FS) (Result, error)   // {Count, Errors, Warnings}
func ValidateTier1(fsys fs.FS) (Result, error)
type CompanyYAML struct { Name, Slug, ATS, BoardToken string; ExtraBoardTokens []string;
    CareersURL, WebsiteURL *string; Active bool; File string; Line int }
```

### `internal/registry/weburl`

`Parse(s) (*URL, error)`, `CanParse(s) bool`; `URL{Protocol, Username,
Password, Hostname, Port, Pathname}` as JavaScript's `new URL` reports them
(default port → "", dot segments resolved, `\` as `/` for special schemes,
IDNA hosts approximated). Sources may find it useful for `parse-ats-url`.

### `internal/state/jsjson`

`Parse(string) (any, error)`, `Marshal(v) []byte`, `MarshalIndent(v, indent)
[]byte`, `AppendString`, `CompareCodeUnits(a, b) int` (TS `byCodeUnit`),
`Len16(s)` (JS `.length`), `DecodeUTF8([]byte) string` (Node's lossy
decoding). Values: nil, bool, float64, string, `[]any`, `Object` (ordered
`[]Member`); the encoder also takes int, int64, `[]string`, `[]int`.

### `internal/health`

```go
func FromState(st *state.State, now time.Time) (Snapshot, error)
func (s Snapshot) JSON() []byte            // health.json bytes (2-space indent, no newline)
func (s Snapshot) Value() jsjson.Object    // same, as an ordered JSON value
type Snapshot struct { GeneratedAt string; Stats Stats; ZeroMatchCompanies []string;
    RecentFailures []Failure; ProprietaryFailures []ProprietaryFailure;
    ProprietaryZeroJobs []ProprietaryZeroJobRun; FunnelTotals proven.FunnelTotals;
    RecentIngestRuns []Run }
func ParseFunnel(errText *string) Funnel   // Funnel{Fetched, Title, Tech, Upserted, DescriptionMissing *int64}
func SumFunnels([]Funnel) proven.FunnelTotals
func IsZeroMatchRun(errText *string) bool
var ProprietaryATS map[string]bool
```

webapi reads health.json (it never builds it), so `internal/board`'s own
types stay as they are; `/api/v1/health` serves the file this builder wrote.

### `internal/dump`

```go
func FromState(st *state.State, outDir string, now time.Time, opts Options) (Result, error)
func PublicRows(st *state.State, now time.Time, opts Options) ([]Row, error)
func WriteFiles(outDir string, rows []Row, companies []*state.Company, snap health.Snapshot, now time.Time, opts Options) (Result, error)
func JobsJSON / JobsCSV / CompaniesJSON / DenylistJSON / Gzip   // the individual files
type Options struct { ApplyURLAllowed func(slug, url string) bool; IsDelisted func(id string) bool; Delisted []registry.Delisted } // nil = embedded registry
func Gate(newDir, prevDir string, opts GateOptions) (GateResult, error)
type GateOptions struct { Table registry.Table; Enforce bool; Approved func(scope string) bool }
func (r GateResult) Report() (stdout, stderr string, code int)
```

### `internal/ingest`

```go
type Fetcher func(ctx context.Context, atsName, boardToken string) ([]model.NormalizedJob, error) // = ats.Fetch
func Run(ctx context.Context, st *state.State, opts Options) (Summary, error)
type Options struct { Fetch Fetcher; Now time.Time; NewID func() string; Registry *registry.Registry;
    SkipRegistrySync bool; Only map[string]bool; EnforceApplyHosts bool; Shadow func() []ShadowResult }
func SyncRegistry(st *state.State, reg *registry.Registry, now time.Time, newID func() string) []string
func PlanCompanyRun(in PlanInput) CompanyRunPlan
func FetchAllBoards / FetchCompanyBoards(ctx, ats string, tokens []string, f Fetcher) ...
func CountPriorEmptyRuns([]HistoryRun) int
func AutoDisableState([]HistoryRun, exempt bool, now time.Time) AutoDisable
func ShouldSkipLastSeenSweep(last *time.Time, now time.Time) bool
func SightJob(onBoard *model.NormalizedJob, matched bool) proven.JobSighting
func IsInternshipTitle(title string) bool
func OnlySlugs(env string) map[string]bool          // OPENINTERN_ONLY_SLUGS
func (s Summary) JSON() []byte; func (s Summary) Failed() bool
func Digest(Summary) string; func StepSummary(Summary) string
type ShadowResult struct { Source string; OK bool; Status, Count *int; LegacyCount int; Error *string }
```

## In progress

- Nothing. The pipeline workstream is complete.

## Blocked on

- Nothing.

## Requests

- infra: workflows call `openintern ingest <state>`, `openintern dump <state>
  <out>`, `openintern state-gate <cur> <new>`, `openintern dump-gate new prev`,
  `openintern validate-curated`, `openintern validate-companies`; CI's
  `gen-apply-hosts --check` and `gen-aliases --check` steps go away (the
  registry tests fail on the same problems). The gate jobs need only the
  built binary.
- sources: `ats.ShadowResult.Count` is `*float64` and `Error` a string;
  ingest converts (empty error = no `error` key, as the TS omitted it).
  Manual captures are read from `data/manual` relative to the working
  directory; `data.FS` embeds `manual/` if you want ingest to work from any
  directory. Either is fine for CI, which runs from the checkout.
- webapi: nothing required. `/api/v1/health` serves the `health.json` this
  builder writes; `internal/health` types are there if `board` ever wants
  to build a snapshot itself.

## Parity evidence

- yamlite: `YAMLITE_TS_GOLDEN=<file> go test -run TypeScriptGolden ./internal/yamlite`
  with golden lines from `/tmp/openintern-pipeline/yaml2json.ts` over
  `data/companies/*.yaml`, `data/curated/tier-1.yaml` and the fixture: 9/9
  files identical. `testdata/features.json` is the TS output for the fixture.
- registry: the effective allowlist rendered with `TableJSON` is byte-identical
  to `packages/db/src/apply-hosts.generated.json`; company and job aliases
  equal `aliases.generated.ts` (test `TestEmbeddedMatchesGeneratedModules`).
- weburl: `testdata/vectors.json` is Node 26 `new URL` on 89 inputs (dot
  segments, `%2e`, backslashes, ports, credentials, IPv4 forms, IDNA,
  non-special schemes); all match.
- jsjson: `testdata/vectors.json` is Node's JSON.parse/stringify (compact and
  indent 2) on 11 documents with lone surrogates, U+2028/9, `<>&`, control
  characters, numbers, array-index keys, duplicate keys; plus code-unit order,
  `.length` and Node's UTF-8 replacement behaviour. All match.

- health + dump: `internal/dump/testdata/gen-state.mts` builds a 1,434-job,
  183-company, 402-run state from the cached dump (mixed timestamp spellings,
  delisted, stale, role-less, inactive, off-allowlist and http apply links,
  quotes/commas/newlines in CSV cells, null run companies, proprietary
  boards, zero-match and funnel errors incl. leading zeros and over-limit
  counts, two runs at one instant spelled with and without a fraction) and
  writes it with the TS writeState. `dump-ts.mts` runs TS
  writeDumpsFromState at 2026-11-14T12:00:00Z. Go output: jobs.json,
  jobs.csv, companies.json, health.json, denylist.json, LICENSE-DATA.txt
  byte-identical in warn mode (1,336 jobs) and enforce mode (1,278 jobs);
  .gz files decompress to the same bytes; MANIFEST.json identical apart from
  the two .gz hashes/sizes (`TestRealStateMatchesTypeScript`). The three odd
  state fixtures (`internal/state/testdata/{constructed,raw-in,export-out}`)
  give identical files too (`TestOddStatesMatchTypeScript`), including a
  lone surrogate in a CSV cell.
- dump gate: `testdata/gen-gate.mjs` builds 9 dump directories (ok, no
  previous, tampered manifest, uppercase delisted id, off-list links in warn
  and enforce, ATS collapse, ATS drop, growth) and records
  `scripts/dump-gate.mjs` exit code, stdout and stderr; Go matches all 9
  exactly (`TestGateMatchesScript`).
- ingest: `internal/ingest/testdata/gen-scenario.mts` builds boards for a
  fake fetcher from the registry and the cached dump (347 boards, 2,178
  postings incl. non-tech, title-only, description-missing, off-allowlist,
  http, invalid and missing posting dates, extra boards with duplicates) and
  two scenarios; `ingest-ts.mts` runs one day through the TS
  runIngestFiles with the ats module and crypto.randomUUID mocked;
  `run-ts.mjs` records sha256 of the four state files, the summary JSON, the
  digest and the step summary after every day (`expected.json`). Go matches
  every file on all 13 days (`TestScenarioMatchesTypeScript`): 11 days from
  an empty state (registry sync of 719 companies, refused/network/plain and
  manual-expired failures, a 4,900-character error cut at 2,000 UTF-16
  units, empty boards to the third-run retirement, dropped postings,
  OPENINTERN_ONLY_SLUGS with spaces and empty entries, enforce mode, skipped
  sync, 14 auto-disables and a weekly recheck, two outage gaps, 26 jobs
  retired by the last-seen sweep, run retention and term staleness after a
  two-month gap) and 2 days from the dump test state (orphaned companies,
  re-sighted jobs with odd timestamp spellings).
- CLI: `openintern validate-companies` and `validate-curated` print the same
  stdout/stderr as the TS scripts on the current registry; `apply-hosts`
  output is byte-identical to `apply-hosts.generated.json`;
  `import-state` on `internal/state/testdata/export-in` writes the same four
  files as `cli.ts state-from-export`, and both state gates accept it with
  the same output; an `ingest` → `dump` → `dump-gate` run of the built
  binary completes with the TS exit codes.
- LocaleCompare: 400 Node 26 `localeCompare` pairs over timestamps in every
  spelling, slugs, punctuation and case variants (`TestLocaleCompareMatchesNode`).

## Divergences

- Commands: `ingest`/`dump` take the state-file paths the `ingest-state`/
  `dump-state` subcommands took (old names kept as aliases); the database
  `ingest`, `dump` and `backfill-role-families` are gone with the database.
  `sync-companies` and `job-aliases` now work on a state directory instead
  of Postgres (same rules; job-alias pairs come out in old-job id order
  rather than database order). `job-aliases` tells you to commit
  `data/job-aliases.tsv` (there is no `gen-aliases` step any more).

- Dump `.gz` files: same content, different compressed bytes (Go's deflate
  is not zlib's; the gzip header is matched: no name, mtime 0, OS 3). So
  MANIFEST.json's two .gz entries differ from what the TypeScript wrote.
  Nothing compares compressed bytes across implementations; the gate checks
  the manifest against the files the same run wrote.
- `dump-gate` reads the allowlist compiled into the binary instead of
  `packages/db/src/apply-hosts.generated.json` from the checkout (same
  table; see the registry evidence).
- Health snapshot: runs sort newest first by `ran_at` *as a string* under
  `localeCompare`, exactly as the TypeScript did (so "2026-11-13 07:00:00+00"
  sorts before "2026-11-13T06:00:00+00:00"). Kept for parity; it only
  matters when one state mixes timestamp spellings.
- Ingest fails on a stored `first_seen_at`/`posted_at` that is not a date
  for any job of a fetched company; the TypeScript threw only when that job
  was re-sighted. Such a state cannot come from Postgres or the writer.
- Registry sync compares a stored company with the registry by fields (and
  treats any unknown key or odd value type as a change, as JSON.stringify
  did); a hand-edited state with reordered keys no longer counts as changed.
- Records whose `job_count`, `status` or `error` hold a JSON type other than
  the column's are read through their Go field (zero value); exports never
  contain them. A null or missing run `company_id` is handled as the
  TypeScript did.

- Generators dropped: `gen-apply-hosts` and `gen-aliases` (and their CI
  `--check`) are replaced by parsing the embedded files at startup; the
  registry tests fail on invalid alias rows exactly where `gen-aliases` threw.
  `openintern apply-hosts` prints the effective table (same JSON as
  `apply-hosts.generated.json`) for reviewing a change.
- Unknown keys in `data/companies/*.yaml` are still ignored (zod strips them)
  but `validate-companies` now prints them as warnings.
- YAML outside the subset is an error even where the `yaml` package would
  accept it (flow lists, anchors, multi-line scalars, ...): no registry file
  uses them, and the message says what to write instead.
