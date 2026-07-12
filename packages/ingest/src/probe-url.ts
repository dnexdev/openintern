import type { AtsName } from "./ats.js";

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
    default:
      throw new Error(`Unknown ATS: ${ats}`);
  }
}

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

export function countJobsFromProbeBody(ats: string, data: unknown): number {
  if (Array.isArray(data)) return data.length;
  if (!data || typeof data !== "object") return 0;
  const o = data as Record<string, unknown>;
  if (Array.isArray(o.jobs)) return o.jobs.length;
  if (Array.isArray(o.content)) return o.content.length;
  if (Array.isArray(o.offers)) return o.offers.length;
  if (Array.isArray(o.result)) return o.result.length;
  if (ats === "workable" && Array.isArray(o.jobs)) return o.jobs.length;
  return o.totalFound ? Number(o.totalFound) : 1;
}
