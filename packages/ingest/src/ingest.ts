import { and, eq, notInArray } from "drizzle-orm";
import { companies, ingestRuns, jobs, type Db } from "@openintern/db";
import { fetchJobsForAts } from "./ats.js";
import { excerptFromHtml, isTechInternship, looksRemote } from "./classifier.js";
import { syncCompaniesFromYaml } from "./sync-companies.js";

export type IngestSummary = {
  companies: number;
  jobsUpserted: number;
  jobsDeactivated: number;
  failures: { slug: string; error: string }[];
};

export async function runIngest(db: Db, opts?: { syncRegistry?: boolean }): Promise<IngestSummary> {
  if (opts?.syncRegistry !== false) {
    await syncCompaniesFromYaml(db);
  }

  const activeCompanies = await db.query.companies.findMany({
    where: eq(companies.active, true),
  });

  let jobsUpserted = 0;
  let jobsDeactivated = 0;
  const failures: { slug: string; error: string }[] = [];

  for (const company of activeCompanies) {
    try {
      const raw = await fetchJobsForAts(company.ats, company.boardToken);
      const filtered = raw.filter((j) => isTechInternship(j.title, j.description));
      const seenIds: string[] = [];

      for (const j of filtered) {
        seenIds.push(j.externalId);
        const excerpt = j.excerpt ?? excerptFromHtml(j.description);
        const isRemote = looksRemote(j.locations, j.title);
        const existing = await db.query.jobs.findFirst({
          where: and(eq(jobs.companyId, company.id), eq(jobs.externalId, j.externalId)),
        });

        if (existing) {
          await db
            .update(jobs)
            .set({
              title: j.title,
              locations: j.locations,
              applyUrl: j.applyUrl,
              excerpt,
              isRemote,
              isActive: true,
              source: company.ats,
              postedAt: j.postedAt,
              lastSeenAt: new Date(),
              updatedAt: new Date(),
            })
            .where(eq(jobs.id, existing.id));
        } else {
          await db.insert(jobs).values({
            companyId: company.id,
            externalId: j.externalId,
            title: j.title,
            locations: j.locations,
            applyUrl: j.applyUrl,
            excerpt,
            isRemote,
            isActive: true,
            source: company.ats,
            postedAt: j.postedAt,
            firstSeenAt: new Date(),
            lastSeenAt: new Date(),
          });
        }
        jobsUpserted += 1;
      }

      if (seenIds.length > 0) {
        const deactivated = await db
          .update(jobs)
          .set({ isActive: false, updatedAt: new Date() })
          .where(
            and(
              eq(jobs.companyId, company.id),
              eq(jobs.isActive, true),
              notInArray(jobs.externalId, seenIds),
            ),
          )
          .returning({ id: jobs.id });
        jobsDeactivated += deactivated.length;
      } else {
        // Successful poll with zero internship matches — deactivate previous actives
        const deactivated = await db
          .update(jobs)
          .set({ isActive: false, updatedAt: new Date() })
          .where(and(eq(jobs.companyId, company.id), eq(jobs.isActive, true)))
          .returning({ id: jobs.id });
        jobsDeactivated += deactivated.length;
      }

      await db.insert(ingestRuns).values({
        companyId: company.id,
        status: "ok",
        jobCount: filtered.length,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      failures.push({ slug: company.slug, error: message });
      await db.insert(ingestRuns).values({
        companyId: company.id,
        status: "error",
        jobCount: 0,
        error: message,
      });
    }
  }

  return {
    companies: activeCompanies.length,
    jobsUpserted,
    jobsDeactivated,
    failures,
  };
}
