/**
 * IBM careers — Avature tenant ibmglobal (HTML SearchJobs).
 * board_token: `ibmglobal` (default).
 */

import type { NormalizedJob } from "./ats.js";
import { fetchAvature, parseAvatureSearchHtml, avatureSearchUrl } from "./avature.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";

export async function fetchIbm(boardToken: string): Promise<NormalizedJob[]> {
  const token = boardToken.trim() || "ibmglobal";
  // Prefer Avature search with Software Intern keywords
  const url = avatureSearchUrl(token, "Software Intern");
  const res = await fetch(url, {
    headers: { Accept: "text/html", "User-Agent": UA },
    redirect: "follow",
  });
  if (res.ok) {
    const html = await res.text();
    const jobs = parseAvatureSearchHtml(html, url);
    if (jobs.length > 0) return jobs;
  }
  // Fallback: generic intern keyword via shared Avature fetcher
  return fetchAvature(token);
}

export async function probeIbm(
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    const jobs = await fetchIbm(boardToken);
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
