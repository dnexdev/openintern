import { NextResponse } from "next/server";
import { and, eq } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { apiDbError } from "@/lib/api-error";
import { getDb } from "@/lib/db";
import { isPublicDumpJob, loadJobsDump } from "@/lib/job-dump";
import { publicJobSql } from "@/lib/freshness";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

export async function GET(
  request: Request,
  context: { params: Promise<{ id: string }> },
) {
  const ip = clientIp(request);
  const rl = rateLimit(`job:${ip}`);
  if (!rl.ok) {
    return NextResponse.json({ error: "Rate limit exceeded" }, { status: 429 });
  }

  const { id } = await context.params;
  try {
    const db = getDb();
    const [row] = await db
      .select({
        id: jobs.id,
        title: jobs.title,
        locations: jobs.locations,
        apply_url: jobs.applyUrl,
        excerpt: jobs.excerpt,
        terms: jobs.terms,
        term_years: jobs.termYears,
        duration_months: jobs.durationMonths,
        cohort_year: jobs.cohortYear,
        roles: jobs.roles,
        regions: jobs.regions,
        is_remote: jobs.isRemote,
        is_active: jobs.isActive,
        source: jobs.source,
        posted_at: jobs.postedAt,
        first_seen_at: jobs.firstSeenAt,
        last_seen_at: jobs.lastSeenAt,
        company: {
          name: companies.name,
          slug: companies.slug,
          ats: companies.ats,
          careers_url: companies.careersUrl,
          website_url: companies.websiteUrl,
        },
      })
      .from(jobs)
      .innerJoin(companies, eq(jobs.companyId, companies.id))
      .where(and(eq(jobs.id, id), eq(jobs.isActive, true), publicJobSql()))
      .limit(1);

    if (!row) {
      return NextResponse.json({ error: "Not found" }, { status: 404 });
    }

    return NextResponse.json({
      data: {
        ...row,
        posted_at: row.posted_at?.toISOString() ?? null,
        first_seen_at: row.first_seen_at.toISOString(),
        last_seen_at: row.last_seen_at.toISOString(),
      },
    });
  } catch (err) {
    console.warn("[api] job: database unavailable, falling back to dump:", err);
    try {
      const dump = await loadJobsDump();
      const job = dump.jobs.find((j) => j.id.toLowerCase() === id.toLowerCase());
      const headers = { "X-Data-Source": "dump", "X-Dump-Generated-At": dump.generatedAt };
      if (!job || !isPublicDumpJob(job)) {
        return NextResponse.json({ error: "Not found" }, { status: 404, headers });
      }
      return NextResponse.json(
        {
          data: {
            id: job.id,
            title: job.title,
            locations: job.locations ?? [],
            apply_url: job.apply_url,
            excerpt: job.excerpt,
            terms: job.terms ?? [],
            term_years: job.term_years ?? [],
            duration_months: job.duration_months ?? [],
            cohort_year: job.cohort_year,
            roles: job.roles ?? [],
            regions: job.regions ?? [],
            is_remote: job.is_remote,
            is_active: true,
            source: job.source,
            posted_at: job.posted_at,
            first_seen_at: job.first_seen_at,
            last_seen_at: job.last_seen_at,
            company: { ...job.company, careers_url: null, website_url: null },
          },
        },
        { headers },
      );
    } catch {
      return apiDbError(err);
    }
  }
}
