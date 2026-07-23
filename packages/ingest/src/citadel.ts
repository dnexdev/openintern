/**
 * Citadel / Citadel Securities careers listing adapters.
 *
 * These sites are WordPress marketing pages (not Greenhouse). Listings are
 * server-rendered as `.careers-listing-card` anchors under
 * `/careers/{board_token}/` with `/page/N/` pagination.
 *
 * board_token: careers path segment, usually `open-opportunities`.
 *
 * CDN may 403 anonymous fetches. Fallbacks:
 *   - OPENINTERN_CITADEL_HTML_PATH / OPENINTERN_CITADEL_SECURITIES_HTML_PATH
 *   - OPENINTERN_BROWSER=1 (Playwright; works from many residential/desktop IPs)
 */

import fs from "node:fs/promises";
import type { NormalizedJob } from "./ats.js";
import { mapWithConcurrency } from "./ats.js";
import { BROWSER_UA, browserEnabled, loadPlaywrightChromium } from "./browser.js";

export type CitadelBrand = "citadel" | "citadel_securities";

const ORIGINS: Record<CitadelBrand, string> = {
  citadel: "https://www.citadel.com",
  citadel_securities: "https://www.citadelsecurities.com",
};

const DUMP_ENV: Record<CitadelBrand, string> = {
  citadel: "OPENINTERN_CITADEL_HTML_PATH",
  citadel_securities: "OPENINTERN_CITADEL_SECURITIES_HTML_PATH",
};

function decodeHtmlEntities(text: string): string {
  return text
    .replace(/&#8211;/g, "–")
    .replace(/&#8212;/g, "—")
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#(\d+);/g, (_, n) => String.fromCharCode(Number(n)))
    .replace(/&nbsp;/g, " ");
}

function stripTags(html: string): string {
  return decodeHtmlEntities(html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim());
}

/** Parse listing cards from a careers page HTML body. */
export function parseCitadelListingHtml(html: string, origin: string): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  const seen = new Set<string>();
  const cardRe =
    /<a\b[^>]*class="[^"]*careers-listing-card[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)<\/a>/gi;

  let match: RegExpExecArray | null;
  while ((match = cardRe.exec(html)) !== null) {
    const hrefRaw = match[1] ?? "";
    const body = match[2] ?? "";
    const href = hrefRaw.replace(/https?:\/\/web\.archive\.org\/web\/\d+\//, "");
    let applyUrl: string;
    try {
      applyUrl = new URL(href, origin).toString().split("?")[0]!;
    } catch {
      continue;
    }
    if (!/\/careers\/details\//i.test(applyUrl)) continue;

    const titleMatch =
      body.match(/<h2[^>]*>([\s\S]*?)<\/h2>/i) ??
      body.match(/data-position="([^"]+)"/i);
    const title = titleMatch ? stripTags(titleMatch[1]!) : "";
    if (!title) continue;

    const locMatch = body.match(/careers-listing-card__location[^>]*>([\s\S]*?)<\//i);
    const locText = locMatch ? stripTags(locMatch[1]!) : "";
    const locations = locText
      ? locText.split(/,\s*/).map((s) => s.trim()).filter(Boolean)
      : [];

    const slug = applyUrl.replace(/\/$/, "").split("/").pop() ?? applyUrl;
    if (seen.has(slug)) continue;
    seen.add(slug);

    jobs.push({
      externalId: slug,
      title,
      locations,
      applyUrl,
      excerpt: null,
      postedAt: null,
      description: "",
    });
  }
  return jobs;
}

export function citadelListingUrl(brand: CitadelBrand, boardToken: string, page = 1): string {
  const origin = ORIGINS[brand];
  const token = boardToken.replace(/^\/+|\/+$/g, "") || "open-opportunities";
  if (page <= 1) return `${origin}/careers/${token}/`;
  return `${origin}/careers/${token}/page/${page}/`;
}

/** Parse job description from a Citadel detail page HTML. */
function parseDetailDescription(html: string): string {
  // Citadel detail pages use a `.career-details__content` or similar wrapper
  const contentMatch =
    html.match(/<div[^>]*class="[^"]*career-details[^"]*"[^>]*>([\s\S]*?)<\/div>\s*<\/div>/i) ??
    html.match(/<article[^>]*>([\s\S]*?)<\/article>/i) ??
    html.match(/<main[^>]*>([\s\S]*?)<\/main>/i);
  if (!contentMatch) return "";
  return stripTags(contentMatch[1] ?? "").slice(0, 2000);
}

/** Fetch descriptions from individual detail pages (best-effort, tolerant of 403). */
async function fetchDetailDescriptions(
  jobs: NormalizedJob[],
): Promise<NormalizedJob[]> {
  return mapWithConcurrency(jobs, 4, async (job) => {
    if (job.description) return job;
    try {
      const html = await fetchHtmlHttp(job.applyUrl);
      const description = parseDetailDescription(html);
      return { ...job, description, excerpt: description ? description.slice(0, 400) : null };
    } catch {
      // CDN may block detail pages — keep the job with empty description
      return job;
    }
  });
}

async function fetchHtmlHttp(url: string): Promise<string> {
  const res = await fetch(url, {
    headers: {
      Accept: "text/html,application/xhtml+xml",
      "User-Agent": BROWSER_UA,
      "Accept-Language": "en-US,en;q=0.9",
    },
    redirect: "follow",
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} for ${url}`);
  }
  return res.text();
}

async function loadHtmlDump(brand: CitadelBrand): Promise<string | null> {
  const envKey = DUMP_ENV[brand];
  const path = process.env[envKey];
  if (!path) return null;
  return fs.readFile(path, "utf8");
}

async function fetchPagesWithBrowser(
  brand: CitadelBrand,
  boardToken: string,
): Promise<string[]> {
  const chromium = await loadPlaywrightChromium();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ userAgent: BROWSER_UA });
    const pages: string[] = [];
    const maxPages = 30;
    for (let p = 1; p <= maxPages; p++) {
      const url = citadelListingUrl(brand, boardToken, p);
      const resp = await page.goto(url, {
        waitUntil: "domcontentloaded",
        timeout: 60_000,
      });
      const status = resp?.status() ?? 0;
      if (status >= 400) {
        if (p === 1) throw new Error(`browser HTTP ${status} for ${url}`);
        break;
      }
      const html = await page.content();
      const count = parseCitadelListingHtml(html, ORIGINS[brand]).length;
      if (count === 0) break;
      pages.push(html);
      if (!/class="[^"]*next page-numbers/i.test(html)) break;
    }
    return pages;
  } finally {
    await browser.close();
  }
}

function mergeJobs(batches: NormalizedJob[]): NormalizedJob[] {
  const all: NormalizedJob[] = [];
  const seen = new Set<string>();
  for (const job of batches) {
    if (seen.has(job.externalId)) continue;
    seen.add(job.externalId);
    all.push(job);
  }
  return all;
}

export async function fetchCitadelBrand(
  brand: CitadelBrand,
  boardToken: string,
): Promise<NormalizedJob[]> {
  const origin = ORIGINS[brand];

  const dump = await loadHtmlDump(brand);
  if (dump) {
    const jobs = parseCitadelListingHtml(dump, origin);
    if (jobs.length === 0) {
      throw new Error(`${DUMP_ENV[brand]} parsed zero listing cards`);
    }
    return fetchDetailDescriptions(jobs);
  }

  const all: NormalizedJob[] = [];
  const seen = new Set<string>();
  const maxPages = 30;
  let httpFailed: Error | null = null;

  try {
    for (let page = 1; page <= maxPages; page++) {
      const url = citadelListingUrl(brand, boardToken, page);
      const html = await fetchHtmlHttp(url);
      const batch = parseCitadelListingHtml(html, origin);
      if (batch.length === 0) break;
      let added = 0;
      for (const job of batch) {
        if (seen.has(job.externalId)) continue;
        seen.add(job.externalId);
        all.push(job);
        added++;
      }
      if (added === 0) break;
      if (!/class="[^"]*next page-numbers/i.test(html) && batch.length < 5) break;
    }
    if (all.length > 0) return fetchDetailDescriptions(all);
    httpFailed = new Error("Citadel listing HTML returned zero cards");
  } catch (err) {
    httpFailed = err instanceof Error ? err : new Error(String(err));
  }

  if (browserEnabled()) {
    const pages = await fetchPagesWithBrowser(brand, boardToken);
    const jobs = mergeJobs(pages.flatMap((html) => parseCitadelListingHtml(html, origin)));
    if (jobs.length === 0) {
      throw new Error(`${brand} browser fetch parsed zero listing cards`);
    }
    return fetchDetailDescriptions(jobs);
  }

  throw new Error(
    `${httpFailed?.message ?? "Citadel fetch failed"}. ` +
      `CDN often blocks anonymous requests. Set ${DUMP_ENV[brand]} to an HTML dump, ` +
      `or OPENINTERN_BROWSER=1 with playwright installed. See CONTRIBUTING.`,
  );
}

export async function fetchCitadel(boardToken: string): Promise<NormalizedJob[]> {
  return fetchCitadelBrand("citadel", boardToken);
}

export async function fetchCitadelSecurities(boardToken: string): Promise<NormalizedJob[]> {
  return fetchCitadelBrand("citadel_securities", boardToken);
}

/** Probe helper: dump env → live HTTP → browser. */
export async function probeCitadelListing(
  brand: CitadelBrand,
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  const origin = ORIGINS[brand];
  try {
    const dump = await loadHtmlDump(brand);
    if (dump) {
      const count = parseCitadelListingHtml(dump, origin).length;
      return {
        ok: count > 0,
        status: 200,
        count,
        error: count > 0 ? undefined : "empty dump listing",
      };
    }

    const url = citadelListingUrl(brand, boardToken, 1);
    try {
      const html = await fetchHtmlHttp(url);
      const count = parseCitadelListingHtml(html, origin).length;
      if (count > 0) return { ok: true, status: 200, count };
    } catch {
      /* fall through */
    }

    if (browserEnabled()) {
      const pages = await fetchPagesWithBrowser(brand, boardToken);
      const count = mergeJobs(
        pages.flatMap((html) => parseCitadelListingHtml(html, origin)),
      ).length;
      return {
        ok: count > 0,
        status: 200,
        count,
        error: count > 0 ? undefined : "empty browser listing",
      };
    }

    return {
      ok: false,
      status: 403,
      error: `HTTP blocked for ${url}; set ${DUMP_ENV[brand]} or OPENINTERN_BROWSER=1`,
    };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
