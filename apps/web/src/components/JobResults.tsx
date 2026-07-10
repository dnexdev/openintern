"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { CompanyAvatar } from "./CompanyAvatar";
import { AppliedToggle, HideAppliedToggle, useAppliedIds } from "./AppliedToggle";

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

function formatDate(iso: string | null) {
  if (!iso) return "—";
  return iso.slice(0, 10);
}

function JobBadges({ job }: { job: JobCardData }) {
  const badges: { key: string; className: string; label: string }[] = [];
  for (const r of (job.roles ?? []).slice(0, 3)) {
    badges.push({ key: `role-${r}`, className: "badge role", label: r });
  }
  for (const r of (job.regions ?? []).slice(0, 2)) {
    badges.push({
      key: `region-${r}`,
      className: "badge region",
      label: r === "europe" ? "UK/Europe" : capitalize(r),
    });
  }
  const terms =
    (job.termYears ?? []).length > 0
      ? (job.termYears ?? []).slice(0, 2).map((ty) => ({
          key: `ty-${ty.term}-${ty.year}`,
          label: `${capitalize(ty.term)} ${ty.year}`,
        }))
      : (job.terms ?? []).slice(0, 2).map((t) => ({
          key: `t-${t}`,
          label: capitalize(t),
        }));
  for (const t of terms) {
    badges.push({ key: t.key, className: "badge term", label: t.label });
  }
  const durations = [
    ...new Set((job.durationMonths ?? []).filter((m) => [3, 4, 6, 8, 12].includes(m))),
  ].sort((a, b) => a - b);
  if (durations.length > 0) {
    badges.push({
      key: "dur",
      className: "badge duration",
      label: `${durations.join("/")} mo`,
    });
  }

  const shown = badges.slice(0, 5);
  const extra = badges.length - shown.length;

  return (
    <div className="meta">
      {shown.map((b) => (
        <span key={b.key} className={b.className}>
          {b.label}
        </span>
      ))}
      {extra > 0 ? <span className="badge">+{extra}</span> : null}
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
}: {
  jobs: JobCardData[];
  total: number;
  hasFilters: boolean;
  page: number;
  totalPages: number;
  filterQuery: string;
  sort: "first_seen" | "posted";
}) {
  const router = useRouter();
  const [hideApplied, setHideApplied] = useState(false);
  const applied = useAppliedIds();
  const visible = hideApplied
    ? jobs.filter((j) => !applied.includes(j.id))
    : jobs;

  const pageHref = (n: number) => {
    const params = new URLSearchParams(filterQuery);
    if (sort !== "first_seen") params.set("sort", sort);
    params.set("page", String(n));
    return `/?${params.toString()}`;
  };

  const pageWindow: number[] = [];
  for (let n = Math.max(1, page - 2); n <= Math.min(totalPages, page + 2); n++) {
    pageWindow.push(n);
  }

  function onSortChange(value: string) {
    const params = new URLSearchParams(filterQuery);
    if (value !== "first_seen") params.set("sort", value);
    const qs = params.toString();
    router.push(qs ? `/?${qs}` : "/", { scroll: false });
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

      <div className="job-list">
        {jobs.length === 0 ? (
          <p className="empty">
            No active internships matched.{" "}
            {hasFilters ? <Link href="/">Clear filters</Link> : "Try running ingest."}
          </p>
        ) : visible.length === 0 ? (
          <p className="empty">
            All matching roles are marked applied. Uncheck “Hide applied” to see them.
          </p>
        ) : (
          visible.map((job) => {
            const isApplied = applied.includes(job.id);
            return (
              <article
                key={job.id}
                className={`job-card${isApplied ? " is-applied" : ""}`}
                data-job-id={job.id}
              >
                <CompanyAvatar
                  name={job.companyName}
                  websiteUrl={job.companyWebsiteUrl}
                  careersUrl={job.companyCareersUrl}
                  slug={job.companySlug}
                />
                <div className="job-card-body">
                  <h2>
                    <Link href={`/jobs/${job.id}`}>{job.title}</Link>
                  </h2>
                  <div className="job-company-line">
                    {job.companyName} · {(job.locations ?? []).join(" · ") || "Location n/a"}
                  </div>
                  <JobBadges job={job} />
                  <div className="meta-secondary">
                    <span className="badge source">{job.source}</span>
                    <span title={job.firstSeenAt}>first seen {timeAgo(job.firstSeenAt)}</span>
                    <span>posted {formatDate(job.postedAt)}</span>
                  </div>
                  {job.excerpt ? <p className="excerpt">{job.excerpt}</p> : null}
                  <div className="job-actions">
                    <a
                      className="btn btn-primary btn-sm"
                      href={job.applyUrl}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Apply
                    </a>
                    <AppliedToggle jobId={job.id} />
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
