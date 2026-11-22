import { and, desc, eq, inArray, lt, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs, parseFunnel, type Db } from "@openintern/db";
import {
  DAY_MS,
  RETIRE_AFTER_EMPTY_RUNS,
  isStale,
  shouldRetireJob,
  type BoardRun,
  type JobSighting,
} from "@openintern/proven";
import { fetchJobsForAts } from "./ats.js";
import {
  LAST_SEEN_STALE_DAYS,
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
} from "./classifier.js";
import {
  jobFingerprint,
  normalizeTitle,
  roleFamilyId,
} from "./normalize-title.js";
import { safeDate } from "./safe-date.js";
import { syncCompaniesFromYaml } from "./sync-companies.js";
import { loadTier1Curated } from "./validate-curated.js";

export type CompanyFunnel = {
  slug: string;
  fetched: number;
  internshipTitle: number;
  techPass: number;
  upserted: number;
  stale: number;
};

export type IngestSummary = {
  companies: number;
  jobsUpserted: number;
  jobsDeactivated: number;
  staleDeactivated: number;
  lastSeenDeactivated: number;
  /** True when the last-seen sweep was skipped because the pipeline had been down. */
  lastSeenSweepSkipped: boolean;
  zeroMatchCompanies: string[];
  /** Companies whose failure this run made them auto-disabled; a person should confirm. */
  autoDisabled: string[];
  /** Auto-disabled companies not fetched this run (retried weekly). */
  autoDisabledSkipped: string[];
  /** Companies that had several active jobs and now list none: possibly blocking us. */
  suspectedBlocks: string[];
  failures: { slug: string; error: string }[];
  /** Per-company fetched → title → tech → upsert funnel (sorted by fetched desc). */
  funnel: CompanyFunnel[];
  funnelTotals: {
    fetched: number;
    internshipTitle: number;
    techPass: number;
    upserted: number;
  };
};

function isTransientDbError(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err);
  const code =
    err && typeof err === "object" && "code" in err
      ? String((err as { code?: unknown }).code)
      : "";
  const cause =
    err && typeof err === "object" && "cause" in err
      ? (err as { cause?: unknown }).cause
      : undefined;
  const causeMsg = cause instanceof Error ? cause.message : String(cause ?? "");
  const causeCode =
    cause && typeof cause === "object" && "code" in cause
      ? String((cause as { code?: unknown }).code)
      : "";
  const hay = `${msg} ${code} ${causeMsg} ${causeCode}`;
  return /CONNECTION_CLOSED|CONNECTION_ENDED|ECONNRESET|ETIMEDOUT|EPIPE|connect ECONNREFUSED|57P01|08006|08003|53300/i.test(
    hay,
  );
}

async function withDbRetry<T>(label: string, fn: () => Promise<T>): Promise<T> {
  const attempts = 3;
  let last: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      return await fn();
    } catch (err) {
      last = err;
      if (!isTransientDbError(err) || i === attempts - 1) throw err;
      const waitMs = 400 * 2 ** i;
      console.warn(
        `Transient DB error during ${label} (attempt ${i + 1}/${attempts}); retrying in ${waitMs}ms`,
      );
      await new Promise((r) => setTimeout(r, waitMs));
    }
  }
  throw last;
}

/** Daily runs in a row that must fail before a company is auto-disabled. */
export const CONSECUTIVE_FAILURE_LIMIT = 7;
/** An auto-disabled company is retried once this many days after its last run. */
export const DISABLED_RECHECK_DAYS = 7;
/** A company that had at least this many active jobs and now lists none is flagged. */
export const SUSPECTED_BLOCK_MIN_JOBS = 5;
const UPSERT_CHUNK = 100;

/** Title looks like an internship/co-op (before tech filter). */
const INTERNSHIP_TITLE_ONLY =
  /\b(intern|internship|co-?op|coop|apprentice(ship)?|project\s+intern|campus\s+\w+\s+intern|year\s+at\s+\w+)\b/i;

/**
 * Empty runs in a row before this one, from the company's recent runs (newest
 * first). Failed runs are skipped: they leave the count unchanged (law
 * dp_failed_keeps_count); the first run that listed postings ends it.
 */
export function countPriorEmptyRuns(runs: { status: string; error: string | null }[]): number {
  let empty = 0;
  for (const r of runs) {
    if (r.status !== "ok") continue;
    if (parseFunnel(r.error).fetched !== 0) break;
    empty += 1;
  }
  return empty;
}

/** Where an existing active job stands after this run's fetch (input to shouldRetireJob). */
export function sightJob(
  onBoard: { description: string } | undefined,
  matched: boolean,
): JobSighting {
  if (!onBoard) return "gone";
  if (matched) return "matched";
  return onBoard.description.trim() ? "rejected" : "unverified";
}

/**
 * Auto-disable is derived from run history, so the YAML `active` flag stays
 * the human switch and registry sync can't silently undo it. A company is
 * disabled when its last CONSECUTIVE_FAILURE_LIMIT runs all failed, unless it
 * is exempt (tier-1). It is still fetched once every DISABLED_RECHECK_DAYS.
 */
export function autoDisableState(
  recentRuns: { status: string; ranAt: Date }[],
  exempt: boolean,
  now: Date,
): "enabled" | "disabled" | "recheck" {
  if (exempt) return "enabled";
  if (recentRuns.length < CONSECUTIVE_FAILURE_LIMIT) return "enabled";
  const lastN = recentRuns.slice(0, CONSECUTIVE_FAILURE_LIMIT);
  if (!lastN.every((r) => r.status === "error")) return "enabled";
  const last = lastN[0]!.ranAt.getTime();
  return now.getTime() - last >= DISABLED_RECHECK_DAYS * DAY_MS ? "recheck" : "disabled";
}

/** A gap longer than this between successful runs counts as an outage. */
export const OUTAGE_GAP_DAYS = 2;

/**
 * The last-seen sweep retires jobs not seen for LAST_SEEN_STALE_DAYS of wall
 * time. After an outage (Neon down, ingest disabled) every job looks unseen,
 * so the first run back would retire jobs that are still open. Skip the sweep
 * when the previous successful run is older than OUTAGE_GAP_DAYS; it resumes
 * on the next run, once this run has refreshed last_seen_at.
 */
export function shouldSkipLastSeenSweep(lastSuccessfulRunAt: Date | null, now: Date): boolean {
  if (!lastSuccessfulRunAt) return true;
  return now.getTime() - lastSuccessfulRunAt.getTime() > OUTAGE_GAP_DAYS * DAY_MS;
}

export async function runIngest(db: Db, opts?: { syncRegistry?: boolean }): Promise<IngestSummary> {
  // Read before this run records anything, so it reflects the previous run.
  const [lastOk] = await db
    .select({ ranAt: ingestRuns.ranAt })
    .from(ingestRuns)
    .where(eq(ingestRuns.status, "ok"))
    .orderBy(desc(ingestRuns.ranAt))
    .limit(1);
  const now = new Date();
  const skipLastSeenSweep = shouldSkipLastSeenSweep(lastOk?.ranAt ?? null, now);

  if (opts?.syncRegistry !== false) {
    await syncCompaniesFromYaml(db);
  }

  const activeCompanies = (
    await db.query.companies.findMany({
      where: eq(companies.active, true),
    })
  ).filter((c) => {
    const only = process.env.OPENINTERN_ONLY_SLUGS?.trim();
    if (!only) return true;
    const allow = new Set(
      only
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
    );
    return allow.has(c.slug);
  });

  const tier1 = new Set((await loadTier1Curated()).slugs);
  const history = await recentRunsByCompany(
    db,
    activeCompanies.map((c) => c.id),
  );
  const autoDisabledSkipped: string[] = [];
  const toFetch = activeCompanies.filter((c) => {
    if (autoDisableState(history.get(c.id) ?? [], tier1.has(c.slug), now) !== "disabled") return true;
    autoDisabledSkipped.push(c.slug);
    return false;
  });

  let jobsUpserted = 0;
  let jobsDeactivated = 0;
  let staleDeactivated = 0;
  let lastSeenDeactivated = 0;
  const zeroMatchCompanies: string[] = [];
  const autoDisabled: string[] = [];
  const suspectedBlocks: string[] = [];
  const failures: { slug: string; error: string }[] = [];
  const funnel: CompanyFunnel[] = [];

  const CHUNK = 8;
  for (let i = 0; i < toFetch.length; i += CHUNK) {
    const chunk = toFetch.slice(i, i + CHUNK);
    const results = await Promise.allSettled(
      chunk.map((company) =>
        ingestCompany(db, company, {
          now,
          priorEmptyRuns: countPriorEmptyRuns(history.get(company.id) ?? []),
        }),
      ),
    );

    for (let j = 0; j < results.length; j++) {
      const company = chunk[j]!;
      const result = results[j]!;
      if (result.status === "fulfilled") {
        const r = result.value;
        jobsUpserted += r.jobsUpserted;
        jobsDeactivated += r.jobsDeactivated;
        staleDeactivated += r.staleDeactivated;
        if (r.zeroMatch) zeroMatchCompanies.push(company.slug);
        if (r.suspectedBlock) suspectedBlocks.push(company.slug);
        funnel.push(r.funnel);
      } else {
        const message =
          result.reason instanceof Error ? result.reason.message : String(result.reason);
        failures.push({ slug: company.slug, error: message });
        funnel.push({
          slug: company.slug,
          fetched: 0,
          internshipTitle: 0,
          techPass: 0,
          upserted: 0,
          stale: 0,
        });
        const prior = history.get(company.id) ?? [];
        const exempt = tier1.has(company.slug);
        if (
          autoDisableState(prior, exempt, now) === "enabled" &&
          autoDisableState([{ status: "error", ranAt: now, error: message }, ...prior], exempt, now) === "disabled"
        ) {
          autoDisabled.push(company.slug);
          console.warn(
            `Auto-disabled ${company.slug} after ${CONSECUTIVE_FAILURE_LIMIT} failed runs in a row; retried every ${DISABLED_RECHECK_DAYS} days.`,
          );
        }
        try {
          await withDbRetry(`ingestRuns:${company.slug}`, () =>
            db.insert(ingestRuns).values({
              companyId: company.id,
              status: "error",
              jobCount: 0,
              error: message.slice(0, 2000),
            }),
          );
        } catch (recordErr) {
          // Never abort the whole ingest because Neon dropped a socket mid-run.
          console.warn(
            `Failed to record failure for ${company.slug}:`,
            recordErr instanceof Error ? recordErr.message : recordErr,
          );
        }
      }
    }
  }

  const cutoff = new Date(Date.now() - LAST_SEEN_STALE_DAYS * DAY_MS);
  if (skipLastSeenSweep) {
    console.warn(
      `Last-seen sweep skipped: previous successful run ${lastOk?.ranAt.toISOString() ?? "never"} is more than ${OUTAGE_GAP_DAYS} days old.`,
    );
  } else {
    try {
      const unseen = await withDbRetry("lastSeenSweep", () =>
        db
          .update(jobs)
          .set({ isActive: false, updatedAt: new Date() })
          .where(and(eq(jobs.isActive, true), lt(jobs.lastSeenAt, cutoff)))
          .returning({ id: jobs.id }),
      );
      lastSeenDeactivated = unseen.length;
    } catch (err) {
      console.warn(
        "Last-seen sweep failed (continuing):",
        err instanceof Error ? err.message : err,
      );
    }
  }

  funnel.sort((a, b) => b.fetched - a.fetched || a.slug.localeCompare(b.slug));
  const funnelTotals = funnel.reduce(
    (acc, f) => {
      acc.fetched += f.fetched;
      acc.internshipTitle += f.internshipTitle;
      acc.techPass += f.techPass;
      acc.upserted += f.upserted;
      return acc;
    },
    { fetched: 0, internshipTitle: 0, techPass: 0, upserted: 0 },
  );

  return {
    companies: toFetch.length,
    jobsUpserted,
    jobsDeactivated,
    staleDeactivated,
    lastSeenDeactivated,
    lastSeenSweepSkipped: skipLastSeenSweep,
    zeroMatchCompanies,
    autoDisabled,
    autoDisabledSkipped,
    suspectedBlocks,
    failures,
    funnel,
    funnelTotals,
  };
}

/**
 * Markdown for things a person must look at, or null when there are none:
 * new auto-disables (confirm or fix the board token) and suspected blocks.
 */
export function ingestDigest(s: IngestSummary): string | null {
  const lines: string[] = [];
  if (s.autoDisabled.length) {
    lines.push(
      `**Auto-disabled** after ${CONSECUTIVE_FAILURE_LIMIT} failed runs in a row (retried every ${DISABLED_RECHECK_DAYS} days). Fix the board token, set \`active: false\` in YAML, or close this as confirmed:`,
      ...s.autoDisabled.map((slug) => `- \`${slug}\``),
      "",
    );
  }
  if (s.suspectedBlocks.length) {
    lines.push(
      `**Suspected blocks**: had at least ${SUSPECTED_BLOCK_MIN_JOBS} active jobs, board now empty. Jobs are kept until ${RETIRE_AFTER_EMPTY_RUNS} empty runs in a row:`,
      ...s.suspectedBlocks.map((slug) => `- \`${slug}\``),
      "",
    );
  }
  return lines.length ? lines.join("\n") : null;
}

type RunRow = { status: string; error: string | null; ranAt: Date };

/** The newest runs per company, enough for auto-disable and the empty-run count. */
async function recentRunsByCompany(db: Db, companyIds: string[]): Promise<Map<string, RunRow[]>> {
  const out = new Map<string, RunRow[]>();
  if (companyIds.length === 0) return out;
  const keep = Math.max(CONSECUTIVE_FAILURE_LIMIT, RETIRE_AFTER_EMPTY_RUNS) * 2;
  const rows = (await db.execute(sql`
    select company_id, status, error, ran_at from (
      select company_id, status, error, ran_at,
        row_number() over (partition by company_id order by ran_at desc) as rn
      from ${ingestRuns}
      where ${inArray(ingestRuns.companyId, companyIds)}
    ) recent
    where rn <= ${keep}
    order by company_id, ran_at desc
  `)) as unknown as { company_id: string; status: string; error: string | null; ran_at: Date | string }[];
  for (const r of rows) {
    const list = out.get(r.company_id) ?? [];
    list.push({ status: r.status, error: r.error, ranAt: new Date(r.ran_at) });
    out.set(r.company_id, list);
  }
  return out;
}

async function ingestCompany(
  db: Db,
  company: typeof companies.$inferSelect,
  opts: { now: Date; priorEmptyRuns: number },
): Promise<{
  jobsUpserted: number;
  jobsDeactivated: number;
  staleDeactivated: number;
  zeroMatch: boolean;
  suspectedBlock: boolean;
  funnel: CompanyFunnel;
}> {
  const raw = await fetchJobsForAts(company.ats, company.boardToken);
  const internshipTitleJobs = raw.filter((j) => INTERNSHIP_TITLE_ONLY.test(j.title));
  const filtered = raw.filter((j) => isTechInternship(j.title, j.description));
  const descriptionMissing = raw.filter((j) => !j.description.trim()).length;
  const matchedIds = new Set<string>();
  let jobsUpserted = 0;
  let staleDeactivated = 0;
  const { now } = opts;

  const existingRows = await db
    .select({
      id: jobs.id,
      externalJobId: jobs.externalJobId,
      isActive: jobs.isActive,
      fingerprint: jobs.fingerprint,
      firstSeenAt: jobs.firstSeenAt,
      postedAt: jobs.postedAt,
    })
    .from(jobs)
    .where(eq(jobs.companyId, company.id));
  const existingByFingerprint = new Map(
    existingRows.map((row) => [row.fingerprint, row]),
  );

  type JobUpsertRow = typeof jobs.$inferInsert;
  const rows: JobUpsertRow[] = [];

  for (const j of filtered) {
    matchedIds.add(j.externalId);
    const excerpt =
      excerptFromHtml(j.excerpt) ?? excerptFromHtml(j.description);
    const isRemote = looksRemote(j.locations, j.title);
    const classifierText = `${j.title} ${j.description}`;
    const terms = extractTerms(classifierText);
    const durationMonths = extractDurationMonthsList(classifierText) ?? [];
    const cohortYear = extractCohortYear(classifierText, now);
    const termYears = extractTermYears(classifierText, cohortYear, now);
    const roles = extractRoles(j.title, j.description);
    const regions = extractRegions(j.locations, isRemote);
    const normalized = normalizeTitle(j.title);
    const fingerprint = jobFingerprint(company.id, company.ats, j.externalId);
    const familyId = roleFamilyId(company.slug, normalized);

    const existing = existingByFingerprint.get(fingerprint);
    const firstSeenAt = existing?.firstSeenAt ?? now;
    const postedAt = safeDate(j.postedAt) ?? existing?.postedAt ?? null;

    // Bend-proven to be exactly !isFreshJob (law fr_stale_iff_not_fresh).
    const stale = isStale({
      termYears,
      anchor: postedAt ?? firstSeenAt,
      now,
      maxAgeDays: MAX_AGE_DAYS_NO_TERM,
    });

    rows.push({
      companyId: company.id,
      externalJobId: j.externalId,
      fingerprint,
      title: j.title,
      normalizedTitle: normalized,
      roleFamilyId: familyId,
      locations: j.locations,
      applyUrl: j.applyUrl,
      excerpt,
      terms,
      termYears,
      durationMonths,
      cohortYear,
      roles,
      regions,
      isRemote,
      isActive: !stale,
      source: company.ats,
      postedAt,
      firstSeenAt,
      lastSeenAt: now,
      updatedAt: now,
    });
    if (stale) staleDeactivated += 1;
  }

  const updateFields = {
    externalJobId: sql`excluded.external_job_id`,
    title: sql`excluded.title`,
    normalizedTitle: sql`excluded.normalized_title`,
    roleFamilyId: sql`excluded.role_family_id`,
    locations: sql`excluded.locations`,
    applyUrl: sql`excluded.apply_url`,
    excerpt: sql`excluded.excerpt`,
    terms: sql`excluded.terms`,
    termYears: sql`excluded.term_years`,
    durationMonths: sql`excluded.duration_months`,
    cohortYear: sql`excluded.cohort_year`,
    roles: sql`excluded.roles`,
    regions: sql`excluded.regions`,
    isRemote: sql`excluded.is_remote`,
    isActive: sql`excluded.is_active`,
    source: sql`excluded.source`,
    postedAt: sql`excluded.posted_at`,
    lastSeenAt: sql`excluded.last_seen_at`,
    updatedAt: sql`excluded.updated_at`,
  };

  for (let i = 0; i < rows.length; i += UPSERT_CHUNK) {
    const chunk = rows.slice(i, i + UPSERT_CHUNK);
    await db
      .insert(jobs)
      .values(chunk)
      .onConflictDoUpdate({
        target: jobs.fingerprint,
        set: updateFields,
      });
    jobsUpserted += chunk.length;
  }

  // Per-job retirement, Bend-proven (laws dp_*): a job gone from a board that
  // was fetched successfully is retired; one whose details failed to load is
  // kept; an empty board retires everything only on the third empty run.
  const run: BoardRun = raw.length === 0 ? "empty" : "listed";
  const onBoard = new Map(raw.map((j) => [j.externalId, j]));
  const activeBefore = existingRows.filter((row) => row.isActive);
  const retireIds = activeBefore
    .filter((row) =>
      shouldRetireJob(
        run,
        opts.priorEmptyRuns,
        sightJob(onBoard.get(row.externalJobId), matchedIds.has(row.externalJobId)),
      ),
    )
    .map((row) => row.id);
  let jobsDeactivated = 0;
  for (let i = 0; i < retireIds.length; i += UPSERT_CHUNK) {
    const ids = retireIds.slice(i, i + UPSERT_CHUNK);
    const retired = await db
      .update(jobs)
      .set({ isActive: false, updatedAt: now })
      .where(and(eq(jobs.companyId, company.id), eq(jobs.isActive, true), inArray(jobs.id, ids)))
      .returning({ id: jobs.id });
    jobsDeactivated += retired.length;
  }
  const suspectedBlock = run === "empty" && activeBefore.length >= SUSPECTED_BLOCK_MIN_JOBS;

  const zeroMatch = raw.length > 0 && filtered.length === 0;
  const funnelMsg = `funnel fetched=${raw.length} title=${internshipTitleJobs.length} tech=${filtered.length} upserted=${jobsUpserted} description_missing=${descriptionMissing}`;
  await db.insert(ingestRuns).values({
    companyId: company.id,
    status: "ok",
    jobCount: filtered.length,
    error: zeroMatch
      ? `zero_match: no tech internships classified; ${funnelMsg}`
      : funnelMsg,
  });

  return {
    jobsUpserted,
    jobsDeactivated,
    staleDeactivated,
    zeroMatch,
    suspectedBlock,
    funnel: {
      slug: company.slug,
      fetched: raw.length,
      internshipTitle: internshipTitleJobs.length,
      techPass: filtered.length,
      upserted: jobsUpserted,
      stale: staleDeactivated,
    },
  };
}
