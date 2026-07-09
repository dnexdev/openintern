import { and, eq, gte, ilike, sql } from "drizzle-orm";
import { companies, jobs, savedSearches, users, type Db } from "@openintern/db";

export type AlertMatch = {
  searchId: string;
  userEmail: string;
  searchName: string;
  webhookUrl: string | null;
  emailEnabled: boolean;
  jobs: {
    id: string;
    title: string;
    company: string;
    applyUrl: string;
    locations: string[];
  }[];
};

/**
 * Find new jobs matching each saved search since lastNotifiedAt (or last 24h).
 * Delivery (Resend / webhook) is handled by the caller.
 */
export async function collectAlertMatches(db: Db): Promise<AlertMatch[]> {
  const searches = await db
    .select({
      id: savedSearches.id,
      name: savedSearches.name,
      query: savedSearches.query,
      location: savedSearches.location,
      companySlug: savedSearches.companySlug,
      remoteOnly: savedSearches.remoteOnly,
      webhookUrl: savedSearches.webhookUrl,
      emailEnabled: savedSearches.emailEnabled,
      lastNotifiedAt: savedSearches.lastNotifiedAt,
      userEmail: users.email,
    })
    .from(savedSearches)
    .innerJoin(users, eq(savedSearches.userId, users.id));

  const results: AlertMatch[] = [];

  for (const s of searches) {
    const since = s.lastNotifiedAt ?? new Date(Date.now() - 24 * 60 * 60 * 1000);
    const conditions = [eq(jobs.isActive, true), gte(jobs.firstSeenAt, since)];

    if (s.query) conditions.push(ilike(jobs.title, `%${s.query}%`));
    if (s.location) {
      conditions.push(
        sql`exists (
          select 1 from jsonb_array_elements_text(${jobs.locations}) loc
          where loc ilike ${"%" + s.location + "%"}
        )`,
      );
    }
    if (s.remoteOnly) conditions.push(eq(jobs.isRemote, true));
    if (s.companySlug) conditions.push(eq(companies.slug, s.companySlug));

    const matched = await db
      .select({
        id: jobs.id,
        title: jobs.title,
        applyUrl: jobs.applyUrl,
        locations: jobs.locations,
        companyName: companies.name,
      })
      .from(jobs)
      .innerJoin(companies, eq(jobs.companyId, companies.id))
      .where(and(...conditions))
      .limit(50);

    if (matched.length === 0) continue;

    results.push({
      searchId: s.id,
      userEmail: s.userEmail,
      searchName: s.name,
      webhookUrl: s.webhookUrl,
      emailEnabled: s.emailEnabled,
      jobs: matched.map((m) => ({
        id: m.id,
        title: m.title,
        company: m.companyName,
        applyUrl: m.applyUrl,
        locations: m.locations ?? [],
      })),
    });

    await db
      .update(savedSearches)
      .set({ lastNotifiedAt: new Date() })
      .where(eq(savedSearches.id, s.id));
  }

  return results;
}
