import { PgDialect } from "drizzle-orm/pg-core";
import { DELISTED_JOBS, isDelisted, isFreshJob, MAX_AGE_DAYS_NO_TERM, publicJobSql } from "@openintern/db";

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

for (const id of DELISTED_JOBS.keys()) {
  assert(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id), `delisted id ${id} is a lowercase UUID`);
  assert(isDelisted(id.toUpperCase()), "delisting ignores case");
}
assert(!isDelisted("00000000-0000-0000-0000-000000000000"), "unlisted job is not delisted");
const publicSql = new PgDialect().sqlToQuery(publicJobSql());
assert(/not in/i.test(publicSql.sql), "public SQL excludes delisted jobs");
assert(
  [...DELISTED_JOBS.keys()].every((id) => publicSql.params.includes(id)),
  "every delisted id is a bound parameter",
);

console.log("freshness policy ok");
