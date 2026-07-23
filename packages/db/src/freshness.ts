import { sql, type SQL } from "drizzle-orm";
import { jobs } from "./schema";

/** Max age for jobs without an explicit term/year. */
export const MAX_AGE_DAYS_NO_TERM = 120;

export type FreshnessInput = {
  termYears: { term: string; year: number }[] | null;
  postedAt: Date | null;
  firstSeenAt: Date;
};

export function isFreshJob(input: FreshnessInput, now = new Date()): boolean {
  const termYears = input.termYears ?? [];
  if (termYears.length > 0) {
    return termYears.some(({ term, year }) => {
      const month = term === "winter" ? 3 : term === "summer" ? 7 : 11;
      const day = term === "winter" ? 30 : 31;
      return new Date(Date.UTC(year, month, day, 23, 59, 59)) >= now;
    });
  }

  const reference = input.postedAt ?? input.firstSeenAt;
  return now.getTime() - reference.getTime() <= MAX_AGE_DAYS_NO_TERM * 86_400_000;
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
