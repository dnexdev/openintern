/**
 * Flat-file state (plan phase D): the same data as the Postgres tables, kept as
 * sorted JSONL in a git repository so every change is a reviewable diff.
 *
 *   companies.jsonl  one company per line, sorted by slug
 *   jobs.jsonl       every job ever seen (inactive too), sorted by id; no
 *                    per-run timestamps, so unchanged jobs give no diff
 *   last_seen.tsv    id, last_seen_at, updated_at (changes every run)
 *   runs.jsonl       ingest runs of the last RUN_RETENTION_DAYS
 *
 * Field names and timestamp strings match Postgres `row_to_json` output, so a
 * state seeded from scripts/export-state.sh keeps ids and first_seen_at byte
 * for byte. Writes go to a temporary directory and replace the files at once.
 */
import { randomUUID } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";

export const RUN_RETENTION_DAYS = 60;

export type CompanyRecord = {
  id: string;
  name: string;
  slug: string;
  ats: string;
  board_token: string;
  careers_url: string | null;
  website_url: string | null;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type JobRecord = {
  id: string;
  company_id: string;
  external_job_id: string;
  fingerprint: string;
  title: string;
  normalized_title: string;
  role_family_id: string;
  locations: string[];
  apply_url: string;
  excerpt: string | null;
  terms: string[];
  term_years: { term: string; year: number }[];
  duration_months: number[];
  cohort_year: number | null;
  roles: string[];
  regions: string[];
  is_remote: boolean;
  is_active: boolean;
  source: string;
  posted_at: string | null;
  first_seen_at: string;
  created_at: string;
  /** Kept in last_seen.tsv, not jobs.jsonl. */
  last_seen_at: string;
  updated_at: string;
};

export type RunRecord = {
  id: string;
  company_id: string;
  status: string;
  job_count: number;
  error: string | null;
  ran_at: string;
};

export type State = {
  companies: Map<string, CompanyRecord>; // by id
  jobs: Map<string, JobRecord>; // by id
  runs: RunRecord[];
};

const JOB_VOLATILE = new Set(["last_seen_at", "updated_at"]);

/**
 * Key order of each file = the table's physical column order, which is the
 * order `row_to_json` uses in scripts/export-state.sh. Records written by file
 * ingest then match exported ones byte for byte. A migration that adds a
 * column appends it here too.
 */
const COMPANY_KEYS = ["id", "name", "slug", "ats", "board_token", "careers_url", "website_url", "active", "created_at", "updated_at"];
const JOB_KEYS = [
  "id", "company_id", "external_job_id", "title", "locations", "apply_url", "excerpt", "is_remote", "is_active",
  "source", "posted_at", "first_seen_at", "last_seen_at", "created_at", "updated_at", "terms", "cohort_year",
  "duration_months", "roles", "regions", "term_years", "fingerprint", "normalized_title", "role_family_id",
];
const RUN_KEYS = ["id", "company_id", "status", "job_count", "error", "ran_at"];

/** The record with its keys in `order` (unknown keys last, sorted), minus `skip`. */
function ordered(record: object, order: string[], skip: Set<string> = new Set()): Record<string, unknown> {
  const r = record as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const k of order) if (k in r && !skip.has(k)) out[k] = r[k];
  for (const k of Object.keys(r).sort(byCodeUnit)) if (!(k in out) && !skip.has(k)) out[k] = r[k];
  return out;
}

/**
 * A timestamp exactly as Postgres `row_to_json` prints a timestamptz in UTC:
 * fraction trimmed of trailing zeros (none when whole), offset "+00:00". New
 * rows use the same format as exported ones, so state → Postgres → export is
 * byte-identical and a rollback never rewrites first_seen_at.
 */
export function pgTimestamp(d: Date): string {
  const [whole, frac = ""] = d.toISOString().slice(0, -1).split(".");
  const f = frac.replace(/0+$/, "");
  return `${whole}${f ? `.${f}` : ""}+00:00`;
}

/** Plain code-unit order: stable across Node/ICU versions, unlike localeCompare. */
export const byCodeUnit = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

function parseJsonl<T>(text: string, file: string): T[] {
  return text
    .split("\n")
    .filter((l) => l.trim())
    .map((l, i) => {
      try {
        return JSON.parse(l) as T;
      } catch {
        throw new Error(`${file}:${i + 1}: invalid JSON`);
      }
    });
}

async function readOptional(file: string): Promise<string> {
  try {
    return await fs.readFile(file, "utf8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") return "";
    throw err;
  }
}

export async function loadState(dir: string): Promise<State> {
  const companies = parseJsonl<CompanyRecord>(await readOptional(path.join(dir, "companies.jsonl")), "companies.jsonl");
  const jobs = parseJsonl<JobRecord>(await readOptional(path.join(dir, "jobs.jsonl")), "jobs.jsonl");
  const runs = parseJsonl<RunRecord>(await readOptional(path.join(dir, "runs.jsonl")), "runs.jsonl");
  const lastSeen = new Map<string, [string, string]>();
  for (const line of (await readOptional(path.join(dir, "last_seen.tsv"))).split("\n")) {
    const [id, seen, updated] = line.split("\t");
    if (id && seen && updated) lastSeen.set(id, [seen, updated]);
  }
  const jobMap = new Map<string, JobRecord>();
  for (const j of jobs) {
    const [seen, updated] = lastSeen.get(j.id) ?? [j.first_seen_at, j.created_at];
    jobMap.set(j.id, { ...j, last_seen_at: seen, updated_at: updated });
  }
  return { companies: new Map(companies.map((c) => [c.id, c])), jobs: jobMap, runs };
}

/** Problems that make a state unsafe to write; empty when it is sound. */
export function validateState(state: State): string[] {
  const problems: string[] = [];
  const slugs = new Set<string>();
  for (const c of state.companies.values()) {
    if (slugs.has(c.slug)) problems.push(`duplicate company slug ${c.slug}`);
    slugs.add(c.slug);
  }
  const fingerprints = new Set<string>();
  const prefixes = new Map<string, string>();
  for (const j of state.jobs.values()) {
    if (!state.companies.has(j.company_id)) problems.push(`job ${j.id} has unknown company ${j.company_id}`);
    if (fingerprints.has(j.fingerprint)) problems.push(`duplicate fingerprint ${j.fingerprint}`);
    fingerprints.add(j.fingerprint);
    if (j.is_active) {
      // Job URLs carry company + the id's first 8 hex digits; they must be unique.
      const key = `${j.company_id} ${j.id.replace(/-/g, "").slice(0, 8)}`;
      const other = prefixes.get(key);
      if (other) problems.push(`active jobs ${other} and ${j.id} share a URL prefix`);
      prefixes.set(key, j.id);
    }
  }
  return problems;
}

const line = (o: unknown) => `${JSON.stringify(o)}\n`;

export async function writeState(dir: string, state: State): Promise<void> {
  const problems = validateState(state);
  if (problems.length) throw new Error(`refusing to write state:\n  ${problems.slice(0, 20).join("\n  ")}`);
  const tmp = await fs.mkdtemp(path.join(path.dirname(path.resolve(dir)), ".state-"));
  const companies = [...state.companies.values()].sort((a, b) => byCodeUnit(a.slug, b.slug));
  const jobs = [...state.jobs.values()].sort((a, b) => byCodeUnit(a.id, b.id));
  const runs = [...state.runs].sort((a, b) => Date.parse(a.ran_at) - Date.parse(b.ran_at) || byCodeUnit(a.id, b.id));
  await fs.writeFile(path.join(tmp, "companies.jsonl"), companies.map((c) => line(ordered(c, COMPANY_KEYS))).join(""));
  await fs.writeFile(path.join(tmp, "jobs.jsonl"), jobs.map((j) => line(ordered(j, JOB_KEYS, JOB_VOLATILE))).join(""));
  await fs.writeFile(path.join(tmp, "last_seen.tsv"), jobs.map((j) => `${j.id}\t${j.last_seen_at}\t${j.updated_at}\n`).join(""));
  await fs.writeFile(path.join(tmp, "runs.jsonl"), runs.map((r) => line(ordered(r, RUN_KEYS))).join(""));
  await fs.mkdir(dir, { recursive: true });
  for (const f of ["companies.jsonl", "jobs.jsonl", "last_seen.tsv", "runs.jsonl"]) {
    await fs.rename(path.join(tmp, f), path.join(dir, f));
  }
  await fs.rm(tmp, { recursive: true, force: true });
}

export function newId(): string {
  return randomUUID();
}

/** Seed a state directory from scripts/export-state.sh JSONL output (row_to_json per table). */
export async function stateFromExport(exportDir: string): Promise<State> {
  const read = async (name: string) => readOptional(path.join(exportDir, name));
  const companies = parseJsonl<CompanyRecord>(await read("public.companies.jsonl"), "companies");
  const jobs = parseJsonl<JobRecord>(await read("public.jobs.jsonl"), "jobs");
  const runs = parseJsonl<RunRecord>(await read("public.ingest_runs.jsonl"), "ingest_runs");
  if (companies.length === 0) throw new Error(`no public.companies.jsonl in ${exportDir}`);
  return {
    companies: new Map(companies.map((c) => [c.id, c])),
    jobs: new Map(jobs.map((j) => [j.id, j])),
    runs,
  };
}
