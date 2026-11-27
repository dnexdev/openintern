# Site workstream status

Owns `DESIGN.md`, `internal/site/**`, this file. Throwaway files in
`/tmp/openintern-site/` (Playwright 1.49.1 in `node_modules`, Chromium at
`~/.cache/ms-playwright/chromium-1148/chrome-linux/chrome`, screenshot script
`shoot.mjs`, contrast checker `contrast.mjs`, font tools in `venv/`).
Ports 8101/8102.

## Done

- Step 1, audit (below). Production screenshots: `/tmp/openintern-site/shots/prod/`.

## In progress

- Step 2, `DESIGN.md`.

## Next

- `internal/site` templates and pages, then verification.

## Audit of the current site (detect mode)

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
  subhead, five pill badges, a `·` stat strip, one round CTA ("Start", CP6),
  then a faded grid of cards. Stats are real numbers, but the composition is
  the stock template.
- SD7 (P2, rendered): the home preview wears macOS traffic-light dots around a
  decorative, `aria-hidden` grid of cards, which repeat real listings but
  cannot be clicked.
- K2 / L2 / K12 (P1, rendered): the board is a three-column grid of identical
  rounded cards with a soft shadow, each with a coloured initials avatar and
  2-4 tinted badge chips. 27 cards per page read as one texture; scanning
  locations or terms across cards means hunting inside every card.
- C4 (P1, rendered): avatar palette of eight saturated colours plus blue,
  green, amber and grey chips: colour spread evenly with no job.
- I2 (P1, code): a flame emoji marks tier-1 employers, plus an orange card
  border (K4-adjacent).
- T1-adjacent (P1, judgment): IBM Plex Sans/Mono everywhere, blue `#1454b4`
  primary on `#f6f7f9`: the stock SaaS palette, no decision traced to the
  subject.
- Stale banner says "Database is temporarily offline" although the site now
  reads the dump first; at 400 px it pushes the listings below the fold.
- 400 px board: every card stacks full width; the page is ~8,000 px tall for
  27 roles. Pagination prints all 49 page numbers.
- Accessibility: focus rings exist; muted text passes AA; filters are
  client-only (React router), so they do not work with JavaScript off.

## Ready APIs

None yet (see "Next").

## Blocked on

Nothing.

## Requests

None yet.

## Parity evidence

None yet.

## Divergences

None yet.
