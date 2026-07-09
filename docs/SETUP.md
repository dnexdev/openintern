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
| `DATABASE_URL` | Neon/Postgres connection string |
| `RESEND_API_KEY` | Alert emails (optional) |
| `ALERT_FROM_EMAIL` | e.g. `alerts@openintern.dev` |

## 3. Vercel (Hobby / free)

1. Import `dnexdev/openintern` at [vercel.com](https://vercel.com)
2. Framework: Next.js; ensure install/build use the monorepo (`pnpm install`, build `apps/web` — see root `vercel.json`)
3. Add env vars from `.env.example` (`DATABASE_URL` required; `AUTH_*` / Resend optional)
4. Domains → add **`openintern.dev`** and point DNS at Vercel
5. Ingest/dumps/alerts stay on **GitHub Actions** (not Vercel Cron) so Hobby limits are less of an issue

Hobby notes: fine for the board + rate-limited API. If you hit commercial-use or bandwidth caps later, upgrade or move the API worker elsewhere. Daily dumps remain the bulk path.

## 4. Database (Neon)

1. Create a free Neon project
2. Copy the connection string → `DATABASE_URL`
3. Locally or in CI: `pnpm db:migrate`
4. `pnpm sync-companies && pnpm ingest`

## 5. Auth (optional)

1. Generate `AUTH_SECRET`: `openssl rand -base64 32`
2. Create GitHub and/or Google OAuth apps
3. Callback URL: `https://openintern.dev/api/auth/callback/github` (and/or `google`)
4. Set `AUTH_URL=https://openintern.dev`

## 6. Enable workflows

In the repo Actions tab, ensure **Ingest**, **Dump**, and **Alerts** are allowed for scheduled runs.
