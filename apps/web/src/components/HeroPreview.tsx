import { CompanyAvatar } from "./CompanyAvatar";
import type { JobCardData } from "./JobResults";
import { previewLocationLabel } from "@/lib/board";

/** Decorative faded preview of live listings for the full-viewport hero. */
export function HeroPreview({ jobs }: { jobs: JobCardData[] }) {
  const preview = jobs.slice(0, 6);
  if (preview.length === 0) return null;

  return (
    <div className="hero-preview" aria-hidden="true">
      <div className="hero-preview-window">
        <div className="hero-preview-chrome">
          <span />
          <span />
          <span />
          <p>Internships</p>
        </div>
        <div className="hero-preview-grid">
          {preview.map((job) => (
            <article key={job.id} className="hero-preview-card">
              <div className="hero-preview-card-top">
                <CompanyAvatar
                  name={job.companyName}
                  websiteUrl={job.companyWebsiteUrl}
                  careersUrl={job.companyCareersUrl}
                  slug={job.companySlug}
                />
                <div>
                  <p className="hero-preview-company">{job.companyName}</p>
                  <p className="hero-preview-loc">{previewLocationLabel(job)}</p>
                </div>
              </div>
              <p className="hero-preview-title">{job.title}</p>
              {(job.roles ?? []).length > 0 ? (
                <div className="hero-preview-badges">
                  {(job.roles ?? []).slice(0, 2).map((r) => (
                    <span key={r} className="badge role">
                      {r}
                    </span>
                  ))}
                </div>
              ) : null}
            </article>
          ))}
        </div>
      </div>
      <div className="hero-preview-fade" />
    </div>
  );
}
