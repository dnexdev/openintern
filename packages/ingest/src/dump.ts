import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { gzipSync } from "node:zlib";
import { and, asc, eq } from "drizzle-orm";
import {
  DELISTED_JOBS,
  applyUrlAllowed,
  buildHealthSnapshot,
  companies,
  jobs,
  publicJobSql,
  type Db,
} from "@openintern/db";

export async function writeDumps(db: Db, outDir: string, now = new Date()) {
  await fs.mkdir(outDir, { recursive: true });

  const rows = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      normalizedTitle: jobs.normalizedTitle,
      roleFamilyId: jobs.roleFamilyId,
      locations: jobs.locations,
      applyUrl: jobs.applyUrl,
      excerpt: jobs.excerpt,
      terms: jobs.terms,
      termYears: jobs.termYears,
      durationMonths: jobs.durationMonths,
      cohortYear: jobs.cohortYear,
      roles: jobs.roles,
      regions: jobs.regions,
      isRemote: jobs.isRemote,
      source: jobs.source,
      postedAt: jobs.postedAt,
      firstSeenAt: jobs.firstSeenAt,
      lastSeenAt: jobs.lastSeenAt,
      companyName: companies.name,
      companySlug: companies.slug,
      companyAts: companies.ats,
      companyWebsiteUrl: companies.websiteUrl,
      companyCareersUrl: companies.careersUrl,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(and(eq(jobs.isActive, true), publicJobSql()))
    .orderBy(asc(jobs.id))
    .then((all) => all.filter((r) => applyUrlAllowed(r.companySlug, r.applyUrl)));

  const generatedAt = now.toISOString();
  const payload = {
    generatedAt,
    count: rows.length,
    jobs: rows.map((r) => ({
      id: r.id,
      title: r.title,
      company: {
        name: r.companyName,
        slug: r.companySlug,
        ats: r.companyAts,
        website_url: r.companyWebsiteUrl,
        careers_url: r.companyCareersUrl,
      },
      locations: r.locations,
      apply_url: r.applyUrl,
      excerpt: r.excerpt,
      terms: r.terms,
      term_years: r.termYears,
      duration_months: r.durationMonths,
      cohort_year: r.cohortYear,
      roles: r.roles,
      regions: r.regions,
      is_remote: r.isRemote,
      source: r.source,
      posted_at: r.postedAt?.toISOString() ?? null,
      first_seen_at: r.firstSeenAt.toISOString(),
      last_seen_at: r.lastSeenAt.toISOString(),
      role_family_id: r.roleFamilyId,
      normalized_title: r.normalizedTitle,
    })),
  };

  const jsonPath = path.join(outDir, "jobs.json");
  await fs.writeFile(jsonPath, JSON.stringify(payload, null, 2));

  // New columns go at the end: spreadsheets address columns by position.
  const csvHeader =
    "id,title,company,company_slug,locations,apply_url,excerpt,terms,term_years,duration_months,cohort_year,roles,regions,is_remote,source,posted_at,first_seen_at,last_seen_at,role_family_id,normalized_title";
  const csvLines = rows.map((r) => {
    const locs = Array.isArray(r.locations) ? r.locations.join("; ") : "";
    const cells = [
      r.id,
      r.title,
      r.companyName,
      r.companySlug,
      locs,
      r.applyUrl,
      r.excerpt ?? "",
      Array.isArray(r.terms) ? r.terms.join("; ") : "",
      Array.isArray(r.termYears)
        ? r.termYears.map((t) => `${t.term}-${t.year}`).join("; ")
        : "",
      Array.isArray(r.durationMonths) ? r.durationMonths.join("; ") : "",
      r.cohortYear != null ? String(r.cohortYear) : "",
      Array.isArray(r.roles) ? r.roles.join("; ") : "",
      Array.isArray(r.regions) ? r.regions.join("; ") : "",
      String(r.isRemote),
      r.source,
      r.postedAt?.toISOString() ?? "",
      r.firstSeenAt.toISOString(),
      r.lastSeenAt.toISOString(),
      r.roleFamilyId,
      r.normalizedTitle,
    ].map(csvEscape);
    return cells.join(",");
  });
  const csvPath = path.join(outDir, "jobs.csv");
  await fs.writeFile(csvPath, [csvHeader, ...csvLines].join("\n"));

  // Every registry company, so v1 /companies can be served without the database.
  const liveByCompany = new Map<string, number>();
  for (const r of rows) liveByCompany.set(r.companySlug, (liveByCompany.get(r.companySlug) ?? 0) + 1);
  const companyRows = await db.select().from(companies).orderBy(asc(companies.slug));
  const companiesPath = path.join(outDir, "companies.json");
  await fs.writeFile(
    companiesPath,
    JSON.stringify(
      {
        generatedAt,
        count: companyRows.length,
        companies: companyRows.map((c) => ({
          id: c.id,
          name: c.name,
          slug: c.slug,
          ats: c.ats,
          careers_url: c.careersUrl,
          website_url: c.websiteUrl,
          active: c.active,
          active_jobs: liveByCompany.get(c.slug) ?? 0,
        })),
      },
      null,
      2,
    ),
  );

  const healthPath = path.join(outDir, "health.json");
  await fs.writeFile(healthPath, JSON.stringify(await buildHealthSnapshot(db, now), null, 2));

  // Mirrors must drop these ids too (DATA.md).
  const denylistPath = path.join(outDir, "denylist.json");
  await fs.writeFile(
    denylistPath,
    JSON.stringify(
      { generatedAt, jobs: [...DELISTED_JOBS].map(([id, reason]) => ({ id, reason })) },
      null,
      2,
    ),
  );

  for (const f of [jsonPath, csvPath]) {
    await fs.writeFile(`${f}.gz`, gzipSync(await fs.readFile(f), { level: 9 }));
  }

  const byAts: Record<string, number> = {};
  for (const r of rows) byAts[r.companyAts] = (byAts[r.companyAts] ?? 0) + 1;
  const files: Record<string, { sha256: string; bytes: number }> = {};
  for (const name of (await fs.readdir(outDir)).sort()) {
    if (name === "MANIFEST.json") continue;
    const buf = await fs.readFile(path.join(outDir, name));
    files[name] = { sha256: createHash("sha256").update(buf).digest("hex"), bytes: buf.length };
  }
  const manifestPath = path.join(outDir, "MANIFEST.json");
  await fs.writeFile(
    manifestPath,
    JSON.stringify({ generatedAt, jobs: rows.length, companies: companyRows.length, byAts, files }, null, 2),
  );

  return { jsonPath, csvPath, healthPath, companiesPath, manifestPath, count: rows.length };
}

function csvEscape(value: string): string {
  if (/[",\n]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}
