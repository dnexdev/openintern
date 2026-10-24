# Proven decision logic (Bend)

The app's decision logic is written in [Bend](https://github.com/bendlang/bend)
and **proven**, not only tested. If code doesn't satisfy its spec, it doesn't
pass the gate, so an AI (or a human) can't ship a change that breaks one.

| File | Role | Who edits it |
|---|---|---|
| `LAWS.bend` | The spec: 40 laws, each a claim that must hold | Humans, on purpose |
| `PROOF.bend` | A proof for every law | Whoever changes the code |
| `packages/proven/bend/*.bend` | The implementations | Whoever changes the code |
| `packages/proven/generated/proven.mjs` | Compiled from `PROOF.bend` (committed) | `pnpm proofs:build` only |
| `packages/proven/src/index.ts` | Typed TS wrappers (marshalling only, no decisions) | Whoever changes the code |

## What is proven

- **Rate limiter** (`rate-limit.ts`): `remaining = limit - count`; an admitted
  request never exceeds the limit; a limit of 0 admits nothing; expired windows
  reset; open windows keep their end.
- **Freshness** (`db/freshness.ts`, `ingest/classifier.ts`, `job-dump.ts`):
  ingest's "stale" is exactly `!fresh` on the board; staleness is monotone in
  time; term end dates; public rows need a role.
- **Apply picker** (`posting-label.ts`): a picker shows iff there are ≥2 postings
  or ≥2 locations; "openings" vs "locations" copy.
- **Deactivation policy** (`ingest.ts`): a failed fetch never deactivates; a
  zero-match board keeps its jobs.
- **Funnel totals** (`ingest-health.ts`): summing in pieces equals summing at once.
- **Pagination** (`board.ts`, `/api/v1/jobs`): any query text gives page in
  [1, 100000] and limit in [1, 100]. This fixed `?page=abc`, which used to give `NaN`.
- **Job slugs** (`job-slug.ts`): `parse(base + "-" + id8) = id8` for every title.

What stays in TypeScript: anything that uses regexes or Unicode classes
(classifier text matching, title normalization, HTML stripping, ATS URL parsing,
`slugifyTitle`). Bend strings have no regex support.

## Commands

```bash
pnpm proofs         # every law proven + generated module up to date (CI runs this)
pnpm proofs:build   # check the proofs, then regenerate proven.mjs
pnpm test           # also runs packages/proven/src/proven.test.ts (TS <-> Bend glue)
```

`scripts/proofs.mjs` also rejects `@unsafe` defs, foreign `import "./x.js"`
code, and hub packages, which would otherwise slip past the proofs. When `lean`
(v4.34.0) is on PATH it uses `bend --verdict`, which rechecks everything with
the Lean-verified kernel.

## Claude Code hooks (`.claude/settings.json`)

- Editing `LAWS.bend` always asks for your approval first.
- After any `.bend` edit, the proofs are rechecked and failures are fed back to Claude.
- Claude can't end a turn that touched proven code while `pnpm proofs` fails.

## Telemetry

`bend` pings `bend-lang.com/check` once a day, and its embedded Bun runtime can
send crash reports to `bun.report`. Both are off wherever bend runs here:
`BEND_NO_TELEMETRY=1 DO_NOT_TRACK=1 BUN_ENABLE_CRASH_REPORTING=0` is set by
`scripts/proofs.mjs`, the Claude Code settings `env`, and CI. Under `strace`
the gate makes no network connections, and it passes with networking disabled
(`unshare -rn pnpm proofs`). `bend login`, `bend update`, `bend link`, and
`--publish` are denied for Claude.
