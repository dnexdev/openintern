# Site workstream status

Owns `DESIGN.md`, `internal/site/**`, this file. Throwaway files in
`/tmp/openintern-site/` (outside the repo):

- `node_modules/` Playwright 1.49.1, driving the Chromium already in
  `~/.cache/ms-playwright/chromium-1148/chrome-linux/chrome`.
- `dev_test.go` + `overlay.json` + `serve.sh`: a dev server that runs inside
  the package through `go test -overlay` (nothing is added to the repo).
  `serve.sh 127.0.0.1:8101` serves the cached dump with "now" fixed at
  2026-11-14 12:00Z; `serve.sh 127.0.0.1:8102 late` moves "now" 60 h later
  (stale banner); `OI_DEV_DUMP=/nonexistent/jobs.json serve.sh …` gives the
  unavailable/error states. `health.json` there is a hand-made snapshot in the
  dump's `health.json` shape (the release publishes none yet).
- `shoot.mjs` (every page × 400/1280 × light/dark × JS on/off, full page),
  `view.mjs` (one viewport shot), `interact.mjs` (keyboard, instant filters,
  Applied toggle, hide applied, picker, back button, copy), `alias.mjs`
  (applied-mark alias migration).

## Done

- Step 1, audit (below).
- Step 2, `DESIGN.md`: the "green-bar batch report" direction (summary below).
- Step 3, templates and pages, all from the one token set in
  `static/site.css`: home, `/jobs` (GET form filters, role families,
  postings picker as `<details>`, pagination window, stale banner, report
  links), `/jobs/{uuid}` legacy redirect, `/jobs/{company}/{jobSlug}` (company
  alias 308, job alias 308, canonical, Open Graph article), `/docs`,
  `/health` (snapshot or dump fallback), `/privacy`, `/vs/intern-insider`,
  404, 503 error page, `/icon.svg`, `/favicon.ico` (PNG-in-ICO drawn on first
  request), `/opengraph-image` and `/twitter-image` (static 1200×630
  `static/og.png`, rendered once from `og/og.html`).
- Step 3 JS (`static/site.js`, no build step): instant filter apply (fetches
  the page as HTML and swaps the `[data-sync]` regions, focus stays put,
  history push/pop), Applied toggle in localStorage (`openintern:applied`,
  `openintern:hide-applied`, same keys as the TS) with the job-alias migration
  from `AppliedToggle.tsx` (map inlined as JSON from `data/job-aliases.tsv`),
  hide-applied, copy buttons. Without JS: the form submits, the Applied,
  Hide-applied and Copy controls stay hidden.
- Step 4: pages read `*board.Store` through a small interface (`reader` in
  `site.go`).
- 2026-11-26 resume: fixed the test build (`board.DumpJobAlias` →
  `model.DumpJob`), a wrong `timeAgo` expectation (3600 s is `1h ago` in the
  TS too), canonical-URL expectation for non-ASCII slugs (html/template
  percent-encodes them, which is the correct URL form); job links now go
  through the canonical company slug; docs parameter names no longer break
  mid-word; the error state no longer prints "0 roles"; the green bar keeps
  alternating when "Hide applied" hides rows.

## Ready APIs

```go
func site.Register(mux *http.ServeMux, store *board.Store) // page routes only, method-qualified, plus a catch-all 404 on "/"
func site.Static() fs.FS                                   // embedded assets; serve under site.AssetPrefix
const site.AssetPrefix = "/assets/"                        // URLs carry ?v=<hash>, so immutable caching is safe
const site.SiteURL = "https://openintern.dev"
```

`Register` reads `OPENINTERN_ANALYTICS=vercel` to add the Vercel Web
Analytics script (same as `layout.tsx`). Routes: `GET /{$}`, `/jobs`,
`/jobs/{company}`, `/jobs/{company}/{jobSlug}`, `/docs`, `/health`,
`/privacy`, `/vs/intern-insider`, `/icon.svg`, `/favicon.ico`,
`/opengraph-image`, `/twitter-image`, and `/` (404 page). `internal/web`
already serves `GET /assets/` from `Options.StaticFS`; pass `site.Static()`
and replace `site_stub.go` with a call to `site.Register`.

- Step 5, verification (2026-11-26), see "Verification evidence".

## In progress

Nothing. The workstream is complete pending the lead's wiring in
`internal/web` and the board request below.

## Blocked on

Nothing.

## Requests

- **webapi (`internal/board`)**: `JobByCompanyAndIDPrefix` compares the URL
  slug with the dump's `job.Company.Slug` as is. With a dump written before a
  company merge (the cached 2026-11-13 dump: `marvell-marvellwd1…`,
  `ge-vernova-…`, `nvidia-nvidiawd5…`, `cadence-design-systems-…`) the page
  redirects to the canonical slug and then 404s. Comparing
  `s.CanonicalCompanySlug(job.Company.Slug)` fixes it (the TS has the same
  gap). `TestRealDump` uses identity slugs until then.
- **pipeline (`internal/htmlx` / dump)**: some excerpts in the cached dump
  carry literal entities (`Date Posted: 2026-11-12&#xa;&#xa;` for RTX); the
  site prints them as text, like the TS. Worth decoding at ingest.
- **lead (`THIRD_PARTY.md`, phase 3)**: list the fonts in
  `internal/site/static/fonts/`: Atkinson Hyperlegible Next and Atkinson
  Hyperlegible Mono (variable `wght`, WOFF2), Braille Institute of America,
  SIL Open Font License 1.1 (licence texts beside the files), from the
  official repositories `github.com/googlefonts/atkinson-hyperlegible-next`
  and `github.com/googlefonts/atkinson-hyperlegible-next-mono`.
- **lead (`internal/web`)**: wire `site.Register` and `StaticFS:
  site.Static()` (see Ready APIs).

## Parity evidence

- `go vet`, `gofmt -l`, `go test ./internal/site` clean. Tests execute every
  template over the fixture dump and the real cached dump (every board page,
  150 job pages), check titles, descriptions, canonical URLs, Open Graph,
  cache headers, status codes, redirects (company alias, job alias, legacy
  UUID), the JS-off form, stale and unavailable states, favicon and OG image
  formats.
- Copy and metadata checked page by page against `apps/web/src/app/**`.

## Verification evidence (2026-11-26)

- `go vet ./internal/site`, `gofmt -l internal/site`: clean.
  `go test ./internal/site`: ok.
- `node ~/.claude/skills/avoid-ai-design/scripts/detect.mjs internal/site
  DESIGN.md`: 13 files, 0 findings (P0 0, P1 0, P2 0). One file-level ignore
  (`health.html`, K7: the scanner sees one template at a time; the
  `:focus-visible` rules are in `site.css`, linked on every page).
- Screenshots, `/tmp/openintern-site/shots/final/`:
  `{home,jobs,jobs-filtered,jobs-p2,job,docs,health,privacy,vs,notfound}-{400,1280}-{light,dark}-{js,nojs}.png`
  (80, full page); `stale-jobs-400-light-js.png`,
  `stale-home-1280-dark-js.png`; `error-{400,1280}-{light,dark}.png` (503 job
  page), `unavailable-jobs-{400,1280}-{light,dark}.png`;
  `interact-{focus-filter,applied,hide-applied,picker}.png`.
- Interaction (`interact.mjs`, `alias.mjs`, JS on, Chromium): 9 Tab presses
  reach the first role checkbox, focus ring 2px solid; Space applies the
  filter instantly (URL `?role=software`, count "612 roles matching
  filters"), focus stays on the checkbox; Mark applied → `aria-pressed=true`,
  "Applied", stored in `openintern:applied`; Hide applied → "showing 26 of 27
  on this page" and the bar keeps alternating; picker opens; Back restores
  the form state; Copy writes the curl command; an old id in localStorage is
  rewritten through the alias map; no console errors.
- No horizontal page scroll at 400 px on any page, JS on or off
  (`overflow.mjs`).
- Contrast (WCAG): ink/paper 15.4, muted/paper 6.7, muted/bar 5.9,
  alarm/paper 6.9, ink/marker 12.8, edge/paper 3.5 (non-text); dark: 14.0,
  7.3, 6.3, 7.7, paper/marker 10.9, edge 4.0.
- Four success tests: justified (green bar and marker both come from the
  batch-listing and highlighter the reader actually uses), coherent (one
  family, one ground, colour only for bar, marker, alarm), not an SD cluster
  (near-white neutral ground, no warm cream, no dark+acid, sentence case, no
  mono labels), consistent (one header, footer, token set and component set
  on every page). Silhouette: the board reads as a columned printout beside a
  filter column, not a card grid or hero.

## Divergences

- Job links and canonical URLs use the canonical company slug, so links from
  an older dump skip one redirect.
- Canonical/OG URLs with non-ASCII job slugs are percent-encoded (html/template
  normalizes `href`); same URL, encoded form.
- Health: wording tightened ("auto-disabled" → "disabled automatically",
  arrows in the funnel sentence replaced by commas), funnel shown as a banded
  list with proportional bars instead of pills; numbers unchanged.
- Home: no centered hero, pills or stat strip; the same facts (counts, last
  update, season note, compare link, curl sample, preview rows) as a text
  block beside real, clickable listing rows.

## Direction (DESIGN.md summary)

Every page is a report printed from the corpus, in the manner of a nightly
batch listing on green-bar paper: paper and ink, one legible family (Atkinson
Hyperlegible Next, Mono for code only), the pale green bar on alternate rows
so a reader can follow one listing across columns, and a highlighter yellow
reserved for the student's own "applied" marks. Flat, square rows, 3 px
controls, no link colour, alarm red only for stale data and errors.

## Audit of the old site (detect mode, 2026-11-25)

Scanner (`detect.mjs apps/web/src/app apps/web/src/components`, 32 files):
P0 0, P1 1, P2 5.

- P1 CP3 `app/page.tsx:80` "Compare to Intern Insider →" (code-certain).
- P2 C5 coloured glow shadows, `globals.css:155-166` (code-certain).
- P2 M8 every job card lifts on hover, `globals.css:1024` (code-certain).
- P2 SD8 emerald accent, `globals.css:507` (code-certain).
- P2 SD4d middle-dot meta strings, job detail page (code-certain).
- P2 SD4w "WORD — fragment", docs page (code-certain).

From the renders (400 px and 1280 px, light) and a pass through the catalog:

- L1 / L4 / L11 (P0/P1, rendered): home is the centered hero: H1, centered
  subhead, five pill badges, a `·` stat strip, one round CTA, then a faded
  grid of cards.
- SD7 (P2, rendered): macOS traffic-light dots around a decorative,
  `aria-hidden` grid of cards that cannot be clicked.
- K2 / L2 / K12 (P1, rendered): the board is a three-column grid of identical
  rounded cards with a soft shadow, coloured initials avatars and tinted badge
  chips; scanning one field across cards means hunting inside every card.
- C4 (P1, rendered): eight saturated avatar colours plus blue, green, amber
  and grey chips.
- I2 (P1, code): a flame emoji marks tier-1 employers.
- T1-adjacent (P1, judgment): IBM Plex everywhere, blue `#1454b4` on
  `#f6f7f9`, the stock SaaS palette.
- Stale banner said "Database is temporarily offline" although the site reads
  the dump first; at 400 px it pushed the listings below the fold.
- 400 px board ~8,000 px tall for 27 roles; pagination printed all 49 pages.
- Filters were client-only, so they did not work with JavaScript off.

## Notes

- `apps/web/next-env.d.ts` shows as modified in `git status`; not this
  workstream's change.
