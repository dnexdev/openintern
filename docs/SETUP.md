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
3. Add env vars from `.env.example` (`DATABASE_URL` required; optional `API_RATE_LIMIT_PER_MINUTE`)
4. Domains → add **`openintern.dev`** and point DNS at Vercel
5. Ingest/dumps stay on **GitHub Actions** (not Vercel Cron)

Hobby notes: fine for the board + rate-limited API. Daily dumps remain the bulk path.

## 4. Database (Neon)

1. Create a free Neon project
2. Copy the connection string → `DATABASE_URL`
3. Locally or in CI: `pnpm db:migrate`
4. `pnpm sync-companies && pnpm ingest`

## 5. Enable workflows

In the repo Actions tab, ensure **Ingest** and **Dump** are allowed for scheduled runs. After setting `DATABASE_URL`, manually run Dump once so `dump-latest` has `jobs.json` / `jobs.csv`.
