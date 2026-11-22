import { shouldRetireJob } from "@openintern/proven";
import {
  CONSECUTIVE_FAILURE_LIMIT,
  DISABLED_RECHECK_DAYS,
  OUTAGE_GAP_DAYS,
  autoDisableState,
  countPriorEmptyRuns,
  ingestDigest,
  shouldSkipLastSeenSweep,
  sightJob,
} from "./ingest.js";
import { safeDate } from "./safe-date.js";

function parseFunnelForTest(error: string | null) {
  const match = error?.match(
    /funnel fetched=(\d+) title=(\d+) tech=(\d+) upserted=(\d+)(?: description_missing=(\d+))?/,
  );
  if (!match) return null;
  return {
    fetched: Number(match[1]),
    descriptionMissing: match[5] != null ? Number(match[5]) : null,
  };
}

function assertEqual<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

// #24: a closed posting on a board where nothing else matches must be retired.
{
  const board = new Map([["8204881", { description: "Product designer, full time" }]]);
  const seen = sightJob(board.get("8250259"), false);
  assertEqual(seen, "gone", "closed posting is gone from the board");
  assertEqual(shouldRetireJob("listed", 0, seen), true, "#24: gone posting is retired");
}
assertEqual(sightJob({ description: "SWE intern" }, true), "matched", "still matching");
assertEqual(sightJob({ description: "Senior engineer" }, false), "rejected", "fetched but no longer matches");
assertEqual(sightJob({ description: "  " }, false), "unverified", "details missing");
assertEqual(shouldRetireJob("listed", 0, "unverified"), false, "missing details keep the job");
assertEqual(shouldRetireJob("failed", 5, "gone"), false, "failed fetch retires nothing");
assertEqual(shouldRetireJob("empty", 0, "gone"), false, "first empty run waits");
assertEqual(shouldRetireJob("empty", 1, "gone"), false, "second empty run waits");
assertEqual(shouldRetireJob("empty", 2, "gone"), true, "third empty run retires");

const ok = (fetched: number) => ({ status: "ok", error: `funnel fetched=${fetched} title=0 tech=0 upserted=0` });
const err = { status: "error", error: "HTTP 500" };
assertEqual(countPriorEmptyRuns([]), 0, "no history");
assertEqual(countPriorEmptyRuns([ok(0), ok(0), ok(4)]), 2, "two empty runs, then a listed one");
assertEqual(countPriorEmptyRuns([ok(0), err, ok(0), ok(3)]), 2, "failed runs don't break or add to the count");
assertEqual(countPriorEmptyRuns([ok(7), ok(0)]), 0, "latest run listed postings");
assertEqual(countPriorEmptyRuns([{ status: "ok", error: "zero_match: no tech internships classified; funnel fetched=0 title=0 tech=0 upserted=0" }]), 1, "zero_match prefix still parses");

{
  const day = 24 * 60 * 60 * 1000;
  const now = new Date("2026-11-20T12:00:00Z");
  const fails = (n: number, newestAgoDays = 1) =>
    Array.from({ length: n }, (_, i) => ({ status: "error", ranAt: new Date(now.getTime() - (newestAgoDays + i) * day) }));
  assertEqual(autoDisableState(fails(CONSECUTIVE_FAILURE_LIMIT - 1), false, now), "enabled", "below the limit");
  assertEqual(autoDisableState(fails(CONSECUTIVE_FAILURE_LIMIT), false, now), "disabled", "at the limit");
  assertEqual(autoDisableState(fails(CONSECUTIVE_FAILURE_LIMIT), true, now), "enabled", "tier-1 is exempt");
  assertEqual(
    autoDisableState([{ status: "ok", ranAt: new Date(now.getTime() - day) }, ...fails(CONSECUTIVE_FAILURE_LIMIT, 2)], false, now),
    "enabled",
    "one success re-enables",
  );
  assertEqual(
    autoDisableState(fails(CONSECUTIVE_FAILURE_LIMIT, DISABLED_RECHECK_DAYS), false, now),
    "recheck",
    "weekly recheck",
  );
}

const day = 24 * 60 * 60 * 1000;
const sweepNow = new Date("2026-11-20T12:00:00Z");
assertEqual(shouldSkipLastSeenSweep(null, sweepNow), true, "no previous run skips the sweep");
assertEqual(
  shouldSkipLastSeenSweep(new Date(sweepNow.getTime() - day), sweepNow),
  false,
  "daily runs keep sweeping",
);
assertEqual(
  shouldSkipLastSeenSweep(new Date(sweepNow.getTime() - OUTAGE_GAP_DAYS * day), sweepNow),
  false,
  "exactly the gap limit still sweeps",
);
assertEqual(
  shouldSkipLastSeenSweep(new Date(sweepNow.getTime() - OUTAGE_GAP_DAYS * day - 1), sweepNow),
  true,
  "first run after an outage skips the sweep",
);
assertEqual(
  shouldSkipLastSeenSweep(new Date("2026-10-23T16:23:00Z"), sweepNow),
  true,
  "Neon quota outage from 2026-10-24 skips the sweep",
);

{
  const base = {
    companies: 1, jobsUpserted: 0, jobsDeactivated: 0, staleDeactivated: 0, lastSeenDeactivated: 0,
    lastSeenSweepSkipped: false, zeroMatchCompanies: [], autoDisabled: [], autoDisabledSkipped: [],
    suspectedBlocks: [], failures: [], funnel: [],
    funnelTotals: { fetched: 0, internshipTitle: 0, techPass: 0, upserted: 0 },
  };
  assertEqual(ingestDigest(base), null, "nothing to report");
  const d = ingestDigest({ ...base, autoDisabled: ["acme"], suspectedBlocks: ["tiktok"] }) ?? "";
  assertEqual(d.includes("`acme`") && d.includes("`tiktok`"), true, "digest lists both");
}

const funnel = parseFunnelForTest(
  "funnel fetched=12 title=4 tech=2 upserted=2 description_missing=3",
);
if (!funnel || funnel.fetched !== 12 || funnel.descriptionMissing !== 3) {
  throw new Error("funnel message with description_missing should parse");
}

assertEqual(safeDate(null), null, "null date");
assertEqual(safeDate("not-a-date"), null, "invalid string");
assertEqual(safeDate(Number.NaN), null, "NaN");
assertEqual(safeDate(new Date("nope")), null, "Invalid Date instance");
const fromSec = safeDate(1785740400);
if (!fromSec || fromSec.getUTCFullYear() < 2026) {
  throw new Error("epoch seconds should coerce");
}
const fromIso = safeDate("2026-08-01T00:00:00Z");
if (!fromIso || fromIso.toISOString() !== "2026-08-01T00:00:00.000Z") {
  throw new Error("ISO string should parse");
}

console.log("ingest policy ok");
