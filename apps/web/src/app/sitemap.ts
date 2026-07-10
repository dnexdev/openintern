import type { MetadataRoute } from "next";
import { and, eq } from "drizzle-orm";
import { jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";

export const dynamic = "force-dynamic";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = "https://openintern.dev";
  const staticEntries: MetadataRoute.Sitemap = [
    { url: `${base}/`, changeFrequency: "hourly", priority: 1 },
    { url: `${base}/jobs`, changeFrequency: "hourly", priority: 0.95 },
    { url: `${base}/docs`, changeFrequency: "weekly", priority: 0.8 },
    { url: `${base}/privacy`, changeFrequency: "yearly", priority: 0.3 },
    { url: `${base}/health`, changeFrequency: "daily", priority: 0.5 },
  ];

  try {
    const db = getDb();
    const rows = await db
      .select({ id: jobs.id, updatedAt: jobs.updatedAt })
      .from(jobs)
      .where(and(eq(jobs.isActive, true), freshnessSql()))
      .limit(5000);
    return [
      ...staticEntries,
      ...rows.map((r) => ({
        url: `${base}/jobs/${r.id}`,
        lastModified: r.updatedAt,
        changeFrequency: "daily" as const,
        priority: 0.7,
      })),
    ];
  } catch {
    return staticEntries;
  }
}
