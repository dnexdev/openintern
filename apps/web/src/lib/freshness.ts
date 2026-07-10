import { sql, type SQL } from "drizzle-orm";
import { jobs } from "@openintern/db";

/** Max age (days) for jobs with empty term_years — mirrors ingest MAX_AGE_DAYS_NO_TERM. */
export const MAX_AGE_DAYS_NO_TERM = 120;

/**
 * Board/API freshness:
 * - Keep if any term_year end date is still current/future
 * - Keep empty term_years only if coalesce(posted_at, first_seen_at) is within MAX_AGE_DAYS_NO_TERM
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
