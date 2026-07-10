/**
 * Recompute fingerprint, normalized_title, and role_family_id for all jobs.
 * Resolves fingerprint collisions by keeping the older first_seen_at row.
 */
import { asc, eq } from "drizzle-orm";
import { companies, jobs, type Db } from "@openintern/db";
import {
  jobFingerprint,
  normalizeTitle,
  roleFamilyId,
} from "./normalize-title.js";

export async function backfillRoleFamilies(db: Db) {
  const rows = await db
    .select({
      id: jobs.id,
      companyId: jobs.companyId,
      externalJobId: jobs.externalJobId,
      title: jobs.title,
      source: jobs.source,
      firstSeenAt: jobs.firstSeenAt,
      companySlug: companies.slug,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .orderBy(asc(jobs.firstSeenAt));

  const seenFp = new Map<string, { id: string; firstSeenAt: Date }>();
  let updated = 0;
  let deactivatedDupes = 0;

  for (const row of rows) {
    const fingerprint = jobFingerprint(
      row.companyId,
      row.source,
      row.externalJobId,
    );
    const normalized = normalizeTitle(row.title);
    const family = roleFamilyId(row.companySlug, normalized);

    const prior = seenFp.get(fingerprint);
    if (prior && prior.id !== row.id) {
      // Keep older first_seen; deactivate newer duplicate
      const keepOlder =
        row.firstSeenAt.getTime() >= prior.firstSeenAt.getTime();
      if (keepOlder) {
        await db
          .update(jobs)
          .set({ isActive: false, updatedAt: new Date() })
          .where(eq(jobs.id, row.id));
        deactivatedDupes += 1;
        continue;
      }
      await db
        .update(jobs)
        .set({ isActive: false, updatedAt: new Date() })
        .where(eq(jobs.id, prior.id));
      deactivatedDupes += 1;
      seenFp.set(fingerprint, { id: row.id, firstSeenAt: row.firstSeenAt });
    } else {
      seenFp.set(fingerprint, { id: row.id, firstSeenAt: row.firstSeenAt });
    }

    await db
      .update(jobs)
      .set({
        fingerprint,
        normalizedTitle: normalized,
        roleFamilyId: family,
        updatedAt: new Date(),
      })
      .where(eq(jobs.id, row.id));
    updated += 1;
  }

  return { updated, deactivatedDupes, total: rows.length };
}
