import { desc, eq, sql } from "drizzle-orm";
import { companies, ingestRuns, jobs } from "@openintern/db";
import { getDb } from "@/lib/db";
import { parseFunnel, sumFunnels } from "@/lib/ingest-health";

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
    funnel: ReturnType<typeof parseFunnel>;
  }[] = [];
  let funnelTotals = { fetched: 0, title: 0, tech: 0, upserted: 0 };

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
    const rows = await db
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
    const latestRows = await db
      .selectDistinctOn([ingestRuns.companyId], {
        company_id: ingestRuns.companyId,
        error: ingestRuns.error,
        ran_at: ingestRuns.ranAt,
      })
      .from(ingestRuns)
      .orderBy(ingestRuns.companyId, desc(ingestRuns.ranAt));

    recent = rows.map((r) => ({
      ...r,
      funnel: parseFunnel(r.error),
    }));
    funnelTotals = sumFunnels(latestRows.map((r) => parseFunnel(r.error)));

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
          Transparent pipeline status. Funnel columns show where volume drops:
          board fetch → internship title → tech pass → upsert. Failed company
          polls stay visible so the corpus stays trustworthy.
        </p>
        <p className="muted" style={{ marginTop: "0.5rem", marginBottom: 0 }}>
          Season note: live board focuses on Fall/Winter 2026–27 and Summer 2027;
          many Summer 2026 boards are already closed.
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
            <p className="muted" style={{ marginBottom: 0 }}>
              Machine-readable:{" "}
              <a href="/api/v1/health">
                <code className="mono">GET /api/v1/health</code>
              </a>
              . Companies with 24 consecutive fetch failures are auto-disabled
              ({stats.total_companies - stats.active_companies} inactive).
            </p>
          </div>

          <div className="panel">
            <h2>Latest funnel state</h2>
            <div className="meta">
              <span className="pill">fetched {funnelTotals.fetched}</span>
              <span className="pill">title {funnelTotals.title}</span>
              <span className="pill">tech {funnelTotals.tech}</span>
              <span className="pill accent">upserted {funnelTotals.upserted}</span>
            </div>
            <p className="muted" style={{ marginBottom: 0 }}>
              Summed from each company’s latest ingest run. This is a current
              per-company snapshot, not one atomic batch. Zero-match companies
              often show high fetched with title=0.
            </p>
          </div>

          <div className="panel">
            <h2>Recent ingest runs</h2>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>When</th>
                    <th>Company</th>
                    <th>Status</th>
                    <th>Fetched</th>
                    <th>Title</th>
                    <th>Tech</th>
                    <th>Upserted</th>
                    <th>Note</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="empty">
                        No ingest runs yet. Run <code className="mono">pnpm ingest</code>.
                      </td>
                    </tr>
                  ) : (
                    recent.map((r, i) => (
                      <tr key={`${r.ran_at.toISOString()}-${i}`}>
                        <td className="mono">{r.ran_at.toISOString().slice(0, 19)}Z</td>
                        <td>
                          {r.company_name ?? "—"}
                          {r.company_slug ? (
                            <div className="mono muted">{r.company_slug}</div>
                          ) : null}
                        </td>
                        <td className={r.status === "ok" ? "status-ok" : "status-error"}>
                          {r.status}
                        </td>
                        <td>{r.funnel?.fetched ?? "—"}</td>
                        <td>{r.funnel?.title ?? "—"}</td>
                        <td>{r.funnel?.tech ?? r.job_count}</td>
                        <td>{r.funnel?.upserted ?? "—"}</td>
                        <td className="mono" style={{ maxWidth: 280 }}>
                          {r.error?.startsWith("zero_match")
                            ? "zero_match"
                            : r.status === "error"
                              ? (r.error ?? "").slice(0, 80)
                              : ""}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </>
  );
}
