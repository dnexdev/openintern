import { and, eq, sql } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";

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

export async function loadJobById(id: string): Promise<JobDetailRow | null> {
  const db = getDb();
  const [row] = await db
    .select(jobSelect)
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(and(eq(jobs.id, id), eq(jobs.isActive, true), freshnessSql()))
    .limit(1);
  return row ?? null;
}

export async function loadJobByCompanyAndIdPrefix(
  companySlug: string,
  idPrefix: string,
): Promise<JobDetailRow | null> {
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
        freshnessSql(),
      ),
    )
    .limit(2);
  if (rows.length !== 1) return null;
  return rows[0]!;
}
