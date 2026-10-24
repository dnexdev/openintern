import { NextResponse } from "next/server";
import { asc, eq, sql } from "drizzle-orm";
import { companies, jobs, publicJobSql } from "@openintern/db";
import { apiDbError } from "@/lib/api-error";
import { getDb } from "@/lib/db";
import { liveDumpJobs, loadJobsDump } from "@/lib/job-dump";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const ip = clientIp(request);
  const rl = rateLimit(`companies:${ip}`);
  if (!rl.ok) {
    return NextResponse.json({ error: "Rate limit exceeded" }, { status: 429 });
  }

  try {
    const url = new URL(request.url);
    const activeOnly = url.searchParams.get("active") !== "false";

    const db = getDb();
    const rows = await db
      .select({
        id: companies.id,
        name: companies.name,
        slug: companies.slug,
        ats: companies.ats,
        careers_url: companies.careersUrl,
        website_url: companies.websiteUrl,
        active: companies.active,
        active_jobs: sql<number>`(
        select count(*)::int from ${jobs}
        where ${jobs.companyId} = ${companies.id}
          and ${jobs.isActive} = true
          and ${publicJobSql()}
      )`,
      })
      .from(companies)
      .where(activeOnly ? eq(companies.active, true) : undefined)
      .orderBy(asc(companies.name));

    return NextResponse.json({ data: rows, total: rows.length });
  } catch (err) {
    console.warn("[api] companies: database unavailable, falling back to dump:", err);
    try {
      const dump = await loadJobsDump();
      const bySlug = new Map<string, { name: string; slug: string; ats: string; active_jobs: number }>();
      for (const job of liveDumpJobs(dump)) {
        const c = bySlug.get(job.company.slug) ?? { ...job.company, active_jobs: 0 };
        c.active_jobs += 1;
        bySlug.set(job.company.slug, c);
      }
      // The dump has no company ids or URLs; only companies with live jobs appear.
      const rows = [...bySlug.values()]
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((c) => ({
          id: c.slug,
          name: c.name,
          slug: c.slug,
          ats: c.ats,
          careers_url: null,
          website_url: null,
          active: true,
          active_jobs: c.active_jobs,
        }));
      return NextResponse.json(
        { data: rows, total: rows.length },
        { headers: { "X-Data-Source": "dump", "X-Dump-Generated-At": dump.generatedAt } },
      );
    } catch {
      return apiDbError(err);
    }
  }
}
