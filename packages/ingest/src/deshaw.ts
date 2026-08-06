/**
 * D. E. Shaw careers — Next.js __NEXT_DATA__ on internships page.
 * board_token: `internships` (default) or unused placeholder.
 */

import type { NormalizedJob } from "./ats.js";
import { stripHtmlTags } from "./strip-html.js";

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";
export const DESHAW_INTERNSHIPS_URL = "https://www.deshaw.com/careers/internships";

type DeshawJobData = {
  id?: number | string;
  displayName?: string;
  jobUrl?: string;
  jobDescription?: string;
  jobMetadata?: {
    jobLocations?: Array<{ name?: string; abbreviation?: string }>;
  };
};

type DeshawJob = {
  id?: number | string;
  displayName?: string;
  office?: Array<{ name?: string; abbreviation?: string }>;
  data?: DeshawJobData;
};

type NextData = {
  props?: {
    pageProps?: {
      internships?: DeshawJob[];
    };
  };
};

export function parseDeshawNextData(raw: unknown): NormalizedJob[] {
  const data = raw as NextData;
  const rows = data.props?.pageProps?.internships ?? [];
  const jobs: NormalizedJob[] = [];
  for (const row of rows) {
    const d: DeshawJobData = row.data ?? {
      id: row.id,
      displayName: row.displayName,
    };
    const id = String(d.id ?? row.id ?? "").trim();
    const title = (d.displayName || row.displayName || "").trim();
    if (!id || !title) continue;
    const jobUrl = d.jobUrl || `${id}`;
    const applyUrl = jobUrl.startsWith("http")
      ? jobUrl
      : `https://www.deshaw.com/careers/${jobUrl.replace(/^\//, "")}`;
    const locations: string[] = [];
    for (const o of row.office ?? []) {
      if (o.name && !locations.includes(o.name)) locations.push(o.name);
    }
    for (const loc of d.jobMetadata?.jobLocations ?? []) {
      if (loc.name && !locations.includes(loc.name)) locations.push(loc.name);
    }
    const description =
      typeof d.jobDescription === "string" ? stripHtmlTags(d.jobDescription) : "";
    jobs.push({
      externalId: id,
      title,
      locations,
      applyUrl,
      excerpt: description ? description.slice(0, 400) : null,
      postedAt: null,
      description,
    });
  }
  return jobs;
}

export function parseDeshawHtml(html: string): NormalizedJob[] {
  const m = html.match(/<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)<\/script>/);
  if (!m?.[1]) return [];
  return parseDeshawNextData(JSON.parse(m[1]));
}

export async function fetchDeshaw(_boardToken: string): Promise<NormalizedJob[]> {
  const res = await fetch(DESHAW_INTERNSHIPS_URL, {
    headers: { Accept: "text/html", "User-Agent": UA },
  });
  if (!res.ok) {
    throw new Error(`D.E. Shaw HTTP ${res.status} for ${DESHAW_INTERNSHIPS_URL}`);
  }
  const jobs = parseDeshawHtml(await res.text());
  if (jobs.length === 0) {
    throw new Error("D.E. Shaw internships page parsed zero roles");
  }
  return jobs;
}

export async function probeDeshaw(): Promise<{
  ok: boolean;
  status: number;
  count?: number;
  error?: string;
}> {
  try {
    const jobs = await fetchDeshaw("internships");
    return { ok: jobs.length > 0, status: 200, count: jobs.length };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
