import { isFreshJob, MAX_AGE_DAYS_NO_TERM } from "@openintern/db";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const now = new Date("2027-01-15T12:00:00Z");
assert(
  isFreshJob(
    {
      termYears: [{ term: "summer", year: 2027 }],
      postedAt: new Date("2025-01-01T00:00:00Z"),
      firstSeenAt: new Date("2025-01-01T00:00:00Z"),
    },
    now,
  ),
  "future term remains fresh regardless of posting age",
);
assert(
  !isFreshJob(
    {
      termYears: [{ term: "fall", year: 2026 }],
      postedAt: new Date("2026-12-01T00:00:00Z"),
      firstSeenAt: new Date("2026-12-01T00:00:00Z"),
    },
    now,
  ),
  "past term is stale",
);
assert(
  isFreshJob(
    {
      termYears: [],
      postedAt: new Date(now.getTime() - (MAX_AGE_DAYS_NO_TERM - 1) * 86_400_000),
      firstSeenAt: new Date("2025-01-01T00:00:00Z"),
    },
    now,
  ),
  "recent termless posting is fresh",
);
assert(
  !isFreshJob(
    {
      termYears: [],
      postedAt: null,
      firstSeenAt: new Date(now.getTime() - (MAX_AGE_DAYS_NO_TERM + 1) * 86_400_000),
    },
    now,
  ),
  "old termless posting is stale",
);

console.log("freshness policy ok");
