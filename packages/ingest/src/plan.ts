/**
 * What one company's ingest run changes, decided without touching storage.
 * Both state backends (Postgres, flat files) apply the same plan, so they
 * can't drift apart.
 */
import { checkApplyUrl } from "@openintern/db";
import { isStale, shouldRetireJob, type BoardRun, type JobSighting } from "@openintern/proven";
import type { NormalizedJob } from "./ats.js";
import {
  MAX_AGE_DAYS_NO_TERM,
  excerptFromHtml,
  extractCohortYear,
  extractDurationMonthsList,
  extractRegions,
  extractRoles,
  extractTermYears,
  extractTerms,
  isTechInternship,
  looksRemote,
  type TermYear,
} from "./classifier.js";
import type { FetchFailure } from "./http.js";
import { jobFingerprint, normalizeTitle, roleFamilyId } from "./normalize-title.js";
import { safeDate } from "./safe-date.js";

/** Title looks like an internship/co-op (before tech filter). */
export const INTERNSHIP_TITLE_ONLY =
  /\b(intern|internship|co-?op|coop|apprentice(ship)?|project\s+intern|campus\s+\w+\s+intern|year\s+at\s+\w+)\b/i;

/** A company that had at least this many active jobs and now lists none is flagged. */
export const SUSPECTED_BLOCK_MIN_JOBS = 5;

export type ExistingJob = {
  id: string;
  externalJobId: string;
  isActive: boolean;
  fingerprint: string;
  firstSeenAt: Date;
  postedAt: Date | null;
};

export type JobUpsert = {
  companyId: string;
  externalJobId: string;
  fingerprint: string;
  title: string;
  normalizedTitle: string;
  roleFamilyId: string;
  locations: string[];
  applyUrl: string;
  excerpt: string | null;
  terms: string[];
  termYears: TermYear[];
  durationMonths: number[];
  cohortYear: number | null;
  roles: string[];
  regions: string[];
  isRemote: boolean;
  isActive: boolean;
  source: string;
  postedAt: Date | null;
  firstSeenAt: Date;
  lastSeenAt: Date;
  updatedAt: Date;
};

export type ApplyHostViolation = { company: string; host: string | null; reason: string; example: string };

export type CompanyFunnel = {
  slug: string;
  fetched: number;
  internshipTitle: number;
  techPass: number;
  upserted: number;
  stale: number;
};

export type CompanyRunPlan = {
  upserts: JobUpsert[];
  retireIds: string[];
  run: { status: "ok" | "error"; jobCount: number; error: string };
  zeroMatch: boolean;
  suspectedBlock: boolean;
  staleDeactivated: number;
  applyHostViolations: ApplyHostViolation[];
  funnel: CompanyFunnel;
};

/** Where an existing active job stands after this run's fetch (input to shouldRetireJob). */
export function sightJob(onBoard: { description: string } | undefined, matched: boolean): JobSighting {
  if (!onBoard) return "gone";
  if (matched) return "matched";
  return onBoard.description.trim() ? "rejected" : "unverified";
}

export function planCompanyRun(input: {
  company: { id: string; slug: string; ats: string };
  /** Postings from every board, or [] when `expired`. */
  raw: NormalizedJob[];
  /** Set when the source is a manual capture past its age limit. */
  expired: FetchFailure | null;
  existing: ExistingJob[];
  priorEmptyRuns: number;
  now: Date;
  enforceApplyHosts: boolean;
}): CompanyRunPlan {
  const { company, raw, expired, existing, now } = input;
  const internshipTitle = raw.filter((j) => INTERNSHIP_TITLE_ONLY.test(j.title)).length;
  const filtered = raw.filter((j) => isTechInternship(j.title, j.description));
  const descriptionMissing = raw.filter((j) => !j.description.trim()).length;
  const byFingerprint = new Map(existing.map((row) => [row.fingerprint, row]));
  const matchedIds = new Set<string>();
  const upserts: JobUpsert[] = [];
  const violations = new Map<string, ApplyHostViolation>();
  let staleDeactivated = 0;

  for (const j of filtered) {
    const check = checkApplyUrl(company.slug, j.applyUrl);
    if (!check.ok) {
      const key = `${check.host} ${check.reason}`;
      if (!violations.has(key)) {
        violations.set(key, { company: company.slug, host: check.host, reason: check.reason, example: j.applyUrl });
      }
      // Held: not ingested, and an existing copy is retired as no longer matching.
      if (input.enforceApplyHosts) continue;
    }
    matchedIds.add(j.externalId);
    const classifierText = `${j.title} ${j.description}`;
    const isRemote = looksRemote(j.locations, j.title);
    const cohortYear = extractCohortYear(classifierText, now);
    const termYears = extractTermYears(classifierText, cohortYear, now);
    const normalized = normalizeTitle(j.title);
    const fingerprint = jobFingerprint(company.id, company.ats, j.externalId);
    const prior = byFingerprint.get(fingerprint);
    const firstSeenAt = prior?.firstSeenAt ?? now;
    const postedAt = safeDate(j.postedAt) ?? prior?.postedAt ?? null;
    // Bend-proven to be exactly !isFreshJob (law fr_stale_iff_not_fresh).
    const stale = isStale({ termYears, anchor: postedAt ?? firstSeenAt, now, maxAgeDays: MAX_AGE_DAYS_NO_TERM });
    if (stale) staleDeactivated += 1;
    upserts.push({
      companyId: company.id,
      externalJobId: j.externalId,
      fingerprint,
      title: j.title,
      normalizedTitle: normalized,
      roleFamilyId: roleFamilyId(company.slug, normalized),
      locations: j.locations,
      applyUrl: j.applyUrl,
      excerpt: excerptFromHtml(j.excerpt) ?? excerptFromHtml(j.description),
      terms: extractTerms(classifierText),
      termYears,
      durationMonths: extractDurationMonthsList(classifierText) ?? [],
      cohortYear,
      roles: extractRoles(j.title, j.description),
      regions: extractRegions(j.locations, isRemote),
      isRemote,
      isActive: !stale,
      source: company.ats,
      postedAt,
      firstSeenAt,
      lastSeenAt: now,
      updatedAt: now,
    });
  }

  // Per-job retirement, Bend-proven (laws dp_*): a job gone from a board that
  // was fetched successfully is retired; one whose details failed to load is
  // kept; an empty board retires everything only on the third empty run.
  const run: BoardRun = expired ? "expired" : raw.length === 0 ? "empty" : "listed";
  const onBoard = new Map(raw.map((j) => [j.externalId, j]));
  const activeBefore = existing.filter((row) => row.isActive);
  const retireIds = activeBefore
    .filter((row) =>
      shouldRetireJob(run, input.priorEmptyRuns, sightJob(onBoard.get(row.externalJobId), matchedIds.has(row.externalJobId))),
    )
    .map((row) => row.id);

  if (expired) {
    return {
      upserts: [],
      retireIds,
      run: { status: "error", jobCount: 0, error: expired.message.slice(0, 2000) },
      zeroMatch: false,
      suspectedBlock: false,
      staleDeactivated: 0,
      applyHostViolations: [],
      funnel: { slug: company.slug, fetched: 0, internshipTitle: 0, techPass: 0, upserted: 0, stale: 0 },
    };
  }

  const zeroMatch = raw.length > 0 && filtered.length === 0;
  const funnelMsg = `funnel fetched=${raw.length} title=${internshipTitle} tech=${filtered.length} upserted=${upserts.length} description_missing=${descriptionMissing}`;
  return {
    upserts,
    retireIds,
    run: {
      status: "ok",
      jobCount: filtered.length,
      error: zeroMatch ? `zero_match: no tech internships classified; ${funnelMsg}` : funnelMsg,
    },
    zeroMatch,
    suspectedBlock: run === "empty" && activeBefore.length >= SUSPECTED_BLOCK_MIN_JOBS,
    staleDeactivated,
    applyHostViolations: [...violations.values()],
    funnel: {
      slug: company.slug,
      fetched: raw.length,
      internshipTitle,
      techPass: filtered.length,
      upserted: upserts.length,
      stale: staleDeactivated,
    },
  };
}
