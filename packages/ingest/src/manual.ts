/**
 * Manual captures for career sites we do not fetch automatically (Tesla,
 * Citadel, Citadel Securities). The maintainer saves the public listing in a
 * normal browser, `pnpm manual:capture` reduces it to the fields below, and
 * the result is committed as data/manual/<ats>.json.
 *
 * Only normalized fields are committed: no raw HTML and no full descriptions
 * (excerpts are capped at 400 characters, like every other source). A capture
 * older than MANUAL_MAX_AGE_DAYS is not published.
 */
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { NormalizedJob } from "./ats.js";
import { FetchFailure } from "./http.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const MANUAL_MAX_AGE_DAYS = 14;
export const MANUAL_EXCERPT_MAX = 400;
const DAY_MS = 24 * 60 * 60 * 1000;

export type ManualJob = {
  external_id: string;
  title: string;
  locations: string[];
  apply_url: string;
  excerpt: string | null;
  posted_at: string | null;
};

export type ManualCapture = {
  /** The public page that was captured. */
  source: string;
  captured_at: string;
  /** A role, never a person's name (e.g. "maintainer"). */
  captured_by: string;
  jobs: ManualJob[];
};

export function defaultManualDir(): string {
  return path.resolve(__dirname, "../../../data/manual");
}

export function manualPath(ats: string, dir = defaultManualDir()): string {
  if (!/^[a-z0-9_]+$/.test(ats)) throw new Error(`invalid manual source name: ${ats}`);
  return path.join(dir, `${ats}.json`);
}

function isHttpsUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  try {
    return new URL(value).protocol === "https:";
  } catch {
    return false;
  }
}

/** Throws with every problem found; returns the capture typed. */
export function validateManualCapture(data: unknown, now = new Date()): ManualCapture {
  const errors: string[] = [];
  const obj = (data ?? {}) as Record<string, unknown>;
  if (!isHttpsUrl(obj.source)) errors.push("source must be an https URL");
  const capturedAt = typeof obj.captured_at === "string" ? Date.parse(obj.captured_at) : Number.NaN;
  if (Number.isNaN(capturedAt)) errors.push("captured_at must be an ISO date");
  else if (capturedAt > now.getTime() + 60 * 60 * 1000) errors.push("captured_at is in the future");
  if (typeof obj.captured_by !== "string" || !obj.captured_by.trim()) errors.push("captured_by is required");
  if (!Array.isArray(obj.jobs)) errors.push("jobs must be an array");
  const ids = new Set<string>();
  for (const [i, raw] of (Array.isArray(obj.jobs) ? obj.jobs : []).entries()) {
    const j = (raw ?? {}) as Record<string, unknown>;
    const at = `jobs[${i}]`;
    if (typeof j.external_id !== "string" || !j.external_id.trim()) errors.push(`${at}.external_id is required`);
    else if (ids.has(j.external_id)) errors.push(`${at}.external_id ${j.external_id} is duplicated`);
    else ids.add(j.external_id);
    if (typeof j.title !== "string" || !j.title.trim()) errors.push(`${at}.title is required`);
    if (!Array.isArray(j.locations) || !j.locations.every((l) => typeof l === "string")) {
      errors.push(`${at}.locations must be a list of strings`);
    }
    if (!isHttpsUrl(j.apply_url)) errors.push(`${at}.apply_url must be an https URL`);
    if (j.excerpt != null && (typeof j.excerpt !== "string" || j.excerpt.length > MANUAL_EXCERPT_MAX)) {
      errors.push(`${at}.excerpt must be a string of at most ${MANUAL_EXCERPT_MAX} characters`);
    }
    if (j.posted_at != null && (typeof j.posted_at !== "string" || Number.isNaN(Date.parse(j.posted_at)))) {
      errors.push(`${at}.posted_at must be an ISO date or null`);
    }
  }
  if (errors.length) throw new Error(`invalid manual capture: ${errors.join("; ")}`);
  return obj as unknown as ManualCapture;
}

export function manualCaptureAgeDays(capture: ManualCapture, now = new Date()): number {
  return (now.getTime() - Date.parse(capture.captured_at)) / DAY_MS;
}

/** The capture's jobs, or a FetchFailure when it is missing or too old to publish. */
export async function loadManualJobs(
  ats: string,
  opts: { dir?: string; now?: Date } = {},
): Promise<NormalizedJob[]> {
  const now = opts.now ?? new Date();
  const file = manualPath(ats, opts.dir);
  let raw: string;
  try {
    raw = await fs.readFile(file, "utf8");
  } catch {
    throw new FetchFailure("not-found", `no manual capture at data/manual/${ats}.json`);
  }
  const capture = validateManualCapture(JSON.parse(raw), now);
  const age = manualCaptureAgeDays(capture, now);
  if (age > MANUAL_MAX_AGE_DAYS) {
    throw new FetchFailure(
      "manual-expired",
      `data/manual/${ats}.json was captured ${Math.floor(age)} days ago (limit ${MANUAL_MAX_AGE_DAYS})`,
    );
  }
  return capture.jobs.map((j) => ({
    externalId: j.external_id,
    title: j.title,
    locations: j.locations,
    applyUrl: j.apply_url,
    excerpt: j.excerpt,
    postedAt: j.posted_at ? new Date(j.posted_at) : null,
    description: j.excerpt ?? "",
  }));
}

/** Reduce parsed listings to a committable capture. */
export function toManualCapture(
  jobs: NormalizedJob[],
  opts: { source: string; now?: Date; capturedBy?: string },
): ManualCapture {
  const capture: ManualCapture = {
    source: opts.source,
    captured_at: (opts.now ?? new Date()).toISOString(),
    captured_by: opts.capturedBy ?? "maintainer",
    jobs: jobs
      .map((j) => ({
        external_id: j.externalId,
        title: j.title,
        locations: j.locations,
        apply_url: j.applyUrl,
        excerpt: (j.excerpt ?? j.description).replace(/\s+/g, " ").trim().slice(0, MANUAL_EXCERPT_MAX) || null,
        posted_at: j.postedAt ? j.postedAt.toISOString() : null,
      }))
      .sort((a, b) => a.external_id.localeCompare(b.external_id)),
  };
  return validateManualCapture(capture, opts.now);
}

/** For probe tools: is there a publishable capture, and how many jobs? */
export async function manualProbe(
  ats: string,
): Promise<{ ok: boolean; status: number; count?: number; error?: string }> {
  try {
    const jobs = await loadManualJobs(ats);
    return { ok: jobs.length > 0, status: 200, count: jobs.length, error: jobs.length ? undefined : "empty capture" };
  } catch (err) {
    return { ok: false, status: 0, error: err instanceof Error ? err.message : String(err) };
  }
}
