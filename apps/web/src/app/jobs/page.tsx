import type { Metadata } from "next";
import { FilterSidebar } from "@/components/FilterSidebar";
import { JobResults } from "@/components/JobResults";
import { getTier1Slugs } from "@/lib/curated";
import { loadJobFamilies } from "@/lib/job-families";
import {
  loadCompanyOptions,
  parseBoardSearchParams,
  type SearchParams,
} from "@/lib/board";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Jobs",
  description:
    "Browse free tech internship listings. No account required. Filter by role, location, term, and company.",
  alternates: { canonical: "/jobs" },
};

export default async function JobsPage({
  searchParams,
}: {
  searchParams: SearchParams;
}) {
  const sp = await searchParams;
  const { query, company, roles, regions, terms, durations, sort, page } =
    parseBoardSearchParams(sp);
  const limit = 27;
  const offset = (page - 1) * limit;

  let families: Awaited<ReturnType<typeof loadJobFamilies>>["families"] = [];
  let total = 0;
  let dbError: string | null = null;
  let companyOptions: { slug: string; name: string }[] = [];

  try {
    const [loaded, options] = await Promise.all([
      loadJobFamilies({
        query,
        company,
        roles,
        regions,
        terms,
        durations,
        sort,
        limit,
        offset,
      }),
      loadCompanyOptions(),
    ]);
    families = loaded.families;
    total = loaded.total;
    companyOptions = options;
  } catch (err) {
    dbError = err instanceof Error ? err.message : "Database unavailable";
  }

  const totalPages = Math.max(1, Math.ceil(total / limit));

  const filterParams = new URLSearchParams();
  if (query) filterParams.set("q", query);
  if (company) filterParams.set("company", company);
  for (const r of roles) filterParams.append("role", r);
  for (const r of regions) filterParams.append("region", r);
  for (const t of terms) filterParams.append("term", t);
  for (const d of durations) filterParams.append("duration", String(d));
  const filterQuery = filterParams.toString();

  const hasFilters = Boolean(
    query || company || roles.length || regions.length || terms.length || durations.length,
  );

  let tier1Slugs: string[] = [];
  try {
    tier1Slugs = [...getTier1Slugs()];
  } catch {
    // Tier-1 badges are optional; never take down the board.
  }

  return (
    <div className="board board-page" id="jobs">
      <header className="board-page-header">
        <h1>Internships</h1>
        <p>Free tech roles from public ATS boards. No account required.</p>
      </header>

      <FilterSidebar
        query={query}
        company={company}
        companyOptions={companyOptions}
        roles={roles}
        regions={regions}
        terms={terms}
        durations={durations}
        hasFilters={hasFilters}
        sort={sort}
      />

      <section className="board-results">
        {dbError ? (
          <div className="panel">
            <p className="empty">
              Database not connected yet. Set <code className="mono">DATABASE_URL</code>,
              run migrations, then <code className="mono">pnpm ingest</code>.
            </p>
            <p className="mono status-error">{dbError}</p>
          </div>
        ) : (
          <JobResults
            families={families}
            total={total}
            hasFilters={hasFilters}
            page={page}
            totalPages={totalPages}
            filterQuery={filterQuery}
            sort={sort}
            tier1Slugs={tier1Slugs}
          />
        )}
      </section>
    </div>
  );
}
