# Security

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting: **Security → Report a
vulnerability** on this repository. Do not open a public issue for security
problems.

## Reporting a dangerous listing

If a listing looks like a scam or phishing (asks for money, a deposit, chat-app
interviews, or links somewhere other than the employer), open a "Report a
listing" issue and choose *unsafe*. Unsafe reports are hidden first and checked
second.

## What we promise

- Apply links are https and limited to hosts on `data/apply-hosts.tsv` per
  company.
- Ingest makes honest requests only (no browser impersonation, forged headers
  or cookies).
- The daily dump is checked before publishing (integrity, allowlist,
  delistings, size swings) and carries a sha256 manifest and a build
  provenance attestation.

See [RUNBOOK.md](RUNBOOK.md) for how incidents are handled.
