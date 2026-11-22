import { applyUrlAllowed, isDelisted } from "@openintern/db";
import { isKilled } from "@/lib/kill-switch";

/**
 * The last gate before a job is shown or returned by the API: not delisted,
 * not on the runtime kill switch, and an apply link on the allowlist.
 */
export function publishable(companySlug: string, jobId: string, applyUrl: string): boolean {
  if (isDelisted(jobId)) return false;
  if (isKilled(companySlug, jobId, applyUrl)) return false;
  return applyUrlAllowed(companySlug, applyUrl);
}
