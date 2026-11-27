# Brief: sources (fetch, HTML helpers, ATS adapters)

Owns: `internal/fetch/**`, `internal/htmlx/**`, `internal/safedate/**`,
`internal/ats/**`, `cmd/openintern/cmd_sources.go`,
`docs/go-rewrite/status/sources.md`. Throwaway files: `/tmp/openintern-sources/`.
TypeScript under `apps/`, `packages/`, `scripts/` is the read-only reference;
`pnpm install` is done (run TS with `packages/ingest/node_modules/.bin/tsx`).

1. `internal/fetch`: port `packages/ingest/src/http.ts`. Honest client: fixed
   User-Agent `OpenIntern/0.1 (+https://github.com/dnexdev/openintern)`,
   callers cannot set User-Agent, Referer, Cookie or other identity headers;
   timeouts, retries, Retry-After parsing; typed failures (`*fetch.Failure`
   with the FetchFailureKind values) inspected with `errors.As`. Every I/O
   function takes a `context.Context`. Port `http.test.ts` with `httptest`.
   Port the intent of `scripts/check-honest-requests.mjs` as a Go test that
   scans `internal/**/*.go` and fails if code outside `internal/fetch` builds
   its own `http.Client`/`http.Request` or sets those headers.
2. `internal/htmlx` (`strip-html.ts`, including leading emoji stripping via
   `unicodex.IsExtendedPictographic`) and `internal/safedate` (`safe-date.ts`).
3. `internal/ats`: port `ats.ts` and every adapter (`workday.ts`,
   `avature.ts`, `bytedance.ts`, `citadel.ts`, `deshaw.ts`, `eightfold.ts`,
   `google.ts`, `ibm.ts`, `icims.ts`, `phenom.ts`, `shopify.ts`, `tesla.ts`),
   `manual.ts` (reads committed captures under `data/manual/`; the embedded
   copy is `data.FS` from package `data`), `parse-ats-url.ts`,
   `probe-url.ts`, and the concurrency helper. Entry points, exactly:
   `ats.Fetch(ctx context.Context, atsName, boardToken string) ([]model.NormalizedJob, error)`
   and `ats.IsProprietary(name string) bool`. Keep the "X is not ingested" and
   unsupported-ATS errors. A failed fetch must surface a `*fetch.Failure` so
   ingest can tell "board fetched and empty" from "fetch failed" (the
   job-retirement laws depend on it). Port every case in `ats.test.ts`,
   `proprietary.test.ts`, `manual.test.ts`.
4. Parity with TypeScript: record real responses once per ATS type (one
   representative company per ATS from `data/companies/*.yaml`; sequential,
   honest User-Agent, a few requests per ATS at most; trim to a handful of
   jobs) as fixtures in `internal/ats/testdata/`. Run the TS adapter (tsx
   script with `globalThis.fetch` mocked to serve the fixtures) and the Go
   adapter (httptest server or a fixture transport in `internal/fetch`) over
   the same fixtures; Go tests assert equality with the TS output stored as
   goldens. Adapters you cannot record (site down, blocks automated requests)
   go under "Blocked on", tested from synthetic fixtures matching the TS
   parser.
5. `cmd/openintern/cmd_sources.go`: register `probe <url>` (probe-url CLI)
   and `manual-capture` (`manual-capture.ts`) with `register()` from `init()`.

Publish `internal/ats` and `internal/fetch` exported signatures in the status
file as soon as they compile (pipeline and tools code against them).

Done: `go vet`, `gofmt -l`, `go test` clean for these packages; every TS test
ported; fixture parity for every adapter; status file complete (PLAN.md rule
10). Final message: what landed, parity evidence per adapter, divergences,
requests.
