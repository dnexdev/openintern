# Brief: tools (maintenance commands)

Owns: `internal/tools/**`, `cmd/openintern/cmd_tools.go`,
`docs/go-rewrite/status/tools.md`. Throwaway files: `/tmp/openintern-tools/`.
TypeScript is the read-only reference (`packages/ingest/node_modules/.bin/tsx`).

Port, as `openintern` subcommands registered from `cmd_tools.go`:
`gap-report.ts`, `discover-accelerators.ts`, `recover-tokens.ts`,
`validate-tokens.ts` (and `gen-aliases.ts` only if pipeline's status says it
was not kept and it still earns a place). Build on `internal/ats`
(`probe`, `parse-ats-url`), `internal/fetch`, `internal/registry`,
`internal/yamlite`; do not copy their logic. Same flags, output and exit codes
as the TS scripts. Port `gap-report.test.ts` (non-parse-ats-url parts) and add
tests for the rest with recorded fixtures (no live network in tests; any live
check: sequential, honest client, a handful of requests). Golden parity vs TS
on the same fixtures where the output is deterministic.

Done: vet/gofmt/test clean; status file complete (PLAN.md rule 10). Commit
your own paths when done (see README). Final message: what landed, parity
evidence, divergences, requests.
