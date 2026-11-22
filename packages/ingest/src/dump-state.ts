/** Dumps from flat-file state: the same files and filters as dump.ts. */
import { applyUrlAllowed, healthSnapshotFromRecords, isDelisted, isFreshJob } from "@openintern/db";
import { writeDumpFiles, type DumpRow } from "./dump.js";
import { byCodeUnit, type State } from "./state.js";

export async function writeDumpsFromState(state: State, outDir: string, now = new Date()) {
  const rows: DumpRow[] = [];
  for (const j of state.jobs.values()) {
    const c = state.companies.get(j.company_id);
    if (!c || !j.is_active || isDelisted(j.id) || (j.roles?.length ?? 0) === 0) continue;
    const fresh = isFreshJob(
      { termYears: j.term_years, postedAt: j.posted_at ? new Date(j.posted_at) : null, firstSeenAt: new Date(j.first_seen_at) },
      now,
    );
    if (!fresh || !applyUrlAllowed(c.slug, j.apply_url)) continue;
    rows.push({
      id: j.id,
      title: j.title,
      normalizedTitle: j.normalized_title,
      roleFamilyId: j.role_family_id,
      locations: j.locations,
      applyUrl: j.apply_url,
      excerpt: j.excerpt,
      terms: j.terms,
      termYears: j.term_years,
      durationMonths: j.duration_months,
      cohortYear: j.cohort_year,
      roles: j.roles,
      regions: j.regions,
      isRemote: j.is_remote,
      source: j.source,
      postedAt: j.posted_at ? new Date(j.posted_at) : null,
      firstSeenAt: new Date(j.first_seen_at),
      lastSeenAt: new Date(j.last_seen_at),
      companyName: c.name,
      companySlug: c.slug,
      companyAts: c.ats,
      companyWebsiteUrl: c.website_url,
      companyCareersUrl: c.careers_url,
    });
  }
  rows.sort((a, b) => byCodeUnit(a.id, b.id));
  const companies = [...state.companies.values()].sort((a, b) => byCodeUnit(a.slug, b.slug));
  return writeDumpFiles(outDir, {
    rows,
    companyRows: companies.map((c) => ({
      id: c.id,
      name: c.name,
      slug: c.slug,
      ats: c.ats,
      careersUrl: c.careers_url,
      websiteUrl: c.website_url,
      active: c.active,
    })),
    health: healthSnapshotFromRecords({ companies, jobs: [...state.jobs.values()], runs: state.runs }, now),
    now,
  });
}
