# Third-party components

Components copied into or shipped with this repository, and their licences.
The Go module has no third-party dependencies (`go.mod` has no `require`).

| Component | Where | Licence | Notes |
|---|---|---|---|
| Atkinson Hyperlegible Next and Atkinson Hyperlegible Mono | `internal/site/static/fonts/` | SIL Open Font License 1.1 | From the googlefonts repositories; licence beside the files |
| Go `golang.org/x/text/unicode/norm` and `transform` v0.44.0 | `internal/third_party/golang.org/x/text/` | BSD-3-Clause | NFKC/NFKD, Unicode 17.0 tables; see its README |
| Unicode `emoji-data.txt` 17.0 | `internal/unicodex/` | Unicode License v3 | Source of the Extended_Pictographic table |
