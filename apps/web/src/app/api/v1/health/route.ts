import { NextResponse } from "next/server";
import { desc, eq, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";

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

  return NextResponse.json({
    status: "ok",
    product: "OpenIntern",
    version: "0.1.0",
    stats: {
      ...jobStats,
      ...companyStats,
      last_successful_ingest: lastOk?.ran_at?.toISOString() ?? null,
    },
    recent_ingest_runs: recentRuns.map((r) => ({
      ...r,
      ran_at: r.ran_at.toISOString(),
    })),
  });
}
