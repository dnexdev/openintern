import { and, asc, desc, eq, ilike, inArray, or, sql, type SQL } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";
import type { FamilySort } from "@/lib/job-families";

/** Lightweight card shape used by the landing hero preview. */
export type JobCardData = {
  id: string;
  title: string;
  locations: string[] | null;
  applyUrl: string;
  excerpt: string | null;
  terms: string[] | null;
  termYears: { term: string; year: number }[] | null;
  durationMonths: number[] | null;
  roles: string[] | null;
  regions: string[] | null;
  isRemote: boolean;
  source: string;
  postedAt: string | null;
  firstSeenAt: string;
  companyName: string;
  companySlug?: string | null;
  companyWebsiteUrl: string | null;
  companyCareersUrl: string | null;
};

export const ROLE_OPTIONS = [
  "software",
  "backend",
  "frontend",
  "fullstack",
  "data",
  "ml",
  "mobile",
  "security",
  "devops",
  "hardware",
  "quant",
  "product",
  "research",
] as const;

export const REGION_OPTIONS = ["remote", "us", "canada", "europe", "other"] as const;
export const TERM_OPTIONS = ["summer", "fall", "winter"] as const;
export const DURATION_OPTIONS = [3, 4, 6, 8, 12] as const;

export type SearchParams = Promise<Record<string, string | string[] | undefined>>;

export function paramAll(
  sp: Record<string, string | string[] | undefined>,
  key: string,
) {
  const v = sp[key];
  if (v === undefined) return [];
  return Array.isArray(v) ? v : [v];
}

export function formatIngestAge(iso: string | null): string | null {
  if (!iso) return null;
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (sec < 60) return "just now";
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

export function toJobCards(
  rows: Awaited<ReturnType<typeof loadJobs>>["rows"],
): JobCardData[] {
  return rows.map((job) => ({
    id: job.id,
    title: job.title,
    locations: job.locations,
    applyUrl: job.applyUrl,
    excerpt: job.excerpt,
    terms: job.terms,
    termYears: job.termYears,
    durationMonths: job.durationMonths,
    roles: job.roles,
    regions: job.regions,
    isRemote: job.isRemote,
    source: job.source,
    postedAt: job.postedAt?.toISOString() ?? null,
    firstSeenAt: job.firstSeenAt.toISOString(),
    companyName: job.companyName,
    companySlug: job.companySlug,
    companyWebsiteUrl: job.companyWebsiteUrl,
    companyCareersUrl: job.companyCareersUrl,
  }));
}

/** US cities preferred for the landing hero location labels. */
const PREVIEW_US_CITY_LABELS = [
  "San Francisco",
  "New York",
  "Seattle",
  "Austin",
  "Boston",
  "Los Angeles",
  "Chicago",
  "Denver",
  "Portland",
  "San Jose",
  "Palo Alto",
  "Mountain View",
  "Cupertino",
  "Redmond",
  "Menlo Park",
  "Washington",
] as const;

const PREVIEW_OTHER_CITY_LABELS = [
  "Toronto",
  "Vancouver",
  "Montreal",
  "Ottawa",
  "Waterloo",
  "Calgary",
] as const;

const PREVIEW_CITY_LABELS = [
  ...PREVIEW_US_CITY_LABELS,
  ...PREVIEW_OTHER_CITY_LABELS,
] as const;

/**
 * Fixed shortlist for the hero preview — active registry companies only.
 * Backups fill a slot if a primary has no live internship right now.
 */
const HERO_PREVIEW_SLUGS = [
  "openai",
  "cloudflare",
  "notion",
  "palantir",
  "figma",
  "databricks",
] as const;

const HERO_PREVIEW_BACKUP_SLUGS = [
  "anthropic",
  "airbnb",
  "stripe",
  "anduril",
  "scale-ai",
  "jump-trading",
] as const;

function locationBlob(job: JobCardData): string {
  return [...(job.locations ?? []), ...(job.isRemote ? ["Remote"] : [])]
    .join(" ")
    .toLowerCase();
}

/** Prefer US postings so the hero doesn't lead with Bengaluru / overseas offices. */
function previewLocationScore(job: JobCardData): number {
  const blob = locationBlob(job);
  for (const city of PREVIEW_US_CITY_LABELS) {
    if (blob.includes(city.toLowerCase())) return 4;
  }
  if (job.regions?.includes("us")) return 3;
  if (job.isRemote || /\bremote\b/i.test(blob)) return 2;
  for (const city of PREVIEW_OTHER_CITY_LABELS) {
    if (blob.includes(city.toLowerCase())) return 1;
  }
  return 0;
}

/** Prefer a recognizable Western city label from a job's location strings. */
export function previewLocationLabel(job: JobCardData): string {
  const blob = locationBlob(job);
  for (const city of PREVIEW_CITY_LABELS) {
    if (blob.includes(city.toLowerCase())) return city;
  }
  if (job.isRemote || /\bremote\b/i.test(blob)) return "Remote";
  const first = (job.locations ?? [])[0]?.trim();
  return first || "Remote";
}

const jobSelect = {
  id: jobs.id,
  title: jobs.title,
  locations: jobs.locations,
  applyUrl: jobs.applyUrl,
  excerpt: jobs.excerpt,
  terms: jobs.terms,
  termYears: jobs.termYears,
  durationMonths: jobs.durationMonths,
  roles: jobs.roles,
  regions: jobs.regions,
  isRemote: jobs.isRemote,
  source: jobs.source,
  postedAt: jobs.postedAt,
  firstSeenAt: jobs.firstSeenAt,
  companyName: companies.name,
  companySlug: companies.slug,
  companyWebsiteUrl: companies.websiteUrl,
  companyCareersUrl: companies.careersUrl,
};

/** One real live job each from a fixed prestige shortlist (no fake listings). */
export async function loadHeroPreviewPool(): Promise<{
  preview: JobCardData[];
  total: number;
}> {
  const db = getDb();
  const slugs = [...HERO_PREVIEW_SLUGS, ...HERO_PREVIEW_BACKUP_SLUGS];

  const [countRow] = await db
    .select({ count: sql<number>`count(*)::int` })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(and(eq(jobs.isActive, true), freshnessSql()));

  const rows = await db
    .select(jobSelect)
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(
      and(
        eq(jobs.isActive, true),
        freshnessSql(),
        inArray(companies.slug, slugs),
      ),
    )
    .orderBy(desc(jobs.firstSeenAt));

  const bySlug = new Map<string, JobCardData>();
  for (const job of toJobCards(rows)) {
    const slug = job.companySlug;
    if (!slug) continue;
    const prev = bySlug.get(slug);
    // Prefer US locations; among equal scores, keep the newer row (query order).
    if (!prev || previewLocationScore(job) > previewLocationScore(prev)) {
      bySlug.set(slug, job);
    }
  }

  const preview: JobCardData[] = [];
  for (const slug of HERO_PREVIEW_SLUGS) {
    const job = bySlug.get(slug);
    if (job) preview.push(job);
  }
  for (const slug of HERO_PREVIEW_BACKUP_SLUGS) {
    if (preview.length >= 6) break;
    const job = bySlug.get(slug);
    if (job) preview.push(job);
  }

  return { preview: preview.slice(0, 6), total: countRow?.count ?? 0 };
}

export async function loadCorpusStats() {
  const db = getDb();
  const [companyRow] = await db
    .select({
      activeCompanies: sql<number>`count(*) filter (where ${companies.active})::int`,
    })
    .from(companies);
  const [lastOk] = await db
    .select({ ranAt: ingestRuns.ranAt })
    .from(ingestRuns)
    .where(eq(ingestRuns.status, "ok"))
    .orderBy(desc(ingestRuns.ranAt))
    .limit(1);
  return {
    activeCompanies: companyRow?.activeCompanies ?? 0,
    lastIngest: lastOk?.ranAt?.toISOString() ?? null,
  };
}

export async function loadCompanyOptions() {
  const db = getDb();
  return db
    .select({ slug: companies.slug, name: companies.name })
    .from(companies)
    .where(eq(companies.active, true))
    .orderBy(asc(companies.name));
}

export async function loadJobs(opts: {
  query: string;
  company: string;
  roles: string[];
  regions: string[];
  terms: string[];
  durations: number[];
  sort: "first_seen" | "posted";
  limit: number;
  offset: number;
}) {
  const db = getDb();
  const conditions: (SQL | undefined)[] = [eq(jobs.isActive, true), freshnessSql()];

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

  const where = and(...conditions);

  const [countRow] = await db
    .select({ count: sql<number>`count(*)::int` })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(where);

  const rows = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      locations: jobs.locations,
      applyUrl: jobs.applyUrl,
      excerpt: jobs.excerpt,
      terms: jobs.terms,
      termYears: jobs.termYears,
      durationMonths: jobs.durationMonths,
      roles: jobs.roles,
      regions: jobs.regions,
      isRemote: jobs.isRemote,
      source: jobs.source,
      postedAt: jobs.postedAt,
      firstSeenAt: jobs.firstSeenAt,
      companyName: companies.name,
      companySlug: companies.slug,
      companyWebsiteUrl: companies.websiteUrl,
      companyCareersUrl: companies.careersUrl,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(where)
    .orderBy(opts.sort === "posted" ? desc(jobs.postedAt) : desc(jobs.firstSeenAt))
    .limit(opts.limit)
    .offset(opts.offset);

  return { rows, total: countRow?.count ?? 0 };
}

export function parseBoardSearchParams(
  sp: Record<string, string | string[] | undefined>,
) {
  const query = (Array.isArray(sp.q) ? sp.q[0] : sp.q)?.trim().slice(0, 100) ?? "";
  const company =
    (Array.isArray(sp.company) ? sp.company[0] : sp.company)?.trim() ?? "";
  const roles = paramAll(sp, "role").filter((r): r is (typeof ROLE_OPTIONS)[number] =>
    (ROLE_OPTIONS as readonly string[]).includes(r),
  );
  const regions = paramAll(sp, "region").filter((r) =>
    (REGION_OPTIONS as readonly string[]).includes(r),
  );
  const terms = paramAll(sp, "term").filter((t): t is (typeof TERM_OPTIONS)[number] =>
    (TERM_OPTIONS as readonly string[]).includes(t),
  );
  const durations = paramAll(sp, "duration")
    .map(Number)
    .filter((n): n is (typeof DURATION_OPTIONS)[number] =>
      (DURATION_OPTIONS as readonly number[]).includes(n),
    );
  const sortRaw = Array.isArray(sp.sort) ? sp.sort[0] : sp.sort;
  const sort: FamilySort =
    sortRaw === "posted"
      ? "posted"
      : sortRaw === "prestige"
        ? "prestige"
        : "first_seen";
  const page = Math.max(1, Number(Array.isArray(sp.page) ? sp.page[0] : sp.page ?? 1));
  return { query, company, roles, regions, terms, durations, sort, page };
}
