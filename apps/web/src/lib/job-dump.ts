import { isPublic } from "@openintern/proven";
import { refreshKillSwitch } from "@/lib/kill-switch";
import { publishable } from "@/lib/publishable";

const DEFAULT_DUMP_URL =
  "https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.json";

export type DumpJob = {
  id: string;
  title: string;
  company: {
    name: string;
    slug: string;
    ats: string;
    /** Added 2026-10; absent in older dumps. */
    website_url?: string | null;
    careers_url?: string | null;
  };
  locations: string[] | null;
  apply_url: string;
  excerpt: string | null;
  terms: string[] | null;
  term_years: { term: string; year: number }[] | null;
  duration_months: number[] | null;
  cohort_year: number | null;
  roles: string[] | null;
  regions: string[] | null;
  is_remote: boolean;
  source: string;
  posted_at: string | null;
  first_seen_at: string;
  last_seen_at: string;
};

export type JobsDump = {
  generatedAt: string;
  count: number;
  jobs: DumpJob[];
};

const globalForDump = globalThis as unknown as {
  __openinternJobsDump?: { dump: JobsDump; fetchedAt: number };
};

const CACHE_MS = 60 * 60 * 1000;

export function dumpUrl() {
  return process.env.JOBS_DUMP_URL?.trim() || DEFAULT_DUMP_URL;
}

export async function loadJobsDump(): Promise<JobsDump> {
  await refreshKillSwitch();
  const cached = globalForDump.__openinternJobsDump;
  if (cached && Date.now() - cached.fetchedAt < CACHE_MS) {
    return cached.dump;
  }

  const res = await fetch(dumpUrl(), { next: { revalidate: 3600 } });
  if (!res.ok) {
    throw new Error(`Failed to fetch jobs dump (${res.status})`);
  }

  const dump = (await res.json()) as JobsDump;
  if (!Array.isArray(dump.jobs)) {
    throw new Error("Invalid jobs dump: missing jobs array");
  }

  globalForDump.__openinternJobsDump = { dump, fetchedAt: Date.now() };
  return dump;
}

/** Same gates as {@link publicJobSql} — dump rows are pre-filtered but re-check on read. */
export function isPublicDumpJob(job: DumpJob, now = new Date()): boolean {
  if (!publishable(job.company.slug, job.id, job.apply_url)) return false;
  const roles = job.roles ?? [];
  try {
    return isPublic(Array.isArray(roles) ? roles.length : 0, {
      termYears: job.term_years,
      anchor: new Date(job.posted_at || job.first_seen_at),
      now,
    });
  } catch {
    // Invalid or out-of-range dates: never show the row.
    return false;
  }
}

export function liveDumpJobs(dump: JobsDump): DumpJob[] {
  return dump.jobs.filter((job) => isPublicDumpJob(job));
}

export type DumpCompany = {
  id: string;
  name: string;
  slug: string;
  ats: string;
  careers_url: string | null;
  website_url: string | null;
  active: boolean;
  active_jobs: number;
};

const globalForCompanies = globalThis as unknown as {
  __openinternCompaniesDump?: { companies: DumpCompany[]; fetchedAt: number };
};

/** companies.json from the same release as jobs.json, or null if it has none yet. */
export async function loadCompaniesDump(): Promise<DumpCompany[] | null> {
  const cached = globalForCompanies.__openinternCompaniesDump;
  if (cached && Date.now() - cached.fetchedAt < CACHE_MS) return cached.companies;
  try {
    const res = await fetch(new URL("companies.json", dumpUrl()), { next: { revalidate: 3600 } });
    if (!res.ok) return null;
    const body = (await res.json()) as { companies?: DumpCompany[] };
    if (!Array.isArray(body.companies)) return null;
    globalForCompanies.__openinternCompaniesDump = { companies: body.companies, fetchedAt: Date.now() };
    return body.companies;
  } catch {
    return null;
  }
}
