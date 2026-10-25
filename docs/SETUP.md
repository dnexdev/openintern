# Setup guide

## 1. Domain

1. Register **`openintern.dev`** (confirm checkout is standard `.dev` pricing, not premium).
2. Optional: register **`openintern.ca`** (Canadian Presence Requirement — citizens/PRs eligible) and 301 to `.dev`.
3. Skip aftermarket `intern.dev`.

## 2. GitHub repo

Already created: `https://github.com/dnexdev/openintern` (public, Apache-2.0).

Suggested topics: `internships`, `open-data`, `job-board`, `typescript`, `nextjs`, `greenhouse`.

Homepage: `https://openintern.dev`

### Secrets (Settings → Secrets and variables → Actions)

| Secret | Purpose |
|--------|---------|
| `DATABASE_URL` | Neon/Postgres connection string (**required** for Ingest + Dump workflows) |

Without `DATABASE_URL`, scheduled Ingest/Dump runs fail immediately. Add the same Neon URL you use locally / on Vercel.

```bash
# From a machine with gh auth:
gh secret set DATABASE_URL --body "$DATABASE_URL"
```

Then re-run: Actions → Ingest → Run workflow (and Dump once dumps are empty).

## 3. Vercel (Hobby / free)

1. Import `dnexdev/openintern` at [vercel.com](https://vercel.com)
2. Framework: Next.js; root `vercel.json` sets install/build for the monorepo
3. Add env vars from `.env.example`:
   - `DATABASE_URL` — use Neon’s **pooled** (`-pooler`) connection string (needed for `/health` and dump→DB fallback)
   - Optional: `API_RATE_LIMIT_PER_MINUTE`
   - Optional: `NEXT_PUBLIC_PLAUSIBLE_DOMAIN=openintern.dev` to enable privacy-friendly traffic analytics (see below)
   - Optional: `OPENINTERN_PREFER_DB=1` only if you need live-DB-first reads (burns Neon CU; default is dump-first)
4. Domains → add **`openintern.dev`** and point DNS at Vercel
5. Ingest/dumps stay on **GitHub Actions** (not Vercel Cron)

Hobby notes: the board and public job APIs read the daily GitHub `dump-latest` release by default so Neon can autosuspend between ingest runs. Daily dumps remain the bulk path.

### Traffic analytics (Plausible)

The site already loads Plausible when the env var is set (no cookies, no Postgres writes).

1. Create a site for `openintern.dev` at [plausible.io](https://plausible.io) (or self-host)
2. In Vercel → Project → Settings → Environment Variables, set `NEXT_PUBLIC_PLAUSIBLE_DOMAIN=openintern.dev`
3. Redeploy

Until that is set, Vercel’s project logs/analytics give a rough traffic picture.

## 4. Database (Neon)

1. Create a free Neon project
2. Copy the connection string → `DATABASE_URL` (pooled URL on Vercel; direct URL is fine for GitHub Actions ingest)
3. In the Neon console: confirm **autosuspend** is enabled and compute is the smallest size — public reads no longer keep the DB awake
4. Locally or in CI: `pnpm db:migrate`
5. `pnpm sync-companies && pnpm ingest`

## 5. Enable workflows

In the repo Actions tab, ensure **Ingest** and **Dump** are allowed for scheduled runs. After setting `DATABASE_URL`, manually run Dump once so `dump-latest` has `jobs.json` / `jobs.csv`.
