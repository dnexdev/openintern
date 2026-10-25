import { NextResponse } from "next/server";
import { loadHealthSnapshot } from "@/lib/health-snapshot";
import { liveDumpJobs, loadJobsDump } from "@/lib/job-dump";

export const revalidate = 3600;

const SEASON_NOTE =
  "Live board focuses on Fall/Winter 2026–27 and Summer 2027; many Summer 2026 boards are closed.";
const TRIAGE =
  "Check /health → pnpm recover-tokens → fix data/companies YAML → pnpm validate-tokens → re-run ingest.";

/**
 * Pipeline health from the snapshot the dump job publishes (health.json).
 * No database queries: polling this endpoint must not keep Neon awake.
 */
export async function GET() {
  const [snapshot, dump] = await Promise.all([
    loadHealthSnapshot(),
    loadJobsDump().catch(() => null),
  ]);

  const headers = {
    "Cache-Control": "public, s-maxage=3600, stale-while-revalidate=600",
    "X-Data-Source": "snapshot",
    ...(snapshot ? { "X-Snapshot-Generated-At": snapshot.generatedAt } : {}),
    ...(dump ? { "X-Dump-Generated-At": dump.generatedAt } : {}),
  };

  if (snapshot) {
    return NextResponse.json(
      {
        status: "ok",
        product: "OpenIntern",
        version: "0.1.0",
        snapshot_generated_at: snapshot.generatedAt,
        stats: { ...snapshot.stats, season_note: SEASON_NOTE },
        pipeline: { ...snapshot.pipeline, triage: TRIAGE },
        funnel_totals_latest_per_company: snapshot.funnel_totals_latest_per_company,
        recent_ingest_runs: snapshot.recent_ingest_runs.slice(0, 50),
      },
      { headers },
    );
  }

  // No snapshot published yet: report what the public dump shows.
  if (!dump) {
    return NextResponse.json(
      { error: "Health data unavailable" },
      { status: 503, headers: { "Cache-Control": "no-store", "Retry-After": "3600" } },
    );
  }
  const live = liveDumpJobs(dump);
  return NextResponse.json(
    {
      status: "degraded",
      product: "OpenIntern",
      version: "0.1.0",
      snapshot_generated_at: null,
      stats: {
        active_jobs: live.length,
        total_jobs: null,
        active_companies: new Set(live.map((j) => j.company.slug)).size,
        total_companies: null,
        last_successful_ingest: null,
        zero_match_companies: null,
        season_note: SEASON_NOTE,
      },
      pipeline: {
        zero_match_companies: [],
        recent_failures: [],
        proprietary_ats: { failures: [], zero_job_runs: [] },
        triage: TRIAGE,
      },
      funnel_totals_latest_per_company: null,
      recent_ingest_runs: [],
    },
    { headers },
  );
}
