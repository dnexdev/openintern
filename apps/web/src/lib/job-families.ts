import { and, desc, eq, gte, ilike, or, sql, type SQL } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { getTier1Slugs } from "@/lib/curated";
import { getDb } from "@/lib/db";
import { publicJobSql } from "@/lib/freshness";
import { formatPostingLocations } from "@/lib/posting-label";

export type FamilySort = "first_seen" | "posted" | "prestige";

export type JobPosting = {
  id: string;
  title: string;
  location: string;
  locations: string[];
  postedAt: string | null;
  applyUrl: string;
  isRemote: boolean;
  firstSeenAt: string;
};

export type JobFamily = {
  roleFamilyId: string;
  title: string;
  company: {
    name: string;
    slug: string;
    ats: string;
    websiteUrl: string | null;
    careersUrl: string | null;
  };
  roles: string[];
  regions: string[];
  terms: string[];
  durationMonths: number[];
  excerpt: string | null;
  source: string;
  firstSeenAt: string;
  postings: JobPosting[];
};

export type FamilyQueryOpts = {
  query: string;
  company: string;
  roles: string[];
  regions: string[];
  terms: string[];
  durations: number[];
  postedAfter?: Date | null;
  sort: FamilySort;
  limit: number;
  offset: number;
};

function buildConditions(opts: FamilyQueryOpts): SQL | undefined {
  const conditions: (SQL | undefined)[] = [eq(jobs.isActive, true), publicJobSql()];

  if (opts.query) conditions.push(ilike(jobs.title, `%${opts.query}%`));
  if (opts.company) conditions.push(eq(companies.slug, opts.company));
  if (opts.roles.length > 0) {
    conditions.push(
      or(
        ...opts.roles.map((r) => sql`${jobs.roles} @> ${JSON.stringify([r])}::jsonb`),
      ),
    );
  }
  if (opts.regions.length > 0) {
    conditions.push(
      or(
        ...opts.regions.map(
          (r) => sql`${jobs.regions} @> ${JSON.stringify([r])}::jsonb`,
        ),
      ),
    );
  }
  if (opts.terms.length > 0) {
    conditions.push(
      or(
        ...opts.terms.map((t) => sql`${jobs.terms} @> ${JSON.stringify([t])}::jsonb`),
      ),
    );
  }
  if (opts.durations.length > 0) {
    conditions.push(
      or(
        ...opts.durations.map(
          (d) => sql`${jobs.durationMonths} @> ${JSON.stringify([d])}::jsonb`,
        ),
      ),
    );
  }
  if (opts.postedAfter) conditions.push(gte(jobs.postedAt, opts.postedAfter));

  return and(...conditions);
}

function displayTitle(titles: string[], normalized: string): string {
  if (titles.length === 0) return normalized;
  // Prefer shortest raw title (usually the least suffix-noisy)
  return [...titles].sort((a, b) => a.length - b.length)[0]!;
}

function primaryLocation(locations: string[] | null, isRemote: boolean): string {
  return formatPostingLocations(locations ?? [], isRemote);
}

type FamilyAcc = {
  roleFamilyId: string;
  titles: string[];
  normalizedTitle: string;
  companyName: string;
  companySlug: string;
  companyAts: string;
  companyWebsiteUrl: string | null;
  companyCareersUrl: string | null;
  roles: string[];
  regions: string[];
  terms: string[];
  durationMonths: number[];
  excerpt: string | null;
  source: string;
  sortKey: number;
  postings: JobPosting[];
};

/**
 * Round-robin by company after time-sort.
 * Batch ingest stamps many roles with the same first_seen/posted time, so a
 * pure chrono feed shows the same employer 5–6 times in a row. Interleave keeps
 * newer roles first while spreading companies across the page.
 */
function diversifyByCompany(sorted: FamilyAcc[]): FamilyAcc[] {
  const queues = new Map<string, FamilyAcc[]>();
  const companyOrder: string[] = [];
  for (const f of sorted) {
    const q = queues.get(f.companySlug);
    if (!q) {
      queues.set(f.companySlug, [f]);
      companyOrder.push(f.companySlug);
    } else {
      q.push(f);
    }
  }

  const out: FamilyAcc[] = [];
  let remaining = sorted.length;
  while (remaining > 0) {
    let progressed = false;
    for (const slug of companyOrder) {
      const q = queues.get(slug);
      if (!q || q.length === 0) continue;
      out.push(q.shift()!);
      remaining -= 1;
      progressed = true;
    }
    if (!progressed) break;
  }
  return out;
}

function orderFamilies(accList: FamilyAcc[], sort: FamilySort): FamilyAcc[] {
  const chrono = [...accList].sort((a, b) => b.sortKey - a.sortKey);

  if (sort === "prestige") {
    const tier1 = getTier1Slugs();
    const prestigious = chrono.filter((f) => tier1.has(f.companySlug));
    const rest = chrono.filter((f) => !tier1.has(f.companySlug));
    return [...diversifyByCompany(prestigious), ...diversifyByCompany(rest)];
  }

  return diversifyByCompany(chrono);
}

/** Load role families (grouped postings) for board + public API. */
export async function loadJobFamilies(opts: FamilyQueryOpts): Promise<{
  families: JobFamily[];
  total: number;
}> {
  const db = getDb();
  const where = buildConditions(opts);

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
      durationMonths: jobs.durationMonths,
      roles: jobs.roles,
      regions: jobs.regions,
      isRemote: jobs.isRemote,
      source: jobs.source,
      postedAt: jobs.postedAt,
      firstSeenAt: jobs.firstSeenAt,
      companyName: companies.name,
      companySlug: companies.slug,
      companyAts: companies.ats,
      companyWebsiteUrl: companies.websiteUrl,
      companyCareersUrl: companies.careersUrl,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(where)
    .orderBy(
      opts.sort === "posted" ? desc(jobs.postedAt) : desc(jobs.firstSeenAt),
    );

  type Acc = FamilyAcc;

  const byFamily = new Map<string, Acc>();

  for (const row of rows) {
    const familyKey =
      row.roleFamilyId ||
      `${row.companySlug}:${row.id}`; /* pre-backfill fallback */
    let acc = byFamily.get(familyKey);
    if (!acc) {
      acc = {
        roleFamilyId: familyKey,
        titles: [],
        normalizedTitle: row.normalizedTitle || row.title,
        companyName: row.companyName,
        companySlug: row.companySlug,
        companyAts: row.companyAts,
        companyWebsiteUrl: row.companyWebsiteUrl,
        companyCareersUrl: row.companyCareersUrl,
        roles: row.roles ?? [],
        regions: row.regions ?? [],
        terms: row.terms ?? [],
        durationMonths: row.durationMonths ?? [],
        excerpt: row.excerpt,
        source: row.source,
        sortKey: 0,
        postings: [],
      };
      byFamily.set(familyKey, acc);
    }
    acc.titles.push(row.title);
    acc.roles = [...new Set([...acc.roles, ...(row.roles ?? [])])];
    acc.regions = [...new Set([...acc.regions, ...(row.regions ?? [])])];
    acc.terms = [...new Set([...acc.terms, ...(row.terms ?? [])])];
    acc.durationMonths = [
      ...new Set([...acc.durationMonths, ...(row.durationMonths ?? [])]),
    ].sort((a, b) => a - b);
    const sortTs = (
      opts.sort === "posted"
        ? row.postedAt ?? row.firstSeenAt
        : row.firstSeenAt
    ).getTime();
    if (sortTs > acc.sortKey) acc.sortKey = sortTs;
    if (!acc.excerpt && row.excerpt) acc.excerpt = row.excerpt;
    acc.postings.push({
      id: row.id,
      title: row.title,
      location: primaryLocation(row.locations, row.isRemote),
      locations: row.locations ?? [],
      postedAt: row.postedAt?.toISOString() ?? null,
      applyUrl: row.applyUrl,
      isRemote: row.isRemote,
      firstSeenAt: row.firstSeenAt.toISOString(),
    });
  }

  const sorted = orderFamilies([...byFamily.values()], opts.sort);
  const total = sorted.length;
  const page = sorted.slice(opts.offset, opts.offset + opts.limit);

  const families: JobFamily[] = page.map((acc) => ({
    roleFamilyId: acc.roleFamilyId,
    title: displayTitle(acc.titles, acc.normalizedTitle),
    company: {
      name: acc.companyName,
      slug: acc.companySlug,
      ats: acc.companyAts,
      websiteUrl: acc.companyWebsiteUrl,
      careersUrl: acc.companyCareersUrl,
    },
    roles: acc.roles,
    regions: acc.regions,
    terms: acc.terms,
    durationMonths: acc.durationMonths,
    excerpt: acc.excerpt,
    source: acc.source,
    firstSeenAt: new Date(acc.sortKey).toISOString(),
    postings: acc.postings,
  }));

  return { families, total };
}
