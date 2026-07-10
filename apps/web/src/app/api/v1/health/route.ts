import { NextResponse } from "next/server";
import { desc, eq, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";

export const dynamic = "force-dynamic";

function parseFunnel(error: string | null): {
  fetched: number | null;
  title: number | null;
  tech: number | null;
  upserted: number | null;
} {
  if (!error) return { fetched: null, title: null, tech: null, upserted: null };
  const m = error.match(
    /funnel fetched=(\d+) title=(\d+) tech=(\d+) upserted=(\d+)/,
  );
  if (!m) return { fetched: null, title: null, tech: null, upserted: null };
  return {
    fetched: Number(m[1]),
    title: Number(m[2]),
    tech: Number(m[3]),
    upserted: Number(m[4]),
  };
}

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

  const mapped = recentRuns.map((r) => {
    const funnel = parseFunnel(r.error);
    return {
      ...r,
      ran_at: r.ran_at.toISOString(),
      funnel,
    };
  });

  const funnelTotals = mapped.reduce(
    (acc, r) => {
      if (r.funnel.fetched != null) acc.fetched += r.funnel.fetched;
      if (r.funnel.title != null) acc.title += r.funnel.title;
      if (r.funnel.tech != null) acc.tech += r.funnel.tech;
      if (r.funnel.upserted != null) acc.upserted += r.funnel.upserted;
      return acc;
    },
    { fetched: 0, title: 0, tech: 0, upserted: 0 },
  );

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
    funnel_totals_recent: funnelTotals,
    recent_ingest_runs: mapped,
  });
}
