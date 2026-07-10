"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { CompanyAvatar } from "./CompanyAvatar";
import {
  AppliedToggle,
  HideAppliedToggle,
  useAppliedIds,
  useHideApplied,
} from "./AppliedToggle";

export type JobCardData = {
  id: string;
  title: string;
  locations: string[] | null;
  applyUrl: string;
  excerpt: string | null;
  terms: string[] | null;
  termYears: { term: string; year: number }[] | null;
  durationMonths: number[] | null;
  roles: string[] | null;
  regions: string[] | null;
  isRemote: boolean;
  source: string;
  postedAt: string | null;
  firstSeenAt: string;
  companyName: string;
  companySlug?: string | null;
  companyWebsiteUrl: string | null;
  companyCareersUrl: string | null;
};

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function timeAgo(iso: string) {
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (sec < 60) return `${sec}s ago`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

function JobBadges({ job }: { job: JobCardData }) {
  const badges: { key: string; className: string; label: string }[] = [];
  for (const r of (job.roles ?? []).slice(0, 2)) {
    badges.push({ key: `role-${r}`, className: "badge role", label: r });
  }
  const terms =
    (job.termYears ?? []).length > 0
      ? (job.termYears ?? []).slice(0, 1).map((ty) => ({
          key: `ty-${ty.term}-${ty.year}`,
          label: `${capitalize(ty.term)} ${ty.year}`,
        }))
      : (job.terms ?? []).slice(0, 1).map((t) => ({
          key: `t-${t}`,
          label: capitalize(t),
        }));
  for (const t of terms) {
    badges.push({ key: t.key, className: "badge term", label: t.label });
  }
  for (const r of (job.regions ?? []).slice(0, 1)) {
    badges.push({
      key: `region-${r}`,
      className: "badge region",
      label: r === "europe" ? "UK/Europe" : capitalize(r),
    });
  }

  const shown = badges.slice(0, 4);

  return (
    <div className="meta">
      {shown.map((b) => (
        <span key={b.key} className={b.className}>
          {b.label}
        </span>
      ))}
    </div>
  );
}

export function JobResults({
  jobs,
  total,
  hasFilters,
  page,
  totalPages,
  filterQuery,
  sort,
  tier1Slugs,
}: {
  jobs: JobCardData[];
  total: number;
  hasFilters: boolean;
  page: number;
  totalPages: number;
  filterQuery: string;
  sort: "first_seen" | "posted";
  tier1Slugs: string[];
}) {
  const router = useRouter();
  const [hideApplied, setHideApplied] = useHideApplied();
  const applied = useAppliedIds();
  const tier1 = new Set(tier1Slugs);
  const visible = hideApplied
    ? jobs.filter((j) => !applied.includes(j.id))
    : jobs;

  const pageHref = (n: number) => {
    const params = new URLSearchParams(filterQuery);
    if (sort !== "first_seen") params.set("sort", sort);
    params.set("page", String(n));
    return `/jobs?${params.toString()}`;
  };

  const pageWindow: number[] = [];
  for (let n = Math.max(1, page - 2); n <= Math.min(totalPages, page + 2); n++) {
    pageWindow.push(n);
  }

  function onSortChange(value: string) {
    const params = new URLSearchParams(filterQuery);
    if (value !== "first_seen") params.set("sort", value);
    const qs = params.toString();
    router.push(qs ? `/jobs?${qs}` : "/jobs", { scroll: false });
  }

  return (
    <>
      <div className="results-header">
        <span>
          <strong>{total}</strong> active role{total === 1 ? "" : "s"}
          {hasFilters ? " matching filters" : ""}
          {hideApplied && jobs.length !== visible.length
            ? ` · showing ${visible.length} of ${jobs.length} on this page`
            : ""}
        </span>
        <span>
          Page {page} of {totalPages}
        </span>
      </div>

      <div className="results-toolbar">
        <HideAppliedToggle checked={hideApplied} onChange={setHideApplied} />
        <label className="sort-control">
          Sort
          <select
            value={sort}
            onChange={(e) => onSortChange(e.target.value)}
            aria-label="Sort jobs"
          >
            <option value="first_seen">Newest first seen</option>
            <option value="posted">Newest posted</option>
          </select>
        </label>
      </div>

      <div className="job-grid">
        {jobs.length === 0 ? (
          <p className="empty">
            No active internships matched.{" "}
            {hasFilters ? <Link href="/jobs">Clear filters</Link> : "Try running ingest."}
          </p>
        ) : visible.length === 0 ? (
          <p className="empty">
            All matching roles are marked applied. Uncheck “Hide applied” to see them.
          </p>
        ) : (
          visible.map((job) => {
            const isApplied = applied.includes(job.id);
            const isTier1 = job.companySlug ? tier1.has(job.companySlug) : false;
            const location =
              (job.locations ?? []).slice(0, 2).join(" · ") || "Location n/a";
            return (
              <article
                key={job.id}
                className={`job-card${isApplied ? " is-applied" : ""}${isTier1 ? " is-tier-1" : ""}`}
                data-job-id={job.id}
              >
                <div className="job-card-top">
                  <CompanyAvatar
                    name={job.companyName}
                    websiteUrl={job.companyWebsiteUrl}
                    careersUrl={job.companyCareersUrl}
                    slug={job.companySlug}
                  />
                  <div className="job-card-company">
                    <div className="job-company-line">
                      {isTier1 ? (
                        <span className="tier-1-mark" title="Tier 1 employer">
                          <span aria-hidden="true">🔥</span>
                          <span className="visually-hidden">Tier 1 employer: </span>
                        </span>
                      ) : null}
                      {job.companyName}
                    </div>
                    <div className="job-card-meta-line">
                      <span>{location}</span>
                      <span aria-hidden="true">·</span>
                      <span title={job.firstSeenAt}>{timeAgo(job.firstSeenAt)}</span>
                    </div>
                  </div>
                </div>
                <div className="job-card-body">
                  <h2>
                    <Link href={`/jobs/${job.id}`}>{job.title}</Link>
                  </h2>
                  <JobBadges job={job} />
                  {job.excerpt ? <p className="excerpt">{job.excerpt}</p> : null}
                  <div className="job-actions">
                    <a
                      className="btn btn-primary btn-sm"
                      href={job.applyUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      aria-label={`Apply for ${job.title} at ${job.companyName} on the employer site (opens in a new tab)`}
                    >
                      Apply
                    </a>
                    <AppliedToggle jobId={job.id} jobTitle={job.title} />
                  </div>
                </div>
              </article>
            );
          })
        )}
      </div>

      {totalPages > 1 ? (
        <nav className="pagination" aria-label="Pagination">
          {page > 1 ? (
            <Link className="btn btn-sm" href={pageHref(page - 1)}>
              ‹ Prev
            </Link>
          ) : null}
          {pageWindow.map((n) => (
            <Link
              key={n}
              className={n === page ? "btn btn-sm btn-primary" : "btn btn-sm"}
              href={pageHref(n)}
              aria-current={n === page ? "page" : undefined}
            >
              {n}
            </Link>
          ))}
          {page < totalPages ? (
            <Link className="btn btn-sm" href={pageHref(page + 1)}>
              Next ›
            </Link>
          ) : null}
        </nav>
      ) : null}
    </>
  );
}
