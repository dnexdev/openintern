# Contributing to OpenIntern

Thanks for helping grow the open tech internship corpus.

## Add a company (5 minutes)

1. Find the public ATS board token:
   - **Greenhouse:** `https://boards.greenhouse.io/{token}` or `boards-api.greenhouse.io/v1/boards/{token}/jobs`
   - **Lever:** `https://jobs.lever.co/{token}`
   - **Ashby:** `https://jobs.ashbyhq.com/{token}`
   - **Workable:** `https://apply.workable.com/{token}`
   - **SmartRecruiters:** `https://jobs.smartrecruiters.com/{token}` (case-sensitive)
   - **Recruitee:** `https://{token}.recruitee.com` → `https://{token}.recruitee.com/api/offers/`
   - **Rippling:** `https://ats.rippling.com/{token}` → `https://api.rippling.com/platform/api/ats/v1/board/{token}/jobs`
   - **BambooHR:** `https://{token}.bamboohr.com/careers` → `https://{token}.bamboohr.com/careers/list`
2. Add an entry to a file under `data/companies/` (or create `data/companies/your-org.yaml`):

```yaml
companies:
  - name: Example Corp
    slug: example-corp
    ats: greenhouse   # greenhouse | lever | ashby | workable | smartrecruiters | recruitee | rippling | bamboohr
    board_token: example
    careers_url: https://example.com/careers
    website_url: https://example.com
    active: true
```

3. Rules:
   - `slug` must be unique, lowercase, hyphens only
   - Prefer real public board tokens (CI validates changed YAML on PRs)
   - Always set `website_url` to the company marketing domain (logos depend on it)
   - Tech-focused employers only for now
   - Set `active: false` if the board is a duplicate or broken
4. Open a PR. Maintainers merge → hourly ingest picks it up.

## Curated tiers (job board highlights)

Tier 1 employers get a flame marker and warm border on the job board. The list lives in [`data/curated/tier-1.yaml`](data/curated/tier-1.yaml) — separate from ATS ingest metadata.

**Bar for Tier 1:** FAANG/MANGA-scale, frontier AI labs, top quant/HFT, or category-defining tech at that level. Solid mid-size employers stay off this list.

To add or remove a slug:

1. The company must already exist in `data/companies/` with a working board token.
2. Edit `data/curated/tier-1.yaml` and run `pnpm validate-curated`.
3. Open a PR with a one-line justification (e.g. “top-3 quant”, “frontier lab”).

Highlighting only appears when that company has active internships in the corpus.

## Ingest triage

When listings look stale or a company is missing jobs:

1. Check **`/health`** or `GET /api/v1/health` for `pipeline.recent_failures` and `pipeline.zero_match_companies`.
2. Run **`pnpm recover-tokens`** (all 8 ATSes) for inactive or broken boards; add `--write` to patch YAML locally after review.
3. Fix `data/companies/*.yaml` (`board_token`, `ats`, `active: true`).
4. Validate with **`pnpm validate-tokens`** on changed files, then **`pnpm ingest`** locally or wait for the hourly Action.

`zero_match` means the board returned jobs but none passed the tech-internship classifier — existing rows are retained until the 14-day last-seen sweep.

## Development

```bash
pnpm install
docker compose up -d
pnpm db:migrate
pnpm sync-companies
pnpm ingest:once
pnpm dev
```

Pipeline helpers (ATS-only discovery — does not scrape LinkedIn):

```bash
pnpm gap-report          # missing companies vs SpeedyApply/Simplify (--top=30 for shorter list)
pnpm recover-tokens      # probe inactive brands across all 8 ATS APIs (--write to patch YAML)
pnpm validate-tokens -- --all
pnpm validate-curated    # Tier 1 slug list vs data/companies/
pnpm validate-companies  # duplicate slugs/tokens (duplicate tokens are errors)
```

- `pnpm typecheck` before pushing
- Don’t commit `.env` or secrets

## Code of conduct

Be respectful. No harassment, spam PRs, or malicious board tokens. See [docs/CODE_OF_CONDUCT.md](docs/CODE_OF_CONDUCT.md).
