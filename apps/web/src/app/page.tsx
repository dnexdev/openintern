import { and, desc, eq, or, sql, type SQL } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { FilterSidebar } from "@/components/FilterSidebar";
import { JobResults, type JobCardData } from "@/components/JobResults";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";

export const dynamic = "force-dynamic";

const ROLE_OPTIONS = [
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

const REGION_OPTIONS = ["remote", "us", "canada", "europe", "other"] as const;
const TERM_OPTIONS = ["summer", "fall", "winter"] as const;
const DURATION_OPTIONS = [3, 4, 6, 8, 12] as const;

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

function paramAll(sp: Record<string, string | string[] | undefined>, key: string) {
  const v = sp[key];
  if (v === undefined) return [];
  return Array.isArray(v) ? v : [v];
}

function formatIngestAge(iso: string | null): string | null {
  if (!iso) return null;
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (sec < 60) return "just now";
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: SearchParams;
}) {
  const sp = await searchParams;
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
  const sort: "first_seen" | "posted" = sortRaw === "posted" ? "posted" : "first_seen";
  const page = Math.max(1, Number(Array.isArray(sp.page) ? sp.page[0] : sp.page ?? 1));
  const limit = 25;
  const offset = (page - 1) * limit;

  let rows: Awaited<ReturnType<typeof loadJobs>>["rows"] = [];
  let total = 0;
  let dbError: string | null = null;
  let corpusStats: { activeCompanies: number; lastIngest: string | null } = {
    activeCompanies: 0,
    lastIngest: null,
  };

  try {
    const loaded = await loadJobs({
      roles,
      regions,
      terms,
      durations,
      sort,
      limit,
      offset,
    });
    rows = loaded.rows;
    total = loaded.total;
    corpusStats = await loadCorpusStats();
  } catch (err) {
    dbError = err instanceof Error ? err.message : "Database unavailable";
  }

  const totalPages = Math.max(1, Math.ceil(total / limit));

  const filterParams = new URLSearchParams();
  for (const r of roles) filterParams.append("role", r);
  for (const r of regions) filterParams.append("region", r);
  for (const t of terms) filterParams.append("term", t);
  for (const d of durations) filterParams.append("duration", String(d));
  const filterQuery = filterParams.toString();

  const hasFilters = Boolean(
    roles.length || regions.length || terms.length || durations.length,
  );

  const cardJobs: JobCardData[] = rows.map((job) => ({
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

  const ingestAge = formatIngestAge(corpusStats.lastIngest);

  return (
    <>
      <section className="hero">
        <p className="eyebrow">openintern.dev</p>
        <h1>Tech internships, open by default</h1>
        <p>
          Free structured corpus from public ATS boards. Browse with no account.
          Public API and daily dumps for builders.
        </p>
        {!dbError ? (
          <ul className="hero-stats">
            <li>
              <strong>{total.toLocaleString()}</strong>
              {hasFilters ? " matching" : " active"} roles
            </li>
            <li>
              <strong>{corpusStats.activeCompanies.toLocaleString()}</strong> companies
              polled
            </li>
            <li>
              Updated hourly
              {ingestAge ? (
                <>
                  {" "}
                  · last ingest <strong>{ingestAge}</strong>
                </>
              ) : null}
            </li>
          </ul>
        ) : null}
        <div className="hero-actions">
          <a className="btn btn-primary" href="#jobs">
            Browse jobs
          </a>
          <a className="btn" href="/docs">
            API docs
          </a>
          <a
            className="btn"
            href="https://github.com/dnexdev/openintern/releases/tag/dump-latest"
          >
            Daily dumps
          </a>
        </div>
      </section>

      <div className="layout" id="jobs">
        <FilterSidebar
          key={filterQuery || sort || "all"}
          roles={roles}
          regions={regions}
          terms={terms}
          durations={durations}
          hasFilters={hasFilters}
          sort={sort}
        />

        <section>
          {dbError ? (
            <div className="panel">
              <p className="empty">
                Database not connected yet. Set <code className="mono">DATABASE_URL</code>,
                run migrations, then <code className="mono">pnpm ingest</code>.
              </p>
              <p className="mono status-error">{dbError}</p>
            </div>
          ) : (
            <JobResults
              jobs={cardJobs}
              total={total}
              hasFilters={hasFilters}
              page={page}
              totalPages={totalPages}
              filterQuery={filterQuery}
              sort={sort}
            />
          )}
        </section>
      </div>
    </>
  );
}

async function loadCorpusStats() {
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

async function loadJobs(opts: {
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
