import type { Metadata } from "next";
import { and, eq } from "drizzle-orm";
import Link from "next/link";
import { notFound } from "next/navigation";
import { companies, jobs } from "@openintern/db";
import { AppliedToggle } from "@/components/AppliedToggle";
import { CompanyAvatar } from "@/components/CompanyAvatar";
import { getDb } from "@/lib/db";
import { freshnessSql } from "@/lib/freshness";

export const dynamic = "force-dynamic";

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function formatDate(d: Date | null | undefined) {
  if (!d) return "—";
  return d.toISOString().slice(0, 10);
}

async function loadJob(id: string) {
  const db = getDb();
  const [row] = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      locations: jobs.locations,
      applyUrl: jobs.applyUrl,
      excerpt: jobs.excerpt,
      terms: jobs.terms,
      termYears: jobs.termYears,
      durationMonths: jobs.durationMonths,
      roles: jobs.roles,
      regions: jobs.regions,
      cohortYear: jobs.cohortYear,
      isRemote: jobs.isRemote,
      isActive: jobs.isActive,
      source: jobs.source,
      postedAt: jobs.postedAt,
      firstSeenAt: jobs.firstSeenAt,
      companyName: companies.name,
      companySlug: companies.slug,
      companyWebsiteUrl: companies.websiteUrl,
      companyCareersUrl: companies.careersUrl,
    })
    .from(jobs)
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(and(eq(jobs.id, id), eq(jobs.isActive, true), freshnessSql()))
    .limit(1);
  return row ?? null;
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  try {
    const job = await loadJob(id);
    if (!job) {
      return { title: "Job not found · OpenIntern" };
    }
    const description =
      job.excerpt?.slice(0, 160) ||
      `${job.title} at ${job.companyName} — tech internship on OpenIntern.`;
    return {
      title: `${job.title} at ${job.companyName} · OpenIntern`,
      description,
      alternates: { canonical: `/jobs/${job.id}` },
      openGraph: {
        title: `${job.title} at ${job.companyName}`,
        description,
        type: "article",
        url: `/jobs/${job.id}`,
      },
    };
  } catch {
    return { title: "OpenIntern" };
  }
}

export default async function JobDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const job = await loadJob(id).catch(() => null);
  if (!job) notFound();

  const durations = [
    ...new Set((job.durationMonths ?? []).filter((m) => [3, 4, 6, 8, 12].includes(m))),
  ].sort((a, b) => a - b);

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
        </div>
      </article>
    </>
  );
}
