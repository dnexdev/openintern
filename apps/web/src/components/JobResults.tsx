"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { CompanyAvatar } from "./CompanyAvatar";
import {
  AppliedToggle,
  HideAppliedToggle,
  useAppliedIds,
  useHideApplied,
} from "./AppliedToggle";
import type { FamilySort, JobFamily } from "@/lib/job-families";
import { jobPath } from "@/lib/job-slug";
import {
  familyApplyRows,
  familyMetaLabel,
  familyPickerButtonLabel,
  needsApplyPicker,
} from "@/lib/posting-label";
import { reportIssueUrl } from "@/lib/report-issue";

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

function FamilyBadges({ family }: { family: JobFamily }) {
  const badges: { key: string; className: string; label: string }[] = [];
  for (const r of family.roles.slice(0, 2)) {
    badges.push({ key: `role-${r}`, className: "badge role", label: r });
  }
  for (const t of family.terms.slice(0, 1)) {
    badges.push({ key: `t-${t}`, className: "badge term", label: capitalize(t) });
  }
  for (const r of family.regions.slice(0, 1)) {
    badges.push({
      key: `region-${r}`,
      className: "badge region",
      label: r === "europe" ? "UK/Europe" : capitalize(r),
    });
  }
  return (
    <div className="meta">
      {badges.map((b) => (
        <span key={b.key} className={b.className}>
          {b.label}
        </span>
      ))}
    </div>
  );
}

function ApplyPicker({
  family,
  rows,
}: {
  family: JobFamily;
  rows: ReturnType<typeof familyApplyRows>;
}) {
  return (
    <ul className="posting-list">
      {rows.map((row) => (
        <li key={row.key} className="posting-row">
          <span className="posting-loc">{row.label}</span>
          <a
            className="btn btn-primary btn-sm"
            href={row.applyUrl}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Apply for ${family.title} (${row.label})`}
          >
            Apply
          </a>
          <AppliedToggle jobId={row.jobId} jobTitle={row.appliedTitle} />
        </li>
      ))}
    </ul>
  );
}

function FamilyCard({
  family,
  isTier1,
  applied,
}: {
  family: JobFamily;
  isTier1: boolean;
  applied: string[];
}) {
  const [open, setOpen] = useState(false);
  const primary = family.postings[0]!;
  const picker = needsApplyPicker(family.postings);
  const applyRows = familyApplyRows(family.title, family.postings);
  const metaLabel = familyMetaLabel(family.title, family.postings);
  const pickerButtonLabel = familyPickerButtonLabel(family.postings);
  const allApplied = family.postings.every((p) => applied.includes(p.id));
  const detailPath = jobPath(family.company.slug, family.title, primary.id);
  const reportUrl = reportIssueUrl({
    jobId: primary.id,
    title: family.title,
    companyName: family.company.name,
    applyUrl: primary.applyUrl,
    pageUrl: `https://openintern.dev${detailPath}`,
  });

  return (
    <article
      className={`job-card${allApplied ? " is-applied" : ""}${isTier1 ? " is-tier-1" : ""}`}
      data-family-id={family.roleFamilyId}
    >
      <div className="job-card-top">
        <CompanyAvatar
          name={family.company.name}
          websiteUrl={family.company.websiteUrl}
          careersUrl={family.company.careersUrl}
          slug={family.company.slug}
        />
        <div className="job-card-company">
          <div className="job-company-line">
            {isTier1 ? (
              <span className="tier-1-mark" title="Tier 1 employer">
                <span aria-hidden="true">🔥</span>
                <span className="visually-hidden">Tier 1 employer: </span>
              </span>
            ) : null}
            {family.company.name}
          </div>
          <div className="job-card-meta-line">
            {picker ? (
              <button
                type="button"
                className="locations-toggle"
                aria-expanded={open}
                onClick={() => setOpen((v) => !v)}
              >
                {metaLabel} {open ? "▴" : "▾"}
              </button>
            ) : (
              <span>{metaLabel}</span>
            )}
            <span aria-hidden="true">·</span>
            <span title={family.firstSeenAt}>{timeAgo(family.firstSeenAt)}</span>
          </div>
        </div>
      </div>
      <div className="job-card-body">
        <h2>
          <Link href={detailPath}>{family.title}</Link>
        </h2>
        <FamilyBadges family={family} />
        {family.excerpt ? <p className="excerpt">{family.excerpt}</p> : null}

        {picker ? (
          <div className="job-picker">
            {open ? <ApplyPicker family={family} rows={applyRows} /> : null}
            <div className="job-actions">
              <button
                type="button"
                className={open ? "btn btn-sm" : "btn btn-primary btn-sm"}
                aria-expanded={open}
                onClick={() => setOpen((v) => !v)}
              >
                {open ? "Hide" : pickerButtonLabel}
              </button>
              <a
                className="report-issue-link"
                href={reportUrl}
                target="_blank"
                rel="noopener noreferrer"
              >
                Report issue
              </a>
            </div>
          </div>
        ) : (
          <div className="job-actions">
            <a
              className="btn btn-primary btn-sm"
              href={primary.applyUrl}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={`Apply for ${family.title} at ${family.company.name}`}
            >
              Apply
            </a>
            <AppliedToggle jobId={primary.id} jobTitle={family.title} />
            <a
              className="report-issue-link"
              href={reportUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              Report issue
            </a>
          </div>
        )}
      </div>
    </article>
  );
}

export function JobResults({
  families,
  total,
  hasFilters,
  page,
  totalPages,
  filterQuery,
  sort,
  tier1Slugs,
}: {
  families: JobFamily[];
  total: number;
  hasFilters: boolean;
  page: number;
  totalPages: number;
  filterQuery: string;
  sort: FamilySort;
  tier1Slugs: string[];
}) {
  const router = useRouter();
  const [hideApplied, setHideApplied] = useHideApplied();
  const applied = useAppliedIds();
  const tier1 = new Set(tier1Slugs);
  const visible = hideApplied
    ? families.filter((f) => !f.postings.every((p) => applied.includes(p.id)))
    : families;

  const pageHref = (n: number) => {
    const params = new URLSearchParams(filterQuery);
    if (sort !== "first_seen") params.set("sort", sort);
    params.set("page", String(n));
    return `/jobs?${params.toString()}`;
  };

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
          <strong>{total}</strong> role{total === 1 ? "" : "s"}
          {hasFilters ? " matching filters" : ""}
          {hideApplied && families.length !== visible.length
            ? ` · showing ${visible.length} of ${families.length} on this page`
            : ""}
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
            <option value="prestige">Prestige sort</option>
          </select>
        </label>
      </div>

      <div className="job-grid">
        {families.length === 0 ? (
          <p className="empty">
            No active internships matched.{" "}
            {hasFilters ? <Link href="/jobs">Clear filters</Link> : "Try running ingest."}
          </p>
        ) : visible.length === 0 ? (
          <p className="empty">
            All matching roles are marked applied. Uncheck “Hide applied” to see them.
          </p>
        ) : (
          visible.map((family) => (
            <FamilyCard
              key={family.roleFamilyId}
              family={family}
              isTier1={tier1.has(family.company.slug)}
              applied={applied}
            />
          ))
        )}
      </div>

      {totalPages > 1 ? (
        <nav className="pagination" aria-label="Pagination">
          {Array.from({ length: totalPages }, (_, i) => i + 1).map((n) => (
            <Link
              key={n}
              className={n === page ? "page-num is-current" : "page-num"}
              href={pageHref(n)}
              aria-current={n === page ? "page" : undefined}
              aria-label={`Page ${n}`}
            >
              {n}
            </Link>
          ))}
        </nav>
      ) : null}
    </>
  );
}
