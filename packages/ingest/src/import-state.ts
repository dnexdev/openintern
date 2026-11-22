/**
 * Rollback path for phase D: load flat-file state back into Postgres, keeping
 * every id and timestamp. Run migrations first. Rows are upserted by id and
 * every column is overwritten, so importing into a database that still holds
 * older copies of the rows gives exactly the state (round trip is lossless).
 */
import { sql, type SQL } from "drizzle-orm";
import type { Db } from "@openintern/db";
import type { State } from "./state.js";

const CHUNK = 200;

// Timestamps go in as text so Postgres parses the stored strings itself
// (microseconds kept), not via JS Dates.
const ts = (s: string | null) => (s == null ? sql`null` : sql`${s}::timestamptz`);
const jsonb = (v: unknown) => sql`${JSON.stringify(v)}::jsonb`;

async function insertChunks<T>(db: Pick<Db, "execute">, rows: T[], head: SQL, row: (r: T) => SQL, tail: SQL) {
  for (let i = 0; i < rows.length; i += CHUNK) {
    const values = sql.join(rows.slice(i, i + CHUNK).map(row), sql`, `);
    await db.execute(sql`${head} values ${values} ${tail}`);
  }
}

export async function importState(db: Db, state: State): Promise<{ companies: number; jobs: number; runs: number }> {
  const cs = [...state.companies.values()];
  const js = [...state.jobs.values()];
  await db.transaction(async (tx) => {
    await insertChunks(
      tx,
      cs,
      sql`insert into companies (id, name, slug, ats, board_token, careers_url, website_url, active, created_at, updated_at)`,
      (c) => sql`(${c.id}, ${c.name}, ${c.slug}, ${c.ats}, ${c.board_token}, ${c.careers_url}, ${c.website_url},
        ${c.active}, ${ts(c.created_at)}, ${ts(c.updated_at)})`,
      sql`on conflict (id) do update set name = excluded.name, slug = excluded.slug, ats = excluded.ats,
        board_token = excluded.board_token, careers_url = excluded.careers_url, website_url = excluded.website_url,
        active = excluded.active, created_at = excluded.created_at, updated_at = excluded.updated_at`,
    );
    await insertChunks(
      tx,
      js,
      sql`insert into jobs (id, company_id, external_job_id, fingerprint, title, normalized_title, role_family_id,
        locations, apply_url, excerpt, terms, term_years, duration_months, cohort_year, roles, regions, is_remote,
        is_active, source, posted_at, first_seen_at, last_seen_at, created_at, updated_at)`,
      (j) => sql`(${j.id}, ${j.company_id}, ${j.external_job_id}, ${j.fingerprint}, ${j.title}, ${j.normalized_title},
        ${j.role_family_id}, ${jsonb(j.locations)}, ${j.apply_url}, ${j.excerpt}, ${jsonb(j.terms)},
        ${jsonb(j.term_years)}, ${jsonb(j.duration_months)}, ${j.cohort_year}, ${jsonb(j.roles)}, ${jsonb(j.regions)},
        ${j.is_remote}, ${j.is_active}, ${j.source}, ${ts(j.posted_at)}, ${ts(j.first_seen_at)}, ${ts(j.last_seen_at)},
        ${ts(j.created_at)}, ${ts(j.updated_at)})`,
      sql`on conflict (id) do update set company_id = excluded.company_id, external_job_id = excluded.external_job_id,
        fingerprint = excluded.fingerprint, title = excluded.title, normalized_title = excluded.normalized_title,
        role_family_id = excluded.role_family_id, locations = excluded.locations, apply_url = excluded.apply_url,
        excerpt = excluded.excerpt, terms = excluded.terms, term_years = excluded.term_years,
        duration_months = excluded.duration_months, cohort_year = excluded.cohort_year, roles = excluded.roles,
        regions = excluded.regions, is_remote = excluded.is_remote, is_active = excluded.is_active,
        source = excluded.source, posted_at = excluded.posted_at, first_seen_at = excluded.first_seen_at,
        last_seen_at = excluded.last_seen_at, created_at = excluded.created_at, updated_at = excluded.updated_at`,
    );
    // Runs are append-only history.
    await insertChunks(
      tx,
      state.runs,
      sql`insert into ingest_runs (id, company_id, status, job_count, error, ran_at)`,
      (r) => sql`(${r.id}, ${r.company_id}, ${r.status}, ${r.job_count}, ${r.error}, ${ts(r.ran_at)})`,
      sql`on conflict (id) do nothing`,
    );
  });
  return { companies: cs.length, jobs: js.length, runs: state.runs.length };
}
