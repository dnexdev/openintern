import { NextResponse } from "next/server";
import { desc, eq, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { parseFunnel, sumFunnels, isZeroMatchRun } from "@/lib/ingest-health";

export const dynamic = "force-dynamic";

const PROPRIETARY_ATS = new Set([
  "citadel",
  "citadel_securities",
  "tesla",
  "bytedance",
  "tiktok",
]);

export async function GET() {
  const db = getDb();

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
    .limit(50);

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

  const mapped = recentRuns.map((r) => {
    const funnel = parseFunnel(r.error);
    return {
      ...r,
      ran_at: r.ran_at.toISOString(),
      funnel,
      zero_match: isZeroMatchRun(r.error),
    };
  });

  const funnelTotals = sumFunnels(latestRuns.map((r) => parseFunnel(r.error)));

  const zeroMatchCompanies = latestRuns
    .filter((r) => r.status === "ok" && isZeroMatchRun(r.error))
    .map((r) => r.company_slug)
    .filter((slug): slug is string => Boolean(slug))
    .sort();

  const recentFailures = recentRuns
    .filter((r) => r.status === "error")
    .slice(0, 20)
    .map((r) => ({
      company_slug: r.company_slug,
      company_name: r.company_name,
      company_ats: r.company_ats,
      error: r.error,
      ran_at: r.ran_at.toISOString(),
    }));

  const proprietaryLatest = latestRuns.filter(
    (r) => r.company_ats && PROPRIETARY_ATS.has(r.company_ats),
  );
  const proprietary_failures = proprietaryLatest
    .filter((r) => r.status === "error")
    .map((r) => ({
      company_slug: r.company_slug,
      company_ats: r.company_ats,
      error: r.error,
      ran_at: r.ran_at.toISOString(),
    }));
  const proprietary_zero_job_runs = proprietaryLatest
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
    }));

  return NextResponse.json({
    status: "ok",
    product: "OpenIntern",
    version: "0.1.0",
    stats: {
      ...jobStats,
      ...companyStats,
      last_successful_ingest: lastOk?.ran_at?.toISOString() ?? null,
      zero_match_companies: zeroMatchCompanies.length,
      season_note:
        "Live board focuses on Fall/Winter 2026–27 and Summer 2027; many Summer 2026 boards are closed.",
    },
    pipeline: {
      zero_match_companies: zeroMatchCompanies,
      recent_failures: recentFailures,
      proprietary_ats: {
        failures: proprietary_failures,
        zero_job_runs: proprietary_zero_job_runs,
      },
      triage:
        "Check /health → pnpm recover-tokens → fix data/companies YAML → pnpm validate-tokens → re-run ingest.",
    },
    funnel_totals_latest_per_company: funnelTotals,
    recent_ingest_runs: mapped,
  });
}
