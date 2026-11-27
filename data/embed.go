// Package data compiles the registry files into the binary, so the website,
// API and ingest need no filesystem: data/companies/*.yaml,
// data/curated/tier-1.yaml, the apply-host, alias and expected-delta tables,
// and the manual captures under data/manual/.
//
// The files stay the source of truth that contributors edit; internal/registry
// parses and validates them. Commands that check a working copy (CI on a pull
// request) can read the same layout from disk with os.DirFS instead.
package data

import "embed"

// FS holds the registry files at their paths relative to data/
// ("companies/tech-a.yaml", "apply-hosts.tsv", ...).
//
//go:embed companies curated manual apply-hosts.tsv company-aliases.tsv job-aliases.tsv expected-delta.json
var FS embed.FS

// Logos holds the committed company logos (logos/<slug>.png) and their
// SOURCES.tsv; see logos/README.md.
//
//go:embed logos
var Logos embed.FS
