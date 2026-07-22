/**
 * Tesla proprietary careers (cua-api) adapter.
 *
 * Primary path: GET https://www.tesla.com/cua-api/apps/careers/state
 * Akamai often returns 403 without a browser session. Fallbacks:
 *   - OPENINTERN_TESLA_STATE_PATH: load a JSON dump of that endpoint
 *   - OPENINTERN_BROWSER=1: optional Playwright session (not used in Hobby CI)
 *
 * board_token: unused site key; conventionally `careers`.
 */

import fs from "node:fs/promises";
import type { NormalizedJob } from "./ats.js";
import { BROWSER_UA, browserEnabled, loadPlaywrightChromium } from "./browser.js";

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

async function fetchStateHttp(): Promise<unknown> {
  const res = await fetch(TESLA_STATE_URL, {
    headers: {
      Accept: "application/json, text/plain, */*",
      "User-Agent": BROWSER_UA,
      Referer: "https://www.tesla.com/careers/search/",
      Origin: "https://www.tesla.com",
    },
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} for ${TESLA_STATE_URL}`);
  }
  return res.json();
}

export async function loadTeslaStateDump(): Promise<unknown | null> {
  const path = process.env.OPENINTERN_TESLA_STATE_PATH;
  if (!path) return null;
  const raw = await fs.readFile(path, "utf8");
  return JSON.parse(raw) as unknown;
}

async function fetchStateWithBrowser(): Promise<unknown> {
  const chromium = await loadPlaywrightChromium();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ userAgent: BROWSER_UA });
    await page.goto("https://www.tesla.com/careers/search/", {
      waitUntil: "domcontentloaded",
      timeout: 60_000,
    });
    await page.waitForTimeout(2000);
    const state = await page.evaluate(async (url: string) => {
      const res = await fetch(url, { credentials: "include" });
      if (!res.ok) throw new Error(`browser fetch HTTP ${res.status}`);
      return res.json();
    }, TESLA_STATE_URL);
    return state;
  } finally {
    await browser.close();
  }
}

export async function fetchTesla(_boardToken: string): Promise<NormalizedJob[]> {
  let state: unknown | null = await loadTeslaStateDump();

  if (!state) {
    try {
      state = await fetchStateHttp();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (browserEnabled()) {
        try {
          state = await fetchStateWithBrowser();
        } catch (browserErr) {
          const bmsg = browserErr instanceof Error ? browserErr.message : String(browserErr);
          throw new Error(
            `${msg}. Browser fallback also failed (${bmsg}). ` +
              `Set OPENINTERN_TESLA_STATE_PATH to a JSON dump of ${TESLA_STATE_URL}. See CONTRIBUTING.`,
          );
        }
      } else {
        throw new Error(
          `${msg}. Tesla cua-api is often blocked without a browser session. ` +
            `Set OPENINTERN_TESLA_STATE_PATH to a JSON dump, or OPENINTERN_BROWSER=1 ` +
            `with playwright installed. See CONTRIBUTING.`,
        );
      }
    }
  }

  const jobs = parseTeslaCareersState(state);
  if (jobs.length === 0) {
    throw new Error("Tesla careers state parsed zero listings");
  }
  return jobs;
}

/** Probe helper: dump → live HTTP → browser. */
export async function probeTeslaState(): Promise<{
  ok: boolean;
  status: number;
  count?: number;
  error?: string;
}> {
  try {
    const dump = await loadTeslaStateDump();
    if (dump) {
      const count = parseTeslaCareersState(dump).length;
      return {
        ok: count > 0,
        status: 200,
        count,
        error: count > 0 ? undefined : "empty dump listing",
      };
    }

    try {
      const state = await fetchStateHttp();
      const count = parseTeslaCareersState(state).length;
      return {
        ok: count > 0,
        status: 200,
        count,
        error: count > 0 ? undefined : "empty listing",
      };
    } catch {
      /* fall through */
    }

    if (browserEnabled()) {
      const state = await fetchStateWithBrowser();
      const count = parseTeslaCareersState(state).length;
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
      error: `HTTP blocked for ${TESLA_STATE_URL}; set OPENINTERN_TESLA_STATE_PATH or OPENINTERN_BROWSER=1`,
    };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
