import { NextResponse } from "next/server";
import { asc, eq, sql } from "drizzle-orm";
import { companies, jobs, publicJobSql } from "@openintern/db";
import { apiDbError } from "@/lib/api-error";
import { dumpSource, withDumpFirst } from "@/lib/data-source";
import { getDb } from "@/lib/db";
import { liveDumpJobs, loadCompaniesDump, loadJobsDump } from "@/lib/job-dump";
import { clientIp, rateLimit } from "@/lib/rate-limit";

export const revalidate = 3600;

type CompanyRow = {
  id: string;
  name: string;
  slug: string;
  ats: string;
  careers_url: string | null;
  website_url: string | null;
  active: boolean;
  active_jobs: number;
};

export async function GET(request: Request) {
  const ip = clientIp(request);
  const rl = rateLimit(`companies:${ip}`);
  if (!rl.ok) {
    return NextResponse.json({ error: "Rate limit exceeded" }, { status: 429 });
  }

  try {
    const url = new URL(request.url);
    const activeOnly = url.searchParams.get("active") !== "false";

    const { rows, dataSource } = await withDumpFirst({
      label: "companies-api",
      fromDump: async () => {
        const [dump, registry] = await Promise.all([loadJobsDump(), loadCompaniesDump()]);
        if (registry) {
          // Live counts come from the jobs the site would show right now.
          const live = new Map<string, number>();
          for (const job of liveDumpJobs(dump)) live.set(job.company.slug, (live.get(job.company.slug) ?? 0) + 1);
          const rows = registry
            .filter((c) => !activeOnly || c.active)
            .map((c) => ({ ...c, active_jobs: live.get(c.slug) ?? 0 }))
            .sort((a, b) => a.name.localeCompare(b.name));
          return { rows, dataSource: dumpSource(dump.generatedAt) };
        }
        // Older releases have no companies.json: derive what we can from jobs.
        const bySlug = new Map<string, CompanyRow>();
        for (const job of liveDumpJobs(dump)) {
          const prev = bySlug.get(job.company.slug);
          if (prev) {
            prev.active_jobs += 1;
          } else {
            bySlug.set(job.company.slug, {
              // Dump has no company UUID; slug is a stable public identifier.
              id: job.company.slug,
              name: job.company.name,
              slug: job.company.slug,
              ats: job.company.ats,
              careers_url: null,
              website_url: null,
              active: true,
              active_jobs: 1,
            });
          }
        }
        const rows = [...bySlug.values()].sort((a, b) =>
          a.name.localeCompare(b.name),
        );
        // Dump only lists companies with public jobs (no inactive registry).
        return { rows, dataSource: dumpSource(dump.generatedAt) };
      },
      fromDb: async () => {
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
        return { rows, dataSource: { kind: "db" as const } };
      },
    });

    return NextResponse.json(
      { data: rows, total: rows.length },
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
