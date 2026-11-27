# Sources workstream status

Owns `internal/fetch`, `internal/htmlx`, `internal/safedate`, `internal/ats/**`,
`cmd/openintern/cmd_sources.go`. Throwaway files: `/tmp/openintern-sources/`.

## Done

- `internal/fetch`: honest client, `*fetch.Failure`, Retry-After, `http.test.ts`
  port, honest-requests scanner (`internal/board` may build its own client:
  it reads OpenIntern's own dump, never a job board). Tests pass.
- `internal/htmlx`: `Strip`, `StripTags`, `TrimJS`, `IsJSSpace`, `JSSpace`.
  Matches the TypeScript on 640 vectors (`testdata/strip_node.json`).
- `internal/safedate`: `DateParse` is a port of V8's date parser (ES format
  plus the legacy fallback); `Parse`, `ParseString`, `ParseMillis`,
  `FormatISO`, `ISOPtr`. Matches Node 26 `Date.parse` on 110 inputs in three
  time zones (`testdata/date_parse_node.json`).
- `internal/ats`: compiles and vets. Every adapter, manual captures,
  parse-ats-url, probe-url, `MapConcurrent`. Unit tests and fixture parity
  are next (see In progress).

## Ready APIs

Stable. `internal/fetch`:

```go
const UserAgent = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)"
type FailureKind string // KindRefused "refused", KindThrottled "throttled", KindNotFound "not-found",
                        // KindHTTP "http", KindNetwork "network", KindInvalid "invalid", KindManualExpired "manual-expired"
type Failure struct { Kind FailureKind; Message string; Status int; Cause error } // Error() = "<kind>: <message>"
func NewFailure(kind FailureKind, message string, status int) *Failure
type Options struct { Method string; Body any; Headers map[string]string; Attempts int; Timeout time.Duration }
func Request(ctx, url string, opts Options) (*Response, error)   // Response{Status, Header, Body []byte}
func JSON(ctx, url string, opts Options, dest any) error
func JSONBody / TextBody(ctx, url string, opts Options) ([]byte, error)
func Text(ctx, url string, opts Options) (string, error)
func WithRoundTripper(ctx, rt http.RoundTripper) context.Context  // tests: serve fixtures
```

`internal/ats`:

```go
func Fetch(ctx context.Context, atsName, boardToken string) ([]model.NormalizedJob, error)
func IsProprietary(name string) bool
var Proprietary, PublicATS []string; func AllATS() []string
// Errors: HTTP and manual-capture failures are *fetch.Failure (errors.As);
// KindManualExpired is the one ingest treats as "retire the jobs". Other
// errors (unsupported ATS, "X is not ingested", bad Workday token, a page that
// parsed to nothing) are plain errors with the TypeScript message. nil error +
// empty slice = board read and empty. Locations is never nil.
// PostedAt is already safeDate-checked ISO (toISOString form) or nil.

func ShadowResults() []ShadowResult   // OPENINTERN_SHADOW_HONEST summary rows (ingest summary "shadow")
type ShadowResult struct { Source string; OK bool; Status *int; Count *float64; LegacyCount int; Error string } // json tags as TS

// Tools (gap-report, recover-tokens, validate-tokens, discover-accelerators):
func ParseATSFromApplyURL(rawURL string) (ParsedBoard, bool)   // ParsedBoard{ATS, BoardToken, CareersURL}
func TokenKey(ats, token string) string
func ExtractATSBoardsFromMarkdown(md string) ExtractedBoards   // {Hits []ATSHit, ApplyLinkCount, UnsupportedApplyCount, UnsupportedHosts []HostCount}
func ProbeURL(ats, token string) (string, error)
func ProbeATSBoard(ctx, ats, token string) ProbeResult          // ProbeResult{OK, Status, Count *int, Error}
func CountJobsFromProbeBody(ats string, data any) float64       // data: jsjson value
func ParseWorkdayToken(token) (WorkdayBoard, bool); ParseWorkdayURL(url) (WorkdayBoard, bool)
func EncodeWorkdayToken / WorkdayJobsURL / WorkdayCareersURL / WorkdayDetailURL / WorkdayApplyURL
func MapConcurrent[T, R any](ctx, items []T, limit int, fn func(context.Context, T) (R, error)) ([]R, error)

// Manual captures (data/manual, embedded via data.FS):
func LoadManualJobs(ats string, opts ManualOptions) ([]model.NormalizedJob, error) // ManualOptions{Dir fs.FS; Now time.Time}
func ValidateManualCapture(v any, now time.Time) (ManualCapture, error)
func ToManualCapture(jobs, source string, now time.Time, capturedBy string) (ManualCapture, error)
func (ManualCapture) MarshalFile() []byte   // JSON.stringify(capture, null, 2) + "\n"
```

## In progress

1. Port `ats.test.ts`, `proprietary.test.ts`, `manual.test.ts` and the
   parse-ats-url cases of `gap-report.test.ts` as Go tests.
2. Record one fixture set per ATS, TS goldens, Go parity test.
3. `cmd/openintern/cmd_sources.go` (`probe`, `manual-capture`).

## Blocked on

Nothing.

## Requests

- docs (phase 3): `data/manual/README.md` still says `pnpm manual:capture`;
  the Go command is `openintern manual-capture`.

## Parity evidence

- htmlx: `testdata/strip_node.json`, TS `stripHtml`/`stripHtmlTags` on crafted
  inputs plus 300 dump titles and excerpts. All equal.
- safedate: `testdata/date_parse_node.json`, Node 26 `Date.parse` under
  TZ=UTC, America/Toronto, Asia/Kolkata (ISO forms, Recruitee's
  "2026-11-10 12:34:56 UTC", RFC 1123, "Nov 10" = 2001, AM/PM, zone names,
  comments, garbage). All equal.

## Divergences

- Error text only: a body that is not JSON reports jsjson's parse message
  (`invalid: unreadable JSON from <url>: <msg>`), not V8's; the ByteDance
  legacy request reports a network error as `network: fetch failed for <url>`
  (typed) where TS threw a bare `fetch failed`, and a non-JSON body there as
  a typed `invalid` failure.
- Board JSON fields of an unexpected type (a number where a string was
  expected, a non-array list) are read leniently or skipped; a few such cases
  made the TypeScript throw. Real boards never send them.
- Regex `i` flag: Go folds `ſ` and the Kelvin sign onto `s`/`k`, JavaScript's
  non-unicode `i` does not. Only affects tag and URL matching on text with
  those characters.
- Manual captures are read from the copy compiled into the binary (data.FS);
  `ManualOptions.Dir` reads another directory.
- `parse-ats-url`: a path segment with a broken %-escape is "not a board"
  instead of a thrown URIError.
