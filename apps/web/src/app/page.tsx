import { and, desc, eq, ilike, sql } from "drizzle-orm";
import { companies, jobs } from "@openintern/db";
import { auth } from "@/auth";
import { saveJob } from "@/app/actions";
import { getDb } from "@/lib/db";

export const dynamic = "force-dynamic";

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

function param(sp: Record<string, string | string[] | undefined>, key: string) {
  const v = sp[key];
  return Array.isArray(v) ? v[0] : v;
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
  const page = Math.max(1, Number(param(sp, "page") ?? 1));
  const limit = 25;
  const offset = (page - 1) * limit;

  let rows: Awaited<ReturnType<typeof loadJobs>>["rows"] = [];
  let total = 0;
  let dbError: string | null = null;

  try {
    const loaded = await loadJobs({ q, location, company, remote, limit, offset });
    rows = loaded.rows;
    total = loaded.total;
  } catch (err) {
    dbError = err instanceof Error ? err.message : "Database unavailable";
  }

  const session = await auth().catch(() => null);
  const totalPages = Math.max(1, Math.ceil(total / limit));

  return (
    <>
      <section className="hero">
        <h1>Tech internships, open by default</h1>
        <p>
          Free structured corpus from Greenhouse, Lever, and Ashby. Browse with
          no account. Public API and daily dumps for everyone else.
        </p>
      </section>

      <form className="filters" method="get" action="/">
        <input
          type="text"
          name="q"
          placeholder="Search titles (e.g. software intern)"
          defaultValue={q}
        />
        <input
          type="text"
          name="location"
          placeholder="Location"
          defaultValue={location}
        />
        <input
          type="text"
          name="company"
          placeholder="Company slug"
          defaultValue={company}
        />
        <label className="checkbox">
          <input type="checkbox" name="remote" value="1" defaultChecked={remote} />
          Remote
        </label>
        <button className="btn btn-primary" type="submit">
          Search
        </button>
      </form>

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
          <div className="pager">
            <span>
              {total} active role{total === 1 ? "" : "s"}
              {q || location || company || remote ? " matching filters" : ""}
            </span>
            <span>
              Page {page} / {totalPages}
            </span>
          </div>

          <div className="job-list">
            {rows.length === 0 ? (
              <p className="empty">No active internships matched. Try clearing filters or run ingest.</p>
            ) : (
              rows.map((job) => (
                <article key={job.id} className="job-card">
                  <h2>
                    <a href={job.applyUrl} target="_blank" rel="noreferrer">
                      {job.title}
                    </a>
                  </h2>
                  <div className="meta">
                    <span>{job.companyName}</span>
                    <span>{(job.locations ?? []).join(" · ") || "Location n/a"}</span>
                    {job.isRemote ? <span className="pill accent">Remote</span> : null}
                    <span className="pill">{job.source}</span>
                    <span title={job.firstSeenAt.toISOString()}>
                      first seen {timeAgo(job.firstSeenAt)}
                    </span>
                    <span>posted {formatDate(job.postedAt)}</span>
                  </div>
                  {job.excerpt ? (
                    <p style={{ margin: 0, color: "var(--muted)", fontSize: "0.92rem" }}>
                      {job.excerpt}
                    </p>
                  ) : null}
                  <div className="job-actions">
                    <a className="btn btn-primary" href={job.applyUrl} target="_blank" rel="noreferrer">
                      Apply
                    </a>
                    {session?.user ? (
                      <form action={saveJob.bind(null, job.id)}>
                        <button className="btn" type="submit">
                          Save
                        </button>
                      </form>
                    ) : null}
                  </div>
                </article>
              ))
            )}
          </div>

          <div className="pager">
            {page > 1 ? (
              <a
                className="btn"
                href={`/?${new URLSearchParams({
                  ...(q ? { q } : {}),
                  ...(location ? { location } : {}),
                  ...(company ? { company } : {}),
                  ...(remote ? { remote: "1" } : {}),
                  page: String(page - 1),
                }).toString()}`}
              >
                Previous
              </a>
            ) : (
              <span />
            )}
            {page < totalPages ? (
              <a
                className="btn"
                href={`/?${new URLSearchParams({
                  ...(q ? { q } : {}),
                  ...(location ? { location } : {}),
                  ...(company ? { company } : {}),
                  ...(remote ? { remote: "1" } : {}),
                  page: String(page + 1),
                }).toString()}`}
              >
                Next
              </a>
            ) : (
              <span />
            )}
          </div>
        </>
      )}
    </>
  );
}

async function loadJobs(opts: {
  q: string;
  location: string;
  company: string;
  remote: boolean;
  limit: number;
  offset: number;
}) {
  const db = getDb();
  const conditions = [eq(jobs.isActive, true)];
  if (opts.q) conditions.push(ilike(jobs.title, `%${opts.q}%`));
  if (opts.company) conditions.push(eq(companies.slug, opts.company));
  if (opts.remote) conditions.push(eq(jobs.isRemote, true));
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
      isRemote: jobs.isRemote,
      source: jobs.source,
      postedAt: jobs.postedAt,
      firstSeenAt: jobs.firstSeenAt,
      companyName: companies.name,
      companySlug: companies.slug,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(where)
    .orderBy(desc(jobs.firstSeenAt))
    .limit(opts.limit)
    .offset(opts.offset);

  return { rows, total: countRow?.count ?? 0 };
}
