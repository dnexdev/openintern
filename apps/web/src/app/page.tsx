import { CurlBar } from "@/components/CurlBar";
import { HeroPreview } from "@/components/HeroPreview";
import {
  formatIngestAge,
  loadCorpusStats,
  loadHeroPreviewPool,
} from "@/lib/board";
import Link from "next/link";

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
            <h1>Tech internships, open by default</h1>
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
              Live focus: Fall/Winter 2026–27 and Summer 2027.{" "}
              <Link href="/vs/intern-insider">Compare to Intern Insider →</Link>
            </p>
            <a className="btn btn-primary hero-start" href="/jobs">
              Start
            </a>
            <CurlBar />
          </div>
          {!dbError ? <HeroPreview jobs={previewJobs} /> : null}
        </div>
      </section>
    </>
  );
}
