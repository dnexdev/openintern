import type { Metadata } from "next";
import { and, eq } from "drizzle-orm";
import { notFound } from "next/navigation";
import { companies, jobs } from "@openintern/db";
import { saveJob } from "@/app/actions";
import { auth } from "@/auth";
import { CompanyAvatar } from "@/components/CompanyAvatar";
import { getDb } from "@/lib/db";

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
      durationMonths: jobs.durationMonths,
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
    .where(and(eq(jobs.id, id), eq(jobs.isActive, true)))
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

  const session = await auth().catch(() => null);

  return (
    <>
      <p style={{ margin: "1.25rem 0 0.5rem" }}>
        <a href="/">← All internships</a>
      </p>

      <article className="panel job-detail">
        <div className="job-card" style={{ border: "none", boxShadow: "none", padding: 0 }}>
          <CompanyAvatar
            name={job.companyName}
            websiteUrl={job.companyWebsiteUrl}
            careersUrl={job.companyCareersUrl}
          />
          <div className="job-card-body">
            <h1 style={{ margin: 0, fontSize: "1.45rem", letterSpacing: "-0.02em" }}>
              {job.title}
            </h1>
            <div className="job-company-line">
              {job.companyName} · {(job.locations ?? []).join(" · ") || "Location n/a"}
            </div>
            <div className="meta">
              {job.isRemote ? <span className="badge remote">Remote</span> : null}
              {(job.terms ?? []).map((t) => (
                <span key={t} className="badge remote">
                  {capitalize(t)}
                </span>
              ))}
              {job.durationMonths ? (
                <span className="badge">{job.durationMonths} mo</span>
              ) : null}
              {job.cohortYear ? <span className="badge">{job.cohortYear}</span> : null}
              <span className="badge source">{job.source}</span>
              <span>posted {formatDate(job.postedAt)}</span>
              <span>first seen {formatDate(job.firstSeenAt)}</span>
            </div>
          </div>
        </div>

        {job.excerpt ? (
          <p className="excerpt" style={{ marginTop: "1.1rem", WebkitLineClamp: "unset" as unknown as number, display: "block" }}>
            {job.excerpt}
          </p>
        ) : (
          <p className="empty" style={{ textAlign: "left", padding: "1rem 0 0" }}>
            No description excerpt available. Open the employer posting for full details.
          </p>
        )}

        <div className="job-actions" style={{ marginTop: "1.25rem" }}>
          <a
            className="btn btn-primary"
            href={job.applyUrl}
            target="_blank"
            rel="noreferrer"
          >
            Apply on employer site
          </a>
          {session?.user ? (
            <form action={saveJob.bind(null, job.id)}>
              <button className="btn" type="submit">
                Save
              </button>
            </form>
          ) : (
            <a className="btn" href="/login">
              Sign in to save
            </a>
          )}
        </div>
      </article>
    </>
  );
}
