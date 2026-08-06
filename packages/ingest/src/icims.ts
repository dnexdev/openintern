/**
 * iCIMS public job search HTML.
 *
 * board_token:
 *   - `sig` or `careers-sig` → https://careers-sig.icims.com/jobs/search
 *   - full host `careers-sig.icims.com`
 */

import type { NormalizedJob } from "./ats.js";
import { stripHtml } from "./strip-html.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
const DUMP_ENV = "OPENINTERN_ICIMS_HTML_PATH";

export function icimsHost(boardToken: string): string {
  const raw = boardToken.trim().replace(/^https?:\/\//, "").replace(/\/.*$/, "");
  if (raw.includes(".")) return raw;
  if (raw.startsWith("careers-")) return `${raw}.icims.com`;
  return `careers-${raw}.icims.com`;
}

export function icimsSearchUrl(boardToken: string, keyword = "intern"): string {
  const host = icimsHost(boardToken);
  const u = new URL(`https://${host}/jobs/search`);
  u.searchParams.set("ss", "1");
  u.searchParams.set("searchKeyword", keyword);
  u.searchParams.set("searchRelation", "keyword_all");
  u.searchParams.set("in_iframe", "1");
  return u.toString();
}

export function parseIcimsSearchHtml(html: string, baseUrl: string): NormalizedJob[] {
  const origin = new URL(baseUrl).origin;
  const seen = new Set<string>();

  // /jobs/11099/analytics-internship.../job
  const linkRe = /href="([^"]*\/jobs\/(\d+)\/([^"?]+)\/job[^"]*)"/gi;
  const h3Re = /<h3[^>]*>([\s\S]*?)<\/h3>/gi;
  const h3s: string[] = [];
  let m: RegExpExecArray | null;
  while ((m = h3Re.exec(html))) {
    const t = stripHtml(m[1]!);
    if (t) h3s.push(t);
  }

  const links: Array<{ id: string; href: string; slug: string }> = [];
  while ((m = linkRe.exec(html))) {
    const href = m[1]!;
    const id = m[2]!;
    const slug = decodeURIComponent(m[3]!).replace(/\+/g, " ");
    if (seen.has(id)) continue;
    seen.add(id);
    links.push({ id, href, slug });
  }

  return links.map((link, i) => {
    const title =
      h3s[i] ||
      link.slug
        .replace(/%3a/gi, ":")
        .replace(/%2b/gi, "+")
        .replace(/-/g, " ")
        .replace(/\s+/g, " ")
        .trim();
    const applyUrl = link.href.startsWith("http")
      ? link.href.replace(/[?&]in_iframe=1/, "")
      : `${origin}${link.href}`.replace(/[?&]in_iframe=1/, "");
    return {
      externalId: link.id,
      title,
      locations: [],
      applyUrl,
      excerpt: null,
      postedAt: null,
      description: "",
    };
  });
}

async function loadDump(): Promise<string | null> {
  const path = process.env[DUMP_ENV]?.trim();
  if (!path) return null;
  const fs = await import("node:fs/promises");
  return fs.readFile(path, "utf8");
}

export async function fetchIcims(boardToken: string): Promise<NormalizedJob[]> {
  const url = icimsSearchUrl(boardToken);
  const dump = await loadDump();
  if (dump) return parseIcimsSearchHtml(dump, url);

  const res = await fetch(url, {
    headers: { Accept: "text/html", "User-Agent": UA },
    redirect: "follow",
  });
  if (!res.ok) {
    throw new Error(`iCIMS HTTP ${res.status} for ${url}. Optional dump: ${DUMP_ENV}`);
  }
  return parseIcimsSearchHtml(await res.text(), url);
}

export async function probeIcims(
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    const jobs = await fetchIcims(boardToken);
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
