/**
 * Citadel / Citadel Securities careers: manual capture only.
 *
 * These WordPress careers pages sit behind a CDN that refuses automated
 * clients, and OpenIntern does not pretend to be a browser. The maintainer
 * saves the listing pages (`/careers/open-opportunities/`, `/page/N/`) in a
 * normal browser and runs `pnpm manual:capture citadel <files…>`, which
 * commits data/manual/<brand>.json.
 *
 * board_token: careers path segment, usually `open-opportunities`.
 */

import type { NormalizedJob } from "./ats.js";
import { loadManualJobs, manualProbe } from "./manual.js";

export type CitadelBrand = "citadel" | "citadel_securities";

export const CITADEL_ORIGINS: Record<CitadelBrand, string> = {
  citadel: "https://www.citadel.com",
  citadel_securities: "https://www.citadelsecurities.com",
};

function decodeHtmlEntities(text: string): string {
  return text
    .replace(/&#8211;/g, "–")
    .replace(/&#8212;/g, "—")
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#(\d+);/g, (_, n) => String.fromCharCode(Number(n)))
    .replace(/&nbsp;/g, " ")
    // Last, so "&amp;lt;" decodes to "&lt;" rather than "<".
    .replace(/&amp;/g, "&");
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
  const origin = CITADEL_ORIGINS[brand];
  const token = boardToken.replace(/^\/+|\/+$/g, "") || "open-opportunities";
  if (page <= 1) return `${origin}/careers/${token}/`;
  return `${origin}/careers/${token}/page/${page}/`;
}

export async function fetchCitadel(_boardToken: string): Promise<NormalizedJob[]> {
  return loadManualJobs("citadel");
}

export async function fetchCitadelSecurities(_boardToken: string): Promise<NormalizedJob[]> {
  return loadManualJobs("citadel_securities");
}

export function probeCitadelListing(brand: CitadelBrand, _boardToken: string) {
  return manualProbe(brand);
}
