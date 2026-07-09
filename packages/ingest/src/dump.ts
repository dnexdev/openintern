import fs from "node:fs/promises";
import path from "node:path";
import { eq } from "drizzle-orm";
import { companies, jobs, type Db } from "@openintern/db";

export async function writeDumps(db: Db, outDir: string) {
  await fs.mkdir(outDir, { recursive: true });

  const rows = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      locations: jobs.locations,
      applyUrl: jobs.applyUrl,
      excerpt: jobs.excerpt,
      terms: jobs.terms,
      durationMonths: jobs.durationMonths,
      cohortYear: jobs.cohortYear,
      isRemote: jobs.isRemote,
      source: jobs.source,
      postedAt: jobs.postedAt,
      firstSeenAt: jobs.firstSeenAt,
      lastSeenAt: jobs.lastSeenAt,
      companyName: companies.name,
      companySlug: companies.slug,
      companyAts: companies.ats,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(eq(jobs.isActive, true));

  const payload = {
    generatedAt: new Date().toISOString(),
    count: rows.length,
    jobs: rows.map((r) => ({
      id: r.id,
      title: r.title,
      company: { name: r.companyName, slug: r.companySlug, ats: r.companyAts },
      locations: r.locations,
      apply_url: r.applyUrl,
      excerpt: r.excerpt,
      terms: r.terms,
      duration_months: r.durationMonths,
      cohort_year: r.cohortYear,
      is_remote: r.isRemote,
      source: r.source,
      posted_at: r.postedAt?.toISOString() ?? null,
      first_seen_at: r.firstSeenAt.toISOString(),
      last_seen_at: r.lastSeenAt.toISOString(),
    })),
  };

  const jsonPath = path.join(outDir, "jobs.json");
  await fs.writeFile(jsonPath, JSON.stringify(payload, null, 2));

  const csvHeader =
    "id,title,company,company_slug,locations,apply_url,terms,duration_months,cohort_year,is_remote,source,posted_at,first_seen_at,last_seen_at";
  const csvLines = rows.map((r) => {
    const locs = Array.isArray(r.locations) ? r.locations.join("; ") : "";
    const cells = [
      r.id,
      r.title,
      r.companyName,
      r.companySlug,
      locs,
      r.applyUrl,
      Array.isArray(r.terms) ? r.terms.join("; ") : "",
      r.durationMonths != null ? String(r.durationMonths) : "",
      r.cohortYear != null ? String(r.cohortYear) : "",
      String(r.isRemote),
      r.source,
      r.postedAt?.toISOString() ?? "",
      r.firstSeenAt.toISOString(),
      r.lastSeenAt.toISOString(),
    ].map(csvEscape);
    return cells.join(",");
  });
  const csvPath = path.join(outDir, "jobs.csv");
  await fs.writeFile(csvPath, [csvHeader, ...csvLines].join("\n"));

  return { jsonPath, csvPath, count: rows.length };
}

function csvEscape(value: string): string {
  if (/[",\n]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}
