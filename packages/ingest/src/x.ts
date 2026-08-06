/**
 * X (Twitter) careers — often CDN-gated.
 * board_token: `x` | greenhouse-style token if a public board appears.
 *
 * Prefers OPENINTERN_X_HTML_PATH dump; optional OPENINTERN_BROWSER.
 * Also probes known Greenhouse tokens as a cheap fallback.
 */

import type { NormalizedJob } from "./ats.js";
import { stripHtml } from "./strip-html.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
const DUMP_ENV = "OPENINTERN_X_HTML_PATH";
const CAREERS_URL = "https://careers.x.com/en";

export function parseXCareersHtml(html: string): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  const seen = new Set<string>();

  // Greenhouse-style embeds or generic job links
  const patterns = [
    /href="(https:\/\/boards\.greenhouse\.io\/[^"]+\/jobs\/(\d+)[^"]*)"[^>]*>([\s\S]*?)<\/a>/gi,
    /href="(https:\/\/job-boards\.greenhouse\.io\/[^"]+\/jobs\/(\d+)[^"]*)"[^>]*>([\s\S]*?)<\/a>/gi,
    /href="(\/en\/jobs\/(\d+)[^"]*)"[^>]*>([\s\S]*?)<\/a>/gi,
  ];
  for (const re of patterns) {
    let m: RegExpExecArray | null;
    while ((m = re.exec(html))) {
      const href = m[1]!;
      const id = m[2]!;
      const title = stripHtml(m[3] || `Job ${id}`);
      if (seen.has(id)) continue;
      seen.add(id);
      const applyUrl = href.startsWith("http") ? href : `https://careers.x.com${href}`;
      jobs.push({
        externalId: id,
        title,
        locations: [],
        applyUrl,
        excerpt: null,
        postedAt: null,
        description: "",
      });
    }
  }
  return jobs;
}

async function tryGreenhouseFallback(): Promise<NormalizedJob[]> {
  for (const token of ["x", "twitter", "twitterintl", "xcorp"]) {
    try {
      const res = await fetch(
        `https://boards-api.greenhouse.io/v1/boards/${token}/jobs`,
        { headers: { Accept: "application/json", "User-Agent": UA } },
      );
      if (!res.ok) continue;
      const data = (await res.json()) as {
        jobs?: Array<{
          id: number | string;
          title: string;
          absolute_url: string;
          location?: { name?: string };
        }>;
      };
      return (data.jobs ?? []).map((j) => ({
        externalId: String(j.id),
        title: j.title,
        locations: j.location?.name ? [j.location.name] : [],
        applyUrl: j.absolute_url,
        excerpt: null,
        postedAt: null,
        description: "",
      }));
    } catch {
      /* try next */
    }
  }
  return [];
}

export async function fetchX(_boardToken: string): Promise<NormalizedJob[]> {
  const dumpPath = process.env[DUMP_ENV]?.trim();
  if (dumpPath) {
    const fs = await import("node:fs/promises");
    return parseXCareersHtml(await fs.readFile(dumpPath, "utf8"));
  }

  try {
    const res = await fetch(CAREERS_URL, {
      headers: { Accept: "text/html", "User-Agent": UA },
      redirect: "follow",
    });
    if (res.ok) {
      const jobs = parseXCareersHtml(await res.text());
      if (jobs.length > 0) return jobs;
    }
  } catch {
    /* gated */
  }

  const gh = await tryGreenhouseFallback();
  if (gh.length > 0) return gh;

  throw new Error(
    `X careers gated or empty. Set ${DUMP_ENV} to an HTML dump of ${CAREERS_URL}`,
  );
}

export async function probeX(): Promise<{
  ok: boolean;
  status: number;
  count?: number;
  error?: string;
}> {
  try {
    const jobs = await fetchX("x");
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
