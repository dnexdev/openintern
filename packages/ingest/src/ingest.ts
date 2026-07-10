import { and, eq, lt, notInArray, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs, type Db } from "@openintern/db";
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
  isStaleByAge,
  isStaleByTermYears,
  isTechInternship,
  looksRemote,
  type TermYear,
} from "./classifier.js";
import { syncCompaniesFromYaml } from "./sync-companies.js";

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
  zeroMatchCompanies: string[];
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

const CONSECUTIVE_FAILURE_LIMIT = 24;

/** Title looks like an internship/co-op (before tech filter). */
const INTERNSHIP_TITLE_ONLY =
  /\b(intern|internship|co-?op|coop|apprentice(ship)?|project\s+intern|campus\s+\w+\s+intern|year\s+at\s+\w+)\b/i;

export type DeactivationPolicy = "missing" | "all" | "none";

/**
 * Preserve existing jobs when a non-empty board suddenly has no classifier
 * matches. The last-seen sweep will retire them if that state persists.
 */
export function companyDeactivationPolicy(input: {
  fetchSucceeded: boolean;
  fetched: number;
  techPass: number;
}): DeactivationPolicy {
  if (!input.fetchSucceeded) return "none";
  if (input.techPass > 0) return "missing";
  if (input.fetched === 0) return "all";
  return "none";
}

export async function runIngest(db: Db, opts?: { syncRegistry?: boolean }): Promise<IngestSummary> {
  if (opts?.syncRegistry !== false) {
    await syncCompaniesFromYaml(db);
  }

  const activeCompanies = await db.query.companies.findMany({
    where: eq(companies.active, true),
  });

  let jobsUpserted = 0;
  let jobsDeactivated = 0;
  let staleDeactivated = 0;
  let lastSeenDeactivated = 0;
  const zeroMatchCompanies: string[] = [];
  const failures: { slug: string; error: string }[] = [];
  const funnel: CompanyFunnel[] = [];

  const CHUNK = 8;
  for (let i = 0; i < activeCompanies.length; i += CHUNK) {
    const chunk = activeCompanies.slice(i, i + CHUNK);
    const results = await Promise.allSettled(
      chunk.map((company) => ingestCompany(db, company)),
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
        await db.insert(ingestRuns).values({
          companyId: company.id,
          status: "error",
          jobCount: 0,
          error: message,
        });
        await maybeAutoDisable(db, company.id, company.slug);
      }
    }
  }

  const cutoff = new Date(Date.now() - LAST_SEEN_STALE_DAYS * 24 * 60 * 60 * 1000);
  const unseen = await db
    .update(jobs)
    .set({ isActive: false, updatedAt: new Date() })
    .where(and(eq(jobs.isActive, true), lt(jobs.lastSeenAt, cutoff)))
    .returning({ id: jobs.id });
  lastSeenDeactivated = unseen.length;

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
    companies: activeCompanies.length,
    jobsUpserted,
    jobsDeactivated,
    staleDeactivated,
    lastSeenDeactivated,
    zeroMatchCompanies,
    failures,
    funnel,
    funnelTotals,
  };
}

async function maybeAutoDisable(
  db: Db,
  companyId: string,
  slug: string,
): Promise<void> {
  const recent = await db
    .select({ status: ingestRuns.status })
    .from(ingestRuns)
    .where(eq(ingestRuns.companyId, companyId))
    .orderBy(sql`${ingestRuns.ranAt} desc`)
    .limit(CONSECUTIVE_FAILURE_LIMIT);

  if (
    recent.length >= CONSECUTIVE_FAILURE_LIMIT &&
    recent.every((r) => r.status === "error")
  ) {
    await db
      .update(companies)
      .set({ active: false, updatedAt: new Date() })
      .where(eq(companies.id, companyId));
    console.warn(
      `Auto-disabled company ${slug} after ${CONSECUTIVE_FAILURE_LIMIT} consecutive failures`,
    );
  }
}

async function ingestCompany(
  db: Db,
  company: typeof companies.$inferSelect,
): Promise<{
  jobsUpserted: number;
  jobsDeactivated: number;
  staleDeactivated: number;
  zeroMatch: boolean;
  funnel: CompanyFunnel;
}> {
  const raw = await fetchJobsForAts(company.ats, company.boardToken);
  const internshipTitleJobs = raw.filter((j) => INTERNSHIP_TITLE_ONLY.test(j.title));
  const filtered = raw.filter((j) => isTechInternship(j.title, j.description));
  const seenIds: string[] = [];
  let jobsUpserted = 0;
  let staleDeactivated = 0;
  const now = new Date();

  for (const j of filtered) {
    seenIds.push(j.externalId);
    const excerpt = j.excerpt ?? excerptFromHtml(j.description);
    const isRemote = looksRemote(j.locations, j.title);
    const classifierText = `${j.title} ${j.description}`;
    const terms = extractTerms(classifierText);
    const durationMonths = extractDurationMonthsList(classifierText) ?? [];
    const cohortYear = extractCohortYear(classifierText);
    const termYears = extractTermYears(classifierText, cohortYear);
    const roles = extractRoles(j.title, j.description);
    const regions = extractRegions(j.locations, isRemote);

    const existing = await db.query.jobs.findFirst({
      where: and(eq(jobs.companyId, company.id), eq(jobs.externalId, j.externalId)),
    });
    const firstSeenAt = existing?.firstSeenAt ?? now;
    const postedAt = j.postedAt ?? existing?.postedAt ?? null;

    const staleByTerms = isStaleByTermYears(termYears as TermYear[], now);
    const staleByAge = isStaleByAge({
      termYears: termYears as TermYear[],
      postedAt,
      firstSeenAt,
      now,
      maxAgeDays: MAX_AGE_DAYS_NO_TERM,
    });
    const stale = staleByTerms || staleByAge;

    const fields = {
      title: j.title,
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
      postedAt: j.postedAt,
      lastSeenAt: now,
      updatedAt: now,
    };

    if (existing) {
      await db.update(jobs).set(fields).where(eq(jobs.id, existing.id));
    } else {
      await db.insert(jobs).values({
        companyId: company.id,
        externalId: j.externalId,
        ...fields,
        firstSeenAt: now,
      });
    }
    jobsUpserted += 1;
    if (stale) staleDeactivated += 1;
  }

  let jobsDeactivated = 0;
  const deactivationPolicy = companyDeactivationPolicy({
    fetchSucceeded: true,
    fetched: raw.length,
    techPass: filtered.length,
  });
  if (deactivationPolicy === "missing") {
    const deactivated = await db
      .update(jobs)
      .set({ isActive: false, updatedAt: now })
      .where(
        and(
          eq(jobs.companyId, company.id),
          eq(jobs.isActive, true),
          notInArray(jobs.externalId, seenIds),
        ),
      )
      .returning({ id: jobs.id });
    jobsDeactivated = deactivated.length;
  } else if (deactivationPolicy === "all") {
    const deactivated = await db
      .update(jobs)
      .set({ isActive: false, updatedAt: now })
      .where(and(eq(jobs.companyId, company.id), eq(jobs.isActive, true)))
      .returning({ id: jobs.id });
    jobsDeactivated = deactivated.length;
  }

  const zeroMatch = raw.length > 0 && filtered.length === 0;
  const funnelMsg = `funnel fetched=${raw.length} title=${internshipTitleJobs.length} tech=${filtered.length} upserted=${jobsUpserted}`;
  await db.insert(ingestRuns).values({
    companyId: company.id,
    status: "ok",
    jobCount: filtered.length,
    error: zeroMatch
      ? `zero_match: no tech internships classified; retaining existing jobs until stale; ${funnelMsg}`
      : funnelMsg,
  });

  return {
    jobsUpserted,
    jobsDeactivated,
    staleDeactivated,
    zeroMatch,
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
