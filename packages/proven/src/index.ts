/**
 * Typed entry to the Bend-proven decision logic.
 *
 * The decisions live in packages/proven/bend/*.bend, are specified by
 * LAWS.bend and proven by PROOF.bend; ../generated/proven.mjs is compiled
 * from PROOF.bend. This file only converts between TS values and Bend
 * values. Keep decisions out of it: anything decided here is unproven.
 */
import proven from "../generated/proven.mjs";

type BendList<T> = { $: "Nil" } | { $: "Con"; head: T; tail: BendList<T> };
type BendMaybe<T> = { $: "None" } | { $: "Some"; value: T };

const MODULE_ROOT = "packages/proven/bend";

function def<A extends unknown[], R>(module: string, name: string): (...args: A) => R {
  const fn = proven[`${MODULE_ROOT}/${module}:${name}`] as unknown as ((...args: A) => R) | undefined;
  if (!fn) throw new Error(`proven.mjs has no ${module}:${name}; run pnpm proofs:build`);
  // The Bend runtime throws a bare string on overflow; surface it as an Error.
  return (...args: A) => {
    try {
      return fn(...args);
    } catch (err) {
      throw err instanceof Error ? err : new Error(`proven ${module}:${name}: ${String(err)}`);
    }
  };
}

/** Bend's JS runtime keeps Nats below 2^48 and throws past that. */
export const NAT_LIMIT = 2 ** 48;

/** A Bend Nat: an integer in [0, 2^48). Anything else is a caller bug, so throw. */
function nat(n: number, what = "value"): number {
  if (!Number.isInteger(n) || n < 0 || n >= NAT_LIMIT) {
    throw new RangeError(`${what} must be an integer in [0, 2^48), got ${n}`);
  }
  return n;
}

function list<T>(xs: readonly T[]): BendList<T> {
  let out: BendList<T> = { $: "Nil" };
  for (let i = xs.length - 1; i >= 0; i--) out = { $: "Con", head: xs[i]!, tail: out };
  return out;
}

function maybe<T>(x: T | null | undefined): BendMaybe<T> {
  return x == null ? { $: "None" } : { $: "Some", value: x };
}

// ---------------------------------------------------------------------------
// Rate limiter
// ---------------------------------------------------------------------------

export type RateBucket = { count: number; resetAt: number };
export type RateVerdict = { ok: boolean; remaining: number; bucket: RateBucket };

type BendBucket = { $: "Bucket"; count: number; reset_at: number };
type BendVerdict = { $: "Verdict"; ok: boolean; remaining: number; bucket: BendBucket };

const rlStep = def<[BendMaybe<BendBucket>, number, number, number], BendVerdict>(
  "rate_limit",
  "step",
);

/** One request against a key's bucket (laws rl_*). */
export function rateLimitStep(
  prev: RateBucket | undefined,
  now: number,
  limit: number,
  windowMs: number,
): RateVerdict {
  if (windowMs < 1) throw new RangeError(`windowMs must be at least 1, got ${windowMs}`);
  const bucket = prev && {
    $: "Bucket" as const,
    count: nat(prev.count, "bucket count"),
    reset_at: nat(prev.resetAt, "bucket resetAt"),
  };
  const v = rlStep(maybe(bucket), nat(now, "now"), nat(limit, "limit"), nat(windowMs, "windowMs"));
  return {
    ok: v.ok,
    remaining: v.remaining,
    bucket: { count: v.bucket.count, resetAt: v.bucket.reset_at },
  };
}

// ---------------------------------------------------------------------------
// Freshness
// ---------------------------------------------------------------------------

export type TermYearLike = { term: string; year: number };

const termEndDef = def<[{ $: string }], { month0: number; day: number }>("freshness", "term_end");
const isFreshDef = def<[BendList<number>, number, number, number], boolean>("freshness", "is_fresh");
const isStaleDef = def<[BendList<number>, number, number, number], boolean>("freshness", "is_stale");
const isPublicDef = def<[number, BendList<number>, number, number, number], boolean>(
  "freshness",
  "is_public",
);
const staleLastSeenDef = def<[number, number, number], boolean>("freshness", "stale_last_seen");

export const DAY_MS = def<[], number>("freshness", "day_ms")();
export const MAX_AGE_DAYS_NO_TERM = def<[], number>("freshness", "max_age_days")();
export const LAST_SEEN_STALE_DAYS = def<[], number>("freshness", "last_seen_days")();

const TERM_CTOR: Record<string, string> = { summer: "Summer", fall: "Fall", winter: "Winter" };

/** Last instant (23:59:59 UTC) of a term window (laws fr_*_end). */
export function termEndDate(ty: TermYearLike): Date {
  const { month0, day } = termEndDef({ $: TERM_CTOR[ty.term] ?? "Other" });
  return new Date(Date.UTC(ty.year, month0, day, 23, 59, 59));
}

/** Epoch ms of a valid Date in [1970, 2^48 ms). Invalid dates throw. */
function ms(d: Date, what = "date"): number {
  return nat(d.getTime(), what);
}

function termEnds(termYears: readonly TermYearLike[] | null | undefined): BendList<number> {
  return list((termYears ?? []).map((ty) => ms(termEndDate(ty))));
}

export type FreshnessFacts = {
  termYears: readonly TermYearLike[] | null | undefined;
  /** posted_at, else first_seen_at. */
  anchor: Date;
  now: Date;
  maxAgeDays?: number;
};

function freshnessArgs(f: FreshnessFacts): [BendList<number>, number, number, number] {
  const maxAgeDays = f.maxAgeDays ?? MAX_AGE_DAYS_NO_TERM;
  return [termEnds(f.termYears), ms(f.anchor), ms(f.now), nat(maxAgeDays * DAY_MS)];
}

/** Shown on the board / API (fr_stale_iff_not_fresh ties it to isStale). */
export function isFresh(f: FreshnessFacts): boolean {
  return isFreshDef(...freshnessArgs(f));
}

/** Deactivated by ingest; always exactly !isFresh. */
export function isStale(f: FreshnessFacts): boolean {
  return isStaleDef(...freshnessArgs(f));
}

/** Fresh and tagged with at least one role. */
export function isPublic(roleCount: number, f: FreshnessFacts): boolean {
  return isPublicDef(nat(roleCount), ...freshnessArgs(f));
}

export function isStaleByLastSeen(lastSeenAt: Date, now: Date, maxDays = LAST_SEEN_STALE_DAYS): boolean {
  return staleLastSeenDef(ms(lastSeenAt), ms(now), nat(maxDays * DAY_MS));
}

// ---------------------------------------------------------------------------
// Apply picker
// ---------------------------------------------------------------------------

const needsPickerDef = def<[number, number], boolean>("apply_picker", "needs_picker");
const qualifyDef = def<[number], boolean>("apply_picker", "qualify_single_title");
const saysOpeningsDef = def<[number, number], boolean>("apply_picker", "says_openings");

/** Laws ap_empty / ap_many_postings / ap_one_posting. */
export function needsPicker(postingCount: number, firstPostingLocationCount: number): boolean {
  return needsPickerDef(nat(postingCount), nat(firstPostingLocationCount));
}

/** Law ap_title_matches_picker. */
export function qualifySingleTitle(locationCount: number): boolean {
  return qualifyDef(nat(locationCount));
}

/** Laws ap_distinct_says_locations / ap_repeats_say_openings. */
export function saysOpenings(postingCount: number, uniqueLocationCount: number): boolean {
  return saysOpeningsDef(nat(postingCount), nat(uniqueLocationCount));
}

// ---------------------------------------------------------------------------
// Deactivation policy
// ---------------------------------------------------------------------------

/**
 * What one ingest run learned about a company's board: "failed" (fetch failed
 * or was refused), "empty" (fetched, no postings), "listed" (fetched, at least
 * one posting).
 */
export type BoardRun = "failed" | "empty" | "listed";

/**
 * Where one existing job stands on a listed board: "gone" (not on it),
 * "rejected" (on it with details, fails the classifier), "unverified" (on it,
 * details missing), "matched" (passes).
 */
export type JobSighting = "gone" | "rejected" | "unverified" | "matched";

const RUN_TAG = { failed: "Failed", empty: "Empty", listed: "Listed" } as const;
const SEEN_TAG = {
  gone: "Gone",
  rejected: "Rejected",
  unverified: "Unverified",
  matched: "Matched",
} as const;

// The compiled matches fall through to their last case, so an unknown tag
// would silently mean "listed" or "matched": reject it here.
function tag<K extends string, V>(table: Record<K, V>, key: K, what: string): V {
  if (!Object.hasOwn(table, key)) throw new RangeError(`unknown ${what}: ${String(key)}`);
  return table[key];
}

const retireDef = def<[{ $: string }, number, { $: string }], boolean>("deactivation", "retire");
const nextEmptyDef = def<[{ $: string }, number], number>("deactivation", "next_empty");
const retireAfterEmptyDef = def<[], number>("deactivation", "retire_after_empty");

/** Empty runs in a row before every job of a company is retired (law dp_retire_after_three). */
export const RETIRE_AFTER_EMPTY_RUNS = retireAfterEmptyDef();

/** Whether this run retires one existing active job (laws dp_*). */
export function shouldRetireJob(run: BoardRun, priorEmptyRuns: number, seen: JobSighting): boolean {
  return retireDef(
    { $: tag(RUN_TAG, run, "board run") },
    nat(priorEmptyRuns, "priorEmptyRuns"),
    { $: tag(SEEN_TAG, seen, "job sighting") },
  );
}

/** The company's empty-run count after this run (laws dp_listed_resets, dp_failed_keeps_count, dp_empty_counts). */
export function nextEmptyRuns(run: BoardRun, priorEmptyRuns: number): number {
  return nextEmptyDef({ $: tag(RUN_TAG, run, "board run") }, nat(priorEmptyRuns, "priorEmptyRuns"));
}

// ---------------------------------------------------------------------------
// Ingest-health funnel
// ---------------------------------------------------------------------------

export type FunnelCounts = {
  fetched: number | null;
  title: number | null;
  tech: number | null;
  upserted: number | null;
  descriptionMissing: number | null;
};

export type FunnelTotals = {
  fetched: number;
  title: number;
  tech: number;
  upserted: number;
  descriptionMissing: number;
};

type BendTotals = {
  fetched: number;
  title: number;
  tech: number;
  upserted: number;
  desc_missing: number;
};

const totalDef = def<[BendList<unknown>], BendTotals>("funnel", "total");
const zeroMatchDef = def<[string], boolean>("funnel", "is_zero_match");

function count(n: number | null): BendMaybe<number> {
  return maybe(n == null ? null : nat(n));
}

/** Laws fn_total_empty / fn_total_append. */
export function sumFunnels(funnels: readonly FunnelCounts[]): FunnelTotals {
  const t = totalDef(
    list(
      funnels.map((f) => ({
        $: "Funnel",
        fetched: count(f.fetched),
        title: count(f.title),
        tech: count(f.tech),
        upserted: count(f.upserted),
        desc_missing: count(f.descriptionMissing),
      })),
    ),
  );
  return {
    fetched: t.fetched,
    title: t.title,
    tech: t.tech,
    upserted: t.upserted,
    descriptionMissing: t.desc_missing,
  };
}

/** Laws fn_zero_match_tagged / fn_funnel_not_zero_match. */
export function isZeroMatchRun(error: string | null | undefined): boolean {
  return error ? zeroMatchDef(error) : false;
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

const pageDef = def<[BendMaybe<string>], number>("pagination", "page");
const limitDef = def<[BendMaybe<string>], number>("pagination", "limit");
const offsetDef = def<[number, number], number>("pagination", "offset");

function rawParam(raw: string | readonly string[] | null | undefined): BendMaybe<string> {
  const v = Array.isArray(raw) ? raw[0] : raw;
  return maybe(typeof v === "string" ? v.trim() : null);
}

/** 1..100000, 1 for missing or invalid text (laws pg_page_*). */
export function parsePage(raw: string | readonly string[] | null | undefined): number {
  return pageDef(rawParam(raw));
}

/** 1..100, 27 for missing or invalid text (laws pg_limit_*). */
export function parseLimit(raw: string | readonly string[] | null | undefined): number {
  return limitDef(rawParam(raw));
}

/** Rows to skip before `page` (law pg_first_page_offset). */
export function pageOffset(page: number, limit: number): number {
  return offsetDef(nat(page), nat(limit));
}

// ---------------------------------------------------------------------------
// Job slugs
// ---------------------------------------------------------------------------

const segmentDef = def<[string, string], string>("job_slug", "segment");
const parseSlugDef = def<[string], BendMaybe<string>>("job_slug", "parse");

/** "<base>-<first 8 id chars without dashes>". */
export function jobSlugSegment(base: string, id: string): string {
  return segmentDef(base, id);
}

/** The id prefix a slug ends with, lowercased (law js_roundtrip). */
export function parseIdPrefix(slug: string): string | null {
  const r = parseSlugDef(slug);
  return r.$ === "Some" ? r.value : null;
}
