import { NextResponse } from "next/server";
import { and, eq } from "drizzle-orm";
import { applyUrlAllowed, companies, jobs } from "@openintern/db";
import { apiDbError } from "@/lib/api-error";
import { dumpSource, withDumpFirst } from "@/lib/data-source";
import { getDb } from "@/lib/db";
import { publicJobSql } from "@/lib/freshness";
import { isPublicDumpJob, loadJobsDump } from "@/lib/job-dump";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const revalidate = 3600;

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
    const { row, dataSource } = await withDumpFirst({
      label: "job-api",
      fromDump: async () => {
        const dump = await loadJobsDump();
        const job = dump.jobs.find((j) => j.id.toLowerCase() === id.toLowerCase());
        if (!job || !isPublicDumpJob(job)) {
          return { row: null, dataSource: dumpSource(dump.generatedAt) };
        }
        return {
          row: {
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
            company: {
              name: job.company.name,
              slug: job.company.slug,
              ats: job.company.ats,
              careers_url: job.company.careers_url ?? null,
              website_url: job.company.website_url ?? null,
            },
          },
          dataSource: dumpSource(dump.generatedAt),
        };
      },
      fromDb: async () => {
        const db = getDb();
        const [dbRow] = await db
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

        if (!dbRow || !applyUrlAllowed(dbRow.company.slug, dbRow.apply_url)) {
          return { row: null, dataSource: { kind: "db" as const } };
        }

        return {
          row: {
            ...dbRow,
            posted_at: dbRow.posted_at?.toISOString() ?? null,
            first_seen_at: dbRow.first_seen_at.toISOString(),
            last_seen_at: dbRow.last_seen_at.toISOString(),
          },
          dataSource: { kind: "db" as const },
        };
      },
    });

    if (!row) {
      return NextResponse.json({ error: "Not found" }, { status: 404 });
    }

    return NextResponse.json(
      { data: row },
      {
        headers: {
          "X-RateLimit-Remaining": String(rl.remaining),
          "Cache-Control": "public, s-maxage=3600, stale-while-revalidate=86400",
          ...(dataSource.kind === "dump"
            ? {
                "X-Data-Source": "dump",
                "X-Dump-Generated-At": dataSource.generatedAt,
              }
            : { "X-Data-Source": "db" }),
        },
      },
    );
  } catch (err) {
    // Both the dump and the database failed.
    return apiDbError(err);
  }
}
