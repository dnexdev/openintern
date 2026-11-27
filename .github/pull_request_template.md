## Summary

<!-- What does this PR change and why? -->

## Company registry checklist

- [ ] `website_url` set (required for logos)
- [ ] `go run ./cmd/openintern validate-tokens <changed files>` passed on changed `data/companies/*.yaml` files
- [ ] `go run ./cmd/openintern validate-companies` passes (no duplicate slugs or ATS tokens)
- [ ] Tech-focused employer with a public ATS board (no LinkedIn scraping)

## Test plan

- [ ] `go vet ./...` and `go test ./...`
- [ ] `go run ./cmd/openintern proofs check` (if `LAWS.bend`, `PROOF.bend`, `packages/proven` or `internal/proven` touched)
- [ ] Local smoke: `go run ./cmd/openintern serve` or the relevant command
