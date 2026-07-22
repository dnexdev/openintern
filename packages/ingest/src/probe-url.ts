import type { AtsName } from "./ats.js";
import { PROPRIETARY_ATS } from "./ats.js";
import { citadelListingUrl, probeCitadelListing } from "./citadel.js";
import { BYTEDANCE_PORTAL, TIKTOK_PORTAL } from "./bytedance.js";
import { TESLA_STATE_URL, parseTeslaCareersState, probeTeslaState } from "./tesla.js";
import { parseWorkdayToken, workdayJobsUrl } from "./workday.js";

export function probeUrl(ats: AtsName | string, token: string): string {
  switch (ats) {
    case "greenhouse":
      return `https://boards-api.greenhouse.io/v1/boards/${encodeURIComponent(token)}/jobs`;
    case "lever":
      return `https://api.lever.co/v0/postings/${encodeURIComponent(token)}?mode=json`;
    case "ashby":
      return `https://api.ashbyhq.com/posting-api/job-board/${encodeURIComponent(token)}`;
    case "workable":
      return `https://www.workable.com/api/accounts/${encodeURIComponent(token)}`;
    case "smartrecruiters":
      return `https://api.smartrecruiters.com/v1/companies/${encodeURIComponent(token)}/postings?limit=1`;
    case "recruitee":
      return `https://${encodeURIComponent(token)}.recruitee.com/api/offers/`;
    case "rippling":
      return `https://api.rippling.com/platform/api/ats/v1/board/${encodeURIComponent(token)}/jobs`;
    case "bamboohr":
      return `https://${encodeURIComponent(token)}.bamboohr.com/careers/list`;
    case "workday": {
      const board = parseWorkdayToken(token);
      if (!board) throw new Error(`Invalid workday board_token (want tenant|wdN|site): ${token}`);
      return workdayJobsUrl(board);
    }
    case "citadel":
      return citadelListingUrl("citadel", token, 1);
    case "citadel_securities":
      return citadelListingUrl("citadel_securities", token, 1);
    case "tesla":
      return TESLA_STATE_URL;
    case "bytedance":
      return `${BYTEDANCE_PORTAL.apiBase}/search/job/posts`;
    case "tiktok":
      return `${TIKTOK_PORTAL.apiBase}/search/job/posts`;
    default:
      throw new Error(`Unknown ATS: ${ats}`);
  }
}

/** ATS values used for recover-tokens guessing (Workday/proprietary tokens are not guessable). */
export const ALL_ATS: AtsName[] = [
  "greenhouse",
  "lever",
  "ashby",
  "workable",
  "smartrecruiters",
  "recruitee",
  "rippling",
  "bamboohr",
];

export const ALL_ATS_INCLUDING_WORKDAY: AtsName[] = [...ALL_ATS, "workday"];

export const ALL_ATS_INCLUDING_PROPRIETARY: AtsName[] = [
  ...ALL_ATS_INCLUDING_WORKDAY,
  ...PROPRIETARY_ATS,
];

export function countJobsFromProbeBody(ats: string, data: unknown): number {
  if (Array.isArray(data)) return data.length;
  if (!data || typeof data !== "object") return 0;
  const o = data as Record<string, unknown>;
  if (ats === "workday") {
    if (typeof o.total === "number") return o.total;
    if (Array.isArray(o.jobPostings)) return o.jobPostings.length;
  }
  if (ats === "tesla") {
    return parseTeslaCareersState(data).length;
  }
  if (ats === "bytedance" || ats === "tiktok") {
    const inner = o.data as Record<string, unknown> | undefined;
    if (inner && typeof inner.count === "number") return inner.count;
    if (inner && Array.isArray(inner.job_post_list)) return inner.job_post_list.length;
  }
  if (Array.isArray(o.jobs)) return o.jobs.length;
  if (Array.isArray(o.content)) return o.content.length;
  if (Array.isArray(o.offers)) return o.offers.length;
  if (Array.isArray(o.result)) return o.result.length;
  if (ats === "workable" && Array.isArray(o.jobs)) return o.jobs.length;
  return o.totalFound ? Number(o.totalFound) : 1;
}

const UA = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)";

async function probeBytedancePortal(
  ats: "bytedance" | "tiktok",
  token: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  const portal = ats === "bytedance" ? BYTEDANCE_PORTAL : TIKTOK_PORTAL;
  const websitePath = (token || portal.defaultWebsitePath).trim();
  const url = `${portal.apiBase}/search/job/posts`;
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        "User-Agent": UA,
        origin: portal.origin,
        Referer: `${portal.origin}/search`,
        "website-path": websitePath,
        "accept-language": "en",
      },
      body: JSON.stringify({
        job_category_id_list: [],
        keyword: "",
        limit: 1,
        offset: 0,
        location_code_list: [],
        recruitment_id_list: ["2"],
        subject_id_list: [],
        portal_type: 2,
        portal_entrance: 1,
      }),
    });
    if (!res.ok) return { ok: false, status: res.status, error: `HTTP ${res.status} for ${url}` };
    const data = (await res.json()) as unknown;
    const count = countJobsFromProbeBody(ats, data);
    return { ok: count > 0, status: res.status, count, error: count > 0 ? undefined : "empty listing" };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}

/** Probe a board; Workday CXS and proprietary portals need custom requests. */
export async function probeAtsBoard(
  ats: string,
  token: string,
  ua = UA,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    if (ats === "citadel" || ats === "citadel_securities") {
      return probeCitadelListing(ats, token);
    }
    if (ats === "tesla") {
      return probeTeslaState();
    }
    if (ats === "bytedance" || ats === "tiktok") {
      return probeBytedancePortal(ats, token);
    }

    const url = probeUrl(ats, token);
    if (ats === "workday") {
      const res = await fetch(url, {
        method: "POST",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "User-Agent": ua,
        },
        body: JSON.stringify({
          appliedFacets: {},
          limit: 1,
          offset: 0,
          searchText: "",
        }),
      });
      if (res.status === 404) return { ok: false, status: 404, error: `HTTP 404 for ${url}` };
      if (!res.ok && res.status !== 429) {
        return { ok: false, status: res.status, error: `HTTP ${res.status} for ${url}` };
      }
      if (res.status === 429) return { ok: true, status: 429 };
      const data = (await res.json()) as unknown;
      return { ok: true, status: res.status, count: countJobsFromProbeBody(ats, data) };
    }

    const res = await fetch(url, {
      method: "GET",
      headers: { Accept: "application/json", "User-Agent": ua },
    });
    if (res.status === 404) return { ok: false, status: 404, error: `HTTP 404 for ${url}` };
    if (!res.ok && res.status !== 429) {
      return { ok: false, status: res.status, error: `HTTP ${res.status} for ${url}` };
    }
    if (res.status === 429) return { ok: true, status: 429 };
    const data = (await res.json()) as unknown;
    return { ok: true, status: res.status, count: countJobsFromProbeBody(ats, data) };
  } catch (err) {
    return {
      ok: false,
      status: 0,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}
