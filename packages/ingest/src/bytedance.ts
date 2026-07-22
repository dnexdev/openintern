/**
 * ByteDance / TikTok careers portal adapters (atsx supplier API).
 *
 * Public JSON (no CSRF when hitting the supplier host):
 *   POST {apiBase}/search/job/posts
 *
 * board_token: `website-path` header value
 *   - bytedance: `en` (joinbytedance.com overseas)
 *   - tiktok: `tiktok` (lifeattiktok.com)
 */

import type { NormalizedJob } from "./ats.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";

export type BytedancePortal = {
  /** ATS name for errors */
  name: "bytedance" | "tiktok";
  apiBase: string;
  origin: string;
  /** Default website-path if board_token empty */
  defaultWebsitePath: string;
  applyBase: string;
};

export const BYTEDANCE_PORTAL: BytedancePortal = {
  name: "bytedance",
  apiBase: "https://jobs.bytedance.com/api/v1/public/supplier",
  origin: "https://joinbytedance.com",
  defaultWebsitePath: "en",
  applyBase: "https://joinbytedance.com/search/",
};

export const TIKTOK_PORTAL: BytedancePortal = {
  name: "tiktok",
  apiBase: "https://api.lifeattiktok.com/api/v1/public/supplier",
  origin: "https://lifeattiktok.com",
  defaultWebsitePath: "tiktok",
  applyBase: "https://lifeattiktok.com/search/",
};

type Named = {
  id?: string;
  en_name?: string | null;
  name?: string | null;
  i18n_name?: string | null;
  parent?: Named | null;
};

type JobPost = {
  id?: string;
  code?: string;
  title?: string;
  description?: string | null;
  requirement?: string | null;
  city_info?: Named & { code?: string };
  recruit_type?: Named;
  job_category?: Named;
};

type SearchResponse = {
  code?: number;
  data?: {
    job_post_list?: JobPost[];
    count?: number;
  };
  message?: string;
};

function displayName(n?: Named | null): string | null {
  if (!n) return null;
  return n.en_name || n.i18n_name || n.name || null;
}

function cityLabel(city?: JobPost["city_info"]): string | null {
  if (!city) return null;
  const parts: string[] = [];
  let cur: Named | null | undefined = city;
  while (cur) {
    const name = displayName(cur);
    if (name) parts.push(name);
    cur = cur.parent;
  }
  // city, region, country — keep city + country when deep
  if (parts.length >= 3) return `${parts[0]}, ${parts[parts.length - 1]}`;
  return parts.join(", ") || null;
}

export function mapBytedancePosts(
  posts: JobPost[],
  portal: BytedancePortal,
): NormalizedJob[] {
  return posts.map((p) => {
    const id = String(p.id ?? p.code ?? "");
    const description = [p.description, p.requirement].filter(Boolean).join("\n\n");
    const loc = cityLabel(p.city_info);
    return {
      externalId: id,
      title: p.title ?? "Untitled",
      locations: loc ? [loc] : [],
      applyUrl: `${portal.applyBase}${encodeURIComponent(id)}`,
      excerpt: description ? description.slice(0, 400) : null,
      postedAt: null,
      description: description ?? "",
    };
  });
}

async function postJson<T>(
  url: string,
  body: unknown,
  headers: Record<string, string>,
): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      "User-Agent": UA,
      ...headers,
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status} for ${url}${text ? `: ${text.slice(0, 120)}` : ""}`);
  }
  return (await res.json()) as T;
}

/**
 * Fetch campus (recruitment_id=2) postings, paginated.
 * Internship classifier on ingest further narrows titles.
 */
export async function fetchBytedancePortal(
  portal: BytedancePortal,
  boardToken: string,
): Promise<NormalizedJob[]> {
  const websitePath = (boardToken || portal.defaultWebsitePath).trim();
  const headers: Record<string, string> = {
    origin: portal.origin,
    Referer: `${portal.origin}/search`,
    "website-path": websitePath,
    "accept-language": "en",
  };

  const limit = 50;
  const all: JobPost[] = [];
  let total = Infinity;

  for (let offset = 0; offset < total && offset < 5000; offset += limit) {
    const page = await postJson<SearchResponse>(
      `${portal.apiBase}/search/job/posts`,
      {
        job_category_id_list: [],
        keyword: "",
        limit,
        offset,
        location_code_list: [],
        // Campus facet — Experienced=1, Campus=2
        recruitment_id_list: ["2"],
        subject_id_list: [],
        portal_type: 2,
        portal_entrance: 1,
      },
      headers,
    );
    if (page.code != null && page.code !== 0) {
      throw new Error(
        `${portal.name} search failed: code=${page.code} ${page.message ?? ""}`.trim(),
      );
    }
    const batch = page.data?.job_post_list ?? [];
    if (typeof page.data?.count === "number") total = page.data.count;
    all.push(...batch);
    if (batch.length === 0) break;
    if (batch.length < limit) break;
  }

  return mapBytedancePosts(all, portal);
}

export async function fetchBytedance(boardToken: string): Promise<NormalizedJob[]> {
  return fetchBytedancePortal(BYTEDANCE_PORTAL, boardToken);
}

export async function fetchTiktok(boardToken: string): Promise<NormalizedJob[]> {
  return fetchBytedancePortal(TIKTOK_PORTAL, boardToken);
}
