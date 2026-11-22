import Base
import ./packages/proven/bend/rate_limit.bend as RL
import ./packages/proven/bend/freshness.bend as FR
import ./packages/proven/bend/apply_picker.bend as AP
import ./packages/proven/bend/deactivation.bend as DP
import ./packages/proven/bend/funnel.bend as FN
import ./packages/proven/bend/pagination.bend as PG
import ./packages/proven/bend/job_slug.bend as JS

# LAWS.bend: the specification of openintern's decision logic.
#
# Every law below is a claim PROOF.bend must prove. `bend PROOF.bend` prints
# ALL PROOFS CHECK only when every one holds. This file is the human-owned
# spec: change a law only on purpose, never to make a proof go through.

# ---------------------------------------------------------------------------
# Rate limiter (apps/web/src/lib/rate-limit.ts)
# ---------------------------------------------------------------------------

# Whatever happens, `remaining` is exactly the unused part of the limit.
law rl_remaining:
  for +prev: Maybe<&2, RL.Bucket>
  for +now: Nat
  for +limit: Nat
  for +window: Nat
  {RL.remaining(RL.step(prev, now, limit, window))
    == Nat.sub(limit, RL.count(RL.step(prev, now, limit, window))) : Nat}

# An admitted request never pushes a bucket past its limit.
law rl_bounded:
  for +prev: Maybe<&2, RL.Bucket>
  for +now: Nat
  for +limit: Nat
  for +window: Nat
  {Bool.or(Bool.not(RL.ok(RL.step(prev, now, limit, window))),
    Nat.is_le(RL.count(RL.step(prev, now, limit, window)), limit)) == True{} : Bool}

# A limit of 0 admits nothing (the old TS admitted the first request).
law rl_zero_limit:
  for +prev: Maybe<&2, RL.Bucket>
  for +now: Nat
  for +window: Nat
  {RL.ok(RL.step(prev, now, 0n, window)) == False{} : Bool}

# Once a window has ended (reset_at <= now), the old bucket is ignored.
law rl_expired:
  for +count: Nat
  for +reset_at: Nat
  for +k: Nat
  for +limit: Nat
  for +window: Nat
  {RL.step(Some{RL.Bucket{count, reset_at}}, Nat.add(reset_at, k), limit, window)
    == RL.fresh(Nat.add(reset_at, k), limit, window) : RL.Verdict}

# While a window is open (now < reset_at), the request is admitted or denied
# against that window's bucket.
law rl_open_window:
  for +count: Nat
  for +now: Nat
  for +k: Nat
  for +limit: Nat
  for +window: Nat
  {RL.step(Some{RL.Bucket{count, 1n+Nat.add(now, k)}}, now, limit, window)
    == RL.admit(count, limit, 1n+Nat.add(now, k)) : RL.Verdict}

# Inside a window, every admitted request is counted once and a denied one
# not at all; with rl_bounded, a window admits at most `limit` requests.
law rl_admit_count:
  for +count: Nat
  for +limit: Nat
  for +reset_at: Nat
  {RL.count(RL.admit(count, limit, reset_at))
    == RL.bump(RL.ok(RL.admit(count, limit, reset_at)), count) : Nat}

# Requests inside a window never move its end.
law rl_admit_keeps_window:
  for +count: Nat
  for +limit: Nat
  for +reset_at: Nat
  {RL.reset_at(RL.admit(count, limit, reset_at)) == reset_at : Nat}

# A new window lasts exactly `window` from the request that opens it.
law rl_fresh_window:
  for +now: Nat
  for +limit: Nat
  for +window: Nat
  {RL.reset_at(RL.fresh(now, limit, window)) == Nat.add(now, window) : Nat}

# ---------------------------------------------------------------------------
# Freshness (packages/db/src/freshness.ts, packages/ingest/src/classifier.ts,
# apps/web/src/lib/job-dump.ts)
# ---------------------------------------------------------------------------

# Ingest's "stale" and the board's "fresh" are exact opposites, so a job is
# never both deactivated and shown, or neither.
law fr_stale_iff_not_fresh:
  for +ends: List<&2, Nat>
  for +anchor: Nat
  for +now: Nat
  for +max_ms: Nat
  {FR.is_stale(ends, anchor, now, max_ms)
    == Bool.not(FR.is_fresh(ends, anchor, now, max_ms)) : Bool}

# A stale job stays stale as time passes.
law fr_stale_monotone:
  for +ends: List<&2, Nat>
  for +anchor: Nat
  for +now: Nat
  for +k: Nat
  for +max_ms: Nat
  for e: {FR.is_stale(ends, anchor, now, max_ms) == True{} : Bool}
  {FR.is_stale(ends, anchor, Nat.add(now, k), max_ms) == True{} : Bool}

# So does a job that ingest has stopped seeing.
law fr_last_seen_monotone:
  for +seen: Nat
  for +now: Nat
  for +k: Nat
  for +max_ms: Nat
  for e: {FR.stale_last_seen(seen, now, max_ms) == True{} : Bool}
  {FR.stale_last_seen(seen, Nat.add(now, k), max_ms) == True{} : Bool}

# A job is fresh up to the instant one of its terms ends.
law fr_term_live_at_end:
  for +h: Nat
  for +t: List<&2, Nat>
  for +anchor: Nat
  for +max_ms: Nat
  {FR.is_fresh(Con{h, t}, anchor, h, max_ms) == True{} : Bool}

# A job without terms is fresh until its age passes max_ms.
law fr_age_live_at_limit:
  for +anchor: Nat
  for +max_ms: Nat
  {FR.is_fresh(Nil{}, anchor, Nat.add(anchor, max_ms), max_ms) == True{} : Bool}

# A public row has at least one role tag ...
law fr_public_needs_role:
  for +ends: List<&2, Nat>
  for +anchor: Nat
  for +now: Nat
  for +max_ms: Nat
  {FR.is_public(0n, ends, anchor, now, max_ms) == False{} : Bool}

# ... and with one, public means fresh.
law fr_public_is_fresh:
  for +roles: Nat
  for +ends: List<&2, Nat>
  for +anchor: Nat
  for +now: Nat
  for +max_ms: Nat
  {FR.is_public(1n+roles, ends, anchor, now, max_ms)
    == FR.is_fresh(ends, anchor, now, max_ms) : Bool}

# Term windows (zero-based months): summer ends Aug 31, fall Dec 31, winter
# Apr 30. An unknown term ends Dec 31, as in freshnessSql().
law fr_summer_end:
  {FR.term_end(FR.Summer{}) == FR.MonthDay{7n, 31n} : FR.MonthDay}

law fr_fall_end:
  {FR.term_end(FR.Fall{}) == FR.MonthDay{11n, 31n} : FR.MonthDay}

law fr_winter_end:
  {FR.term_end(FR.Winter{}) == FR.MonthDay{3n, 30n} : FR.MonthDay}

law fr_other_end:
  {FR.term_end(FR.Other{}) == FR.term_end(FR.Fall{}) : FR.MonthDay}

# ---------------------------------------------------------------------------
# Apply picker (apps/web/src/lib/posting-label.ts)
# ---------------------------------------------------------------------------

# An empty family has nothing to pick.
law ap_empty:
  for +locations: Nat
  {AP.needs_picker(0n, locations) == False{} : Bool}

# Two or more postings always need a picker.
law ap_many_postings:
  for +p: Nat
  for +locations: Nat
  {AP.needs_picker(2n+p, locations) == True{} : Bool}

# One posting needs a picker exactly when it lists two or more locations.
law ap_one_posting:
  for +locations: Nat
  {AP.needs_picker(1n, locations) == Nat.is_le(2n, locations) : Bool}

# A lone posting's title names its locations exactly when a picker shows.
law ap_title_matches_picker:
  for +locations: Nat
  {AP.qualify_single_title(locations) == AP.needs_picker(1n, locations) : Bool}

# All-distinct locations read "N locations" ...
law ap_distinct_says_locations:
  for +n: Nat
  {AP.says_openings(n, n) == False{} : Bool}

# ... and any repeated location reads "N openings".
law ap_repeats_say_openings:
  for +unique: Nat
  for +k: Nat
  {AP.says_openings(1n+Nat.add(unique, k), unique) == True{} : Bool}

# ---------------------------------------------------------------------------
# Deactivation policy (packages/ingest/src/ingest.ts)
# ---------------------------------------------------------------------------

# A failed fetch never deactivates anything.
law dp_failed_fetch_keeps:
  for +fetched: Nat
  for +tech: Nat
  {DP.policy(False{}, fetched, tech) == DP.Keep{} : DP.Policy}

# With matches, only postings missing from the board are retired.
law dp_matches_retire_missing:
  for +fetched: Nat
  for +tech: Nat
  {DP.policy(True{}, fetched, 1n+tech) == DP.Missing{} : DP.Policy}

# A board that is genuinely empty retires everything.
law dp_empty_board_retires_all:
  {DP.policy(True{}, 0n, 0n) == DP.All{} : DP.Policy}

# A non-empty board with zero classifier matches keeps existing jobs.
law dp_zero_match_keeps:
  for +fetched: Nat
  {DP.policy(True{}, 1n+fetched, 0n) == DP.Keep{} : DP.Policy}

# ---------------------------------------------------------------------------
# Ingest-health funnel (apps/web/src/lib/ingest-health.ts)
# ---------------------------------------------------------------------------

law fn_total_empty:
  {FN.total(Nil{}) == FN.zero() : FN.Totals}

# Summing in pieces gives the same totals as summing at once.
law fn_total_append:
  for +xs: List<&2, FN.Funnel>
  for +ys: List<&2, FN.Funnel>
  {FN.total(List.append(&2, FN.Funnel, xs, ys))
    == FN.plus(FN.total(xs), FN.total(ys)) : FN.Totals}

# Any error tagged zero_match: is a zero-match run.
law fn_zero_match_tagged:
  for +rest: String
  {FN.is_zero_match(FN.zero_match_tag() ++ rest) == True{} : Bool}

# A funnel report is not.
law fn_funnel_not_zero_match:
  for +rest: String
  {FN.is_zero_match("funnel " ++ rest) == False{} : Bool}

# ---------------------------------------------------------------------------
# Pagination (apps/web/src/lib/board.ts, apps/web/src/app/api/v1/jobs)
# ---------------------------------------------------------------------------

# Any query text, even garbage, gives a page within [1, 100000] ...
law pg_page_min:
  for +raw: Maybe<&2, String>
  {Nat.is_le(1n, PG.page(raw)) == True{} : Bool}

law pg_page_max:
  for +raw: Maybe<&2, String>
  {Nat.is_le(PG.page(raw), 100000n) == True{} : Bool}

# ... and a limit within [1, 100].
law pg_limit_min:
  for +raw: Maybe<&2, String>
  {Nat.is_le(1n, PG.limit(raw)) == True{} : Bool}

law pg_limit_max:
  for +raw: Maybe<&2, String>
  {Nat.is_le(PG.limit(raw), 100n) == True{} : Bool}

# Missing params fall back to page 1 and 27 per page.
law pg_defaults:
  {(PG.page(None{}), PG.limit(None{})) == (1n, 27n) : Nat & Nat}

# Page 1 starts at the first row.
law pg_first_page_offset:
  for +limit: Nat
  {PG.offset(1n, limit) == 0n : Nat}

# ---------------------------------------------------------------------------
# Job slugs (apps/web/src/lib/job-slug.ts)
# ---------------------------------------------------------------------------

# Whatever the title part is, a slug ending in "-" and an id prefix parses
# back to exactly that prefix, so every job link resolves to its job.
law js_roundtrip:
  for +base: String
  for +id: JS.Hex8
  {JS.parse(base ++ SCon{'-', JS.hex8_str(id)}) == Some{JS.hex8_str(id)}
    : Maybe<&2, String>}
