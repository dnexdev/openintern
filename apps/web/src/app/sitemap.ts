import type { MetadataRoute } from "next";
import { eq } from "drizzle-orm";
import { jobs } from "@openintern/db";
import { getDb } from "@/lib/db";

export const dynamic = "force-dynamic";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = "https://openintern.dev";
  const staticEntries: MetadataRoute.Sitemap = [
    { url: `${base}/`, changeFrequency: "hourly", priority: 1 },
    { url: `${base}/docs`, changeFrequency: "weekly", priority: 0.8 },
    { url: `${base}/health`, changeFrequency: "daily", priority: 0.5 },
  ];

  try {
    const db = getDb();
    const rows = await db
      .select({ id: jobs.id, updatedAt: jobs.updatedAt })
      .from(jobs)
      .where(eq(jobs.isActive, true));
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
