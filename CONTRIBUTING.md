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
   - **Workday:** `https://{tenant}.{wdN}.myworkdayjobs.com/{site}` → `board_token: tenant|wdN|site` (e.g. `nvidia|wd5|NVIDIAExternalCareerSite`). Uses public CXS JSON (`POST .../wday/cxs/.../jobs`).
   - **Platform adapters:** `phenom` (`amd` → careers.amd.com), `avature` (`bloomberg`), `icims` (`careers-sig`), `eightfold` (`microsoft` or `microsoft|ai`).
   - **Named proprietary:** `citadel` / `citadel_securities` (`open-opportunities`), `tesla` (`careers`), `bytedance` (`en`), `tiktok` (`tiktok`), `google` (`google`|`deepmind`), `ibm` (`ibmglobal`), `deshaw` (`internships`), `shopify` (`careers`), `x` (`x`). See [Custom careers](#custom-careers-proprietary-adapters).
2. Add an entry to a file under `data/companies/` (or create `data/companies/your-org.yaml`):

```yaml
companies:
  - name: Example Corp
    slug: example-corp
    ats: greenhouse   # greenhouse | lever | ashby | workable | smartrecruiters | recruitee | rippling | bamboohr | workday | phenom | avature | icims | eightfold | google | ibm | deshaw | shopify | x | citadel | …
    board_token: example
    careers_url: https://example.com/careers
    website_url: https://example.com
    active: true
```

Workday example:

```yaml
  - name: NVIDIA
    slug: nvidia
    ats: workday
    board_token: "nvidia|wd5|NVIDIAExternalCareerSite"
    careers_url: https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite
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
2. Run **`pnpm recover-tokens`** (Greenhouse–BambooHR; Workday tokens are not guessable) for inactive or broken boards; add `--write` to patch YAML locally after review.
3. Fix `data/companies/*.yaml` (`board_token`, `ats`, `active: true`).
4. Validate with **`pnpm validate-tokens`** on changed files, then **`pnpm ingest`** locally or wait for the hourly Action.

`zero_match` means the board returned jobs but none passed the tech-internship classifier. Existing jobs that are no longer on the board are retired; ones still listed are kept unless their details loaded and they no longer match.

### Accelerator discovery

```bash
pnpm discover-accelerators -- --yc --write --limit=120
pnpm discover-accelerators -- --a16z --write --limit=60
```

Writes probe-OK boards to `data/companies/tech-accelerators.yaml`. YC uses the public [yc-oss hiring dump](https://yc-oss.github.io/api/companies/hiring.json); a16z uses a curated portfolio seed (site is JS-rendered).

### Custom careers (proprietary adapters)

Named adapters for employers without Greenhouse/Lever/Ashby/Workday boards.

**Honest requests only.** Ingest identifies itself as `OpenIntern/0.1 (+repo)`
on every request and never sends a browser user-agent, a forged `Referer` or
`Origin`, or cookies (`packages/ingest/src/http.ts` refuses them, and CI greps
for them). If a site refuses that, the answer is a manual capture or dropping
the source, never a workaround.

| `ats` | `board_token` | Source | Notes |
|-------|---------------|--------|-------|
| `citadel` | `open-opportunities` | manual capture (`data/manual/citadel.json`) | CDN refuses automated clients |
| `citadel_securities` | `open-opportunities` | manual capture | Same |
| `tesla` | `careers` | manual capture (`data/manual/tesla.json`) | Careers API refuses automated clients |
| `bytedance` | `en` | `jobs.bytedance.com/.../supplier/search/job/posts` | Campus facet (`recruitment_id=2`); honest-request test in shadow, see the plan |
| `tiktok` | `tiktok` | `api.lifeattiktok.com/.../supplier/search/job/posts` | Same protocol; apply links on `lifeattiktok.com/search/{id}` |
| `phenom` | `amd` | `careers.{token}.com/api/jobs` | Phenom JSON (AMD, etc.) |
| `avature` | `bloomberg` | `{token}.avature.net/careers/SearchJobs/` | HTML parse |
| `icims` | `careers-sig` | `{token}.icims.com/jobs/search` | HTML parse |
| `eightfold` | `microsoft` or `microsoft\|ai` | PCSX search on apply.careers.microsoft.com | AI filter for microsoft-ai |
| `google` | `google` / `deepmind` | Google careers applications HTML | Company filter for DeepMind |
| `ibm` | `ibmglobal` | IBM Avature SearchJobs | Same Avature parser |
| `deshaw` | `internships` | `deshaw.com/careers/internships` `__NEXT_DATA__` | Internship roles |
| `shopify` | `careers` | `shopify.com/careers` job cards | Public listings only (many internships gated) |

Manual captures are described in [`data/manual/README.md`](data/manual/README.md):
save the public page in a normal browser, run `pnpm manual:capture`, commit the
normalized JSON. Captures older than 14 days are not published.

A refusal (401/403), throttling (429 after retries) or any other non-2xx fails
that company's run, so it never looks like an empty board. Failed runs retire
nothing.

Do **not** add LinkedIn scrapers, browser automation, or X (its terms forbid
scraping; removed 2026-10-24).

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
pnpm gap-report -- --ats --write   # Simplify Apply URLs → board tokens (incl. Workday)
pnpm discover-accelerators -- --yc --a16z --write --limit=100
pnpm recover-tokens                # probe inactive brands across Greenhouse–BambooHR
pnpm validate-tokens -- --all
pnpm validate-curated    # Tier 1 slug list vs data/companies/
pnpm validate-companies  # duplicate slugs/tokens (duplicate tokens are errors)
pnpm backfill-role-families  # after normalizeTitle changes, refresh role_family_id in DB
```

After changing title normalization rules, run `pnpm backfill-role-families` once so existing postings regroup on the board.

- `pnpm typecheck` before pushing
- Don’t commit `.env` or secrets

## Code of conduct

Be respectful. No harassment, spam PRs, or malicious board tokens. See [docs/CODE_OF_CONDUCT.md](docs/CODE_OF_CONDUCT.md).
