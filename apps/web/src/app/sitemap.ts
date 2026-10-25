import type { MetadataRoute } from "next";
import { and, eq } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { withDumpFirst } from "@/lib/data-source";
import { getDb } from "@/lib/db";
import { publicJobSql } from "@/lib/freshness";
import { liveDumpJobs, loadJobsDump } from "@/lib/job-dump";
import { jobPath } from "@/lib/job-slug";

/** Align with dump fetch cache (`revalidate: 3600` in job-dump). */
export const revalidate = 3600;

/** Cap per sitemap file; add sitemap index if corpus exceeds this. */
const JOB_SITEMAP_LIMIT = 5000;

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = "https://openintern.dev";
  const staticEntries: MetadataRoute.Sitemap = [
    { url: `${base}/`, changeFrequency: "daily", priority: 1 },
    { url: `${base}/jobs`, changeFrequency: "daily", priority: 0.95 },
    { url: `${base}/docs`, changeFrequency: "weekly", priority: 0.8 },
    { url: `${base}/privacy`, changeFrequency: "yearly", priority: 0.3 },
    { url: `${base}/vs/intern-insider`, changeFrequency: "monthly", priority: 0.6 },
  ];

  try {
    const jobEntries = await withDumpFirst({
      label: "sitemap",
      fromDump: async () => {
        const dump = await loadJobsDump();
        const live = liveDumpJobs(dump).slice(0, JOB_SITEMAP_LIMIT);
        return live.map((j) => ({
          url: `${base}${jobPath(j.company.slug, j.title, j.id)}`,
          lastModified: new Date(j.last_seen_at),
          changeFrequency: "daily" as const,
          priority: 0.7,
        }));
      },
      fromDb: async () => {
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
        return rows.map((r) => ({
          url: `${base}${jobPath(r.companySlug, r.title, r.id)}`,
          lastModified: r.updatedAt,
          changeFrequency: "daily" as const,
          priority: 0.7,
        }));
      },
    });
    return [...staticEntries, ...jobEntries];
  } catch {
    // Both the dump and the database failed.
    return staticEntries;
  }
}
