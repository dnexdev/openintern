import { companyDeactivationPolicy } from "./ingest.js";

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

console.log("ingest policy ok");
