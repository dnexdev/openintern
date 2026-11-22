/**
 * Shopify careers — public job cards on shopify.com/careers.
 * Intern roles are often password-gated; we ingest publicly listed roles.
 * board_token: `careers` (unused beyond probe URL).
 */

import type { NormalizedJob } from "./ats.js";
import { httpText } from "./http.js";

export const SHOPIFY_CAREERS_URL = "https://www.shopify.com/careers";

function titleFromSlug(slug: string): string {
  return slug
    .replace(/_/g, "-")
    .split("-")
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

export function parseShopifyCareersHtml(html: string): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  const seen = new Set<string>();
  // /careers/role-name_uuid
  const re = /href="(\/careers\/([a-z0-9-]+)_([a-f0-9-]{36}))"/gi;
  let m: RegExpExecArray | null;
  while ((m = re.exec(html))) {
    const path = m[1]!;
    const slug = m[2]!;
    const uuid = m[3]!;
    if (seen.has(uuid)) continue;
    seen.add(uuid);
    jobs.push({
      externalId: uuid,
      title: titleFromSlug(slug),
      locations: [],
      applyUrl: `https://www.shopify.com${path}`,
      excerpt: null,
      postedAt: null,
      description: "",
    });
  }
  return jobs;
}

export async function fetchShopify(_boardToken: string): Promise<NormalizedJob[]> {
  const pages = [SHOPIFY_CAREERS_URL, "https://www.shopify.com/careers/search"];
  const seen = new Set<string>();
  const all: NormalizedJob[] = [];
  for (const url of pages) {
    for (const job of parseShopifyCareersHtml(await httpText(url))) {
      if (seen.has(job.externalId)) continue;
      seen.add(job.externalId);
      all.push(job);
    }
  }
  return all;
}

export async function probeShopify(): Promise<{
  ok: boolean;
  status: number;
  count?: number;
  error?: string;
}> {
  try {
    const jobs = await fetchShopify("careers");
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
