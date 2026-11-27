# OpenIntern Go rewrite

Branch `go-rewrite`, one branch for the whole rewrite. **Nothing is committed by
agents; the maintainer commits.** Status of each workstream lives in
`docs/go-rewrite/status/<workstream>.md`.

## Why

The TypeScript stack (Next.js, React, Drizzle, Postgres/Neon, zod, tsx, pnpm)
brings constant dependency churn, two Neon quota outages, and a site that only
runs where Next.js runs. The rewrite moves everything to:

- one Go module, **standard library only**, one binary (`cmd/openintern`);
- flat-file state (the phase D format), no database;
- a server-rendered site (`html/template`, `embed`) with no build step and a
  little vanilla JS for progressive enhancement;
- a new visual design (see "Design" below).

## What must not change (public contracts)

Anything outside the repo depends on these. Treat a difference as a bug unless
it is listed under "Divergences" in your status file with a reason.

| Contract | Source of truth (TypeScript reference) |
|---|---|
| Page URLs: `/`, `/jobs`, `/jobs/{company}`, `/jobs/{company}/{jobSlug}`, `/docs`, `/health`, `/privacy`, `/vs/intern-insider`, `/sitemap.xml`, `/robots.txt`, icons | `apps/web/src/app/**` |
| Old-slug redirects (company and job aliases) | `data/company-aliases.tsv`, `data/job-aliases.tsv`, `load-job.ts` |
| API v1: `/api/v1/jobs`, `/api/v1/jobs/{id}`, `/api/v1/companies`, `/api/v1/health`, `/api/v1/status`: same query params, status codes, JSON keys, key order, value types, error bodies, rate-limit and cache headers | `apps/web/src/app/api/v1/**`, `lib/board.ts`, `lib/job-families.ts`, `lib/api-error.ts`, `lib/rate-limit.ts` |
| Dump files: `jobs.json`, `jobs.csv`, `companies.json`, `health.json`, `denylist.json`, `LICENSE-DATA.txt`, `*.gz`, `MANIFEST.json` | `packages/ingest/src/dump.ts`, `scripts/dump-gate.mjs` |
| State files: `companies.jsonl`, `jobs.jsonl`, `last_seen.tsv`, `runs.jsonl` | `packages/ingest/src/state.ts`, `scripts/state-gate.mjs` |
| Job URL slugs (`{title-slug}-{id8}`) | `apps/web/src/lib/job-slug.ts`, `packages/proven/bend/job_slug.bend` |
| Honest requests: User-Agent `OpenIntern/0.1 (+https://github.com/dnexdev/openintern)`, callers cannot set UA/Referer/cookies | `packages/ingest/src/http.ts`, `scripts/check-honest-requests.mjs` |
| Registry formats contributors edit: `data/companies/*.yaml`, `data/curated/tier-1.yaml`, `data/*.tsv`, `data/manual/**` | `packages/ingest/src/schema.ts`, `sync-companies.ts` |
| Kill switch file and env vars: `JOBS_DUMP_URL`, `HEALTH_SNAPSHOT_URL`, `OPENINTERN_KILL_SWITCH_URL`, `OPENINTERN_MAINTENANCE`, `API_RATE_LIMIT_PER_MINUTE`, `OPENINTERN_APPLY_HOSTS`, `OPENINTERN_ONLY_SLUGS`, `OPENINTERN_SHADOW_HONEST`, `OPENINTERN_DIGEST_PATH`, `OPENINTERN_ANALYTICS` | `lib/kill-switch.ts`, `.env.example` |
| Decisions: internship classifier, terms/years/durations/regions/roles, title normalization, fingerprints, role families, freshness, job retirement, pagination, rate limiting, apply picker | `packages/ingest/src/classifier.ts`, `normalize-title.ts`, `packages/proven/**`, `LAWS.bend` |

Dropped on purpose: Postgres/Neon and everything DB-only (`DATABASE_URL`,
`OPENINTERN_PREFER_DB`, `OPENINTERN_DB_POOL_MAX`, drizzle migrations,
`backfill-role-families`, the DB ingest path in `ingest.ts`). The file ingest
path (`ingest-files.ts`) is the one to port; port shared logic from `ingest.ts`
that it calls.

## Decisions

1. **Go 1.26, module `github.com/dnexdev/openintern`, zero third-party
   modules.** `go.mod` has no `require` lines and must keep none. Code copied
   from elsewhere lives in `internal/third_party/<import path>/` with its
   LICENSE and a README saying what was copied and changed, and is listed in
   `THIRD_PARTY.md`. So far: `golang.org/x/text/unicode/norm` and `transform`
   (Unicode 17.0, matches Node 26). Ask the lead before copying anything else.
2. **Unicode:** `internal/unicodex` provides `NFKC`, `NFKD`, `NFC`,
   `IsExtendedPictographic`, `IsCombiningMark`. Go's `regexp` is RE2: `\p{L}`,
   `\p{N}`, `(?i)` work; lookahead, lookbehind and backreferences do not, so
   rewrite those as code and test them against the TypeScript output.
3. **State is flat files only** (phase D format). Migration from Neon is the
   existing `scripts/export-state.sh` output, imported with a port of
   `stateFromExport` in `state.ts`.
4. **Website:** `net/http` + `html/template` + `embed`. Every page works with
   JavaScript off (filters are a GET form). Vanilla JS adds instant filtering,
   the Applied toggle (localStorage, including the job-alias migration in
   `AppliedToggle.tsx`) and copy buttons. No npm, no bundler, no CSS framework,
   no external CDNs or font services. Fonts are self-hosted files.
5. **Data at runtime:** the server reads the daily dump (`JOBS_DUMP_URL`,
   default the `dump-latest` release), keeps it in memory and refreshes it
   hourly, which is today's dump-first path without the DB fallback. Registry
   files are compiled in through a root package `data` (`data/embed.go`,
   `//go:embed`), so nothing needs the filesystem on the server.
6. **Hosting:** stays on Vercel through the Go runtime: `api/index.go` calls
   `web.Handler()` and `vercel.json` routes everything to it.
   `openintern serve` runs the same handler on any host and locally.
7. **Proofs:** `LAWS.bend` (human-owned spec), `PROOF.bend` and
   `packages/proven/bend/*.bend` stay the reference model. `internal/proven` is
   a Go port of the Bend functions and of the wrapper API in
   `packages/proven/src/index.ts`, checked three ways: vectors computed by the
   proven model, one Go test per law in `LAWS.bend`, and every row of
   `packages/proven/mutants.tsv` applied to the Go code must make a test fail.
   The proof gate (`scripts/proofs.mjs`) is ported to Go in phase 2.
8. **The TypeScript stays in the tree as the reference implementation** until
   parity is shown, then the lead deletes it in phase 3. It is read-only for
   everyone until then.

## Layout and ownership

Edit only what your workstream owns. To change something someone else owns,
write it under "Requests" in your status file; the lead routes it.

| Path | Owner | Ports |
|---|---|---|
| `go.mod`, `cmd/openintern/main.go`, `internal/model`, `internal/unicodex`, `internal/third_party`, `docs/go-rewrite/PLAN.md` | lead | |
| `internal/proven` | core | `packages/proven/src/index.ts` + `packages/proven/bend/*.bend` |
| `internal/classify` | core | `classifier.ts`, `normalize-title.ts` (incl. `shortHash`, `jobFingerprint`, `roleFamilyId`) |
| `internal/jobslug` | core | `apps/web/src/lib/job-slug.ts` |
| `internal/fetch` | sources | `http.ts` (honest client, typed failures, Retry-After) |
| `internal/htmlx`, `internal/safedate` | sources | `strip-html.ts`, `safe-date.ts` |
| `internal/ats/**` | sources | `ats.ts`, every adapter (`workday.ts` … `shopify.ts`), `manual.ts`, `parse-ats-url.ts`, `probe-url.ts` |
| `cmd/openintern/cmd_sources.go` | sources | `manual-capture.ts`, a `probe` command |
| `data/embed.go` (new; data files themselves are shared, edit only on request) | pipeline | |
| `internal/registry`, `internal/yamlite` | pipeline | `schema.ts`, `sync-companies.ts`, `validate-companies.ts`, `validate-curated.ts`, `apply-hosts.ts`, `gen-apply-hosts.ts`, `aliases.ts`, `gen-aliases.ts`, `job-aliases.ts`, `delisted.ts` |
| `internal/state` | pipeline | `state.ts`, `import-state.ts` (export → state), `scripts/state-gate.mjs` |
| `internal/ingest` | pipeline | `ingest-files.ts`, `plan.ts`, the parts of `ingest.ts` they use |
| `internal/dump` | pipeline | `dump.ts`, `dump-state.ts`, `scripts/dump-gate.mjs` |
| `internal/health` | pipeline | `packages/db/src/health-snapshot.ts`, `ingest-health.ts`, `freshness.ts` (non-SQL parts) |
| `cmd/openintern/cmd_pipeline.go` | pipeline | `cli.ts` commands |
| `internal/board` | webapi | `lib/board.ts`, `job-families.ts`, `posting-label.ts`, `publishable.ts`, `load-job.ts`, `curated.ts`, `job-dump.ts`, `freshness.ts`, `health-snapshot.ts`, `ingest-health.ts` (web side) |
| `internal/web` | webapi | server, routing, maintenance (`proxy.ts`), `kill-switch.ts`, `rate-limit.ts`, `api-error.ts`, `app/api/v1/**`, `sitemap.ts`, `robots.ts`, static file serving, caching headers |
| `cmd/openintern/cmd_serve.go` | webapi | `serve` command |
| `DESIGN.md`, `internal/site/**` (templates, page handlers, CSS, JS, fonts, images, OG image) | site | every page and component in `apps/web/src/app/**` and `components/**`, `globals.css`, `og-wordmark.tsx` |
| `internal/tools/**`, `cmd/openintern/cmd_tools.go` | tools (phase 2) | `gap-report.ts`, `discover-accelerators.ts`, `recover-tokens.ts`, `validate-tokens.ts` |
| `.github/**`, `api/`, `vercel.json`, `internal/proofgate`, `cmd/openintern/cmd_proofs.go`, `internal/proven/mutants_test.go` | infra (phase 2, started) | workflows, `scripts/proofs.mjs`, `bend-gate.mjs` (prepared; `.claude/**` is swapped in by the lead after the Claude agents finish) |
| README, SETUP, RUNBOOK, CONTRIBUTING, PROOFS, THIRD_PARTY, DATA docs | phase 3 | |

Read-only for every agent until phase 3: `apps/**`, `packages/**`,
`scripts/**`, `LAWS.bend`, `PROOF.bend`, `.claude/**`, `.github/**`,
`package.json`, `pnpm-*`. Never edit `LAWS.bend`.

## Contracts between workstreams

Go names mirror the TypeScript exports (`isTechInternship` →
`classify.IsTechInternship`), so anyone can code against a TS module before its
port lands. Shared records are in `internal/model` (read it first).

- **core → everyone:** `internal/proven` mirrors `packages/proven/src/index.ts`
  one to one (same functions, same errors for out-of-range input).
  `internal/classify` mirrors `classifier.ts` and `normalize-title.ts`.
  `internal/jobslug` mirrors `job-slug.ts`. Land `internal/proven` first.
- **sources → pipeline:** `ats.Fetch(ctx context.Context, atsName, boardToken
  string) ([]model.NormalizedJob, error)`. A failed fetch returns an error that
  `errors.As` to `*fetch.Failure` (with the `FetchFailureKind` values from
  `http.ts`), so ingest can tell "board fetched and empty" from "fetch failed"
  (the retirement laws depend on it). Also `ats.IsProprietary(name string) bool`
  and the parse/probe functions the tools need.
- **pipeline → webapi/tools:** `registry` (load companies, curated slugs,
  aliases, apply-host allowlist with `ApplyURLAllowed(companySlug, url)`,
  delisted jobs), `health` snapshot builder (same JSON as `health.json` and
  `/api/v1/health`). `ingest` takes the fetcher as a function value, so it can
  be tested with fakes and wired to `ats.Fetch` in `cmd_pipeline.go`.
- **webapi → site:** `internal/board` loads a dump and answers everything a
  page needs (board query with families and pagination, one job, one company,
  company list, corpus stats, health snapshot, hero preview, stale-data state).
  Publish its exported API in your status file as soon as it compiles. The site
  plugs in with `site.Register(mux *http.ServeMux, store *board.Store)` (or
  whatever signature webapi and site agree in their status files).

## Rules for every agent

1. **No git commits, ever.** Do not run any git command that changes the
   repository or index: no `commit`, `add`, `stash`, `checkout`, `switch`,
   `restore`, `reset`, `clean`, `rebase`, `merge`, `rm`, `mv`, `apply`.
   Read-only git (`status`, `diff`, `log`, `show`, `grep`, `ls-files`) is fine.
   Five agents share this checkout; one stray `git checkout .` destroys
   everyone's work.
2. **No credit or attribution anywhere.** No `Co-Authored-By`, no "Generated
   with", no model, agent or tool names in code, comments, docs, status files or
   anything else. Write as the project.
3. **Stay in your lane.** Edit only paths your workstream owns. Do not reformat,
   "fix" or delete other workstreams' files, even if they break the build.
4. **No new dependencies.** No `require` in `go.mod`, nothing added to the
   project's npm manifests, no CDN links. Throwaway dev tools outside the repo
   are fine (for example Playwright for screenshots in `/tmp`).
5. **TypeScript is the oracle.** `pnpm install` is already done; do not run it
   again. Run reference code with `packages/ingest/node_modules/.bin/tsx`
   (or `apps/web/node_modules/.bin/tsx` if present). Put throwaway scripts in
   `/tmp/openintern-<workstream>/`, not in the repo. Golden files that tests
   need go in your package's `testdata/` (keep each under ~1 MB; gzip if
   larger).
6. **Test what you port.** Every TS test case becomes a Go test (table-driven).
   Add golden-parity tests against TypeScript output on real inputs. The cached
   public dump is at `.cache/dump/jobs.json` (2026-11-13, 1434 jobs, older format
   without `role_family_id`/`normalized_title`).
7. **Build only your packages:** `go build ./internal/<pkg>/...`,
   `go test ./internal/<pkg>/...`, `go vet` on yours, `gofmt -l` clean. Other
   packages may be mid-edit.
8. **Go style:** standard library only; small packages; `context.Context` on
   anything that does I/O; errors wrapped with context; no global mutable state
   except guarded caches. Keep the comments that explain *why* a decision was
   made (they carry hard-won knowledge, incident numbers, employer quirks);
   drop comments that restate code.
9. **Byte compatibility** for every JSON file or response that the TS wrote:
   - `json.Encoder` with `SetEscapeHTML(false)` (Go escapes `<>&` by default);
   - Go always escapes U+2028/U+2029; JS does not, so unescape them after
     encoding (only where the backslash is not itself escaped);
   - nil slices become `null` in Go; write `[]` wherever TS wrote arrays;
   - `JSON.stringify(x, null, 2)` matches `json.MarshalIndent(x, "", "  ")`
     apart from the above; files written by TS without a trailing newline must
     stay without one;
   - state timestamps are Postgres-style strings and must pass through
     unchanged; new ones use `pgTimestamp` from `state.ts`; dump timestamps are
     JS `toISOString()` (always 3 fraction digits, `Z`);
   - sort with plain byte order where TS used `byCodeUnit`; where TS used
     `localeCompare`, match its output on the real data and document the rule.
10. **Status file:** keep `docs/go-rewrite/status/<workstream>.md` current with
    sections: Done, Ready APIs (exported signatures others can call), In
    progress, Blocked on, Requests (to other workstreams), Parity evidence
    (what you compared and how), Divergences (intended behavior changes, with
    reasons). Update it at every milestone; others read it to unblock
    themselves. Check other workstreams' status files when you need their API.
11. **Hooks:** this checkout has Claude Code hooks (`.claude/settings.json`)
    that run the Bend gate when `LAWS.bend`, `PROOF.bend` or `packages/proven`
    change. Leave those paths alone and the hooks stay quiet.
12. **Finish line:** your packages build, vet, gofmt and test clean; every TS
    test is ported; parity evidence recorded; status file final. Your last
    message summarizes what landed, what diverges and what is left.

## Design

The current site is a stock SaaS layout (blue primary, IBM Plex, cards with
shadows, centered hero with pills and stats). The new one follows the
`avoid-ai-design` skill (MIT, <https://github.com/funboy322/avoid-ai-design>):
read its `SKILL.md`, `references/ai-tells-catalog.md`,
`references/aesthetic-directions.md` and `references/design-md-template.md`,
derive a direction from the subject (an open, free, daily-refreshed corpus of
tech internships for students, built on public job boards), write it to
`DESIGN.md`, and build every page from that one contract. Its scanner
(`scripts/detect.mjs`, zero dependencies) runs over templates and CSS as a dev
check. Keep accessibility (focus states, contrast AA, reduced motion,
keyboard use of filters) and keep the copy's meaning.

## Verification

- Unit tests: every TS test ported.
- Golden parity: TS vs Go on real inputs (all titles, locations and
  descriptions in the dump, every company in `data/`, recorded ATS responses).
- State and dump: a state written by TS loads and re-writes byte-identical in
  Go; `dump` from the same state gives identical files apart from
  `generatedAt` and dependent hashes.
- API: the Go server and the TS server on the same dump answer a fixed set of
  queries with identical bodies and headers (fix "now" where freshness
  depends on it).
- Site: every page renders with JS on and off; the scanner reports no P0/P1;
  screenshots at 400 px and 1280 px, light and dark.
- Phase 3: each workstream is reviewed by a different model than wrote it.

## Phases

0. **Lead (done):** branch, `go.mod`, `internal/third_party` (x/text norm),
   `internal/unicodex`, `internal/model`, `cmd/openintern/main.go`
   (subcommand registry: add yours with `register()` from an `init()` in your
   own `cmd_<area>.go`), TS deps installed, dump cached in `.cache/dump/`.
1. **Parallel ports:** core, sources, pipeline, webapi, site.
2. **Tools and infra:** maintenance tools; CI, ingest, dump and ops workflows
   on Go; Vercel adapter; proof gate in Go; hooks; docs. End-to-end run:
   ingest (recorded fixtures) → state → dump → serve.
3. **Review and cut-over:** cross-model review, fixes, delete the TypeScript,
   final docs, hand the branch to the maintainer to commit.
   **2026-11-27:** `apps/web` (Next frontend) deleted; SETUP/README describe
   the live Go+Vercel path. `packages/*` remain as parity reference for Bend
   glue and golden regeneration until a later cleanup.

## Follow-ups after the rewrite (reported 2026-11-26)

- ~~Company logos fail to load on some listings.~~ Done 2026-11-27: the Go
  site rendered no logos at all; logos are now fetched once by
  `openintern logos` into `data/logos` (embedded, served from `/logos/`),
  with an initials mark for the rest. See `data/logos/README.md`.
- ~~Dead apply links / excerpt entities / manual README.~~ Done 2026-11-27:
  `openintern check-apply <jobs.json>` probes dump apply URLs; numeric HTML
  entities (including `&#xa;`) decode in `ExcerptFromHtml`; manual README
  documents `openintern manual-capture`.
- ~~Dead apply links stay on the board.~~ Done 2026-11-27: ingest probes
  apply links (`OPENINTERN_APPLY_PROBE=1`, set in the Ingest workflow) and
  retires a job after 3 dead answers (404/410 confirmed by GET, or a
  redirect that drops the posting id) at least 20h apart; 401/403/429/5xx
  and timeouts never count. Strikes live in `jobs.jsonl`
  (`apply_strikes`, `apply_struck_at`, written only when non-zero). Caps:
  300 probes/run, 40/host, weekly rotation, host backoff on 401/403/429,
  companies losing every link at once held for a person. See
  `docs/STATE-MACHINES.md` §16. Check of the 2026-11-27 dump (1173 links):
  5 dead (Saronic: hosted Ashby board switched off, API still lists the
  jobs); the old check-apply's 13 Pinterest "dead" links were HEAD-only
  404s (GET 200); 105 inconclusive (93 SmartRecruiters 403s).
  Open: propose the strike rule as Bend laws (maintainer's call); SPA apply
  pages (TikTok, ByteDance, Waymo, Workday) answer 200 for closed postings,
  so only board-run retirement catches those.
- `packages/*` remain for Bend glue and golden regeneration; peel further
  when those goldens no longer need the TypeScript tree.
- Neon `export-state` workflow removed 2026-11-27 (state repo is the source
  of truth). Delete any leftover `DATABASE_URL` Actions secret by hand.
