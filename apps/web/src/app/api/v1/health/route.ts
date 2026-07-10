import { NextResponse } from "next/server";
import { desc, eq, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { parseFunnel, sumFunnels } from "@/lib/ingest-health";

export const dynamic = "force-dynamic";

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
      error: ingestRuns.error,
      ran_at: ingestRuns.ranAt,
    })
    .from(ingestRuns)
    .orderBy(ingestRuns.companyId, desc(ingestRuns.ranAt));

  const mapped = recentRuns.map((r) => {
    const funnel = parseFunnel(r.error);
    return {
      ...r,
      ran_at: r.ran_at.toISOString(),
      funnel,
    };
  });

  const funnelTotals = sumFunnels(latestRuns.map((r) => parseFunnel(r.error)));

  return NextResponse.json({
    status: "ok",
    product: "OpenIntern",
    version: "0.1.0",
    stats: {
      ...jobStats,
      ...companyStats,
      last_successful_ingest: lastOk?.ran_at?.toISOString() ?? null,
      season_note:
        "Live board focuses on Fall/Winter 2026–27 and Summer 2027; many Summer 2026 boards are closed.",
    },
    funnel_totals_latest_per_company: funnelTotals,
    recent_ingest_runs: mapped,
  });
}
