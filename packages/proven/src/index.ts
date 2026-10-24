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
  const fn = proven[`${MODULE_ROOT}/${module}:${name}`];
  if (!fn) throw new Error(`proven.mjs has no ${module}:${name}; run pnpm proofs:build`);
  return fn as unknown as (...args: A) => R;
}

/** Bend Nat: a non-negative safe integer. NaN, negatives and fractions floor to 0. */
function nat(n: number): number {
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.min(Math.floor(n), Number.MAX_SAFE_INTEGER);
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
  const bucket = prev && { $: "Bucket" as const, count: nat(prev.count), reset_at: nat(prev.resetAt) };
  const v = rlStep(maybe(bucket), nat(now), nat(limit), nat(windowMs));
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

/** Epoch ms, with invalid dates as 0 (the distant past, so never fresh). */
function ms(d: Date | null | undefined): number {
  return d ? nat(d.getTime()) : 0;
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

export type DeactivationPolicy = "missing" | "all" | "none";

const policyDef = def<[boolean, number, number], { $: "Missing" | "All" | "Keep" }>(
  "deactivation",
  "policy",
);

const POLICY: Record<"Missing" | "All" | "Keep", DeactivationPolicy> = {
  Missing: "missing",
  All: "all",
  Keep: "none",
};

/** Laws dp_*. */
export function deactivationPolicy(input: {
  fetchSucceeded: boolean;
  fetched: number;
  techPass: number;
}): DeactivationPolicy {
  return POLICY[policyDef(input.fetchSucceeded, nat(input.fetched), nat(input.techPass)).$];
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
