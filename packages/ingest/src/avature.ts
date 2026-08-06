/**
 * Avature public careers SearchJobs HTML.
 *
 * board_token:
 *   - `bloomberg` → https://bloomberg.avature.net/careers/SearchJobs/
 *   - `ibmglobal` → https://ibmglobal.avature.net/en_US/careers/SearchJobs/
 *   - `bloomberg.avature.net` or full SearchJobs URL
 */

import type { NormalizedJob } from "./ats.js";
import { stripHtml } from "./strip-html.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
const DUMP_ENV = "OPENINTERN_AVATURE_HTML_PATH";

export function avatureSearchUrl(boardToken: string, keywords = "intern"): string {
  const raw = boardToken.trim();
  if (/^https?:\/\//i.test(raw)) {
    const u = new URL(raw);
    if (!u.searchParams.has("keywords")) u.searchParams.set("keywords", keywords);
    if (!u.searchParams.has("jobRecordsPerPage")) u.searchParams.set("jobRecordsPerPage", "100");
    return u.toString();
  }
  const host = raw.includes(".")
    ? raw.replace(/^https?:\/\//, "").replace(/\/.*$/, "")
    : `${raw}.avature.net`;
  const path = host.startsWith("ibm") ? "/en_US/careers/SearchJobs/" : "/careers/SearchJobs/";
  const u = new URL(`https://${host}${path}`);
  u.searchParams.set("listFilterMode", "1");
  u.searchParams.set("jobRecordsPerPage", "100");
  u.searchParams.set("keywords", keywords);
  return u.toString();
}

export function parseAvatureSearchHtml(html: string, baseUrl: string): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  const seen = new Set<string>();
  const origin = new URL(baseUrl).origin;

  // <a class="link" href=".../JobDetail/Slug/12345">Title</a>
  const linkRe =
    /<a[^>]+href="([^"]*\/(?:careers\/)?JobDetail\/([^"/]+)\/(\d+))"\s*[^>]*>([\s\S]*?)<\/a>/gi;
  let m: RegExpExecArray | null;
  while ((m = linkRe.exec(html))) {
    const href = m[1]!;
    const slug = m[2]!;
    const id = m[3]!;
    const title = stripHtml(m[4] || slug.replace(/-/g, " "));
    if (!title || seen.has(id)) continue;
    seen.add(id);
    const applyUrl = href.startsWith("http") ? href : `${origin}${href}`;
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

  // Attach nearby locations when present after each JobDetail block
  if (jobs.length > 0) {
    const locRe =
      /JobDetail\/[^"/]+\/(\d+)[\s\S]{0,1200}?class="[^"]*list-item-location[^"]*"[^>]*>([^<]+)/gi;
    const locMap = new Map<string, string>();
    while ((m = locRe.exec(html))) {
      locMap.set(m[1]!, stripHtml(m[2]!));
    }
    for (const job of jobs) {
      const loc = locMap.get(job.externalId);
      if (loc) job.locations = [loc];
    }
  }

  return jobs;
}

async function loadDump(): Promise<string | null> {
  const path = process.env[DUMP_ENV]?.trim();
  if (!path) return null;
  const fs = await import("node:fs/promises");
  return fs.readFile(path, "utf8");
}

export async function fetchAvature(boardToken: string): Promise<NormalizedJob[]> {
  const dump = await loadDump();
  if (dump) {
    const url = avatureSearchUrl(boardToken);
    return parseAvatureSearchHtml(dump, url);
  }

  const keywordQueries = ["intern", "Intern", "Software Engineer Intern", ""];
  const seen = new Set<string>();
  const all: NormalizedJob[] = [];

  for (const keywords of keywordQueries) {
    const url = avatureSearchUrl(boardToken, keywords);
    const res = await fetch(url, {
      headers: {
        Accept: "text/html",
        "User-Agent": UA,
      },
      redirect: "follow",
    });
    if (!res.ok) continue;
    const html = await res.text();
    for (const job of parseAvatureSearchHtml(html, url)) {
      if (seen.has(job.externalId)) continue;
      seen.add(job.externalId);
      all.push(job);
    }
  }

  if (all.length === 0) {
    throw new Error(
      `Avature empty response for ${boardToken}. Optional dump: ${DUMP_ENV}`,
    );
  }
  return all;
}

export async function probeAvature(
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    const jobs = await fetchAvature(boardToken);
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
