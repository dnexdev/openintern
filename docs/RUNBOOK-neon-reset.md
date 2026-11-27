# Neon quota reset: export first

Neon's free quota ran out on 2026-10-24. It reset on 2026-11-01, ran out again
on 2026-11-14 after 13 days of DB-first web reads plus daily ingest, and the next
reset is expected on **2026-12-01** (the 1st of each month). Company and job
UUIDs, `first_seen_at`, inactive jobs and ingest history exist only there, so
the first thing to do when it is readable again is copy everything out.

## On reset day, before 12:00 UTC (the daily ingest)

1. Actions → **Export state** → Run workflow.
2. When it finishes, read the run summary: every table has a row count,
   `fingerprint_mismatch` and `source_ne_company_ats` are reported (both are
   informational; record the numbers).
3. Download the `openintern-state-<run id>` artifact and keep a second copy off
   GitHub. Check it with `sha256sum -c SHA256SUMS`.
4. Seed the state repo from it and switch ingest off Neon:
   `RUNBOOK-phase-d.md`.

If ingest runs first anyway, it no longer retires outage-era jobs: the last-seen
sweep is skipped when the previous successful run is more than 2 days old.

## After the first ingest that follows

Companies merged on 2026-10-24 (data/company-aliases.tsv) get new job ids for
postings that moved to the surviving company. Once ingest has run:

```sh
DATABASE_URL='postgres://…' pnpm job-aliases && pnpm gen-aliases
```

and commit `data/job-aliases.tsv` plus the regenerated module, so old links and
applied marks follow.

## Laptop fallback (historical)

`scripts/export-state.sh` and `.github/workflows/export-state.yml` were removed
after the state repo cutover (2026-11-27). If you still have a Neon snapshot,
use an old checkout of that script, or re-run the SQL export by hand against a
backup.

## What the export holds

- `public.companies.csv`, `public.jobs.csv` (active and inactive),
  `public.ingest_runs.csv`, and `drizzle.*` migration history.
- The same rows as `*.jsonl` (`row_to_json`), the input for flat-file state
  (plan phase D).
- Timestamps as Postgres prints them in UTC, microseconds included.
- `counts.tsv`, `server-version.txt`, `SHA256SUMS`, and `neon.dump` when
  `pg_dump` matches the server version.
