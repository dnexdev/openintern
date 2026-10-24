import { sql, type SQL } from "drizzle-orm";
import { isFresh, MAX_AGE_DAYS_NO_TERM } from "@openintern/proven";
import { jobs } from "./schema";

/** Max age for jobs without an explicit term/year. */
export { MAX_AGE_DAYS_NO_TERM };

export type FreshnessInput = {
  termYears: { term: string; year: number }[] | null;
  postedAt: Date | null;
  firstSeenAt: Date;
};

/** Bend-proven (law fr_stale_iff_not_fresh): exactly the opposite of ingest's staleness. */
export function isFreshJob(input: FreshnessInput, now = new Date()): boolean {
  return isFresh({
    termYears: input.termYears,
    anchor: input.postedAt ?? input.firstSeenAt,
    now,
  });
}

/**
 * Canonical live-job freshness predicate shared by the board, API, sitemap,
 * detail pages, and bulk dumps.
 */
export function freshnessSql(): SQL {
  return sql`(
    (
      jsonb_array_length(coalesce(${jobs.termYears}, '[]'::jsonb)) > 0
      AND EXISTS (
        SELECT 1 FROM jsonb_array_elements(coalesce(${jobs.termYears}, '[]'::jsonb)) ty
        WHERE CASE ty->>'term'
          WHEN 'summer' THEN make_date((ty->>'year')::int, 8, 31)
          WHEN 'fall' THEN make_date((ty->>'year')::int, 12, 31)
          WHEN 'winter' THEN make_date((ty->>'year')::int, 4, 30)
          ELSE make_date((ty->>'year')::int, 12, 31)
        END >= CURRENT_DATE
      )
    )
    OR (
      jsonb_array_length(coalesce(${jobs.termYears}, '[]'::jsonb)) = 0
      AND coalesce(${jobs.postedAt}, ${jobs.firstSeenAt})
        >= (CURRENT_DATE - ${MAX_AGE_DAYS_NO_TERM}::int)
    )
  )`;
}

/**
 * Public corpus rows must be fresh and have at least one canonical role tag.
 * This is a defensive gate for legacy data while stricter ingest retires noise.
 */
export function publicJobSql(): SQL {
  return sql`(
    ${freshnessSql()}
    AND jsonb_typeof(coalesce(${jobs.roles}, '[]'::jsonb)) = 'array'
    AND jsonb_array_length(coalesce(${jobs.roles}, '[]'::jsonb)) > 0
  )`;
}
