# State machines

Explicit and implicit state machines in openintern: states, invariants,
transitions, and confirmed sharp edges. Dual Go ⟷ TypeScript paths are noted
where both still exist. Evidence points are `path:line` in this checkout.

Related: Bend decision laws live in [`LAWS.bend`](../LAWS.bend) /
[`docs/PROOFS.md`](PROOFS.md). They are **not** the full app state machines;
they cover a few predicates (rate limit, freshness, retirement, pagination,
slugs) that the machines below call into.

## Glossary

| Term | Meaning |
|---|---|
| **active** (`is_active`) | Stored flag: ingest still considers the listing live |
| **fresh** | Within term apply-by / age / last-seen windows (`proven.IsFresh`) |
| **public** | fresh ∧ has ≥1 role tag (`proven.IsPublic`) |
| **delisted** | Hand denylist; hidden on read, row may still be `is_active` |
| **killed** | Runtime kill-switch hide (job / company / apply host) |
| **dump-published** | Survives dump filters: active ∧ ¬delisted ∧ roles ∧ fresh ∧ apply allowed |

Visibility is layered: storage → dump → board/API live set. A job can be
active in state and still invisible on the site.

---

## 1. Job lifecycle

**Where:** `internal/model/model.go`, `internal/ingest/plan.go`,
`internal/ingest/ingest.go`, `internal/dump/dump.go`, `internal/board/*`,
`internal/ingest/plan.go`, `internal/health/freshness.go`

**Kind:** Explicit `is_active` boolean + implicit public predicates.

### States (storage)

| State | Meaning |
|---|---|
| `is_active=true` | Live according to ingest |
| `is_active=false` | Retired (row kept forever) |

### Derived read states

`stored-active` → `fresh` / `stale` → `public` / not → `delisted` / `killed` /
`off-allowlist` → `dump-published`.

### Invariants

- Jobs are never deleted (`internal/state/gate.go`).
- Identity immutable: `id`, `company_id`, `first_seen_at`.
- Active jobs unique per company + 8-hex URL prefix.
- `IsStale` ≡ ¬`IsFresh` (law `fr_stale_iff_not_fresh`).
- `IsPublic` ≡ roleCount>0 ∧ fresh.

### Transitions

| From → To | Trigger |
|---|---|
| ∅ → active | Upsert matched tech internship, not stale |
| active → inactive | `ShouldRetireJob` (board run × sighting); last-seen sweep (>14d); orphan company; stale rematch upsert |
| inactive → active | Rematch upsert overwrites with `IsActive:!isStale` (no dedicated Reactivate) |
| any → hidden | delist / kill-switch / enforce allowlist (no storage change for delist/kill) |

**Board-run retirement** (`proven.BoardRun` × `JobSighting`):

| Run \ Sighting | gone | rejected | unverified | matched |
|---|---|---|---|---|
| failed | keep | keep | keep | keep |
| empty | retire iff priorEmpty+1 ≥ 3 | same | same | same |
| listed | retire | retire | keep | keep |
| expired | retire | retire | retire | retire |

Fetch failures never call `PlanCompanyRun`; they append `status:error` and
leave jobs alone (`internal/ingest/ingest.go` around the fetch-error branch).
`BoardRunFailed` exists in the proven API for the laws; the planner emits
listed / empty / expired only.

### Concurrency / retries

Ingest fetches companies in chunks of 8 (`Concurrency`) with a wait-group;
state upserts run **sequentially** after each chunk completes. No shared
mutable job map is written from goroutines. HTTP fetch retries (3 attempts,
backoff on 429/5xx/network) stay inside one company's fetch and do not
interleave state writes.

### Reachable sharp edges

1. **Reactivation by overwrite** — an inactive job can become active again
   on the next successful match without an explicit transition helper
   (`internal/ingest/ingest.go` upsert path). Intended for reopened roles;
   easy to miss in ops reasoning.
2. **Delisted while still active** — denylist hides in dump/board; state
   may keep `is_active=true` until a normal retirement path fires.

---

## 2. Company control (three layers)

**Where:** YAML `active`, `internal/ingest/ingest.go` auto-disable,
empty-run counter via `proven.NextEmptyRuns`.

### 2a. YAML / state `active`

| State | Transition |
|---|---|
| active / inactive | Registry sync copies YAML each run; orphan companies forced inactive |

### 2b. Auto-disable overlay (not written to `active`)

| State | Rule |
|---|---|
| Enabled | default |
| Disabled | 7 consecutive error runs (tier-1 / manual ATS exempt) |
| Recheck | Disabled and last failure ≥7 days ago → fetch once |

**Invariant (claimed in code comments):** YAML `active` is the human switch;
auto-disable must not be silently undone by registry sync. Effect: a company
can be `active:true` in state while skipped as auto-disabled.

### 2c. Consecutive empty-run counter

Not stored; recomputed from run history. Empty ok-runs increment; listed
resets to 0; failed leaves unchanged. At prior ≥ 2, the next empty run
retires all active jobs for that company (`RetireAfterEmptyRuns = 3`).

---

## 3. Ingest run records

**Where:** `internal/model` runs, `internal/ingest/plan.go`

| Status | When | Retires jobs? |
|---|---|---|
| `ok` | Successful fetch+plan (incl. empty / zero_match) | per retirement table |
| `error` | Fetch failure | no |
| `error` | Manual capture expired | **yes** (`BoardRunExpired`) |

`error` text is dual-use: failure message, or on `ok` runs a funnel /
`zero_match:` report. Retention: 60 days of runs.

**Sharp edge:** manual-expired looks like a failed run in status but is the
one error-shaped path that retires everything. Intentional; easy to misread
in digests.

---

## 4. State write gate

**Where:** `internal/state/gate.go`

| Outcome | Meaning |
|---|---|
| ok (0) | New state may replace current |
| refused (1) | Integrity or volume swing |

Invariants: no duplicate ids/slugs/active URL prefixes; no disappearing
ids; immutable company/first_seen; ≤200 new jobs/company unless
`Approved("company:"+slug)`; no ≥5→0 active collapse on a still-active
company unless approved.

---

## 5. Dump gate (hold / publish)

**Where:** `internal/dump/gate.go`,
`cmd/openintern` `dump-gate`

| Outcome | Meaning |
|---|---|
| publish (0) | Release may proceed |
| hold (1) | Problems and/or circuit-breaker Holds |

Checks: MANIFEST integrity; no delisted ids; apply URL allowlist
(enforce → Problem, warn → count); vs previous dump: |Δtotal|/prev > 10%,
ATS ≥5 → 0, ATS drop >10 and >15% — unless `data/expected-delta.json`
approves `total` / `ats:<name>` with a future `until` and reason.

### Confirmed bug (fixed): env parse mismatch

- Ingest / dump build use `ApplyHostModeFromEnv()` (trims whitespace).
- Dump-gate CLI and `scripts/dump-gate.mjs` historically used
  `os.Getenv(...) == "enforce"` / untrimmed equality.
- Reachable: `OPENINTERN_APPLY_HOSTS="enforce "` → dump filters off-list
  jobs, but the gate stayed in warn mode and would not Problem on any that
  slipped through another path.
- See GitHub issue linked from the changelog below.

---

## 6. Expected-delta approvals

**Where:** `internal/registry/expected.go`, `data/expected-delta.json`

Per entry: valid ∧ scope match ∧ `until ≥ today` ∧ non-empty reason →
approved; else inert. Scopes: `total`, `ats:<name>`, `company:<slug>`.

---

## 7. Apply-hosts mode

**Where:** `internal/registry/applyhosts.go`

| Mode | Effect |
|---|---|
| `off` | no checks |
| `warn` (default) | log once, allow |
| `enforce` | block ingest upsert / dump / board |

Check reasons: `not-https` | `malformed` | `credentials-or-port` |
`unknown-company` | `off-list`.

---

## 8. Rate-limit bucket

**Where:** Bend `rate_limit.bend`, `internal/web/rate_limit.go`

| State | Meaning |
|---|---|
| no bucket | first request opens a window |
| open | admit while `count < limit` |
| exhausted | deny until `reset_at` |
| expired | `reset_at ≤ now` → fresh window |

Invariants (laws): `remaining = limit - count`; admitted count ≤ limit;
`limit=0` admits nothing; open window keeps `reset_at`.

**Panic / overflow:** wrappers fail **open** (allow the request).

**Env:** unset → 60/min. Present but empty string → **0 → deny all**
(Go and TS). Treat as an emergency kill, not a typo-friendly default.

Progress laws `rl_admit_progress` / `rl_fresh_progress` require admit while
`count < limit` and a positive-limit fresh window (see `docs/PROOFS.md`).

---

## 9. Kill switch + maintenance

**Where:** `internal/board/kill_switch.go`

Fields: `jobs` / `companies` / `hosts` sets, `maintenance`, `message`.
Site maintenance = env `OPENINTERN_MAINTENANCE=1` ∨ kill-switch flag.
Failed refresh keeps last good cache. Malformed apply URL with a non-empty
`hosts` list → fail-closed hide (security).

---

## 10. Freshness / public predicates

Constants (PINNED in Bend): day_ms=86400000, max age without terms=120d,
last-seen sweep=14d. Term apply-by: summer Apr 30; fall Aug 31;
winter/spring Dec 31 of prior year.

Board / dump recover from invalid dates → not public. SQL freshness path
can disagree with Bend by up to one day (`docs/PROOFS.md`).

---

## 11. Dump / API corpus age

**Where:** `internal/board` `MaxDumpAge=30h`, `/api/v1/status`, Next status
route.

| `/api/v1/status` | Meaning |
|---|---|
| `ok` | dump age ≤ 30h |
| `stale` | age > 30h (503) |
| `down` | dump unreadable |

Go site banner uses `StaleData` only when age > 30h.

### Confirmed bug (fixed, Next path removed): stale banner on dump-first

The old Next.js board showed a “DB offline” banner whenever the data
source was the dump (the default). Go `internal/site` already gated on
`MaxDumpAge` (30h). The Next frontend is deleted in phase 3; see
changelog below.

---

## 12. Manual capture age

Usable | missing | expired (>14d). Expired → empty raw + retire all +
run `status:error` (see §3).

---

## 13. HTTP fetch failure machine

Kinds: `refused` | `throttled` | `not-found` | `http` | `network` |
`invalid` | `manual-expired`. Retries on 429/5xx/network; immediate fail
on 401/403/404/410. Refusal ≠ empty board.

---

## 14. Dump cache / data-source preference

Cache hit within 1h; fetch fail keeps last. The board reads the dump only
(no database fallback on the request path).

---

## 15. Client “applied” marks

Browser localStorage only; not server state.

---

## Publish pipeline (machines composed)

```
YAML registry
    → ingest (fetch∥8, sequential upserts)
    → state-gate
    → dump
    → dump-gate (+ expected-delta)
    → GitHub release dump-latest
    → board/API cache (≤1h) → liveJobs filters
```

---

## Priority of confirmed issues

| Impact | Issue | Status |
|---|---|---|
| High (user-facing false alarm on every page) | [#59](https://github.com/dnexdev/openintern/issues/59) Next stale banner vs dump-age | fixed |
| Medium (gate/ingest mode skew under padded env) | [#58](https://github.com/dnexdev/openintern/issues/58) dump-gate apply-hosts env trim | fixed |
| Low (ops semantics) | dual company active vs auto-disable; manual-expired as `error` | documented, not bugs |
| Enhancement | Bend rate-limit progress law | fixed — `rl_admit_progress` / `rl_fresh_progress` |

### Changelog

- 2026-11-27: Initial inventory. Fixed [#58](https://github.com/dnexdev/openintern/issues/58)
  (dump-gate env trim) and [#59](https://github.com/dnexdev/openintern/issues/59)
  (stale banner gating); regression tests added. Bend audit in PROOFS.md.
- 2026-11-27: Phase 3 — deleted `apps/web` (Next frontend); docs/SETUP and
  `.env.example` describe the Go+Vercel path ([#60](https://github.com/dnexdev/openintern/issues/60)).
