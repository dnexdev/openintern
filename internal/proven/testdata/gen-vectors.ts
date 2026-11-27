// Regenerate with:
// packages/ingest/node_modules/.bin/tsx internal/proven/testdata/gen-vectors.ts
import { writeFileSync } from "node:fs";
import {
  DAY_MS,
  LAST_SEEN_STALE_DAYS,
  MAX_AGE_DAYS_NO_TERM,
  NAT_LIMIT,
  RETIRE_AFTER_EMPTY_RUNS,
  applyByDate,
  isFresh,
  isPublic,
  isStale,
  isStaleByLastSeen,
  isZeroMatchRun,
  jobSlugSegment,
  needsPicker,
  nextEmptyRuns,
  pageOffset,
  parseIdPrefix,
  parseLimit,
  parsePage,
  qualifySingleTitle,
  rateLimitStep,
  saysOpenings,
  shouldRetireJob,
  sumFunnels,
  type BoardRun,
  type FunnelCounts,
  type JobSighting,
  type RateBucket,
  type TermYearLike,
} from "../../../packages/proven/src/index.ts";

type Captured = { value?: unknown; error?: { name: string; message: string } };
function capture(fn: () => unknown): Captured {
  try {
    return { value: fn() };
  } catch (error) {
    if (error instanceof Error) return { error: { name: error.name, message: error.message } };
    return { error: { name: typeof error, message: String(error) } };
  }
}

const vectors = {
  constants: { NAT_LIMIT, DAY_MS, MAX_AGE_DAYS_NO_TERM, LAST_SEEN_STALE_DAYS, RETIRE_AFTER_EMPTY_RUNS },
  rateLimit: [] as unknown[],
  applyBy: [] as unknown[],
  freshness: [] as unknown[],
  staleLastSeen: [] as unknown[],
  picker: [] as unknown[],
  deactivation: [] as unknown[],
  funnels: [] as unknown[],
  zeroMatch: [] as unknown[],
  pagination: [] as unknown[],
  offsets: [] as unknown[],
  slugSegments: [] as unknown[],
  slugParses: [] as unknown[],
};

const rateNows = [0, 1, 999, 1_700_000_000_000, NAT_LIMIT - 2];
for (const now of rateNows) {
  for (const limit of [0, 1, 2, 5]) {
    for (const windowMs of [1, 2, 60_000]) {
      if (now + windowMs >= NAT_LIMIT) continue;
      const prevs: (RateBucket | undefined)[] = [
        undefined,
        { count: 0, resetAt: now },
        { count: 1, resetAt: now },
        { count: limit, resetAt: now },
      ];
      if (now + 1 < NAT_LIMIT) {
        prevs.push({ count: 0, resetAt: now + 1 }, { count: 1, resetAt: now + 1 }, { count: limit, resetAt: now + 1 });
      }
      for (const prev of prevs) {
        vectors.rateLimit.push({ prev: prev ?? null, now, limit, windowMs, ...capture(() => rateLimitStep(prev, now, limit, windowMs)) });
      }
    }
  }
}
for (const [prev, now, limit, windowMs] of [
  [undefined, -1, 1, 1],
  [undefined, NAT_LIMIT, 1, 1],
  [undefined, 0, -1, 1],
  [undefined, 0, NAT_LIMIT, 1],
  [undefined, 0, 1, 0],
  [undefined, NAT_LIMIT - 1, 1, 1],
  [{ count: NAT_LIMIT, resetAt: 2 }, 1, 1, 1],
  [{ count: 0, resetAt: NAT_LIMIT }, 1, 1, 1],
] as [RateBucket | undefined, number, number, number][]) {
  vectors.rateLimit.push({ prev: prev ?? null, now, limit, windowMs, ...capture(() => rateLimitStep(prev, now, limit, windowMs)) });
}

const terms = ["summer", "fall", "winter", "spring", "other", "constructor"];
for (const term of terms) {
  for (const year of [1970, 2026, 2027]) {
    const input = { term, year };
    vectors.applyBy.push({ input, iso: applyByDate(input).toISOString(), ms: applyByDate(input).getTime() });
  }
}
const freshnessInputs: { termYears: TermYearLike[]; anchor: number; now: number; maxAgeDays?: number }[] = [];
for (const anchor of [0, 1, Date.UTC(2026, 0, 1), NAT_LIMIT - DAY_MS - 1]) {
  for (const delta of [0, 1, DAY_MS, 120 * DAY_MS, 120 * DAY_MS + 1]) {
    const now = anchor + delta;
    if (now >= NAT_LIMIT) continue;
    freshnessInputs.push({ termYears: [], anchor, now });
    freshnessInputs.push({ termYears: [], anchor, now, maxAgeDays: 0 });
    freshnessInputs.push({ termYears: [], anchor, now, maxAgeDays: 1 });
  }
}
for (const term of terms) {
  const end = applyByDate({ term, year: 2027 }).getTime();
  for (const delta of [-1, 0, 1]) {
    freshnessInputs.push({ termYears: [{ term, year: 2027 }], anchor: 0, now: end + delta });
  }
}
freshnessInputs.push({ termYears: [], anchor: NAT_LIMIT - 1, now: NAT_LIMIT - 1, maxAgeDays: 1 });
for (const input of freshnessInputs) {
  const facts = {
    termYears: input.termYears,
    anchor: new Date(input.anchor),
    now: new Date(input.now),
    ...(input.maxAgeDays === undefined ? {} : { maxAgeDays: input.maxAgeDays }),
  };
  vectors.freshness.push({ input, fresh: capture(() => isFresh(facts)), stale: capture(() => isStale(facts)), public0: capture(() => isPublic(0, facts)), public2: capture(() => isPublic(2, facts)) });
}
for (const [seen, now, maxDays] of [
  [0, 0, undefined],
  [Date.UTC(2026, 6, 1), Date.UTC(2026, 6, 15), undefined],
  [Date.UTC(2026, 6, 1), Date.UTC(2026, 6, 15) + 1, undefined],
  [1, 1, 0],
  [NAT_LIMIT - 1, NAT_LIMIT - 1, 1],
] as [number, number, number | undefined][]) {
  vectors.staleLastSeen.push({ seen, now, maxDays: maxDays ?? null, ...capture(() => isStaleByLastSeen(new Date(seen), new Date(now), maxDays)) });
}

for (const postings of [0, 1, 2, 3, NAT_LIMIT - 1]) {
  for (const locations of [0, 1, 2, 3, NAT_LIMIT - 1]) {
    vectors.picker.push({ postings, locations, needs: capture(() => needsPicker(postings, locations)), qualify: capture(() => qualifySingleTitle(locations)), openings: capture(() => saysOpenings(postings, locations)) });
  }
}
vectors.picker.push({ postings: -1, locations: -1, needs: capture(() => needsPicker(-1, -1)), qualify: capture(() => qualifySingleTitle(-1)), openings: capture(() => saysOpenings(-1, -1)) });
vectors.picker.push({ postings: NAT_LIMIT, locations: NAT_LIMIT, needs: capture(() => needsPicker(NAT_LIMIT, NAT_LIMIT)), qualify: capture(() => qualifySingleTitle(NAT_LIMIT)), openings: capture(() => saysOpenings(NAT_LIMIT, NAT_LIMIT)) });

const runs: BoardRun[] = ["failed", "empty", "listed", "expired"];
const sightings: JobSighting[] = ["gone", "rejected", "unverified", "matched"];
for (const run of runs) {
  for (const prior of [0, 1, 2, 3, 4, NAT_LIMIT - 1]) {
    for (const seen of sightings) {
      vectors.deactivation.push({ run, prior, seen, retire: capture(() => shouldRetireJob(run, prior, seen)), next: capture(() => nextEmptyRuns(run, prior)) });
    }
  }
}
vectors.deactivation.push({ run: "unknown", prior: 0, seen: "gone", retire: capture(() => shouldRetireJob("unknown" as BoardRun, 0, "gone")), next: capture(() => nextEmptyRuns("unknown" as BoardRun, 0)) });
vectors.deactivation.push({ run: "listed", prior: 0, seen: "unknown", retire: capture(() => shouldRetireJob("listed", 0, "unknown" as JobSighting)), next: capture(() => nextEmptyRuns("listed", 0)) });

const z = 0;
const one = 1;
const max = NAT_LIMIT - 1;
const funnelSets: FunnelCounts[][] = [
  [],
  [{ fetched: null, title: null, tech: null, upserted: null, descriptionMissing: null }],
  [{ fetched: 10, title: 5, tech: 3, upserted: 2, descriptionMissing: null }, { fetched: null, title: 1, tech: null, upserted: 4, descriptionMissing: 7 }],
  [{ fetched: z, title: one, tech: max, upserted: z, descriptionMissing: one }],
  [{ fetched: 2 ** 47, title: 0, tech: 0, upserted: 0, descriptionMissing: 0 }, { fetched: 2 ** 47, title: 0, tech: 0, upserted: 0, descriptionMissing: 0 }],
];
for (const funnels of funnelSets) vectors.funnels.push({ funnels, ...capture(() => sumFunnels(funnels)) });
for (const error of [null, "", "zero_match:", "zero_match: board had 12 jobs", "funnel fetched=1", "HTTP 500", " zero_match:x"]) {
  vectors.zeroMatch.push({ error, value: isZeroMatchRun(error) });
}

const params: (string | string[] | null | undefined)[] = [undefined, null, "", " ", "0", "1", "3", " 5 ", "+2", "-2", "2.5", "2abc", "0003", "100", "101", "250000", "99999999999999999999", ["4", "9"], []];
for (const raw of params) vectors.pagination.push({ raw: raw ?? null, isArray: Array.isArray(raw), page: parsePage(raw), limit: parseLimit(raw) });
for (const [page, limit] of [[0, 0], [1, 27], [2, 1], [3, 27], [100000, 100], [NAT_LIMIT - 1, 0], [NAT_LIMIT - 1, 1], [NAT_LIMIT - 1, NAT_LIMIT - 1], [-1, 1], [NAT_LIMIT, 1]]) {
  vectors.offsets.push({ page, limit, ...capture(() => pageOffset(page, limit)) });
}

const bases = ["", "job", "software-intern", "über-praktikum", "日本語"];
const ids = ["", "12345678", "1a2b3c4d-0000-0000-0000-000000000000", "-12345678", "12-34-56-78", "abcdef"];
for (const base of bases) for (const id of ids) vectors.slugSegments.push({ base, id, value: jobSlugSegment(base, id) });
for (const slug of ["", "12345678", "-12345678", "a-1234567", "a-12345678", "a-ABCDEF12", "a-1234567g", "a--deadbeef", "日本語-0123abcd", "emoji😀-deadbeef", "a-١٢٣٤٥٦٧٨"]) {
  vectors.slugParses.push({ slug, value: parseIdPrefix(slug) });
}

writeFileSync(new URL("./vectors.json", import.meta.url), `${JSON.stringify(vectors, null, 2)}\n`);
console.log(`wrote ${Object.values(vectors).reduce((n, value) => n + (Array.isArray(value) ? value.length : 0), 0)} vectors`);
