import {
  parseWorkdayToken,
  workdayApplyUrl,
  workdayDetailUrl,
  workdayJobsUrl,
} from "./workday.js";
import { fetchCitadel, fetchCitadelSecurities } from "./citadel.js";
import { fetchTesla } from "./tesla.js";
import { fetchBytedance, fetchTiktok } from "./bytedance.js";

export type NormalizedJob = {
  externalId: string;
  title: string;
  locations: string[];
  applyUrl: string;
  excerpt: string | null;
  postedAt: Date | null;
  description: string;
};

export type AtsName =
  | "greenhouse"
  | "lever"
  | "ashby"
  | "workable"
  | "smartrecruiters"
  | "recruitee"
  | "rippling"
  | "bamboohr"
  | "workday"
  | "citadel"
  | "citadel_securities"
  | "tesla"
  | "bytedance"
  | "tiktok";

/** Proprietary careers adapters (not Greenhouse-style public boards). */
export const PROPRIETARY_ATS: AtsName[] = [
  "citadel",
  "citadel_securities",
  "tesla",
  "bytedance",
  "tiktok",
];

async function sleep(ms: number) {
  await new Promise((r) => setTimeout(r, ms));
}

export async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  fn: (item: T) => Promise<R>,
): Promise<R[]> {
  const results: R[] = new Array(items.length);
  let index = 0;
  const workers = Array.from({ length: Math.min(concurrency, items.length) }, async () => {
    while (index < items.length) {
      const i = index++;
      results[i] = await fn(items[i]!);
    }
  });
  await Promise.all(workers);
  return results;
}

async function fetchJson<T>(url: string, retries = 3): Promise<T> {
  let lastErr: Error | null = null;
  for (let attempt = 0; attempt < retries; attempt++) {
    try {
      const res = await fetch(url, {
        headers: {
          Accept: "application/json",
          "User-Agent": "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)",
        },
      });
      if (res.ok) {
        return (await res.json()) as T;
      }
      // Don't retry client errors except 429
      if (res.status !== 429 && res.status < 500) {
        throw new Error(`HTTP ${res.status} for ${url}`);
      }
      lastErr = new Error(`HTTP ${res.status} for ${url}`);
    } catch (err) {
      lastErr = err instanceof Error ? err : new Error(String(err));
      if (lastErr.message.startsWith("HTTP 4") && !lastErr.message.includes("HTTP 429")) {
        throw lastErr;
      }
    }
    await sleep(400 * 2 ** attempt);
  }
  throw lastErr ?? new Error(`Failed to fetch ${url}`);
}

type GreenhouseJob = {
  id: number;
  title: string;
  absolute_url: string;
  updated_at?: string;
  location?: { name?: string };
  content?: string;
};

type GreenhouseResponse = { jobs: GreenhouseJob[] };

export async function fetchGreenhouse(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<GreenhouseResponse>(
    `https://boards-api.greenhouse.io/v1/boards/${encodeURIComponent(boardToken)}/jobs?content=true`,
  );
  return (data.jobs ?? []).map((j) => ({
    externalId: String(j.id),
    title: j.title,
    locations: j.location?.name ? [j.location.name] : [],
    applyUrl: j.absolute_url,
    excerpt: null,
    postedAt: j.updated_at ? new Date(j.updated_at) : null,
    description: j.content ?? "",
  }));
}

type LeverJob = {
  id: string;
  text: string;
  hostedUrl: string;
  createdAt?: number;
  categories?: { location?: string; commitment?: string; team?: string };
  descriptionPlain?: string;
  description?: string;
};

export async function fetchLever(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<LeverJob[]>(
    `https://api.lever.co/v0/postings/${encodeURIComponent(boardToken)}?mode=json`,
  );
  return (data ?? []).map((j) => ({
    externalId: j.id,
    title: j.text,
    locations: j.categories?.location ? [j.categories.location] : [],
    applyUrl: j.hostedUrl,
    excerpt: j.descriptionPlain?.slice(0, 400) ?? null,
    postedAt: j.createdAt ? new Date(j.createdAt) : null,
    description: j.descriptionPlain ?? j.description ?? "",
  }));
}

type AshbyJob = {
  id: string;
  title: string;
  jobUrl: string;
  location?: string;
  isRemote?: boolean;
  publishedAt?: string;
  descriptionPlain?: string;
  descriptionHtml?: string;
};

type AshbyResponse = { jobs: AshbyJob[] };

export async function fetchAshby(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<AshbyResponse>(
    `https://api.ashbyhq.com/posting-api/job-board/${encodeURIComponent(boardToken)}?includeCompensation=true`,
  );
  return (data.jobs ?? []).map((j) => {
    const locations: string[] = [];
    if (j.location) locations.push(j.location);
    if (j.isRemote && !locations.some((l) => /remote/i.test(l))) {
      locations.push("Remote");
    }
    return {
      externalId: j.id,
      title: j.title,
      locations,
      applyUrl: j.jobUrl,
      excerpt: j.descriptionPlain?.slice(0, 400) ?? null,
      postedAt: j.publishedAt ? new Date(j.publishedAt) : null,
      description: j.descriptionPlain ?? j.descriptionHtml ?? "",
    };
  });
}

type WorkableJob = {
  shortcode?: string;
  id?: string;
  title: string;
  url?: string;
  application_url?: string;
  shortlink?: string;
  location?: { city?: string; region?: string; country?: string; telecommuting?: boolean };
  locations?: { city?: string; region?: string; country?: string }[];
  telecommuting?: boolean;
  published_on?: string;
  created_at?: string;
  description?: string;
  full_description?: string;
};

type WorkableResponse = { name?: string; jobs?: WorkableJob[] };

function workableLocation(loc?: {
  city?: string;
  region?: string;
  country?: string;
}): string | null {
  if (!loc) return null;
  const parts = [loc.city, loc.region, loc.country].filter(Boolean);
  return parts.length > 0 ? parts.join(", ") : null;
}

export async function fetchWorkable(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<WorkableResponse>(
    `https://www.workable.com/api/accounts/${encodeURIComponent(boardToken)}?details=true`,
  );
  return (data.jobs ?? []).map((j) => {
    const locations: string[] = [];
    const primary = workableLocation(j.location);
    if (primary) locations.push(primary);
    for (const extra of j.locations ?? []) {
      const s = workableLocation(extra);
      if (s && !locations.includes(s)) locations.push(s);
    }
    if ((j.telecommuting || j.location?.telecommuting) && !locations.some((l) => /remote/i.test(l))) {
      locations.push("Remote");
    }
    const description = j.full_description ?? j.description ?? "";
    const posted = j.published_on ?? j.created_at;
    return {
      externalId: j.shortcode ?? j.id ?? j.url ?? j.title,
      title: j.title,
      locations,
      applyUrl: j.application_url ?? j.url ?? j.shortlink ?? "",
      excerpt: null,
      postedAt: posted ? new Date(posted) : null,
      description,
    };
  });
}

type SmartRecruitersPosting = {
  id: string;
  uuid?: string;
  name: string;
  releasedDate?: string;
  location?: { city?: string; region?: string; country?: string; remote?: boolean };
  company?: { identifier?: string };
  jobAd?: { sections?: Record<string, { title?: string; text?: string }> };
  applyUrl?: string;
  postingUrl?: string;
  ref?: string;
};

type SmartRecruitersResponse = { content?: SmartRecruitersPosting[]; totalFound?: number };

export async function fetchSmartRecruiters(boardToken: string): Promise<NormalizedJob[]> {
  const limit = 100;
  const all: SmartRecruitersPosting[] = [];
  for (let offset = 0; ; offset += limit) {
    const data = await fetchJson<SmartRecruitersResponse>(
      `https://api.smartrecruiters.com/v1/companies/${encodeURIComponent(boardToken)}/postings?limit=${limit}&offset=${offset}`,
    );
    const batch = data.content ?? [];
    all.push(...batch);
    if (batch.length < limit) break;
  }
  return all.map((j) => {
    const locations: string[] = [];
    const parts = [j.location?.city, j.location?.region, j.location?.country].filter(Boolean);
    if (parts.length > 0) locations.push(parts.join(", "));
    if (j.location?.remote && !locations.some((l) => /remote/i.test(l))) {
      locations.push("Remote");
    }
    const description = Object.values(j.jobAd?.sections ?? {})
      .map((s) => s?.text ?? "")
      .join(" ");
    return {
      externalId: j.uuid ?? j.id,
      title: j.name,
      locations,
      applyUrl:
        j.applyUrl ??
        j.postingUrl ??
        `https://jobs.smartrecruiters.com/${encodeURIComponent(boardToken)}/${j.id}`,
      excerpt: null,
      postedAt: j.releasedDate ? new Date(j.releasedDate) : null,
      description,
    };
  });
}

type RecruiteeOffer = {
  id: number | string;
  title?: string;
  slug?: string;
  careers_url?: string;
  published_at?: string;
  created_at?: string;
  description?: string;
  requirements?: string;
  location?: string;
  city?: string;
  country?: string;
  remote?: boolean;
  locations?: { city?: string; country?: string; name?: string }[];
};

type RecruiteeResponse = { offers?: RecruiteeOffer[] };

export async function fetchRecruitee(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<RecruiteeResponse>(
    `https://${encodeURIComponent(boardToken)}.recruitee.com/api/offers/`,
  );
  return (data.offers ?? []).map((j) => {
    const locations: string[] = [];
    if (j.location) locations.push(j.location);
    const cityCountry = [j.city, j.country].filter(Boolean).join(", ");
    if (cityCountry && !locations.includes(cityCountry)) locations.push(cityCountry);
    for (const loc of j.locations ?? []) {
      const s = loc.name ?? [loc.city, loc.country].filter(Boolean).join(", ");
      if (s && !locations.includes(s)) locations.push(s);
    }
    if (j.remote && !locations.some((l) => /remote/i.test(l))) locations.push("Remote");
    const description = [j.description, j.requirements].filter(Boolean).join("\n\n");
    return {
      externalId: String(j.id),
      title: j.title ?? "Untitled",
      locations,
      applyUrl:
        j.careers_url ??
        `https://${encodeURIComponent(boardToken)}.recruitee.com/o/${j.slug ?? j.id}`,
      excerpt: null,
      postedAt: j.published_at
        ? new Date(j.published_at)
        : j.created_at
          ? new Date(j.created_at)
          : null,
      description,
    };
  });
}

type RipplingJob = {
  uuid: string;
  name: string;
  url?: string;
  workLocation?: { label?: string };
  workLocations?: { label?: string }[];
  createdOn?: string;
  description?: string | { company?: string; role?: string };
};

export async function fetchRippling(boardToken: string): Promise<NormalizedJob[]> {
  const list = await fetchJson<RipplingJob[]>(
    `https://api.rippling.com/platform/api/ats/v1/board/${encodeURIComponent(boardToken)}/jobs`,
  );
  const jobs = list ?? [];
  const withDesc = await mapWithConcurrency(jobs, 8, async (j) => {
    try {
      const detail = await fetchJson<RipplingJob>(
        `https://api.rippling.com/platform/api/ats/v1/board/${encodeURIComponent(boardToken)}/jobs/${j.uuid}`,
      );
      return { ...j, ...detail };
    } catch {
      return j;
    }
  });
  return withDesc.map((j) => {
    const locations: string[] = [];
    if (j.workLocation?.label) locations.push(j.workLocation.label);
    for (const loc of j.workLocations ?? []) {
      if (loc.label && !locations.includes(loc.label)) locations.push(loc.label);
    }
    let description = "";
    if (typeof j.description === "string") description = j.description;
    else if (j.description && typeof j.description === "object") {
      description = [j.description.company, j.description.role].filter(Boolean).join("\n\n");
    }
    return {
      externalId: j.uuid,
      title: j.name,
      locations,
      applyUrl:
        j.url ??
        `https://ats.rippling.com/${encodeURIComponent(boardToken)}/jobs/${j.uuid}`,
      excerpt: null,
      postedAt: j.createdOn ? new Date(j.createdOn) : null,
      description,
    };
  });
}

type BambooJob = {
  id: string;
  jobOpeningName: string;
  location?: { city?: string; state?: string };
  atsLocation?: { city?: string; state?: string; country?: string };
  isRemote?: boolean | null;
};

type BambooResponse = { result?: BambooJob[]; meta?: { totalCount?: number } };

type BambooDetailResponse = {
  result?: {
    jobOpening?: {
      description?: string;
      datePosted?: string;
    };
  };
};

function stripHtmlTags(html: string): string {
  return html
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/p>/gi, "\n")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/\s+/g, " ")
    .trim();
}

async function fetchBambooHrDetail(
  boardToken: string,
  jobId: string,
): Promise<{ description: string; postedAt: Date | null }> {
  try {
    const data = await fetchJson<BambooDetailResponse>(
      `https://${encodeURIComponent(boardToken)}.bamboohr.com/careers/${encodeURIComponent(jobId)}/detail`,
    );
    const opening = data.result?.jobOpening;
    const description = opening?.description ? stripHtmlTags(opening.description) : "";
    const postedAt = opening?.datePosted ? new Date(opening.datePosted) : null;
    return { description, postedAt };
  } catch {
    return { description: "", postedAt: null };
  }
}

export async function fetchBambooHr(boardToken: string): Promise<NormalizedJob[]> {
  const data = await fetchJson<BambooResponse>(
    `https://${encodeURIComponent(boardToken)}.bamboohr.com/careers/list`,
  );
  const openings = data.result ?? [];
  const withDetails = await mapWithConcurrency(openings, 8, async (j) => {
    const detail = await fetchBambooHrDetail(boardToken, String(j.id));
    return { job: j, ...detail };
  });
  return withDetails.map(({ job: j, description, postedAt }) => {
    const locations: string[] = [];
    const loc = j.atsLocation ?? j.location;
    if (loc) {
      const parts = [
        "city" in loc ? loc.city : undefined,
        "state" in loc ? loc.state : undefined,
        "country" in loc ? (loc as { country?: string }).country : undefined,
      ].filter(Boolean);
      if (parts.length) locations.push(parts.join(", "));
    }
    if (j.isRemote && !locations.some((l) => /remote/i.test(l))) locations.push("Remote");
    return {
      externalId: String(j.id),
      title: j.jobOpeningName,
      locations,
      applyUrl: `https://${encodeURIComponent(boardToken)}.bamboohr.com/careers/${j.id}`,
      excerpt: description ? description.slice(0, 400) : null,
      postedAt,
      description,
    };
  });
}

type WorkdayPosting = {
  title?: string;
  externalPath?: string;
  locationsText?: string;
  bulletFields?: { timeType?: string; workerSubType?: string };
};

type WorkdayListResponse = {
  total?: number;
  jobPostings?: WorkdayPosting[];
};

type WorkdayDetailResponse = {
  jobPostingInfo?: {
    id?: string;
    jobReqId?: string;
    title?: string;
    jobDescription?: string;
    location?: string;
    additionalLocations?: string[];
    postedOn?: string;
    startDate?: string;
    timeType?: string;
  };
  hiringOrganization?: { name?: string };
};

async function fetchJsonPost<T>(url: string, body: unknown, retries = 3): Promise<T> {
  let lastErr: Error | null = null;
  for (let attempt = 0; attempt < retries; attempt++) {
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "User-Agent": "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)",
        },
        body: JSON.stringify(body),
      });
      if (res.ok) return (await res.json()) as T;
      if (res.status !== 429 && res.status < 500) {
        throw new Error(`HTTP ${res.status} for ${url}`);
      }
      lastErr = new Error(`HTTP ${res.status} for ${url}`);
    } catch (err) {
      lastErr = err instanceof Error ? err : new Error(String(err));
      if (lastErr.message.startsWith("HTTP 4") && !lastErr.message.includes("HTTP 429")) {
        throw lastErr;
      }
    }
    await sleep(500 * 2 ** attempt);
  }
  throw lastErr ?? new Error(`Failed to POST ${url}`);
}

/**
 * Fetch public Workday CXS board.
 * boardToken format: tenant|wdN|site (see workday.ts)
 */
export async function fetchWorkday(boardToken: string): Promise<NormalizedJob[]> {
  const board = parseWorkdayToken(boardToken);
  if (!board) {
    throw new Error(`Invalid workday board_token (want tenant|wdN|site): ${boardToken}`);
  }

  const listUrl = workdayJobsUrl(board);
  const limit = 20;
  const postings: WorkdayPosting[] = [];
  let total = Infinity;

  for (let offset = 0; offset < total; offset += limit) {
    const page = await fetchJsonPost<WorkdayListResponse>(listUrl, {
      appliedFacets: {},
      limit,
      offset,
      searchText: "",
    });
    const batch = page.jobPostings ?? [];
    if (typeof page.total === "number") total = page.total;
    postings.push(...batch);
    if (batch.length === 0) break;
    if (batch.length < limit) break;
    // polite pacing — Workday throttles aggressive paging
    await sleep(400);
  }

  const withDetails = await mapWithConcurrency(postings, 4, async (p) => {
    if (!p.externalPath) {
      return { posting: p, description: "", postedAt: null as Date | null, id: p.externalPath ?? p.title ?? "" };
    }
    try {
      const detail = await fetchJson<WorkdayDetailResponse>(
        workdayDetailUrl(board, p.externalPath),
      );
      const info = detail.jobPostingInfo;
      const description = info?.jobDescription
        ? stripHtmlTags(info.jobDescription)
        : "";
      const postedAt = info?.postedOn
        ? new Date(info.postedOn)
        : info?.startDate
          ? new Date(info.startDate)
          : null;
      const id = info?.id ?? info?.jobReqId ?? p.externalPath;
      return { posting: p, description, postedAt, id: String(id) };
    } catch {
      return {
        posting: p,
        description: "",
        postedAt: null as Date | null,
        id: p.externalPath,
      };
    }
  });

  return withDetails.map(({ posting: p, description, postedAt, id }) => {
    const locations: string[] = [];
    if (p.locationsText) locations.push(p.locationsText);
    return {
      externalId: id,
      title: p.title ?? "Untitled",
      locations,
      applyUrl: p.externalPath ? workdayApplyUrl(board, p.externalPath) : workdayJobsUrl(board),
      excerpt: description ? description.slice(0, 400) : null,
      postedAt,
      description,
    };
  });
}

export async function fetchJobsForAts(
  ats: AtsName,
  boardToken: string,
): Promise<NormalizedJob[]> {
  switch (ats) {
    case "greenhouse":
      return fetchGreenhouse(boardToken);
    case "lever":
      return fetchLever(boardToken);
    case "ashby":
      return fetchAshby(boardToken);
    case "workable":
      return fetchWorkable(boardToken);
    case "smartrecruiters":
      return fetchSmartRecruiters(boardToken);
    case "recruitee":
      return fetchRecruitee(boardToken);
    case "rippling":
      return fetchRippling(boardToken);
    case "bamboohr":
      return fetchBambooHr(boardToken);
    case "workday":
      return fetchWorkday(boardToken);
    case "citadel":
      return fetchCitadel(boardToken);
    case "citadel_securities":
      return fetchCitadelSecurities(boardToken);
    case "tesla":
      return fetchTesla(boardToken);
    case "bytedance":
      return fetchBytedance(boardToken);
    case "tiktok":
      return fetchTiktok(boardToken);
    default:
      throw new Error(`Unsupported ATS: ${ats}`);
  }
}
