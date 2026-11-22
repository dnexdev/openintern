import { desc, eq, sql } from "drizzle-orm";
import { isZeroMatchRun, parseFunnel, sumFunnels, type Funnel, type FunnelTotals } from "./ingest-health";
import { companies, ingestRuns, jobs } from "./schema";
import type { Db } from "./index";

const PROPRIETARY_ATS = new Set([
  "citadel",
  "citadel_securities",
  "tesla",
  "bytedance",
  "tiktok",
]);

export type HealthRun = {
  id: string;
  status: string;
  job_count: number;
  error: string | null;
  ran_at: string;
  company_slug: string | null;
  company_name: string | null;
  company_ats: string | null;
  funnel: Funnel;
  zero_match: boolean;
};

/**
 * Pipeline health, computed once by the dump job and published as health.json
 * next to jobs.json, so the site and API never query the database for it.
 */
export type HealthSnapshot = {
  generatedAt: string;
  stats: {
    active_jobs: number;
    total_jobs: number;
    active_companies: number;
    total_companies: number;
    last_successful_ingest: string | null;
    zero_match_companies: number;
  };
  pipeline: {
    zero_match_companies: string[];
    recent_failures: {
      company_slug: string | null;
      company_name: string | null;
      company_ats: string | null;
      error: string | null;
      ran_at: string;
    }[];
    proprietary_ats: {
      failures: { company_slug: string | null; company_ats: string | null; error: string | null; ran_at: string }[];
      zero_job_runs: {
        company_slug: string | null;
        company_ats: string | null;
        job_count: number;
        error: string | null;
        ran_at: string;
      }[];
    };
  };
  funnel_totals_latest_per_company: FunnelTotals;
  /** Newest first, up to 100. */
  recent_ingest_runs: HealthRun[];
};

export async function buildHealthSnapshot(db: Db, now = new Date()): Promise<HealthSnapshot> {
  const [jobStats] = await db
    .select({
      active_jobs: sql<number>`count(*) filter (where ${jobs.isActive})::int`,
      total_jobs: sql<number>`count(*)::int`,
    })
    .from(jobs);

  const [companyStats] = await db
    .select({
      active_companies: sql<number>`count(*) filter (where ${companies.active})::int`,
      total_companies: sql<number>`count(*)::int`,
    })
    .from(companies);

  const recentRuns = await db
    .select({
      id: ingestRuns.id,
      status: ingestRuns.status,
      job_count: ingestRuns.jobCount,
      error: ingestRuns.error,
      ran_at: ingestRuns.ranAt,
      company_slug: companies.slug,
      company_name: companies.name,
      company_ats: companies.ats,
    })
    .from(ingestRuns)
    .leftJoin(companies, eq(ingestRuns.companyId, companies.id))
    .orderBy(desc(ingestRuns.ranAt))
    .limit(100);

  const [lastOk] = await db
    .select({ ran_at: ingestRuns.ranAt })
    .from(ingestRuns)
    .where(eq(ingestRuns.status, "ok"))
    .orderBy(desc(ingestRuns.ranAt))
    .limit(1);

  const latestRuns = await db
    .selectDistinctOn([ingestRuns.companyId], {
      company_id: ingestRuns.companyId,
      status: ingestRuns.status,
      error: ingestRuns.error,
      job_count: ingestRuns.jobCount,
      ran_at: ingestRuns.ranAt,
      company_slug: companies.slug,
      company_ats: companies.ats,
    })
    .from(ingestRuns)
    .leftJoin(companies, eq(ingestRuns.companyId, companies.id))
    .orderBy(ingestRuns.companyId, desc(ingestRuns.ranAt));

  const zeroMatchCompanies = latestRuns
    .filter((r) => r.status === "ok" && isZeroMatchRun(r.error))
    .map((r) => r.company_slug)
    .filter((slug): slug is string => Boolean(slug))
    .sort();

  const proprietaryLatest = latestRuns.filter(
    (r) => r.company_ats && PROPRIETARY_ATS.has(r.company_ats),
  );

  return {
    generatedAt: now.toISOString(),
    stats: {
      active_jobs: jobStats?.active_jobs ?? 0,
      total_jobs: jobStats?.total_jobs ?? 0,
      active_companies: companyStats?.active_companies ?? 0,
      total_companies: companyStats?.total_companies ?? 0,
      last_successful_ingest: lastOk?.ran_at?.toISOString() ?? null,
      zero_match_companies: zeroMatchCompanies.length,
    },
    pipeline: {
      zero_match_companies: zeroMatchCompanies,
      recent_failures: recentRuns
        .filter((r) => r.status === "error")
        .slice(0, 20)
        .map((r) => ({
          company_slug: r.company_slug,
          company_name: r.company_name,
          company_ats: r.company_ats,
          error: r.error,
          ran_at: r.ran_at.toISOString(),
        })),
      proprietary_ats: {
        failures: proprietaryLatest
          .filter((r) => r.status === "error")
          .map((r) => ({
            company_slug: r.company_slug,
            company_ats: r.company_ats,
            error: r.error,
            ran_at: r.ran_at.toISOString(),
          })),
        zero_job_runs: proprietaryLatest
          .filter(
            (r) =>
              r.status === "ok" &&
              (isZeroMatchRun(r.error) || (typeof r.job_count === "number" && r.job_count === 0)),
          )
          .map((r) => ({
            company_slug: r.company_slug,
            company_ats: r.company_ats,
            job_count: r.job_count,
            error: r.error,
            ran_at: r.ran_at.toISOString(),
          })),
      },
    },
    funnel_totals_latest_per_company: sumFunnels(latestRuns.map((r) => parseFunnel(r.error))),
    recent_ingest_runs: recentRuns.map((r) => ({
      ...r,
      ran_at: r.ran_at.toISOString(),
      funnel: parseFunnel(r.error),
      zero_match: isZeroMatchRun(r.error),
    })),
  };
}
