import { NextResponse } from "next/server";
import { isDumpStale, MAX_DUMP_AGE_MS } from "@/lib/dump-age";
import { liveDumpJobs, loadJobsDump } from "@/lib/job-dump";

export const dynamic = "force-dynamic";

const MAX_DUMP_AGE_HOURS = MAX_DUMP_AGE_MS / 3_600_000;

/**
 * Uptime check for monitors: 200 while the site serves a fresh dump, 503 when
 * the dump is missing or stale. Never touches the database.
 */
export async function GET() {
  const headers = { "Cache-Control": "no-store" };
  try {
    const dump = await loadJobsDump();
    const ageHours = (Date.now() - new Date(dump.generatedAt).getTime()) / 3_600_000;
    const fresh = !isDumpStale(dump.generatedAt);
    return NextResponse.json(
      {
        status: fresh ? "ok" : "stale",
        dump_generated_at: dump.generatedAt,
        dump_age_hours: Number.isFinite(ageHours) ? Math.round(ageHours * 10) / 10 : null,
        max_dump_age_hours: MAX_DUMP_AGE_HOURS,
        active_jobs: liveDumpJobs(dump).length,
      },
      { status: fresh ? 200 : 503, headers },
    );
  } catch {
    return NextResponse.json({ status: "down", error: "Dump unavailable" }, { status: 503, headers });
  }
}
