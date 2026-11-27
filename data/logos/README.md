# Company logos

One `<slug>.png` per company, 64×64, re-encoded by `openintern logos` from the
company's own icon (apple-touch-icon, `<link rel="icon">` or `/favicon.ico` on
its `website_url`). The site embeds these files and serves them from its own
origin, so a visitor's browser never asks a company site or a favicon service
for anything. A company without a file gets a plain initials mark.

`SOURCES.tsv` records where each file came from and when.

- **Refresh or add logos:** `go run ./cmd/openintern logos` (every active
  company that has no logo yet), `-only slug,slug` for some, `-dump jobs.json`
  for the companies on the board, `-force` to refetch. Commit the files and
  rebuild: they are compiled into the binary.
- **Wrong or missing logo:** fix `website_url` in `data/companies/*.yaml` and
  rerun with `-only <slug> -force`. If the site's icon is still wrong, drop a
  square PNG here as `<slug>.png` (64×64 or larger is fine; the test checks it
  decodes) and set its `SOURCES.tsv` source to `manual`, so refetches keep it.
