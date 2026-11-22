# Proven decision logic (Bend)

Part of the app's decision logic is written in [Bend](https://github.com/bendlang/bend)
as a machine-checked reference model. A change to the Bend functions that breaks
a law in `LAWS.bend` fails the gate.

**What that guarantees, and what it doesn't.** The Bend functions satisfy the
stated laws, as checked by bend2. Not covered:
- the TS wrappers in `packages/proven/src/index.ts`, and the SQL freshness
  query in `packages/db/src/freshness.ts` (the board's DB path uses SQL, not Bend);
- Bend's Base library, its compiler and its JS runtime. Numbers past 2^48 throw
  at runtime, and nothing proves the compiled `proven.mjs` matches the checked
  source;
- whether the laws are the right ones. Several pin a definition rather than
  state a property, and some useless implementations still pass (for example,
  a rate limiter that denies everyone). Stronger "progress" laws are planned.

| File | Role | Who edits it |
|---|---|---|
| `LAWS.bend` | The spec: 40 laws, each a claim that must hold | Humans, on purpose |
| `PROOF.bend` | A proof for every law | Whoever changes the code |
| `packages/proven/bend/*.bend` | The implementations | Whoever changes the code |
| `packages/proven/generated/proven.mjs` | Compiled from `PROOF.bend` (committed) | `pnpm proofs:build` only |
| `packages/proven/src/index.ts` | Typed TS wrappers (marshalling only, no decisions) | Whoever changes the code |

## What the laws cover

- **Rate limiter** (`rate-limit.ts`): `remaining = limit - count`; an admitted
  request never exceeds the limit; a limit of 0 admits nothing; expired windows
  reset; open windows keep their end. Per instance only: each server instance
  has its own buckets.
- **Freshness** (`ingest/classifier.ts`, `job-dump.ts`, `isFreshJob`): the Bend
  `is_stale` is exactly `!is_fresh`; staleness is monotone in time; term end
  dates; public rows need a role. The board's SQL path (`freshnessSql`) works
  in whole days and can disagree with Bend for up to a day.
- **Apply picker** (`posting-label.ts`): a picker shows iff there are ≥2 postings
  or ≥2 locations; "openings" vs "locations" copy.
- **Job retirement** (`ingest.ts`): a failed fetch retires nothing; a job gone
  from a board fetched successfully is retired, even when nothing else on the
  board matches (#24); a job whose details failed to load is kept; an empty
  board retires a company's jobs only on the third empty run in a row.
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

A pass needs bend's exit status 0 and exactly its success output.
`scripts/proofs.mjs` also rejects `@unsafe` defs, foreign `import "./x.js"`
code, hub packages, imports from outside the proof tree, and any `main` or IO
in the proof files (which could print a fake pass line).

`BEND_VERDICT=1 pnpm proofs` adds `bend --verdict`, which rechecks the
non-Base defs with a kernel whose consistency is proven in Lean (needs Lean
v4.34.0). CI does not run it, and the translation and compiler are not
covered by that proof.

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
