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

## 3. Student Pack / Vercel

1. Open [education.github.com/pack](https://education.github.com/pack)
2. Redeem **Vercel Pro**
3. Import the `openintern` repo; root directory / monorepo: set project to `apps/web` or use Vercel monorepo settings
4. Add env vars from `.env.example` (`DATABASE_URL`, `AUTH_*`, etc.)
5. Optional before **2026-07-31**: DigitalOcean $200 credit (backup worker only)

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
