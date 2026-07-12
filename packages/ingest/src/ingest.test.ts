import { companyDeactivationPolicy } from "./ingest.js";

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

const funnel = parseFunnelForTest(
  "funnel fetched=12 title=4 tech=2 upserted=2 description_missing=3",
);
if (!funnel || funnel.fetched !== 12 || funnel.descriptionMissing !== 3) {
  throw new Error("funnel message with description_missing should parse");
}

console.log("ingest policy ok");
