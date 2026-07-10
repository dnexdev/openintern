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

## Development

```bash
pnpm install
docker compose up -d
pnpm db:migrate
pnpm sync-companies
pnpm ingest:once
pnpm dev
```

- `pnpm typecheck` before pushing
- Don’t commit `.env` or secrets

## Code of conduct

Be respectful. No harassment, spam PRs, or malicious board tokens. See [docs/CODE_OF_CONDUCT.md](docs/CODE_OF_CONDUCT.md).
