## Summary

<!-- What does this PR change and why? -->

## Company registry checklist

- [ ] `website_url` set (required for logos)
- [ ] `pnpm validate-tokens` passed on changed `data/companies/*.yaml` files
- [ ] `pnpm validate-companies` passes (no duplicate slugs or ATS tokens)
- [ ] Tech-focused employer with a public ATS board (no LinkedIn scraping)

## Test plan

- [ ] `pnpm typecheck`
- [ ] `pnpm test` (if ingest/classifier touched)
- [ ] Local smoke: `pnpm dev` or relevant CLI command
