import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "API & dumps · OpenIntern",
  description:
    "Public OpenIntern API reference, query parameters, rate limits, and daily JSON/CSV dump downloads.",
  alternates: { canonical: "/docs" },
};

const DUMP_JSON =
  "https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.json";
const DUMP_CSV =
  "https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.csv";

export default function DocsPage() {
  return (
    <>
      <section className="hero">
        <h1>API &amp; dumps</h1>
        <p>
          OpenIntern is a free, structured corpus of tech internships. Browse the
          board with no account, query the public API, or download the daily dump
          for bulk use.
        </p>
      </section>

      <div className="panel">
        <h2>Endpoints</h2>
        <table className="table">
          <thead>
            <tr>
              <th>Method</th>
              <th>Path</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td className="mono">GET</td>
              <td className="mono">/api/v1/jobs</td>
              <td>List active internships (paginated, filterable)</td>
            </tr>
            <tr>
              <td className="mono">GET</td>
              <td className="mono">/api/v1/jobs/{"{id}"}</td>
              <td>Single job by UUID</td>
            </tr>
            <tr>
              <td className="mono">GET</td>
              <td className="mono">/api/v1/companies</td>
              <td>Company registry</td>
            </tr>
            <tr>
              <td className="mono">GET</td>
              <td className="mono">/api/v1/health</td>
              <td>Ingest pipeline health</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="panel">
        <h2>
          <code className="mono">GET /api/v1/jobs</code> query params
        </h2>
        <table className="table">
          <thead>
            <tr>
              <th>Param</th>
              <th>Notes</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td className="mono">q</td>
              <td>Title substring (case-insensitive)</td>
            </tr>
            <tr>
              <td className="mono">location</td>
              <td>Location substring</td>
            </tr>
            <tr>
              <td className="mono">company</td>
              <td>Company slug (exact)</td>
            </tr>
            <tr>
              <td className="mono">remote</td>
              <td>
                <code className="mono">true</code> or <code className="mono">1</code>
              </td>
            </tr>
            <tr>
              <td className="mono">season</td>
              <td>
                <code className="mono">winter|spring|summer|fall</code> — repeatable or
                comma-separated
              </td>
            </tr>
            <tr>
              <td className="mono">duration_months</td>
              <td>Exact duration (1–24)</td>
            </tr>
            <tr>
              <td className="mono">cohort_year</td>
              <td>Program year (e.g. 2026 from “Summer 2026”)</td>
            </tr>
            <tr>
              <td className="mono">posted_after</td>
              <td>ISO date — jobs posted on/after</td>
            </tr>
            <tr>
              <td className="mono">page</td>
              <td>Page number (default 1)</td>
            </tr>
            <tr>
              <td className="mono">limit</td>
              <td>Page size (default 25, max 100)</td>
            </tr>
          </tbody>
        </table>

        <h2 style={{ marginTop: "1.25rem" }}>Examples</h2>
        <pre className="mono" style={{ overflow: "auto", fontSize: "0.85rem", margin: 0 }}>
{`curl "https://openintern.dev/api/v1/jobs?q=software&limit=10"
curl "https://openintern.dev/api/v1/jobs?season=summer,fall&duration_months=4"
curl "https://openintern.dev/api/v1/jobs?cohort_year=2026&remote=true"
curl "https://openintern.dev/api/v1/jobs/{id}"
curl "https://openintern.dev/api/v1/companies"
curl "https://openintern.dev/api/v1/health"`}
        </pre>
      </div>

      <div className="panel">
        <h2>Rate limits &amp; bulk access</h2>
        <p style={{ color: "var(--muted)", marginTop: 0 }}>
          The hosted API is rate-limited per IP. For bulk consumers, prefer the
          daily dumps below or self-host — don’t paginate the hosted API all day.
        </p>
      </div>

      <div className="panel">
        <h2>Daily dumps</h2>
        <p style={{ color: "var(--muted)", marginTop: 0 }}>
          Stable download URLs (updated daily via GitHub Actions):
        </p>
        <ul>
          <li>
            <a href={DUMP_JSON}>
              <code className="mono">jobs.json</code>
            </a>
          </li>
          <li>
            <a href={DUMP_CSV}>
              <code className="mono">jobs.csv</code>
            </a>
          </li>
        </ul>
        <p style={{ color: "var(--muted)", marginBottom: 0 }}>
          Release tag:{" "}
          <a href="https://github.com/dnexdev/openintern/releases/tag/dump-latest">
            <code className="mono">dump-latest</code>
          </a>
        </p>
      </div>

      <div className="panel">
        <h2>Add a company</h2>
        <p style={{ color: "var(--muted)", margin: 0 }}>
          Extend the corpus with a YAML PR — see{" "}
          <a href="https://github.com/dnexdev/openintern/blob/main/CONTRIBUTING.md">
            CONTRIBUTING.md
          </a>
          . Supported ATS: Greenhouse, Lever, Ashby, Workable, SmartRecruiters.
        </p>
      </div>
    </>
  );
}
