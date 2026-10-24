import type { MetadataRoute } from "next";
import { and, eq } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { publicJobSql } from "@/lib/freshness";
import { liveDumpJobs, loadJobsDump } from "@/lib/job-dump";
import { jobPath } from "@/lib/job-slug";

export const dynamic = "force-dynamic";

/** Cap per sitemap file; add sitemap index if corpus exceeds this. */
const JOB_SITEMAP_LIMIT = 5000;

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = "https://openintern.dev";
  const staticEntries: MetadataRoute.Sitemap = [
    { url: `${base}/`, changeFrequency: "hourly", priority: 1 },
    { url: `${base}/jobs`, changeFrequency: "hourly", priority: 0.95 },
    { url: `${base}/docs`, changeFrequency: "weekly", priority: 0.8 },
    { url: `${base}/privacy`, changeFrequency: "yearly", priority: 0.3 },
    { url: `${base}/vs/intern-insider`, changeFrequency: "monthly", priority: 0.6 },
  ];

  try {
    const db = getDb();
    const rows = await db
      .select({
        id: jobs.id,
        title: jobs.title,
        updatedAt: jobs.updatedAt,
        companySlug: companies.slug,
      })
      .from(jobs)
      .innerJoin(companies, eq(jobs.companyId, companies.id))
      .where(and(eq(jobs.isActive, true), publicJobSql()))
      .limit(JOB_SITEMAP_LIMIT);
    return [
      ...staticEntries,
      ...rows.map((r) => ({
        url: `${base}${jobPath(r.companySlug, r.title, r.id)}`,
        lastModified: r.updatedAt,
        changeFrequency: "daily" as const,
        priority: 0.7,
      })),
    ];
  } catch {
    try {
      const dump = await loadJobsDump();
      return [
        ...staticEntries,
        ...liveDumpJobs(dump)
          .slice(0, JOB_SITEMAP_LIMIT)
          .map((j) => ({
            url: `${base}${jobPath(j.company.slug, j.title, j.id)}`,
            lastModified: new Date(j.last_seen_at),
            changeFrequency: "daily" as const,
            priority: 0.7,
          })),
      ];
    } catch {
      return staticEntries;
    }
  }
}
