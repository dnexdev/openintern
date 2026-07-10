# OpenIntern

<p align="center">
  <img src="docs/brand/wordmark.svg#gh-light-mode-only" alt="OpenIntern" width="420" />
  <img src="docs/brand/wordmark-dark.svg#gh-dark-mode-only" alt="OpenIntern" width="420" />
</p>

<p align="center">
  <img src="docs/brand/icon.svg" alt="" width="28" />
  &nbsp;
  <strong>Open-source tech internship corpus</strong> — free structured listings, public API, no-account board, community company registry.
</p>

> Browse without signing up. Build on the API. Add companies with a PR. Listings are never paywalled.

- **Site:** [openintern.dev](https://openintern.dev)
- **Repo:** [github.com/dnexdev/openintern](https://github.com/dnexdev/openintern)
- **License:** Apache-2.0

## What this is

OpenIntern is an **open data layer** for tech internships (SWE, data, AI/ML, quant, hardware, PM):

| Surface | Policy |
|---------|--------|
| Corpus | Free forever; daily JSON/CSV dumps |
| API | `GET /api/v1/jobs`, `/companies`, `/health` (rate-limited hosted) |
| Board | Filter by role / region / term / duration — **no account** |
| Applied | Local browser only (`localStorage`) |
| Apply | Always outbound to the employer |

We poll public **Greenhouse / Lever / Ashby / Workable / SmartRecruiters / Recruitee / Rippling / BambooHR** job board APIs. We do **not** scrape LinkedIn, sell recruiter emails, or gate listings.

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
curl "https://openintern.dev/api/v1/jobs?role=software&limit=10"

# Filter by region + season
curl "https://openintern.dev/api/v1/jobs?region=canada&season=fall,winter"

# Duration overlap (e.g. 4–6 month postings match 4 or 6)
curl "https://openintern.dev/api/v1/jobs?duration_months=4,6"

# Single job
curl "https://openintern.dev/api/v1/jobs/{id}"

# Companies in the registry
curl "https://openintern.dev/api/v1/companies"

# Pipeline health
curl "https://openintern.dev/api/v1/health"
```

Query params for `/api/v1/jobs`: `q`, `company` (slug), `role`, `region` (`remote|us|canada|europe|other`), `season` (`summer|fall|winter`; `spring`→summer), `duration_months` (overlap), `posted_after`, `page`, `limit` (max 100). Board-compatible aliases `term` and `duration` are also accepted. List responses include `total_pages` and `has_more`; the hosted API defaults to 60 requests/minute/IP.

For bulk use, prefer **daily dumps** (stable URLs):

- https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.json
- https://github.com/dnexdev/openintern/releases/download/dump-latest/jobs.csv

Or run `pnpm dump` / self-host — don’t paginate the hosted API all day. Dumps use the same live-job freshness policy as the board and API.

## Monorepo layout

```
apps/web            Next.js board + /api/v1
packages/db         Drizzle schema + migrations
packages/ingest     ATS pollers, classifier, dumps
data/companies      Community YAML registry (PR to add)
docs/brand          Icon + wordmark SVGs
```

See [docs/SETUP.md](docs/SETUP.md).

## Contributing companies

Add a YAML entry under `data/companies/` — see [CONTRIBUTING.md](CONTRIBUTING.md). Always include `website_url` so logos resolve correctly.

## Non-goals (v1)

LinkedIn scraping, contact/email finders, AI resume studio, Selenium farms, non-tech majors, paywalled listings, Auth.js accounts.
