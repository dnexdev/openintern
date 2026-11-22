/**
 * Tesla careers: manual capture only.
 *
 * Tesla's careers API refuses automated clients, and OpenIntern does not
 * pretend to be a browser. The maintainer saves the state JSON from
 * https://www.tesla.com/cua-api/apps/careers/state in a normal browser and runs
 * `pnpm manual:capture tesla <file>`, which commits data/manual/tesla.json.
 *
 * board_token: unused; conventionally `careers`.
 */

import type { NormalizedJob } from "./ats.js";
import { loadManualJobs, manualProbe } from "./manual.js";

export const TESLA_STATE_URL = "https://www.tesla.com/cua-api/apps/careers/state";

type LooseListing = Record<string, unknown>;

function asString(v: unknown): string | null {
  if (typeof v === "string" && v.trim()) return v.trim();
  if (typeof v === "number") return String(v);
  return null;
}

function listingId(row: LooseListing): string | null {
  return (
    asString(row.id) ??
    asString(row.jobId) ??
    asString(row.job_id) ??
    asString(row.Id)
  );
}

function listingTitle(row: LooseListing): string | null {
  return (
    asString(row.t) ??
    asString(row.title) ??
    asString(row.name) ??
    asString(row.jobTitle)
  );
}

function listingLocation(row: LooseListing, sites: Record<string, string>): string[] {
  const direct =
    asString(row.l) ??
    asString(row.location) ??
    asString(row.locations) ??
    asString(row.city);
  if (direct) return [direct];

  const siteId = asString(row.sid) ?? asString(row.siteId) ?? asString(row.locationId);
  if (siteId && sites[siteId]) return [sites[siteId]!];

  const locIds = row.locationIds ?? row.siteIds;
  if (Array.isArray(locIds)) {
    const names = locIds
      .map((id) => sites[String(id)])
      .filter((x): x is string => Boolean(x));
    if (names.length) return names;
  }
  return [];
}

function buildSitesMap(state: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {};
  const candidates = [state.sites, state.lookup && (state.lookup as LooseListing).sites];
  for (const c of candidates) {
    if (!c || typeof c !== "object") continue;
    for (const [k, v] of Object.entries(c as Record<string, unknown>)) {
      if (typeof v === "string") out[k] = v;
      else if (v && typeof v === "object") {
        const name = asString((v as LooseListing).name) ?? asString((v as LooseListing).l);
        if (name) out[k] = name;
      }
    }
  }
  return out;
}

function extractListings(state: unknown): LooseListing[] {
  if (!state || typeof state !== "object") return [];
  const root = state as Record<string, unknown>;
  for (const key of ["listings", "jobs", "results", "data"]) {
    const v = root[key];
    if (Array.isArray(v)) return v.filter((x) => x && typeof x === "object") as LooseListing[];
  }
  if (Array.isArray(root)) return root as LooseListing[];
  return [];
}

export function parseTeslaCareersState(state: unknown): NormalizedJob[] {
  const root = state && typeof state === "object" ? (state as Record<string, unknown>) : {};
  const sites = buildSitesMap(root);
  const listings = extractListings(state);
  const jobs: NormalizedJob[] = [];

  for (const row of listings) {
    const id = listingId(row);
    const title = listingTitle(row);
    if (!id || !title) continue;
    const description =
      asString(row.description) ??
      asString(row.d) ??
      asString(row.jobDescription) ??
      "";
    const applyFromRow = asString(row.url) ?? asString(row.applyUrl) ?? asString(row.absolute_url);
    jobs.push({
      externalId: id,
      title,
      locations: listingLocation(row, sites),
      applyUrl:
        applyFromRow ??
        `https://www.tesla.com/careers/search/job/${encodeURIComponent(id)}`,
      excerpt: description ? description.slice(0, 400) : null,
      postedAt: null,
      description,
    });
  }
  return jobs;
}

export async function fetchTesla(_boardToken: string): Promise<NormalizedJob[]> {
  return loadManualJobs("tesla");
}

export function probeTeslaState() {
  return manualProbe("tesla");
}
