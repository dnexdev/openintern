import { desc, eq, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";

export const dynamic = "force-dynamic";

export default async function HealthPage() {
  let error: string | null = null;
  let stats = {
    active_jobs: 0,
    total_jobs: 0,
    active_companies: 0,
    total_companies: 0,
    last_ok: null as string | null,
  };
  let recent: {
    status: string;
    job_count: number;
    error: string | null;
    ran_at: Date;
    company_slug: string | null;
    company_name: string | null;
  }[] = [];

  try {
    const db = getDb();
    const [jobStats] = await db
      .select({
        active_jobs: sql<number>`count(*) filter (where ${jobs.isActive})::int`,
        total_jobs: sql<number>`count(*)::int`,
      })
      .from(jobs);
    const [companyStats] = await db
      .select({
        active_companies: sql<number>`count(*) filter (where ${companies.active})::int`,
        total_companies: sql<number>`count(*)::int`,
      })
      .from(companies);
    const [lastOk] = await db
      .select({ ran_at: ingestRuns.ranAt })
      .from(ingestRuns)
      .where(eq(ingestRuns.status, "ok"))
      .orderBy(desc(ingestRuns.ranAt))
      .limit(1);
    recent = await db
      .select({
        status: ingestRuns.status,
        job_count: ingestRuns.jobCount,
        error: ingestRuns.error,
        ran_at: ingestRuns.ranAt,
        company_slug: companies.slug,
        company_name: companies.name,
      })
      .from(ingestRuns)
      .leftJoin(companies, eq(ingestRuns.companyId, companies.id))
      .orderBy(desc(ingestRuns.ranAt))
      .limit(100);

    stats = {
      active_jobs: jobStats?.active_jobs ?? 0,
      total_jobs: jobStats?.total_jobs ?? 0,
      active_companies: companyStats?.active_companies ?? 0,
      total_companies: companyStats?.total_companies ?? 0,
      last_ok: lastOk?.ran_at?.toISOString() ?? null,
    };
  } catch (err) {
    error = err instanceof Error ? err.message : "Database unavailable";
  }

  return (
    <>
      <section className="hero">
        <h1>Ingest health</h1>
        <p>
          Transparent pipeline status. Failed company polls stay visible so the
          corpus stays trustworthy.
        </p>
      </section>

      {error ? (
        <div className="panel">
          <p className="mono status-error">{error}</p>
        </div>
      ) : (
        <>
          <div className="panel">
            <h2>Corpus</h2>
            <div className="meta">
              <span className="pill accent">{stats.active_jobs} active jobs</span>
              <span className="pill">{stats.total_jobs} total seen</span>
              <span className="pill">
                {stats.active_companies}/{stats.total_companies} companies active
              </span>
              <span className="pill">
                last ok: {stats.last_ok ?? "never"}
              </span>
            </div>
            <p style={{ color: "var(--muted)", marginBottom: 0 }}>
              Machine-readable:{" "}
              <a href="/api/v1/health">
                <code className="mono">GET /api/v1/health</code>
              </a>
            </p>
          </div>

          <div className="panel">
            <h2>Recent ingest runs</h2>
            <table className="table">
              <thead>
                <tr>
                  <th>When</th>
                  <th>Company</th>
                  <th>Status</th>
                  <th>Jobs</th>
                  <th>Error</th>
                </tr>
              </thead>
              <tbody>
                {recent.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      No ingest runs yet. Run <code className="mono">pnpm ingest</code>.
                    </td>
                  </tr>
                ) : (
                  recent.map((r, i) => (
                    <tr key={`${r.ran_at.toISOString()}-${i}`}>
                      <td className="mono">{r.ran_at.toISOString()}</td>
                      <td>
                        {r.company_name ?? "—"}
                        {r.company_slug ? (
                          <div className="mono" style={{ color: "var(--muted)" }}>
                            {r.company_slug}
                          </div>
                        ) : null}
                      </td>
                      <td className={r.status === "ok" ? "status-ok" : "status-error"}>
                        {r.status}
                      </td>
                      <td>{r.job_count}</td>
                      <td className="mono" style={{ maxWidth: 320 }}>
                        {r.error ?? ""}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </>
      )}
    </>
  );
}
