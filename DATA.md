# Data

## What the data is

OpenIntern publishes internship listings from employers' public job boards
and careers pages: titles, locations, apply links, terms, dates, and an excerpt
of at most 400 characters. Full job descriptions are never redistributed.

## Licence

- **Code** in this repository is Apache-2.0 (see LICENSE).
- **Listing data** (the API, the dumps, and `data/manual/`) is not covered by
  the Apache-2.0 grant. Factual fields (title, company, location, dates, link)
  are provided as-is for anyone to use. **Excerpts remain the employers'
  copyright**; they are included so listings can be found and are licensed to
  no one. Logos and company names are trademarks of their owners.
- No warranty: listings can be wrong, stale or closed. Always confirm on the
  employer's site before applying.

## Mirrors and reuse

If you mirror the dumps, apply `denylist.json` from the latest release and
refresh at least weekly. Dated snapshots are deleted after 30 days so removals
reach everyone; please do the same.

## Takedowns and corrections

- **Employers**: to remove or correct a listing, open an issue or a GitHub
  security advisory from an account or email on your company's own domain, or
  include a link from your careers site that confirms the request. Requests
  that can't be tied to the employer's domain are not acted on.
- **Unsafe listings** (scams, phishing): anyone can report them with the
  "Report a listing" issue form. They are hidden first and verified second.
- Every removal is logged in [INCIDENTS.md](INCIDENTS.md) with the date,
  requesting domain and what was removed. Removals take effect on the site
  within minutes (kill switch) and in the next dump.

## Personal data

Listings name employers, not people. If a listing contains a person's contact
details, report it and the excerpt will be removed. Site privacy:
[openintern.dev/privacy](https://openintern.dev/privacy).
