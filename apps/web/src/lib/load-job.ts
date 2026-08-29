import { and, eq, sql } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import type { DataSource } from "@/lib/data-source";
import { dumpSource } from "@/lib/data-source";
import { getDb } from "@/lib/db";
import { publicJobSql } from "@/lib/freshness";
import { isPublicDumpJob, loadJobsDump, type DumpJob } from "@/lib/job-dump";

export type JobDetailRow = {
  id: string;
  title: string;
  locations: string[];
  applyUrl: string;
  excerpt: string | null;
  terms: string[];
  termYears: { term: string; year: number }[];
  durationMonths: number[];
  roles: string[];
  regions: string[];
  cohortYear: number | null;
  isRemote: boolean;
  isActive: boolean;
  source: string;
  postedAt: Date | null;
  firstSeenAt: Date;
  companyName: string;
  companySlug: string;
  companyWebsiteUrl: string | null;
  companyCareersUrl: string | null;
};

const jobSelect = {
  id: jobs.id,
  title: jobs.title,
  locations: jobs.locations,
  applyUrl: jobs.applyUrl,
  excerpt: jobs.excerpt,
  terms: jobs.terms,
  termYears: jobs.termYears,
  durationMonths: jobs.durationMonths,
  roles: jobs.roles,
  regions: jobs.regions,
  cohortYear: jobs.cohortYear,
  isRemote: jobs.isRemote,
  isActive: jobs.isActive,
  source: jobs.source,
  postedAt: jobs.postedAt,
  firstSeenAt: jobs.firstSeenAt,
  companyName: companies.name,
  companySlug: companies.slug,
  companyWebsiteUrl: companies.websiteUrl,
  companyCareersUrl: companies.careersUrl,
};

function dumpJobToDetailRow(job: DumpJob): JobDetailRow {
  return {
    id: job.id,
    title: job.title,
    locations: job.locations ?? [],
    applyUrl: job.apply_url,
    excerpt: job.excerpt,
    terms: job.terms ?? [],
    termYears: job.term_years ?? [],
    durationMonths: job.duration_months ?? [],
    roles: job.roles ?? [],
    regions: job.regions ?? [],
    cohortYear: job.cohort_year,
    isRemote: job.is_remote,
    isActive: true,
    source: job.source,
    postedAt: job.posted_at ? new Date(job.posted_at) : null,
    firstSeenAt: new Date(job.first_seen_at),
    companyName: job.company.name,
    companySlug: job.company.slug,
    companyWebsiteUrl: null,
    companyCareersUrl: null,
  };
}

function idPrefixMatches(id: string, prefix: string) {
  return id.replace(/-/g, "").startsWith(prefix);
}

export async function loadJobById(
  id: string,
): Promise<{ job: JobDetailRow | null; dataSource: DataSource }> {
  try {
    const db = getDb();
    const [row] = await db
      .select(jobSelect)
      .from(jobs)
      .innerJoin(companies, eq(jobs.companyId, companies.id))
      .where(and(eq(jobs.id, id), eq(jobs.isActive, true), publicJobSql()))
      .limit(1);
    return { job: row ?? null, dataSource: { kind: "db" } };
  } catch (dbErr) {
    console.warn("[job] database unavailable, falling back to dump:", dbErr);
    const dump = await loadJobsDump();
    const job = liveDumpJobById(dump.jobs, id);
    return { job, dataSource: dumpSource(dump.generatedAt) };
  }
}

export async function loadJobByCompanyAndIdPrefix(
  companySlug: string,
  idPrefix: string,
): Promise<{ job: JobDetailRow | null; dataSource: DataSource }> {
  try {
    const db = getDb();
    const rows = await db
      .select(jobSelect)
      .from(jobs)
      .innerJoin(companies, eq(jobs.companyId, companies.id))
      .where(
        and(
          eq(companies.slug, companySlug),
          sql`replace(${jobs.id}::text, '-', '') like ${idPrefix + "%"}`,
          eq(jobs.isActive, true),
          publicJobSql(),
        ),
      )
      .limit(2);
    if (rows.length !== 1) return { job: null, dataSource: { kind: "db" } };
    return { job: rows[0]!, dataSource: { kind: "db" } };
  } catch (dbErr) {
    console.warn("[job] database unavailable, falling back to dump:", dbErr);
    const dump = await loadJobsDump();
    const job = liveDumpJobByCompanyPrefix(dump.jobs, companySlug, idPrefix);
    return { job, dataSource: dumpSource(dump.generatedAt) };
  }
}

function liveDumpJobById(jobs: DumpJob[], id: string): JobDetailRow | null {
  const job = jobs.find((j) => j.id === id);
  if (!job || !isPublicDumpJob(job)) return null;
  return dumpJobToDetailRow(job);
}

function liveDumpJobByCompanyPrefix(
  jobs: DumpJob[],
  companySlug: string,
  idPrefix: string,
): JobDetailRow | null {
  const matches = jobs.filter(
    (j) =>
      j.company.slug === companySlug &&
      idPrefixMatches(j.id, idPrefix) &&
      isPublicDumpJob(j),
  );
  if (matches.length !== 1) return null;
  return dumpJobToDetailRow(matches[0]!);
}
