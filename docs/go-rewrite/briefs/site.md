# Brief: site (redesign, DESIGN.md, every page)

Owns: `DESIGN.md` (repo root), `internal/site/**`,
`docs/go-rewrite/status/site.md`. Throwaway files: `/tmp/openintern-site/`
(a previous run installed Playwright and a headless Chromium there and took
audit screenshots in `shots/`; reuse them). Ports: 8101, 8102 only. Do not run
the Next.js app; look at production https://openintern.dev for the current
look. TypeScript under `apps/` is the read-only reference for behavior and
copy. No npm packages in the project, no CSS framework, no CDN or font-service
links.

Design reference: the `avoid-ai-design` skill (MIT,
https://github.com/funboy322/avoid-ai-design), installed at
`~/.claude/skills/avoid-ai-design` (scanner: `node ~/.claude/skills/avoid-ai-design/scripts/detect.mjs <files>`; every P0/P1 must be fixed before the work is done). Read `SKILL.md`,
`references/ai-tells-catalog.md`, `references/aesthetic-directions.md`,
`references/design-md-template.md`, and `examples/demo/`. Reference material
only; the one script you may run is `scripts/detect.mjs` (zero-dependency
scanner; read it first) with Node. The maintainer's taste: clean, minimal,
personal; no eyebrow labels, no rule-of-three spam, no cream+terracotta, no
Inter everywhere, no near-black+acid accent.

1. Audit the current site (`apps/web/src/app/**`, `components/**`,
   `globals.css`, production screenshots at 400px and 1280px) in detect mode;
   record findings in the status file.
2. Direction → `DESIGN.md`, derived from the subject: a free, open,
   daily-refreshed corpus of tech internships for students, read honestly from
   public job boards, with a public API and dumps. Users: students scanning
   many listings fast; developers reading the API docs. Follow the skill's
   steps 5–6: 4–6 named colors with roles, typefaces and roles, type scale,
   spacing rhythm, radius, the layout idea in one sentence, one signature
   detail; check against SD1–SD9 and C10 and revise. Light and dark
   (`prefers-color-scheme`), focus states, AA contrast, reduced motion. The
   board is a dense listings tool: favor density, legibility, hierarchy over
   decoration. Fonts: self-hosted OFL files from the official source into
   `internal/site/static/fonts/` with licence; list source and licence under
   Requests for THIRD_PARTY.md.
3. Templates and pages in `internal/site` (`html/template` + `embed`): home,
   `/jobs` board (filters as a GET form that works with JS off; role families
   with postings and the apply picker; pagination; stale-data banner;
   report-issue links), `/jobs/{company}`, `/jobs/{company}/{jobSlug}`,
   `/docs` (API reference, all content accurate), `/health`, `/privacy`,
   `/vs/intern-insider`, 404, error page, icon, favicon, static Open Graph
   image (1200×630 PNG rendered once at dev time and committed). Preserve each
   page's information, metadata (titles, descriptions, canonical URLs, JSON-LD
   if any), analytics behavior (`layout.tsx`), and the copy's meaning. Vanilla
   JS only, small, no build step: instant filter apply, Applied toggle with
   localStorage including the job-alias migration from `AppliedToggle.tsx`,
   copy-to-clipboard for the curl bar; all degrade cleanly without JS and honor
   reduced motion.
4. Data: pages read from `*board.Store` (`internal/board`, finished; API in
   `docs/go-rewrite/status/webapi.md`). Construct a store for local runs with
   `board.NewStore(board.Options{DumpSource: ".cache/dump/jobs.json"})`.
   Export `site.Register(mux *http.ServeMux, store *board.Store)` registering
   page routes only (`internal/web` owns APIs, maintenance, robots, sitemap)
   and `site.Static() fs.FS` for the embedded assets (served by `internal/web`
   under a path you document). The lead wires both into `internal/web`; for
   local checks run your own small server on 8101 from a test or a
   `/tmp` main.
5. Verify: render every page at 400px and 1280px, light and dark, JS on and
   off; look at the screenshots and iterate until each page passes the skill's
   four success tests and the silhouette test. `detect.mjs` over
   `internal/site`: no P0/P1. Keyboard navigation and focus on filters. Keep
   final screenshots in `/tmp/openintern-site/shots/` and list paths in the
   status file.

Done: `DESIGN.md`; all pages built from one set of tokens and components;
`go vet`, `gofmt -l`, `go test` clean for `internal/site` (template-execution
tests over real data); scanner clean; status file complete (PLAN.md rule 10)
plus the audit and direction summary. Final message: the direction in a few
lines, what landed, verification evidence, divergences, requests.
