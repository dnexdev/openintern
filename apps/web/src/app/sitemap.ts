import type { MetadataRoute } from "next";
import { and, eq } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";
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
    { url: `${base}/health`, changeFrequency: "daily", priority: 0.5 },
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
      .where(and(eq(jobs.isActive, true), freshnessSql()))
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
    return staticEntries;
  }
}
