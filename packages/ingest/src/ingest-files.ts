/**
 * Ingest against flat-file state (state.ts) instead of Postgres. Same plan
 * (plan.ts), same rules: registry sync, auto-disable, per-job retirement,
 * the outage guard on the last-seen sweep.
 */
import { applyHostMode } from "@openintern/db";
import { DAY_MS } from "@openintern/proven";
import { shadowResults } from "./bytedance.js";
import { LAST_SEEN_STALE_DAYS } from "./classifier.js";
import {
  MANUAL_ATS,
  autoDisableState,
  countPriorEmptyRuns,
  fetchCompanyBoards,
  shouldSkipLastSeenSweep,
  type IngestSummary,
} from "./ingest.js";
import { planCompanyRun, type ApplyHostViolation, type CompanyFunnel } from "./plan.js";
import { RUN_RETENTION_DAYS, newId, pgTimestamp, type CompanyRecord, type JobRecord, type State } from "./state.js";
import { loadCompanyYamlFiles } from "./sync-companies.js";
import { loadTier1Curated } from "./validate-curated.js";

const iso = pgTimestamp;

/** Registry YAML → company records: new slugs get ids, missing slugs go inactive and their jobs come down. */
export async function syncRegistryIntoState(state: State, now: Date): Promise<{ orphans: string[] }> {
  const list = await loadCompanyYamlFiles();
  const bySlug = new Map(list.map((c) => [c.slug, c]));
  const existing = new Map([...state.companies.values()].map((c) => [c.slug, c]));
  for (const c of bySlug.values()) {
    const prev = existing.get(c.slug);
    const next: CompanyRecord = {
      id: prev?.id ?? newId(),
      name: c.name,
      slug: c.slug,
      ats: c.ats,
      board_token: c.board_token,
      careers_url: c.careers_url ?? null,
      website_url: c.website_url ?? null,
      active: c.active ?? true,
      created_at: prev?.created_at ?? iso(now),
      updated_at: prev?.updated_at ?? iso(now),
    };
    const changed = !prev || JSON.stringify({ ...prev, updated_at: "" }) !== JSON.stringify({ ...next, updated_at: "" });
    if (changed) next.updated_at = iso(now);
    state.companies.set(next.id, next);
  }
  const orphans: string[] = [];
  for (const c of state.companies.values()) {
    if (c.active && !bySlug.has(c.slug)) {
      c.active = false;
      c.updated_at = iso(now);
      orphans.push(c.slug);
      for (const j of state.jobs.values()) {
        if (j.company_id === c.id && j.is_active) {
          j.is_active = false;
          j.updated_at = iso(now);
        }
      }
    }
  }
  return { orphans };
}

export async function runIngestFiles(
  state: State,
  opts: { now?: Date; syncRegistry?: boolean; only?: Set<string> } = {},
): Promise<IngestSummary> {
  const now = opts.now ?? new Date();
  const lastOk = state.runs
    .filter((r) => r.status === "ok")
    .reduce<number | null>((m, r) => Math.max(m ?? -Infinity, Date.parse(r.ran_at)), null);
  const skipLastSeenSweep = shouldSkipLastSeenSweep(lastOk == null ? null : new Date(lastOk), now);

  if (opts.syncRegistry !== false) await syncRegistryIntoState(state, now);

  const registry = await loadCompanyYamlFiles();
  const extraBoards = new Map(registry.map((c) => [c.slug, c.extra_board_tokens ?? []] as const));
  const tier1 = new Set((await loadTier1Curated()).slugs);
  const history = new Map<string, { status: string; error: string | null; ranAt: Date }[]>();
  for (const r of [...state.runs].sort((a, b) => Date.parse(b.ran_at) - Date.parse(a.ran_at))) {
    const list = history.get(r.company_id) ?? [];
    list.push({ status: r.status, error: r.error, ranAt: new Date(r.ran_at) });
    history.set(r.company_id, list);
  }

  const active = [...state.companies.values()]
    .filter((c) => c.active && (!opts.only || opts.only.has(c.slug)))
    .sort((a, b) => a.slug.localeCompare(b.slug));
  const autoDisabledSkipped: string[] = [];
  const toFetch = active.filter((c) => {
    const exempt = tier1.has(c.slug) || MANUAL_ATS.has(c.ats);
    if (autoDisableState(history.get(c.id) ?? [], exempt, now) !== "disabled") return true;
    autoDisabledSkipped.push(c.slug);
    return false;
  });

  const jobsByCompany = new Map<string, JobRecord[]>();
  for (const j of state.jobs.values()) {
    const list = jobsByCompany.get(j.company_id) ?? [];
    list.push(j);
    jobsByCompany.set(j.company_id, list);
  }
  const byFingerprint = new Map([...state.jobs.values()].map((j) => [j.fingerprint, j]));

  const summary = {
    jobsUpserted: 0,
    jobsDeactivated: 0,
    staleDeactivated: 0,
    zeroMatchCompanies: [] as string[],
    applyHostViolations: [] as ApplyHostViolation[],
    autoDisabled: [] as string[],
    suspectedBlocks: [] as string[],
    failures: [] as { slug: string; error: string }[],
    funnel: [] as CompanyFunnel[],
  };
  const enforceApplyHosts = applyHostMode() === "enforce";

  const CHUNK = 8;
  for (let i = 0; i < toFetch.length; i += CHUNK) {
    const chunk = toFetch.slice(i, i + CHUNK);
    const fetched = await Promise.allSettled(
      chunk.map((c) => fetchCompanyBoards({ ats: c.ats, boardToken: c.board_token }, extraBoards.get(c.slug) ?? [])),
    );
    for (let k = 0; k < chunk.length; k++) {
      const company = chunk[k]!;
      const result = fetched[k]!;
      const prior = history.get(company.id) ?? [];
      if (result.status === "rejected") {
        const message = result.reason instanceof Error ? result.reason.message : String(result.reason);
        summary.failures.push({ slug: company.slug, error: message });
        summary.funnel.push({ slug: company.slug, fetched: 0, internshipTitle: 0, techPass: 0, upserted: 0, stale: 0 });
        const exempt = tier1.has(company.slug) || MANUAL_ATS.has(company.ats);
        if (
          autoDisableState(prior, exempt, now) === "enabled" &&
          autoDisableState([{ status: "error", ranAt: now }, ...prior], exempt, now) === "disabled"
        ) {
          summary.autoDisabled.push(company.slug);
        }
        state.runs.push({ id: newId(), company_id: company.id, status: "error", job_count: 0, error: message.slice(0, 2000), ran_at: iso(now) });
        continue;
      }
      const own = jobsByCompany.get(company.id) ?? [];
      const plan = planCompanyRun({
        company: { id: company.id, slug: company.slug, ats: company.ats },
        raw: result.value.raw,
        expired: result.value.expired,
        existing: own.map((j) => ({
          id: j.id,
          externalJobId: j.external_job_id,
          isActive: j.is_active,
          fingerprint: j.fingerprint,
          firstSeenAt: new Date(j.first_seen_at),
          postedAt: j.posted_at ? new Date(j.posted_at) : null,
        })),
        priorEmptyRuns: countPriorEmptyRuns(prior),
        now,
        enforceApplyHosts,
      });
      for (const u of plan.upserts) {
        const prev = byFingerprint.get(u.fingerprint);
        const record: JobRecord = {
          id: prev?.id ?? newId(),
          company_id: u.companyId,
          external_job_id: u.externalJobId,
          fingerprint: u.fingerprint,
          title: u.title,
          normalized_title: u.normalizedTitle,
          role_family_id: u.roleFamilyId,
          locations: u.locations,
          apply_url: u.applyUrl,
          excerpt: u.excerpt,
          terms: u.terms,
          term_years: u.termYears,
          duration_months: u.durationMonths,
          cohort_year: u.cohortYear,
          roles: u.roles,
          regions: u.regions,
          is_remote: u.isRemote,
          is_active: u.isActive,
          source: u.source,
          // Keep stored strings when the instant is unchanged (byte-for-byte state).
          posted_at:
            prev?.posted_at && u.postedAt && new Date(prev.posted_at).getTime() === u.postedAt.getTime()
              ? prev.posted_at
              : u.postedAt
                ? iso(u.postedAt)
                : null,
          first_seen_at: prev?.first_seen_at ?? iso(u.firstSeenAt),
          created_at: prev?.created_at ?? iso(now),
          last_seen_at: iso(now),
          updated_at: iso(now),
        };
        state.jobs.set(record.id, record);
        byFingerprint.set(record.fingerprint, record);
        if (!prev) own.push(record);
      }
      for (const id of plan.retireIds) {
        const j = state.jobs.get(id);
        if (j?.is_active) {
          j.is_active = false;
          j.updated_at = iso(now);
          summary.jobsDeactivated++;
        }
      }
      jobsByCompany.set(company.id, own);
      state.runs.push({
        id: newId(),
        company_id: company.id,
        status: plan.run.status,
        job_count: plan.run.jobCount,
        error: plan.run.error,
        ran_at: iso(now),
      });
      summary.jobsUpserted += plan.upserts.length;
      summary.staleDeactivated += plan.staleDeactivated;
      if (plan.zeroMatch) summary.zeroMatchCompanies.push(company.slug);
      if (plan.suspectedBlock) summary.suspectedBlocks.push(company.slug);
      summary.applyHostViolations.push(...plan.applyHostViolations);
      summary.funnel.push(plan.funnel);
    }
  }

  let lastSeenDeactivated = 0;
  if (!skipLastSeenSweep) {
    const cutoff = now.getTime() - LAST_SEEN_STALE_DAYS * DAY_MS;
    for (const j of state.jobs.values()) {
      if (j.is_active && new Date(j.last_seen_at).getTime() < cutoff) {
        j.is_active = false;
        j.updated_at = iso(now);
        lastSeenDeactivated++;
      }
    }
  }

  const keepAfter = now.getTime() - RUN_RETENTION_DAYS * DAY_MS;
  state.runs = state.runs.filter((r) => Date.parse(r.ran_at) >= keepAfter);

  summary.funnel.sort((a, b) => b.fetched - a.fetched || a.slug.localeCompare(b.slug));
  const funnelTotals = summary.funnel.reduce(
    (acc, f) => ({
      fetched: acc.fetched + f.fetched,
      internshipTitle: acc.internshipTitle + f.internshipTitle,
      techPass: acc.techPass + f.techPass,
      upserted: acc.upserted + f.upserted,
    }),
    { fetched: 0, internshipTitle: 0, techPass: 0, upserted: 0 },
  );
  return {
    companies: toFetch.length,
    jobsUpserted: summary.jobsUpserted,
    jobsDeactivated: summary.jobsDeactivated,
    staleDeactivated: summary.staleDeactivated,
    lastSeenDeactivated,
    lastSeenSweepSkipped: skipLastSeenSweep,
    zeroMatchCompanies: summary.zeroMatchCompanies,
    autoDisabled: summary.autoDisabled,
    autoDisabledSkipped,
    suspectedBlocks: summary.suspectedBlocks,
    shadow: [...shadowResults],
    applyHostViolations: summary.applyHostViolations,
    failures: summary.failures,
    funnel: summary.funnel,
    funnelTotals,
  };
}
