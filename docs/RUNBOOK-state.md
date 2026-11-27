# State repository

Ingest state lives in the private repository `dnexdev/openintern-state`:
`companies.jsonl`, `jobs.jsonl` (every job ever seen), `last_seen.tsv` and
`runs.jsonl` (last 60 days). Format: `internal/state`. It replaced Neon on
2026-11-27 and was seeded from the 2026-11-13 public dump with
`openintern import-dump` (Neon was over quota; its inactive-job and run
history was not carried over).

## How the workflows use it

- **Ingest job:** checks the state out read-only (`STATE_READ_KEY`), runs
  `openintern ingest`, and uploads the new state as an artifact. It holds no
  write credential.
- **`publish-state` job** (`state-publish` environment, `main` only):
  - refuses if the state repo moved during the run;
  - runs `openintern state-gate` (ids, slugs and fingerprints unique; ids,
    companies and `first_seen_at` never change; no job deleted; at most 200
    new jobs per company; no company with 5+ active jobs drops to 0 unless
    approved in `data/expected-delta.json`);
  - pushes with `STATE_DEPLOY_KEY`.
- **Dump:** builds from the state repo's HEAD with `openintern dump`, then
  the `dump-publish` job runs `openintern dump-gate` before releasing.

## Setup (done 2026-11-27)

- Repository variable `OPENINTERN_STATE_REPO=dnexdev/openintern-state`.
- Repository secret `STATE_READ_KEY`: a read-only deploy key on the state repo.
- Environment `state-publish` (branch `main`) with secret `STATE_DEPLOY_KEY`:
  a write deploy key on the state repo.
- Environment `dump-publish` (branch `main`).
- Rulesets on both repositories block force pushes and deleting `main`.

## When a gate holds

Read the run log: the gate names each problem. A real, expected swing (a
season ending, an employer moving ATS) is approved with a dated entry in
`data/expected-delta.json`; anything else is a bug to fix before publishing.

## When the digest says "every apply link dead"

The apply-link probe (`docs/STATE-MACHINES.md` §16) found every apply link of a
company listing 5+ jobs failing (strikes on all of them) and kept the jobs up
rather than trip the state gate. Open one link: if the employer moved its
careers site, fix the source (board token, ATS); if the jobs are really gone,
set `active: false` or add a dated `company:<slug>` entry to
`data/expected-delta.json`, which lets the next run retire them and the
state gate accept it.

## Rebuilding state from scratch

`openintern sync-companies <dir>` then
`openintern import-dump <jobs.json> <dir>`: job ids survive because dump jobs
are matched to live postings by apply URL. Jobs it cannot match are kept as
inactive.
