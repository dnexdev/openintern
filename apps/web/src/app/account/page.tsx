import { desc, eq } from "drizzle-orm";
import { redirect } from "next/navigation";
import { companies, jobs, savedJobs, savedSearches } from "@openintern/db";
import { createSavedSearch, deleteSavedSearch, unsaveJob } from "@/app/actions";
import { auth } from "@/auth";
import { CompanyAvatar } from "@/components/CompanyAvatar";
import { getDb } from "@/lib/db";

export const dynamic = "force-dynamic";

export default async function AccountPage() {
  const session = await auth().catch(() => null);
  if (!session?.user?.id) redirect("/login");

  const db = getDb();
  const saved = await db
    .select({
      id: jobs.id,
      title: jobs.title,
      applyUrl: jobs.applyUrl,
      companyName: companies.name,
      companyWebsiteUrl: companies.websiteUrl,
      companyCareersUrl: companies.careersUrl,
      savedAt: savedJobs.createdAt,
    })
    .from(savedJobs)
    .innerJoin(jobs, eq(savedJobs.jobId, jobs.id))
    .innerJoin(companies, eq(jobs.companyId, companies.id))
    .where(eq(savedJobs.userId, session.user.id))
    .orderBy(desc(savedJobs.createdAt));

  const searches = await db
    .select()
    .from(savedSearches)
    .where(eq(savedSearches.userId, session.user.id))
    .orderBy(desc(savedSearches.createdAt));

  return (
    <>
      <section className="hero">
        <h1>Your account</h1>
        <p>
          Signed in as {session.user.email ?? session.user.name ?? "user"}. Saves
          and alert digests are optional — the board stays free without an
          account.
        </p>
      </section>

      <div className="panel">
        <h2>Saved jobs</h2>
        {saved.length === 0 ? (
          <p className="empty">No saved jobs yet.</p>
        ) : (
          <div className="job-list">
            {saved.map((j) => (
              <article key={j.id} className="job-card">
                <CompanyAvatar
                  name={j.companyName}
                  websiteUrl={j.companyWebsiteUrl}
                  careersUrl={j.companyCareersUrl}
                />
                <div className="job-card-body">
                  <h2>
                    <a href={`/jobs/${j.id}`}>{j.title}</a>
                  </h2>
                  <div className="job-company-line">{j.companyName}</div>
                  <div className="meta">
                    <span>saved {j.savedAt.toISOString().slice(0, 10)}</span>
                  </div>
                  <div className="job-actions">
                    <a className="btn btn-primary btn-sm" href={j.applyUrl} target="_blank" rel="noreferrer">
                      Apply
                    </a>
                    <form action={unsaveJob.bind(null, j.id)}>
                      <button className="btn btn-sm" type="submit">
                        Remove
                      </button>
                    </form>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>

      <div className="panel">
        <h2>Saved searches / alerts</h2>
        <p style={{ color: "var(--muted)" }}>
          Digests run via <code className="mono">pnpm --filter @openintern/ingest alerts</code>{" "}
          (GitHub Action daily). Email needs Resend; webhooks are optional.
        </p>
        <form action={createSavedSearch} style={{ display: "grid", gap: "0.6rem", marginBottom: "1rem" }}>
          <input type="text" name="name" placeholder="Search name" required />
          <input type="text" name="query" placeholder="Title contains (optional)" />
          <input type="text" name="location" placeholder="Location contains (optional)" />
          <input type="text" name="company" placeholder="Company slug (optional)" />
          <input type="url" name="webhook_url" placeholder="Webhook URL (optional)" />
          <label className="checkbox">
            <input type="checkbox" name="remote" /> Remote only
          </label>
          <label className="checkbox">
            <input type="checkbox" name="email" defaultChecked /> Email digest
          </label>
          <button className="btn btn-primary" type="submit">
            Create alert
          </button>
        </form>

        {searches.length === 0 ? (
          <p className="empty">No saved searches yet.</p>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Filters</th>
                <th>Last notified</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {searches.map((s) => (
                <tr key={s.id}>
                  <td>{s.name}</td>
                  <td className="mono">
                    {[s.query, s.location, s.companySlug, s.remoteOnly ? "remote" : null]
                      .filter(Boolean)
                      .join(" · ") || "—"}
                  </td>
                  <td className="mono">
                    {s.lastNotifiedAt?.toISOString() ?? "never"}
                  </td>
                  <td>
                    <form action={deleteSavedSearch.bind(null, s.id)}>
                      <button className="btn" type="submit">
                        Delete
                      </button>
                    </form>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
