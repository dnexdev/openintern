# Manual captures

Some career sites refuse automated clients (Tesla, Citadel, Citadel
Securities). OpenIntern does not pretend to be a browser, so these are captured
by hand:

1. Open the public listing page in a normal browser and save it (Tesla: the
   JSON at `https://www.tesla.com/cua-api/apps/careers/state`; Citadel /
   Citadel Securities: each `/careers/open-opportunities/` page as HTML).
2. Run:
   ```bash
   go run ./cmd/openintern manual-capture <tesla|citadel|citadel_securities> <saved files…>
   ```
3. Commit the resulting `data/manual/<source>.json`. Never commit the raw page.
4. Rebuild/redeploy so the embedded capture is picked up (`data/manual` is
   compiled into the binary via `data/embed.go`).

Each file holds only normalized fields (title, locations, apply URL, an excerpt
of at most 400 characters), plus `captured_at` and `captured_by` (a role, not a
name). Ingest stops publishing a capture 14 days after `captured_at` and takes
its jobs down; `/health` shows the source as failed until it is refreshed.

Citadel (and Citadel Securities) cannot be automated: if `data/manual/` has
only a README for those sources, dump counts stay at 0 until someone refreshes
the capture.

These files are excluded from the repository's Apache-2.0 grant (see NOTICE):
the listing text belongs to the employers.
