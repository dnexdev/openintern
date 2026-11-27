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

## Are the laws strong enough?

A law that only restates its definition proves nothing useful. Two checks keep
the laws honest:

- **PINNED laws** fix constants and concrete examples (the 120-day age limit,
  14 days unseen, one day in ms, which characters are hex, what parses as a
  page number). Changing one of those values breaks a proof.
- **Mutants** (`packages/proven/mutants.tsv`, `node scripts/proofs.mjs
  mutants`, run in CI): each row is a plausible bug, such as "offset ignores
  limit", "#24: gone jobs kept", or "deny-all limiter". The mutated tree must
  fail to check. A surviving mutant fails CI until a law catches it.

Limits: a mutant also "dies" if it only breaks a proof script rather than
falsifying a law. The pinned laws are concrete, so for them the two coincide;
for property laws, a killed mutant is evidence, not proof, that the law would
catch the bug.

### Audit (2026-11-27): not troll, incomplete on purpose

Checked against `LAWS.bend`, `PROOF.bend`, `packages/proven/mutants.tsv`,
the `bend` CI job in `.github/workflows/ci.yml`, and the Go port vectors.

**Why these are real laws, not theater:**

- Most laws are quantified properties (`for +…`), not single concrete
  examples. PINNED laws exist on purpose so a constant drift fails CI.
- Proofs that look like `match … {==}` are definitional equality after
  case split — normal for this style of checker, not a vacuous “assert true”.
- CI runs `openintern proofs check` and `proofs mutants` with a pinned Bend
  binary, no network, and the gate rejects `@unsafe`, foreign JS imports,
  hub packages, and fake `main`/IO that could print a pass line.
- Mutants encode past production bugs (#24 gone-jobs-kept, `?page=abc` →
  NaN, limit 0 admitting the first request). Killing them is evidence the
  corresponding laws constrain the model.
- The Go port re-checks the same properties in `internal/proven` (laws tests
  + vectors regenerated from Bend).

**Known gaps (documented, not hidden):**

- No **progress** law for the rate limiter: “`limit > 0` and `count < limit`
  ⇒ admit”. Safety laws (`rl_bounded`, `rl_remaining`, `rl_zero_limit`) and
  the “deny first request of a fresh window” mutant hold; a limiter that
  admits the first request of a window and then denies every later request
  while keeping `remaining` consistent can still pass. Stronger progress
  laws remain planned (see the paragraph above this audit).
- Wrappers, SQL freshness, Bend Base, the compiler, and the JS runtime are
  out of scope (as the header of this file already says).
- App-level state machines (ingest retirement sequencing, dump gates, kill
  switch, auto-disable) are **not** Bend laws; they are inventoried in
  [`docs/STATE-MACHINES.md`](STATE-MACHINES.md).
