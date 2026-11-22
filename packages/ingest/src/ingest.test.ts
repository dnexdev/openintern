import { OUTAGE_GAP_DAYS, companyDeactivationPolicy, shouldSkipLastSeenSweep } from "./ingest.js";
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

assertEqual(
  companyDeactivationPolicy({ fetchSucceeded: true, fetched: 20, techPass: 3 }),
  "missing",
  "normal run deactivates jobs missing from the fetched board",
);
assertEqual(
  companyDeactivationPolicy({ fetchSucceeded: true, fetched: 0, techPass: 0 }),
  "all",
  "confirmed empty board deactivates all company jobs",
);
assertEqual(
  companyDeactivationPolicy({ fetchSucceeded: true, fetched: 20, techPass: 0 }),
  "none",
  "classifier zero-match retains existing jobs",
);
assertEqual(
  companyDeactivationPolicy({ fetchSucceeded: false, fetched: 0, techPass: 0 }),
  "none",
  "failed fetch retains existing jobs",
);

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
