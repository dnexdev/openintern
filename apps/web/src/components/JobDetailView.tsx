import Link from "next/link";
import { AppliedToggle } from "@/components/AppliedToggle";
import { CompanyAvatar } from "@/components/CompanyAvatar";
import { jobPath } from "@/lib/job-slug";
import type { JobDetailRow } from "@/lib/load-job";
import { reportIssueUrl } from "@/lib/report-issue";

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function formatDate(d: Date | null | undefined) {
  if (!d) return "—";
  return d.toISOString().slice(0, 10);
}

export function JobDetailView({ job }: { job: JobDetailRow }) {
  const durations = [
    ...new Set((job.durationMonths ?? []).filter((m) => [3, 4, 6, 8, 12].includes(m))),
  ].sort((a, b) => a - b);

  const pagePath = jobPath(job.companySlug, job.title, job.id);
  const pageUrl = `https://openintern.dev${pagePath}`;

  return (
    <>
      <p className="back-link">
        <Link href="/jobs">← All internships</Link>
      </p>

      <article className="panel job-detail">
        <div className="job-detail-header">
          <CompanyAvatar
            name={job.companyName}
            websiteUrl={job.companyWebsiteUrl}
            careersUrl={job.companyCareersUrl}
            slug={job.companySlug}
          />
          <div className="job-card-body">
            <h1 className="job-detail-title">{job.title}</h1>
            <div className="job-company-line">
              {job.companyName} · {(job.locations ?? []).join(" · ") || "Location n/a"}
            </div>
            <div className="meta">
              {(job.roles ?? []).map((r) => (
                <span key={r} className="badge role">
                  {r}
                </span>
              ))}
              {(job.regions ?? []).map((r) => (
                <span key={r} className="badge region">
                  {r === "europe" ? "UK/Europe" : capitalize(r)}
                </span>
              ))}
              {(job.termYears ?? []).length > 0
                ? (job.termYears ?? []).map((ty) => (
                    <span key={`${ty.term}-${ty.year}`} className="badge term">
                      {capitalize(ty.term)} {ty.year}
                    </span>
                  ))
                : (job.terms ?? []).map((t) => (
                    <span key={t} className="badge term">
                      {capitalize(t)}
                    </span>
                  ))}
              {durations.length > 0 ? (
                <span className="badge duration">{durations.join("/")} mo</span>
              ) : null}
            </div>
            <div className="meta-secondary">
              <span className="badge source">{job.source}</span>
              <span>posted {formatDate(job.postedAt)}</span>
              <span>first seen {formatDate(job.firstSeenAt)}</span>
            </div>
          </div>
        </div>

        {job.excerpt ? (
          <p className="excerpt job-detail-excerpt">{job.excerpt}</p>
        ) : (
          <p className="empty job-detail-empty">
            No description excerpt available. Open the employer posting for full details.
          </p>
        )}

        <div className="job-actions job-detail-actions">
          <a
            className="btn btn-primary"
            href={job.applyUrl}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Apply for ${job.title} at ${job.companyName} on the employer site (opens in a new tab)`}
          >
            Apply on employer site
          </a>
          <AppliedToggle jobId={job.id} jobTitle={job.title} />
          <a
            className="btn btn-sm report-issue-link"
            href={reportIssueUrl({
              jobId: job.id,
              title: job.title,
              companyName: job.companyName,
              applyUrl: job.applyUrl,
              pageUrl,
            })}
            target="_blank"
            rel="noopener noreferrer"
          >
            Report issue
          </a>
        </div>
      </article>
    </>
  );
}
