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

export type IngestSummary = {
  companies: number;
  jobsUpserted: number;
  jobsDeactivated: number;
  staleDeactivated: number;
  lastSeenDeactivated: number;
  zeroMatchCompanies: string[];
  failures: { slug: string; error: string }[];
};

const CONSECUTIVE_FAILURE_LIMIT = 24;

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

  // Process in chunks for parallelism
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
      } else {
        const message =
          result.reason instanceof Error ? result.reason.message : String(result.reason);
        failures.push({ slug: company.slug, error: message });
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

  // Global lastSeenAt safety net
  const cutoff = new Date(Date.now() - LAST_SEEN_STALE_DAYS * 24 * 60 * 60 * 1000);
  const unseen = await db
    .update(jobs)
    .set({ isActive: false, updatedAt: new Date() })
    .where(and(eq(jobs.isActive, true), lt(jobs.lastSeenAt, cutoff)))
    .returning({ id: jobs.id });
  lastSeenDeactivated = unseen.length;

  return {
    companies: activeCompanies.length,
    jobsUpserted,
    jobsDeactivated,
    staleDeactivated,
    lastSeenDeactivated,
    zeroMatchCompanies,
    failures,
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
}> {
  const raw = await fetchJobsForAts(company.ats, company.boardToken);
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
    // Always persist an array — never null (DB column is jsonb NOT NULL).
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
  if (seenIds.length > 0) {
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
  } else {
    const deactivated = await db
      .update(jobs)
      .set({ isActive: false, updatedAt: now })
      .where(and(eq(jobs.companyId, company.id), eq(jobs.isActive, true)))
      .returning({ id: jobs.id });
    jobsDeactivated = deactivated.length;
  }

  const zeroMatch = filtered.length === 0;
  await db.insert(ingestRuns).values({
    companyId: company.id,
    status: "ok",
    jobCount: filtered.length,
    error: zeroMatch ? "zero_match: no tech internships on board" : null,
  });

  return { jobsUpserted, jobsDeactivated, staleDeactivated, zeroMatch };
}
