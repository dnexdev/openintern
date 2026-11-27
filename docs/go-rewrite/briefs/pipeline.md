# Brief: pipeline (registry, state, health, dump, ingest)

Owns: `data/embed.go`, `internal/registry/**`, `internal/yamlite/**`,
`internal/state/**`, `internal/ingest/**`, `internal/dump/**`,
`internal/health/**`, `cmd/openintern/cmd_pipeline.go`,
`docs/go-rewrite/status/pipeline.md`. Data files under `data/` are shared:
edit only after asking the lead in the status file. Throwaway files:
`/tmp/openintern-pipeline/`. TypeScript is the read-only reference
(`packages/ingest/node_modules/.bin/tsx`). Also read `docs/RUNBOOK-phase-d.md`.

Dependencies: `internal/proven`, `internal/classify`, `internal/jobslug`
(core, finished; see `status/core.md`); `internal/ats`, `internal/fetch`
(sources, in progress; see `status/sources.md`).

1. `internal/yamlite`: strict parser for the YAML subset in
   `data/companies/*.yaml` and `data/curated/tier-1.yaml`; errors name
   file:line.
2. `data/embed.go` and `internal/registry`: ports of `schema.ts`,
   `sync-companies.ts`, `validate-companies.ts`, `validate-curated.ts`,
   `apply-hosts.ts` + `gen-apply-hosts.ts`, `aliases.ts` + `gen-aliases.ts`,
   `job-aliases.ts`, `delisted.ts`. The TS `*.generated.ts` copies existed only
   because Vercel had no filesystem; with embed, keep only generators that
   still earn their place and record the decision. Expose
   `ApplyURLAllowed(companySlug, url string) bool`, alias and delisted lookups
   (webapi already calls `registry.CanonicalCompanySlug`, `JobAlias`,
   `JobAliasByPrefix`, `ApplyURLAllowed`, `IsDelisted`; keep those names).
   Port `validate-companies.test.ts`, `validate-curated.test.ts`,
   `apply-hosts.test.ts`.
3. `internal/state`: port `state.ts` (load, validate, write, `pgTimestamp`,
   `byCodeUnit`, `stateFromExport`) and `scripts/state-gate.mjs`. A state
   written by TS `writeState` must load and re-write byte-identical in Go,
   including U+2028/2029, `<>&`, emoji, nulls, empty arrays; prove it with a
   tsx harness. Port `state.test.ts`.
4. `internal/health`: port `packages/db/src/health-snapshot.ts` and
   `ingest-health.ts` (non-SQL parts; funnel via `internal/proven`) so
   `health.json` and `/api/v1/health` share one builder.
5. `internal/dump`: port `dump.ts`, `dump-state.ts`, `scripts/dump-gate.mjs`.
   Every file must equal TS output for the same state and fixed clock (compare
   gzip files decompressed).
6. `internal/ingest`: port `ingest-files.ts`, `plan.ts` and the parts of
   `ingest.ts` they use: planning, fetch with concurrency, classification,
   fingerprints and role families, job retirement and run-history auto-disable
   (`internal/proven` laws), `OPENINTERN_APPLY_HOSTS` warn/enforce/off,
   `OPENINTERN_ONLY_SLUGS`, digest (`OPENINTERN_DIGEST_PATH`),
   `GITHUB_STEP_SUMMARY`. Fetcher injected as a function value with the
   signature of `ats.Fetch`; clock and id generator injectable. Port
   `ingest.test.ts`. Parity: TS `ingest-files` and Go from the same starting
   state and fake fetch results (fixed `Date`, `randomUUID`) give
   byte-identical state files.
7. `cmd/openintern/cmd_pipeline.go`: `ingest`, `dump`, `sync-companies`,
   `validate-companies`, `validate-curated`, `import-state`, `state-gate`,
   `dump-gate`, `job-aliases`, and any generator kept. Clear flags; exit codes
   matching the TS scripts (workflows call these).

Publish exported signatures in the status file as each package compiles.

Done: `go vet`, `gofmt -l`, `go test` clean; every TS test ported; byte-parity
evidence for state, dump and ingest; status file complete (PLAN.md rule 10).
Final message: what landed, parity evidence, divergences, requests.
