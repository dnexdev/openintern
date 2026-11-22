/**
 * The laws prove the Bend code; these checks cover the TS <-> Bend glue by
 * comparing each wrapper with the TS it replaced, on fixed and random inputs.
 * Where behaviour changed on purpose, the expected value says so.
 */
import assert from "node:assert/strict";
import {
  DAY_MS,
  nextEmptyRuns,
  RETIRE_AFTER_EMPTY_RUNS,
  shouldRetireJob,
  isFresh,
  isPublic,
  isStale,
  isStaleByLastSeen,
  isZeroMatchRun,
  jobSlugSegment,
  needsPicker,
  pageOffset,
  parseIdPrefix,
  parseLimit,
  parsePage,
  qualifySingleTitle,
  rateLimitStep,
  saysOpenings,
  sumFunnels,
  applyByDate,
  type RateBucket,
} from "./index";

let seed = 42;
function rand(n: number): number {
  seed = (seed * 1103515245 + 12345) % 2 ** 31;
  return seed % n;
}

// --- rate limiter: replay against the old TS limiter -----------------------

function oldStep(b: RateBucket | undefined, now: number, limit: number, windowMs: number) {
  if (!b || b.resetAt <= now) {
    return { ok: true, remaining: limit - 1, bucket: { count: 1, resetAt: now + windowMs } };
  }
  if (b.count >= limit) return { ok: false, remaining: 0, bucket: b };
  return { ok: true, remaining: limit - (b.count + 1), bucket: { count: b.count + 1, resetAt: b.resetAt } };
}

for (let run = 0; run < 200; run++) {
  const limit = 1 + rand(5);
  const windowMs = 1 + rand(50);
  let now = 1_700_000_000_000 + rand(1000);
  let a: RateBucket | undefined;
  let b: RateBucket | undefined;
  for (let i = 0; i < 30; i++) {
    now += rand(10);
    const x = rateLimitStep(a, now, limit, windowMs);
    const y = oldStep(b, now, limit, windowMs);
    assert.deepEqual(x, y, `rate limit run ${run} step ${i}`);
    a = x.bucket;
    b = y.bucket;
  }
}
// Changed on purpose: a limit of 0 used to admit the first request.
assert.equal(rateLimitStep(undefined, 1000, 0, 60_000).ok, false, "limit 0 admits nothing");
// Invalid input is a caller bug: it throws a real Error instead of being clamped.
for (const [now, limit, windowMs] of [
  [1000, Number.NaN, 60_000],
  [1000, -1, 60_000],
  [1000, 1.5, 60_000],
  [2 ** 48, 60, 60_000],
  [Number.POSITIVE_INFINITY, 60, 60_000],
  [1000, 60, 0],
]) {
  assert.throws(() => rateLimitStep(undefined, now!, limit!, windowMs!), RangeError, `rejects ${now}/${limit}/${windowMs}`);
}
assert.equal(rateLimitStep(undefined, 2 ** 48 - 60_001, 60, 60_000).ok, true, "just below 2^48 works");
// now + window past 2^48 overflows inside Bend; it must surface as an Error, not a string.
assert.throws(() => rateLimitStep(undefined, 2 ** 48 - 1000, 60, 1000), Error, "overflow is an Error");

// --- freshness: against an independent reference -------------------------
// Changed on purpose (2026-10): a listing stays up until its term starts, not
// until it ends, and spring is its own term.

function oldEnd(term: string, year: number): Date {
  if (term === "summer") return new Date(Date.UTC(year, 3, 30, 23, 59, 59));
  if (term === "fall") return new Date(Date.UTC(year, 7, 31, 23, 59, 59));
  if (term === "winter" || term === "spring") return new Date(Date.UTC(year - 1, 11, 31, 23, 59, 59));
  return new Date(Date.UTC(year, 11, 31, 23, 59, 59));
}

function oldFresh(termYears: { term: string; year: number }[], anchor: Date, now: Date): boolean {
  if (termYears.length > 0) return termYears.some((ty) => oldEnd(ty.term, ty.year) >= now);
  return now.getTime() - anchor.getTime() <= 120 * 86_400_000;
}

const TERMS = ["summer", "fall", "winter", "spring"] as const;
for (let i = 0; i < 2000; i++) {
  const termYears = Array.from({ length: rand(3) }, () => ({
    term: TERMS[rand(3)]!,
    year: 2024 + rand(4),
  }));
  const now = new Date(Date.UTC(2025, 0, 1) + rand(3 * 365) * DAY_MS + rand(DAY_MS));
  const anchor = new Date(now.getTime() - (rand(300) - 50) * DAY_MS);
  const facts = { termYears, anchor, now };
  assert.equal(isFresh(facts), oldFresh(termYears, anchor, now), `fresh #${i}`);
  assert.equal(isStale(facts), !oldFresh(termYears, anchor, now), `stale #${i}`);
  assert.equal(isPublic(0, facts), false, `public needs roles #${i}`);
  assert.equal(isPublic(1 + rand(3), facts), oldFresh(termYears, anchor, now), `public #${i}`);
}

// Boundaries: fresh through the last second before a term starts, stale one ms later.
const summerEnd = applyByDate({ term: "summer", year: 2026 });
assert.equal(summerEnd.toISOString(), "2026-04-30T23:59:59.000Z");
assert.equal(applyByDate({ term: "fall", year: 2026 }).toISOString(), "2026-08-31T23:59:59.000Z");
assert.equal(applyByDate({ term: "winter", year: 2027 }).toISOString(), "2026-12-31T23:59:59.000Z");
assert.equal(applyByDate({ term: "spring", year: 2027 }).toISOString(), "2026-12-31T23:59:59.000Z");
assert.equal(isFresh({ termYears: [{ term: "summer", year: 2026 }], anchor: summerEnd, now: summerEnd }), true);
assert.equal(
  isStale({ termYears: [{ term: "summer", year: 2026 }], anchor: summerEnd, now: new Date(summerEnd.getTime() + 1) }),
  true,
);
// Unknown terms keep the whole year; "constructor" must not reach a prototype key.
assert.equal(applyByDate({ term: "autumnal", year: 2026 }).toISOString(), "2026-12-31T23:59:59.000Z");
assert.equal(applyByDate({ term: "constructor", year: 2026 }).toISOString(), "2026-12-31T23:59:59.000Z");
// Invalid or out-of-range dates throw; callers decide what that means.
assert.throws(() => isFresh({ termYears: [], anchor: new Date("nope"), now: new Date() }), RangeError);
assert.throws(() => isFresh({ termYears: [], anchor: new Date(-1), now: new Date() }), RangeError);
assert.throws(() => isFresh({ termYears: [], anchor: new Date(2 ** 48), now: new Date() }), RangeError);
assert.throws(
  () => isFresh({ termYears: [], anchor: new Date(), now: new Date(), maxAgeDays: Number.POSITIVE_INFINITY }),
  RangeError,
);

const seen = new Date("2026-07-01T00:00:00Z");
assert.equal(isStaleByLastSeen(seen, new Date("2026-07-15T00:00:00Z")), false, "exactly 14 days");
assert.equal(isStaleByLastSeen(seen, new Date("2026-07-15T00:00:00.001Z")), true, "past 14 days");

// --- apply picker ----------------------------------------------------------

for (let p = 0; p < 5; p++) {
  for (let l = 0; l < 5; l++) {
    const old = p > 1 ? true : p === 1 ? l > 1 : false;
    assert.equal(needsPicker(p, l), old, `picker ${p} ${l}`);
    assert.equal(saysOpenings(p, l), l < p, `openings ${p} ${l}`);
  }
  assert.equal(qualifySingleTitle(p), p > 1, `qualify ${p}`);
}

// --- deactivation ----------------------------------------------------------
// Reference spec, written independently of the Bend model.

function refRetire(run: string, prior: number, seen: string): boolean {
  if (run === "failed") return false;
  if (run === "expired") return true;
  if (run === "empty") return prior + 1 >= 3;
  return seen === "gone" || seen === "rejected";
}
const RUNS = ["failed", "empty", "listed", "expired"] as const;
const SEENS = ["gone", "rejected", "unverified", "matched"] as const;
assert.equal(RETIRE_AFTER_EMPTY_RUNS, 3);
for (const run of RUNS) {
  for (let prior = 0; prior < 6; prior++) {
    for (const seen of SEENS) {
      assert.equal(shouldRetireJob(run, prior, seen), refRetire(run, prior, seen), `retire ${run} ${prior} ${seen}`);
    }
    const next = run === "failed" || run === "expired" ? prior : run === "empty" ? prior + 1 : 0;
    assert.equal(nextEmptyRuns(run, prior), next, `next ${run} ${prior}`);
  }
}
// Unknown tags would fall through to the last compiled case; they must throw.
assert.throws(() => shouldRetireJob("lisetd" as never, 0, "gone"), RangeError);
assert.throws(() => shouldRetireJob("listed", 0, "constructor" as never), RangeError);
assert.throws(() => shouldRetireJob("listed", -1, "gone"), RangeError);

// --- funnel ----------------------------------------------------------------

assert.deepEqual(
  sumFunnels([
    { fetched: 10, title: 5, tech: 3, upserted: 2, descriptionMissing: null },
    { fetched: null, title: 1, tech: null, upserted: 4, descriptionMissing: 7 },
  ]),
  { fetched: 10, title: 6, tech: 3, upserted: 6, descriptionMissing: 7 },
);
assert.deepEqual(sumFunnels([]), { fetched: 0, title: 0, tech: 0, upserted: 0, descriptionMissing: 0 });
assert.equal(isZeroMatchRun("zero_match: board had 12 jobs"), true);
assert.equal(isZeroMatchRun("funnel fetched=1 title=0 tech=0 upserted=0"), false);
assert.equal(isZeroMatchRun(null), false);
assert.equal(isZeroMatchRun("zero_matc"), false);
assert.throws(
  () => sumFunnels([{ fetched: 2 ** 48, title: 0, tech: 0, upserted: 0, descriptionMissing: 0 }]),
  RangeError,
);
assert.throws(
  () =>
    sumFunnels([
      { fetched: 2 ** 47, title: 0, tech: 0, upserted: 0, descriptionMissing: 0 },
      { fetched: 2 ** 47, title: 0, tech: 0, upserted: 0, descriptionMissing: 0 },
    ]),
  Error,
  "a sum past 2^48 is an Error",
);

// --- pagination ------------------------------------------------------------

assert.equal(parsePage(undefined), 1);
assert.equal(parsePage("3"), 3);
assert.equal(parsePage(["4", "9"]), 4);
assert.equal(parsePage(" 5 "), 5);
assert.equal(parsePage("0"), 1);
assert.equal(parsePage("-2"), 1);
assert.equal(parsePage("abc"), 1, "old code returned NaN here");
assert.equal(parsePage("2.5"), 1, "old code returned 2.5 here");
assert.equal(parsePage("99999999999999999999"), 1);
assert.equal(parsePage("250000"), 100000);
assert.equal(parseLimit(null), 27);
assert.equal(parseLimit("10"), 10);
assert.equal(parseLimit("500"), 100);
assert.equal(parseLimit("0"), 27);
assert.equal(pageOffset(1, 27), 0);
assert.equal(pageOffset(3, 27), 54);
assert.equal(pageOffset(100000, 100), 9_999_900);
assert.throws(() => needsPicker(Number.NaN, 0), RangeError);
assert.throws(() => needsPicker(1.5, 0), RangeError);

// --- job slugs -------------------------------------------------------------

const oldParse = (s: string) => s.match(/-([a-f0-9]{8})$/i)?.[1]?.toLowerCase() ?? null;
const oldSegment = (base: string, id: string) => `${base}-${id.replace(/-/g, "").slice(0, 8)}`;
const slugCases = [
  "software-intern-1a2b3c4d",
  "x-ABCDEF12",
  "-12345678",
  "12345678",
  "a-1234567",
  "a-1234567g",
  "très-élevé-deadbeef",
  "",
  "a--deadbeef",
  "日本語-0123abcd",
];
for (const s of slugCases) assert.equal(parseIdPrefix(s), oldParse(s), `parse ${JSON.stringify(s)}`);
const hex = "0123456789abcdef";
for (let i = 0; i < 300; i++) {
  const id = Array.from({ length: 32 }, () => hex[rand(16)]).join("");
  const uuid = `${id.slice(0, 8)}-${id.slice(8, 12)}-${id.slice(12, 16)}-${id.slice(16, 20)}-${id.slice(20)}`;
  const base = ["swe-intern", "job", "über-praktikum", "a"][rand(4)]!;
  const seg = jobSlugSegment(base, uuid);
  assert.equal(seg, oldSegment(base, uuid), `segment #${i}`);
  assert.equal(parseIdPrefix(seg), id.slice(0, 8), `roundtrip #${i}`);
}

console.log("proven.test: ok");
