# Phase D cutover: Neon → flat-file state

State moves from Neon to a separate git repository (`openintern-state`):
`companies.jsonl`, `jobs.jsonl` (every job ever seen), `last_seen.tsv` and
`runs.jsonl` (last 60 days). Ingest and Dump switch on the repository variable
`OPENINTERN_STATE`; until it is `files`, they keep using Neon.

How the workflows treat the state repo:
- **Ingest job** (installs npm): reads the state repo, runs
  `cli.ts ingest-state`, and uploads the new state as an artifact. It holds no
  write credential.
- **`publish-state` job** (installs nothing, `state-publish` environment):
  - refuses if the state repo moved during the run;
  - runs `scripts/state-gate.mjs` (ids, slugs and fingerprints unique; ids,
    companies and `first_seen_at` never change; no job deleted; at most 200 new
    jobs per company; no company with 5+ active jobs drops to 0 unless approved
    in `data/expected-delta.json`);
  - pushes with the deploy key.
- **Dump** builds from the state repo's HEAD with `cli.ts dump-state`.

## Owner setup (once, before reset day)

1. Create the repository `openintern-state`. Public is simplest: the same data
   is already published in the dumps, and reads then need no credential. If
   you make it private, add a second, read-only deploy key as the Actions
   secret `STATE_READ_KEY`.
2. Generate a deploy key and add it with write access:
   ```sh
   ssh-keygen -t ed25519 -N '' -C openintern-state-publish -f state_key
   gh repo deploy-key add state_key.pub --repo <owner>/openintern-state --allow-write --title publish-state
   ```
3. In openintern, under Settings → Environments, create `state-publish`.
   - Deployment branches: `main` only.
   - Secret `STATE_DEPLOY_KEY`: the contents of `state_key`.
   - Then delete `state_key` locally.
4. Under Settings → Variables, set `OPENINTERN_STATE_REPO` to
   `<owner>/openintern-state`. Don't set `OPENINTERN_STATE` yet.
5. Optional: a ruleset on openintern-state that blocks force pushes and branch
   deletion.

## Reset day

1. **Export.** Run **Export state** (see `RUNBOOK-neon-reset.md`). Download the
   artifact, run `sha256sum -c SHA256SUMS`, and keep a copy off GitHub.
2. **Seed.** Run this from the openintern checkout. `$W` is an absolute path,
   because the CLI runs in `packages/ingest`.
   ```sh
   W=$(mktemp -d)
   gh run download <run-id> -n openintern-state-<run-id> -D "$W/export"
   (cd "$W/export" && sha256sum -c SHA256SUMS)
   git clone git@github.com:<owner>/openintern-state.git "$W/state"
   pnpm --filter @openintern/ingest exec tsx src/cli.ts state-from-export "$W/export" "$W/state"
   node scripts/state-gate.mjs "$W/state" "$W/state"   # job/company counts match export/counts.tsv
   git -C "$W/state" add -A && git -C "$W/state" commit -m "seed from Neon export $(date -u +%F)"
   git -C "$W/state" push
   ```
3. **Rehearse.**
   - Run Ingest by hand with **files_dry_run** ticked. It ingests against the
     state repo and uploads `state-<run id>`, but publishes nothing.
   - Read the summary. Optionally download the artifact and run
     `node scripts/state-gate.mjs "$W/state" <artifact dir>`.
   - The first run after a gap skips the 14-day last-seen sweep, as in DB mode.
4. **Switch.** Set the variable `OPENINTERN_STATE` to `files`. From the next
   run on, Ingest writes the state repo and Dump reads it. No workflow touches
   Neon any more, apart from a manual Export state.
5. **First dump after a gap.**
   - Expect the dump gate to hold ATSes whose counts fell while ingest was down.
     A rehearsal from the 2026-11-13 state held citadel 17 → 4, tiktok 336 → 273
     and bytedance 135 → 113.
   - If a drop is real, add an `ats:<name>` approval to
     `data/expected-delta.json` and re-run Dump.

## Retiring Neon

- Leave the database untouched (read-only) for 30 days after the switch.
- Then delete `DATABASE_URL` from the Actions secrets and from Vercel. That
  also turns off the web's DB fallback, since `withDumpFirst` then has nothing
  to fall back to.
- Removing drizzle-orm, postgres and the Export state workflow is a follow-up
  PR.

## Rollback

1. Delete the variable `OPENINTERN_STATE`. Ingest and Dump use Neon again.
2. Before the next ingest, load the state into Neon:
   ```sh
   DATABASE_URL='postgres://…' pnpm db:migrate
   DATABASE_URL='postgres://…' pnpm --filter @openintern/ingest exec tsx src/cli.ts import-state /abs/path/to/openintern-state
   ```

The import upserts by id and overwrites every column, all in one transaction.
Runs are append-only, so Neon keeps any history that is older than the state's
60 days.

This was verified on 2026-11-22 against Postgres 17, starting from the
2026-11-13 dump plus synthetic rows (inactive jobs; tabs, newlines, quotes and
emoji; NULL vs ""). The path covered was export → state → import → export, a
live file-mode ingest, then import over the older rows → export. Companies,
jobs and `last_seen` came back byte-identical, including `first_seen_at`
microseconds. File-mode ingest writes timestamps and keys exactly as
`row_to_json` prints them.
