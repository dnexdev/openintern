# Setup guide

Production is a single Go binary served on Vercel. Job data comes from the
daily GitHub dump; ingest state lives in a separate flat-file state repo.
There is no Next.js app and no database on the request path.

## 1. Domain

1. Register **`openintern.dev`** (confirm checkout is standard `.dev` pricing, not premium).
2. Optional: register **`openintern.ca`** (Canadian Presence Requirement — citizens/PRs eligible) and 301 to `.dev`.
3. Skip aftermarket `intern.dev`.

## 2. GitHub repo

Already created: `https://github.com/dnexdev/openintern` (public, Apache-2.0).

Suggested topics: `internships`, `open-data`, `job-board`, `go`, `greenhouse`.

Homepage: `https://openintern.dev`

### Actions variables and secrets

| Name | Kind | Purpose |
|------|------|---------|
| `OPENINTERN_STATE_REPO` | variable | `owner/name` of the flat-file state repository |
| `STATE_READ_KEY` | secret (optional) | read-only deploy key if the state repo is private |
| `STATE_WRITE_KEY` / publish secrets | secrets | as required by the Ingest `publish-state` and Dump publish jobs (see the workflow files) |

Neon/`DATABASE_URL` is only used by the one-shot `export-state` migration workflow. Delete that workflow and the secret once the state repo holds migrated state.

```bash
gh variable set OPENINTERN_STATE_REPO --body "dnexdev/openintern-state"
```

## 3. Vercel (Hobby / free)

1. Import `dnexdev/openintern` at [vercel.com](https://vercel.com).
2. Root [`vercel.json`](../vercel.json) already selects the Go function
   (`api/index.go` → `server.Handler()`), skips pnpm install, and rewrites
   every path to that handler. Do **not** set the framework to Next.js.
3. Environment variables (see [`.env.example`](../.env.example)):
   - Optional: `JOBS_DUMP_URL` (defaults to the `dump-latest` release)
   - Optional: `HEALTH_SNAPSHOT_URL`
   - Optional: `OPENINTERN_KILL_SWITCH_URL`
   - Optional: `OPENINTERN_MAINTENANCE=1` for a full-site 503
   - Optional: `API_RATE_LIMIT_PER_MINUTE` (default 60; empty string denies all)
   - Optional: `OPENINTERN_APPLY_HOSTS` (`warn` default, `enforce`, or `off`)
   - Optional: `OPENINTERN_ANALYTICS=vercel` to load Vercel Web Analytics
4. Domains → add **`openintern.dev`** and point DNS at Vercel.
5. Ingest/dumps stay on **GitHub Actions** (not Vercel Cron).

The function loads the dump into memory and refreshes it about hourly. Cold
starts fetch `JOBS_DUMP_URL` once per instance.

### Local parity

```bash
go run ./cmd/openintern serve -addr 127.0.0.1:3000
# or a local dump:
go run ./cmd/openintern serve -dump ./jobs.json -addr 127.0.0.1:3000
```

## 4. Enable workflows

In the repo Actions tab, allow **Ingest** and **Dump** for scheduled runs.
After the state repo is readable, run Dump once so `dump-latest` has
`jobs.json` / `jobs.csv`.
