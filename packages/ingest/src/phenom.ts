/**
 * Phenom People public careers JSON API.
 *
 * board_token forms:
 *   - `amd` → https://careers.amd.com/api/jobs
 *   - `careers.amd.com` → same host
 *   - full origin `https://careers.amd.com`
 */

import type { NormalizedJob } from "./ats.js";
import { stripHtmlTags } from "./strip-html.js";
import { safeDate } from "./safe-date.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
const PAGE_LIMIT = 100;
const MAX_PAGES = 30;

export type PhenomJobData = {
  slug?: string;
  req_id?: string;
  title?: string;
  description?: string;
  location_name?: string;
  city?: string;
  state?: string;
  country?: string;
  full_location?: string;
  short_location?: string;
  apply_url?: string;
  posted_date?: string;
  create_date?: string;
  meta_data?: { canonical_url?: string };
  multipleLocations?: Array<{ city?: string; state?: string; country?: string; name?: string }>;
};

export type PhenomJobRow = { data?: PhenomJobData };

export type PhenomJobsResponse = {
  jobs?: PhenomJobRow[];
  totalCount?: number;
  count?: number;
};

export function phenomCareersOrigin(boardToken: string): string {
  const raw = boardToken.trim();
  if (/^https?:\/\//i.test(raw)) {
    return raw.replace(/\/+$/, "");
  }
  if (raw.includes(".")) {
    return `https://${raw.replace(/\/+$/, "")}`;
  }
  return `https://careers.${raw}.com`;
}

export function phenomJobsApiUrl(boardToken: string, page: number): string {
  const origin = phenomCareersOrigin(boardToken);
  const u = new URL(`${origin}/api/jobs`);
  u.searchParams.set("keywords", "");
  u.searchParams.set("limit", String(PAGE_LIMIT));
  u.searchParams.set("page", String(page));
  u.searchParams.set("sortBy", "relevance");
  u.searchParams.set("descending", "false");
  u.searchParams.set("internal", "false");
  return u.toString();
}

function locationLabels(d: PhenomJobData): string[] {
  const out: string[] = [];
  const push = (v?: string) => {
    const t = v?.trim();
    if (t && !out.includes(t)) out.push(t);
  };
  push(d.full_location);
  push(d.short_location);
  if (d.city || d.state || d.country) {
    push([d.city, d.state, d.country].filter(Boolean).join(", "));
  }
  push(d.location_name);
  for (const loc of Array.isArray(d.multipleLocations) ? d.multipleLocations : []) {
    push(loc.name || [loc.city, loc.state, loc.country].filter(Boolean).join(", "));
  }
  return out;
}

export function mapPhenomJobs(
  rows: PhenomJobRow[],
  origin: string,
): NormalizedJob[] {
  const jobs: NormalizedJob[] = [];
  for (const row of rows) {
    const d = row.data;
    if (!d?.title) continue;
    const id = String(d.req_id || d.slug || "").trim();
    if (!id) continue;
    const applyUrl =
      d.meta_data?.canonical_url?.trim() ||
      d.apply_url?.trim() ||
      `${origin}/jobs/${encodeURIComponent(d.slug || id)}`;
    const description = d.description ? stripHtmlTags(d.description) : "";
    const postedRaw = d.posted_date || d.create_date;
    jobs.push({
      externalId: id,
      title: d.title,
      locations: locationLabels(d),
      applyUrl,
      excerpt: description ? description.slice(0, 400) : null,
      postedAt: safeDate(postedRaw),
      description,
    });
  }
  return jobs;
}

export async function fetchPhenom(boardToken: string): Promise<NormalizedJob[]> {
  const origin = phenomCareersOrigin(boardToken);
  const all: NormalizedJob[] = [];
  const seen = new Set<string>();

  for (let page = 1; page <= MAX_PAGES; page++) {
    const url = phenomJobsApiUrl(boardToken, page);
    const res = await fetch(url, {
      headers: { Accept: "application/json", "User-Agent": UA },
    });
    if (!res.ok) {
      throw new Error(`Phenom HTTP ${res.status} for ${url}`);
    }
    const data = (await res.json()) as PhenomJobsResponse;
    const rows = data.jobs ?? [];
    if (rows.length === 0) break;
    for (const job of mapPhenomJobs(rows, origin)) {
      if (seen.has(job.externalId)) continue;
      seen.add(job.externalId);
      all.push(job);
    }
    const total = data.totalCount ?? data.count ?? all.length;
    if (all.length >= total || rows.length < PAGE_LIMIT) break;
  }

  return all;
}

export async function probePhenom(
  boardToken: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  const url = phenomJobsApiUrl(boardToken, 1);
  try {
    const res = await fetch(url, {
      headers: { Accept: "application/json", "User-Agent": UA },
    });
    if (!res.ok) return { ok: false, status: res.status, error: `HTTP ${res.status}` };
    const data = (await res.json()) as PhenomJobsResponse;
    const count = data.totalCount ?? data.count ?? (data.jobs?.length ?? 0);
    return { ok: count > 0, status: res.status, count, error: count > 0 ? undefined : "empty" };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
