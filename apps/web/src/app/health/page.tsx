import { loadHealthSnapshot } from "@/lib/health-snapshot";
import { liveDumpJobs, loadJobsDump } from "@/lib/job-dump";

export const revalidate = 3600;

export default async function HealthPage() {
  // Reads the dump job's published snapshot; never queries the database.
  const snapshot = await loadHealthSnapshot();
  let error: string | null = null;
  let stats = {
    active_jobs: 0,
    total_jobs: null as number | null,
    active_companies: 0,
    total_companies: null as number | null,
    last_ok: null as string | null,
  };
  let recent: NonNullable<typeof snapshot>["recent_ingest_runs"] = [];
  let funnelTotals = { fetched: 0, title: 0, tech: 0, upserted: 0 };

  if (snapshot) {
    stats = {
      active_jobs: snapshot.stats.active_jobs,
      total_jobs: snapshot.stats.total_jobs,
      active_companies: snapshot.stats.active_companies,
      total_companies: snapshot.stats.total_companies,
      last_ok: snapshot.stats.last_successful_ingest,
    };
    recent = snapshot.recent_ingest_runs;
    funnelTotals = snapshot.funnel_totals_latest_per_company;
  } else {
    try {
      const live = liveDumpJobs(await loadJobsDump());
      stats.active_jobs = live.length;
      stats.active_companies = new Set(live.map((j) => j.company.slug)).size;
    } catch {
      error = "Health data unavailable";
    }
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
              {stats.total_jobs != null ? (
                <span className="pill">{stats.total_jobs} total seen</span>
              ) : null}
              <span className="pill">
                {stats.active_companies}
                {stats.total_companies != null ? `/${stats.total_companies}` : ""} companies
                active
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
              . Companies with 7 failed daily runs in a row are auto-disabled and retried weekly
              {stats.total_companies != null
                ? ` (${stats.total_companies - stats.active_companies} inactive)`
                : ""}
              .{" "}
              {snapshot
                ? `Snapshot from ${snapshot.generatedAt.slice(0, 16).replace("T", " ")} UTC.`
                : "No pipeline snapshot published yet; counts come from the public dump."}
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
                      <tr key={`${r.ran_at}-${i}`}>
                        <td className="mono">{r.ran_at.slice(0, 19)}Z</td>
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
