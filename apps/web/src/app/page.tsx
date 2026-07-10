import { CurlBar } from "@/components/CurlBar";
import { HeroPreview } from "@/components/HeroPreview";
import {
  formatIngestAge,
  loadCorpusStats,
  loadHeroPreviewPool,
} from "@/lib/board";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  let previewJobs: Awaited<ReturnType<typeof loadHeroPreviewPool>>["preview"] = [];
  let total = 0;
  let dbError: string | null = null;
  let corpusStats: { activeCompanies: number; lastIngest: string | null } = {
    activeCompanies: 0,
    lastIngest: null,
  };

  try {
    const [hero, stats] = await Promise.all([loadHeroPreviewPool(), loadCorpusStats()]);
    previewJobs = hero.preview;
    total = hero.total;
    corpusStats = stats;
  } catch (err) {
    dbError = err instanceof Error ? err.message : "Database unavailable";
  }

  const ingestAge = formatIngestAge(corpusStats.lastIngest);

  return (
    <>
      <section className="hero hero-viewport">
        <div className="hero-viewport-inner">
          <div className="hero-copy">
            <h1>Stop wasting money on Intern Insider.</h1>
            <p className="hero-subhead">
              Free tech internship listings. No account. No paywall. Open source —
              with a public API and daily dumps.
            </p>
            <ul className="trust-pills" aria-label="Product highlights">
              <li>Free</li>
              <li>No account</li>
              <li>Apache-2.0</li>
              <li>Public API</li>
              <li>Daily dumps</li>
            </ul>
            {!dbError ? (
              <ul className="hero-stats">
                <li>
                  <strong>{total.toLocaleString()}</strong> active roles
                </li>
                <li>
                  <strong>{corpusStats.activeCompanies.toLocaleString()}</strong>{" "}
                  companies polled
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
            <p className="hero-season">
              Live focus: Fall/Winter 2026–27 and Summer 2027.
            </p>
            <a className="btn btn-primary hero-start" href="/jobs">
              Start
            </a>
            <CurlBar />
          </div>
          {!dbError ? <HeroPreview jobs={previewJobs} /> : null}
        </div>
      </section>

      <section className="contrast contrast-below" aria-labelledby="contrast-heading">
        <div className="panel contrast-panel">
          <h2 id="contrast-heading">Why OpenIntern</h2>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>Feature</th>
                  <th>OpenIntern</th>
                  <th>Intern Insider</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Price</td>
                  <td>Free</td>
                  <td>Paid subscription</td>
                </tr>
                <tr>
                  <td>Account to browse</td>
                  <td>No</td>
                  <td>Required / gated</td>
                </tr>
                <tr>
                  <td>Open source</td>
                  <td>Apache-2.0</td>
                  <td>Closed</td>
                </tr>
                <tr>
                  <td>Public API</td>
                  <td>Yes</td>
                  <td>No</td>
                </tr>
                <tr>
                  <td>Bulk export</td>
                  <td>Daily JSON/CSV</td>
                  <td>Locked</td>
                </tr>
                <tr>
                  <td>Apply destination</td>
                  <td>Employer site</td>
                  <td>In-platform tools</td>
                </tr>
                <tr>
                  <td>Recruiter email finder</td>
                  <td>No</td>
                  <td>Sold as a feature</td>
                </tr>
                <tr>
                  <td>AI resume builder</td>
                  <td>No</td>
                  <td>Sold as a feature</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p className="contrast-links">
            <a href="https://github.com/dnexdev/openintern">GitHub</a>
            <span aria-hidden="true">·</span>
            <a href="https://github.com/dnexdev/openintern/blob/main/LICENSE">
              Apache-2.0
            </a>
            <span aria-hidden="true">·</span>
            <a href="https://github.com/dnexdev/openintern/releases/tag/dump-latest">
              Daily dumps
            </a>
          </p>
        </div>
      </section>
    </>
  );
}
