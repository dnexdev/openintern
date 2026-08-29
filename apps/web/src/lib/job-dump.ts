import { isFreshJob } from "@openintern/db";

const DEFAULT_DUMP_URL =
  "https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.json";

export type DumpJob = {
  id: string;
  title: string;
  company: { name: string; slug: string; ats: string };
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
  const roles = job.roles ?? [];
  if (!Array.isArray(roles) || roles.length === 0) return false;
  return isFreshJob(
    {
      termYears: job.term_years,
      postedAt: job.posted_at ? new Date(job.posted_at) : null,
      firstSeenAt: new Date(job.first_seen_at),
    },
    now,
  );
}

export function liveDumpJobs(dump: JobsDump): DumpJob[] {
  return dump.jobs.filter((job) => isPublicDumpJob(job));
}
