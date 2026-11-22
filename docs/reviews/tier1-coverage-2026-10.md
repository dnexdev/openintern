# Tier-1 coverage review, 2026-10-24

Tier-1 companies with no listings in the 2026-10-23 dump, probed with honest
requests (OpenIntern user-agent) on 2026-10-24.

| Company | Board | Registry | Probe |
|---|---|---|---|
| openai | ashby `openai` | active | board OK, 794 postings |
| anthropic | greenhouse `anthropic` | active | board OK, 681 postings |
| jane-street | greenhouse `janestreet` | active | board OK, 220 postings |
| airbnb | greenhouse `airbnb` | active | board OK, 191 postings |
| deepmind | google `deepmind` | active | board OK, 13 postings |
| optiver | greenhouse `optiver` | active | board OK, 0 postings |
| citadel | manual capture | active | no `data/manual/citadel.json` yet |
| tesla | manual capture | active | no `data/manual/tesla.json` yet |
| groq | ashby `groq` | inactive | 404 |
| mistral | ashby `mistral` | inactive | 404 |
| two-sigma | greenhouse `twosigma` | inactive | 404 |
| hrt | greenhouse `hudsonrivertrading` | inactive | 404 |
| netflix | greenhouse `netflix` | inactive | 404 |
| uber | greenhouse `uber` | inactive | 404 |

## Findings

- **Boards that work but publish nothing (openai, anthropic, jane-street,
  airbnb, deepmind):** the fetch succeeds, so either none of their postings are
  internships right now or the classifier rejects them. Check the next ingest
  run's funnel for these companies (`/health`, `fetched → title → tech`). A high
  `title` count with `tech=0` means the classifier is too strict for them.
- **optiver:** the board is empty today; nothing to do.
- **citadel, tesla:** need a first manual capture (`data/manual/README.md`).
- **Six inactive entries return 404:** their board tokens are wrong or the
  companies moved ATS. Run `pnpm recover-tokens` for them; Netflix and Uber are
  known to use other systems (Netflix: Eightfold-style careers site; Uber:
  its own careers site) and may need a manual capture or removal from tier-1.

## Decisions needed

- Keep a tier-1 company with no working board, or drop it from
  `data/curated/tier-1.yaml` until it has one.
