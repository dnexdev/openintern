# Runbook

One maintainer, often on a phone. Each section starts with the fastest step
that contains the problem; fix the cause afterwards. Log every incident in
[INCIDENTS.md](INCIDENTS.md) with the times it was introduced, detected and
contained.

## Tools you can use from a phone

| Lever | Effect | How | Delay |
|---|---|---|---|
| Kill switch file | Hide a job, company or apply host; or maintenance mode | Edit `killswitch.json` in the `openintern-ops` repo in the GitHub app | ≤ 10 min |
| `OPENINTERN_MAINTENANCE=1` | Whole site and API answer 503 (status stays up) | Vercel → Settings → Environment Variables, then Redeploy | ~2 min |
| `JOBS_DUMP_URL` | Serve a known-good snapshot instead of `dump-latest` | Set to `https://github.com/dnexdev/openintern/releases/download/dump-YYYY-MM-DD/jobs.json`, then Redeploy | ~2 min |
| Disable Ingest / Dump | Stop new data | Actions → workflow → ⋯ → Disable workflow | immediate |
| Instant rollback | Previous production deploy | Vercel → Deployments → ⋯ → Promote | ~1 min |

Kill switch format (`OPENINTERN_KILL_SWITCH_URL` points at the raw file):

```json
{ "jobs": ["<job uuid>"], "companies": ["<slug>"], "hosts": ["evil.example"],
  "maintenance": false, "message": "optional text for the maintenance page" }
```

If the file can't be read, the last good copy stays in force. Entries that must
outlive it go in `packages/db/src/delisted.ts` (needs a PR and deploy).

## Phishing or scam listing

1. Kill switch: add the job id, and the company or apply host if more than one
   listing is affected.
2. Check `data/apply-hosts.tsv` and the company's YAML entry: who added the
   host or board token, and was the employer verified? Remove the row.
3. Add the job id to `packages/db/src/delisted.ts` so it stays down in the next
   dump and for mirrors (`denylist.json`).
4. If the dump already carries it, delete the affected dated snapshots
   (`gh release delete dump-YYYY-MM-DD --cleanup-tag`) and let the next run
   replace `dump-latest`.

## Bad data (wrong, stale or missing listings at scale)

1. If the latest dump is wrong, pin `JOBS_DUMP_URL` to the last good dated
   snapshot.
2. The circuit breaker holds releases that swing more than 10% overall or 15%
   (and more than 10 jobs) per ATS. If a real change was held, approve it in
   `data/expected-delta.json` with a reason and an expiry.
3. Fix the adapter or classifier; let the next run publish.

## Account compromise (GitHub, Vercel, Google)

1. From another device: change the password, revoke sessions, revoke tokens
   (GitHub → Settings → Applications and Developer settings; Vercel → Tokens).
2. Maintenance mode if anything was published.
3. Audit: GitHub security log, recent pushes to `main`, releases, rulesets,
   Actions secrets; Vercel deployments and environment variables.
4. Rotate deploy keys and any remaining Actions/Vercel secrets.

## API or site down

1. `GET /api/v1/status`: 503 "stale" means the dump is over 30 hours old (see
   below); "down" means the dump can't be fetched.
2. Vercel status and deployment logs (Hobby keeps 1 hour).
3. Roll back the last deploy if it started with it.

## Stale dump (status 503 "stale")

1. Actions → Ingest and Dump: did they run? GitHub disables scheduled
   workflows after 60 days without repository activity; re-enable them.
2. A held dump-gate run says why; approve or fix, then run Dump by hand.
3. Confirm `vars.OPENINTERN_STATE_REPO` is readable and the last Ingest
   `publish-state` job succeeded.

## Compromised action, package or toolchain

1. Disable the affected workflows.
2. Pin to the last good SHA or version; check `pnpm-lock.yaml` diffs and the
   Bend sha256 in `ci.yml`.
3. Treat secrets the job could read as leaked: rotate them.
4. Check releases and `main` for unexpected changes since the compromise.

## Takedown request

Act only on requests from the company's own domain (see DATA.md).

1. Kill switch for the listing or company, then the delist registry (or set the
   company `active: false` in YAML.
2. Delete dated snapshots that carry it.
3. Log it in INCIDENTS.md (date, requester domain, what was removed).

## Freeze periods (Dec 16 – Jan 4, Jul 15 – Dec 15)

Held items stay held (fail closed); one digest issue collects them. Before a
freeze, drill the phone-only steps above once and confirm the uptime monitor
pages the phone.
