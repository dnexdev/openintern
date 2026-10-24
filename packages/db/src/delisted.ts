/**
 * Listings pulled by hand: reported closed, wrong, or unsafe. They are hidden
 * from the board, API, sitemap and dumps on the next deploy, even while ingest
 * can't run. Remove an entry once ingest has retired the job.
 *
 * Key: job id (lowercase UUID). Value: issue and reason, for the audit trail.
 */
export const DELISTED_JOBS: ReadonlyMap<string, string> = new Map([
  ["cbb76a8d-0726-4a2a-b84d-8f359be23a90", "#24 Khan Academy SWE Intern Summer 2027: closed (Greenhouse 404), 2026-10-24"],
]);

export function isDelisted(jobId: string): boolean {
  return DELISTED_JOBS.has(jobId.toLowerCase());
}
