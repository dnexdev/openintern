# OpenIntern

**Open-source tech internship corpus** — free structured listings, public API, no-account board, community company registry.

> Browse without signing up. Build on the API. Add companies with a PR. Listings are never paywalled.

- **Site (planned):** [openintern.dev](https://openintern.dev)
- **Repo:** [github.com/dnexdev/openintern](https://github.com/dnexdev/openintern)
- **License:** Apache-2.0

## What this is

OpenIntern is an **open data layer** for tech internships (SWE, data, AI/ML, quant, hardware, PM):

| Surface | Policy |
|---------|--------|
| Corpus | Free forever; daily JSON/CSV dumps |
| API | `GET /api/v1/jobs`, `/companies`, `/health` (rate-limited hosted) |
| Board | Search/filter with **no account** |
| Accounts | Optional — saved jobs + alert digests |
| Apply | Always outbound to the employer |

We poll public **Greenhouse / Lever / Ashby / Workable / SmartRecruiters** job board APIs. We do **not** scrape LinkedIn, sell recruiter emails, or gate listings.

## Quick start (local)

```bash
# prerequisites: Node 20+, Docker, pnpm 9
corepack enable && corepack prepare pnpm@9.15.0 --activate

cp .env.example .env
docker compose up -d
pnpm install
pnpm db:migrate
pnpm sync-companies
pnpm ingest          # polls ATS boards (takes a while on first run)
pnpm dev             # http://localhost:3000
```

## Public API

Full reference: **[/docs](https://openintern.dev/docs)** (also at `/docs` locally).

```bash
# List active internships
curl "http://localhost:3000/api/v1/jobs?q=software&limit=10"

# Filter
curl "http://localhost:3000/api/v1/jobs?location=Toronto&remote=true"

# Filter by internship term, duration, and cohort year
curl "http://localhost:3000/api/v1/jobs?season=summer,fall&duration_months=4"
curl "http://localhost:3000/api/v1/jobs?cohort_year=2026"

# Single job
curl "http://localhost:3000/api/v1/jobs/{id}"

# Companies in the registry
curl "http://localhost:3000/api/v1/companies"

# Pipeline health
curl "http://localhost:3000/api/v1/health"
```

Query params for `/api/v1/jobs`: `q`, `location`, `company` (slug), `remote`, `season` (`winter|spring|summer|fall`, repeatable or comma-separated), `duration_months`, `cohort_year`, `posted_after`, `page`, `limit` (max 100).

For bulk use, prefer **daily dumps** (stable URLs):

- https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.json
- https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.csv

Or run `pnpm dump` / self-host — don’t paginate the hosted API all day.

## Monorepo layout

```
apps/web            Next.js board + /api/v1
packages/db         Drizzle schema + migrations
packages/ingest     ATS pollers, classifier, dumps, alerts
data/companies      Community YAML registry (PR to add)
```

See [docs/SETUP.md](docs/SETUP.md).

## Contributing companies

Add a YAML entry under `data/companies/` — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Non-goals (v1)

LinkedIn scraping, contact/email finders, AI resume studio, Selenium farms, non-tech majors, paywalled listings.
