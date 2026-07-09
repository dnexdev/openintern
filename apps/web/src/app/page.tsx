import { and, desc, eq, ilike, or, sql, type SQL } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { auth } from "@/auth";
import { saveJob } from "@/app/actions";
import { CompanyAvatar } from "@/components/CompanyAvatar";
import { getDb } from "@/lib/db";

export const dynamic = "force-dynamic";

const TERM_OPTIONS = ["winter", "spring", "summer", "fall"] as const;
const DURATION_OPTIONS = [3, 4, 6, 8, 12] as const;

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

function param(sp: Record<string, string | string[] | undefined>, key: string) {
  const v = sp[key];
  return Array.isArray(v) ? v[0] : v;
}

function paramAll(sp: Record<string, string | string[] | undefined>, key: string) {
  const v = sp[key];
  if (v === undefined) return [];
  return Array.isArray(v) ? v : [v];
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function formatDate(d: Date | null | undefined) {
  if (!d) return "—";
  return d.toISOString().slice(0, 10);
}

function timeAgo(d: Date) {
  const sec = Math.floor((Date.now() - d.getTime()) / 1000);
  if (sec < 60) return `${sec}s ago`;
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
  const q = param(sp, "q")?.trim() || "";
  const location = param(sp, "location")?.trim() || "";
  const company = param(sp, "company")?.trim() || "";
  const remote = param(sp, "remote") === "1";
  const terms = paramAll(sp, "term").filter((t): t is (typeof TERM_OPTIONS)[number] =>
    (TERM_OPTIONS as readonly string[]).includes(t),
  );
  const durationRaw = Number(param(sp, "duration") ?? NaN);
  const duration = (DURATION_OPTIONS as readonly number[]).includes(durationRaw)
    ? durationRaw
    : null;
  const page = Math.max(1, Number(param(sp, "page") ?? 1));
  const limit = 25;
  const offset = (page - 1) * limit;

  let rows: Awaited<ReturnType<typeof loadJobs>>["rows"] = [];
  let total = 0;
  let dbError: string | null = null;

  try {
    const loaded = await loadJobs({
      q,
      location,
      company,
      remote,
      terms,
      duration,
      limit,
      offset,
    });
    rows = loaded.rows;
    total = loaded.total;
  } catch (err) {
    dbError = err instanceof Error ? err.message : "Database unavailable";
  }

  const session = await auth().catch(() => null);
  const totalPages = Math.max(1, Math.ceil(total / limit));

  const pageHref = (n: number) => {
    const params = new URLSearchParams({
      ...(q ? { q } : {}),
      ...(location ? { location } : {}),
      ...(company ? { company } : {}),
      ...(remote ? { remote: "1" } : {}),
      ...(duration ? { duration: String(duration) } : {}),
      page: String(n),
    });
    for (const t of terms) params.append("term", t);
    return `/?${params.toString()}`;
  };

  const hasFilters = Boolean(
    q || location || company || remote || terms.length > 0 || duration,
  );

  const pageWindow: number[] = [];
  for (let n = Math.max(1, page - 2); n <= Math.min(totalPages, page + 2); n++) {
    pageWindow.push(n);
  }

  return (
    <>
      <section className="hero">
        <h1>Tech internships, open by default</h1>
        <p>
          Free structured corpus from Greenhouse, Lever, Ashby, Workable, and
          SmartRecruiters. Browse with no account. Public API and daily dumps
          for everyone else.
        </p>
      </section>

      <div className="layout">
        <aside className="sidebar">
          <h2>Filters</h2>
          <form method="get" action="/">
            <div className="field">
              <label htmlFor="q">Title</label>
              <input
                id="q"
                type="text"
                name="q"
                placeholder="e.g. software intern"
                defaultValue={q}
              />
            </div>
            <div className="field">
              <label htmlFor="location">Location</label>
              <input
                id="location"
                type="text"
                name="location"
                placeholder="e.g. Remote, NYC"
                defaultValue={location}
              />
            </div>
            <div className="field">
              <label htmlFor="company">Company slug</label>
              <input
                id="company"
                type="text"
                name="company"
                placeholder="e.g. stripe"
                defaultValue={company}
              />
            </div>
            <div className="field">
              <span className="field-label">Term</span>
              {TERM_OPTIONS.map((t) => (
                <label key={t} className="checkbox">
                  <input
                    type="checkbox"
                    name="term"
                    value={t}
                    defaultChecked={terms.includes(t)}
                  />
                  {capitalize(t)}
                </label>
              ))}
            </div>
            <div className="field">
              <label htmlFor="duration">Duration</label>
              <select id="duration" name="duration" defaultValue={duration ?? ""}>
                <option value="">Any</option>
                {DURATION_OPTIONS.map((m) => (
                  <option key={m} value={m}>
                    {m} months{m === 3 ? " (Summer)" : ""}
                  </option>
                ))}
              </select>
            </div>
            <label className="checkbox">
              <input type="checkbox" name="remote" value="1" defaultChecked={remote} />
              Remote only
            </label>
            <div className="sidebar-actions">
              <button className="btn btn-primary" type="submit">
                Search
              </button>
              {hasFilters ? (
                <a className="btn" href="/">
                  Clear
                </a>
              ) : null}
            </div>
          </form>
        </aside>

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
            <>
              <div className="results-header">
                <span>
                  <strong>{total}</strong> active role{total === 1 ? "" : "s"}
                  {hasFilters ? " matching filters" : ""}
                </span>
                <span>
                  Page {page} of {totalPages}
                </span>
              </div>

              <div className="job-list">
                {rows.length === 0 ? (
                  <p className="empty">No active internships matched. Try clearing filters or run ingest.</p>
                ) : (
                  rows.map((job) => (
                    <article key={job.id} className="job-card">
                      <CompanyAvatar
                        name={job.companyName}
                        websiteUrl={job.companyWebsiteUrl}
                        careersUrl={job.companyCareersUrl}
                      />
                      <div className="job-card-body">
                        <h2>
                          <a href={job.applyUrl} target="_blank" rel="noreferrer">
                            {job.title}
                          </a>
                        </h2>
                        <div className="job-company-line">
                          {job.companyName} · {(job.locations ?? []).join(" · ") || "Location n/a"}
                        </div>
                        <div className="meta">
                          {job.isRemote ? <span className="badge remote">Remote</span> : null}
                          {(job.terms ?? []).map((t) => (
                            <span key={t} className="badge remote">
                              {capitalize(t)}
                            </span>
                          ))}
                          {job.durationMonths ? (
                            <span className="badge">{job.durationMonths} mo</span>
                          ) : null}
                          <span className="badge source">{job.source}</span>
                          <span title={job.firstSeenAt.toISOString()}>
                            first seen {timeAgo(job.firstSeenAt)}
                          </span>
                          <span>posted {formatDate(job.postedAt)}</span>
                        </div>
                        {job.excerpt ? <p className="excerpt">{job.excerpt}</p> : null}
                        <div className="job-actions">
                          <a
                            className="btn btn-primary btn-sm"
                            href={job.applyUrl}
                            target="_blank"
                            rel="noreferrer"
                          >
                            Apply
                          </a>
                          {session?.user ? (
                            <form action={saveJob.bind(null, job.id)}>
                              <button className="btn btn-sm" type="submit">
                                Save
                              </button>
                            </form>
                          ) : null}
                        </div>
                      </div>
                    </article>
                  ))
                )}
              </div>

              {totalPages > 1 ? (
                <nav className="pagination" aria-label="Pagination">
                  {page > 1 ? (
                    <a className="btn btn-sm" href={pageHref(page - 1)}>
                      ‹ Prev
                    </a>
                  ) : null}
                  {pageWindow.map((n) => (
                    <a
                      key={n}
                      className={n === page ? "btn btn-sm btn-primary" : "btn btn-sm"}
                      href={pageHref(n)}
                    >
                      {n}
                    </a>
                  ))}
                  {page < totalPages ? (
                    <a className="btn btn-sm" href={pageHref(page + 1)}>
                      Next ›
                    </a>
                  ) : null}
                </nav>
              ) : null}
            </>
          )}
        </section>
      </div>
    </>
  );
}

async function loadJobs(opts: {
  q: string;
  location: string;
  company: string;
  remote: boolean;
  terms: string[];
  duration: number | null;
  limit: number;
  offset: number;
}) {
  const db = getDb();
  const conditions: (SQL | undefined)[] = [eq(jobs.isActive, true)];
  if (opts.q) conditions.push(ilike(jobs.title, `%${opts.q}%`));
  if (opts.company) conditions.push(eq(companies.slug, opts.company));
  if (opts.remote) conditions.push(eq(jobs.isRemote, true));
  if (opts.terms.length > 0) {
    conditions.push(
      or(
        ...opts.terms.map(
          (t) => sql`${jobs.terms} @> ${JSON.stringify([t])}::jsonb`,
        ),
      ),
    );
  }
  if (opts.duration) conditions.push(eq(jobs.durationMonths, opts.duration));
  if (opts.location) {
    conditions.push(
      sql`exists (
        select 1 from jsonb_array_elements_text(${jobs.locations}) loc
        where loc ilike ${"%" + opts.location + "%"}
      )`,
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
      durationMonths: jobs.durationMonths,
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
    .orderBy(desc(jobs.firstSeenAt))
    .limit(opts.limit)
    .offset(opts.offset);

  return { rows, total: countRow?.count ?? 0 };
}
