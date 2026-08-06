/**
 * Google / DeepMind careers applications HTML results.
 *
 * board_token:
 *   - `google` → q=Software Engineering Intern + target_level=INTERN pages
 *   - `deepmind` → company=DeepMind filter
 */

import type { NormalizedJob } from "./ats.js";
import { stripHtml } from "./strip-html.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
const APPLY_BASE = "https://www.google.com/about/careers/applications/jobs/results";

export function googleCareersSearchUrl(boardToken: string): string {
  const token = boardToken.trim().toLowerCase();
  const u = new URL(APPLY_BASE);
  u.searchParams.set("pageSize", "100");
  if (token === "deepmind" || token === "google-deepmind") {
    u.searchParams.set("company", "DeepMind");
    return u.toString();
  }
  u.searchParams.set("q", "Software Engineering Intern");
  u.searchParams.set("target_level", "INTERN");
  return u.toString();
}

export function parseGoogleCareersHtml(html: string): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  const seen = new Set<string>();

  // Job cards use h3.QJPWVe; filter sidebars also use <h3> so require that class.
  const cardRe =
    /ssk='17:(\d+)'([\s\S]{0,20000}?)<h3 class="QJPWVe">([\s\S]*?)<\/h3>/gi;
  let m: RegExpExecArray | null;
  while ((m = cardRe.exec(html))) {
    const id = m[1]!;
    const block = m[2]!;
    const title = stripHtml(m[3]!);
    if (!title || seen.has(id)) continue;
    seen.add(id);
    const slugMatch = html.match(new RegExp(`jobs/results/${id}-([a-z0-9-]+)`));
    const slug = slugMatch?.[1] ? `${id}-${slugMatch[1]}` : id;
    const locations: string[] = [];
    jobs.push({
      externalId: id,
      title,
      locations,
      applyUrl: `${APPLY_BASE}/${slug}`,
      excerpt: null,
      postedAt: null,
      description: "",
    });
  }

  // Fallback: jobs/results/ID-slug paths with title from slug
  if (jobs.length === 0) {
    const pathRe = /jobs\/results\/(\d+)-([a-z0-9-]+)/g;
    while ((m = pathRe.exec(html))) {
      const id = m[1]!;
      const slugTail = m[2]!;
      if (seen.has(id)) continue;
      seen.add(id);
      const title = slugTail.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
      jobs.push({
        externalId: id,
        title,
        locations: [],
        applyUrl: `${APPLY_BASE}/${id}-${slugTail}`,
        excerpt: null,
        postedAt: null,
        description: "",
      });
    }
  }

  return jobs;
}

export async function fetchGoogle(boardToken: string): Promise<NormalizedJob[]> {
  const token = boardToken.trim().toLowerCase();
  const urls = [googleCareersSearchUrl(boardToken)];
  if (token === "google" || token === "") {
    urls.push(
      `${APPLY_BASE}?q=intern&pageSize=100`,
      `${APPLY_BASE}?target_level=INTERN&pageSize=100`,
      `${APPLY_BASE}?q=Software%20Engineering%20Intern&pageSize=100`,
    );
  }
  if (token === "deepmind" || token === "google-deepmind") {
    urls.push(`${APPLY_BASE}?company=DeepMind&q=Research&pageSize=100`);
  }

  const seen = new Set<string>();
  const all: NormalizedJob[] = [];
  for (const url of urls) {
    const res = await fetch(url, {
      headers: {
        Accept: "text/html",
        "User-Agent": UA,
        "Accept-Language": "en-US,en;q=0.9",
      },
    });
    if (!res.ok) continue;
    for (const job of parseGoogleCareersHtml(await res.text())) {
      if (seen.has(job.externalId)) continue;
      seen.add(job.externalId);
      all.push(job);
    }
  }
  return all;
}

export async function probeGoogle(
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    const jobs = await fetchGoogle(boardToken);
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
