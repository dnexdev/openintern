# Core workstream

## Done

- `internal/proven`: complete Go port of the Bend decision logic and the typed TypeScript wrapper contract, including Nat validation, arithmetic overflow, enum validation, and default values.
- `internal/classify`: complete classifier, extraction, freshness, excerpt, title normalization, fingerprint, and role-family port. RE2-safe implementations replace JavaScript-only regexp behavior, and NFKC uses `internal/unicodex`.
- `internal/jobslug`: Unicode title slugification, id-prefix construction/parsing, and public job paths.
- Every applicable case from `proven.test.ts`, `classifier.test.ts`, `normalize-title.test.ts`, and the classifier freshness cases is represented in Go tests.
- Proven-model vectors, all named law tests, all listed mutations, and real/synthetic TypeScript parity goldens are checked by Go tests.

## Ready APIs

### `internal/proven`

```go
const NatLimit int64
const DayMS int64
const MaxAgeDaysNoTerm int64
const LastSeenStaleDays int64
const RetireAfterEmptyRuns int64
// Compatibility aliases: NAT_LIMIT, DAY_MS, MAX_AGE_DAYS_NO_TERM,
// LAST_SEEN_STALE_DAYS, RETIRE_AFTER_EMPTY_RUNS.

type RangeError struct { Message string }
type RateBucket struct { Count, ResetAt int64 }
type RateVerdict struct { OK bool; Remaining int64; Bucket RateBucket }
type TermYearLike struct { Term string; Year int }
type FreshnessFacts struct {
    TermYears []TermYearLike
    Anchor, Now time.Time
    MaxAgeDays *int64
}
type BoardRun string
type JobSighting string
type FunnelCounts struct {
    Fetched, Title, Tech, Upserted, DescriptionMissing *int64
}
type FunnelTotals struct {
    Fetched, Title, Tech, Upserted, DescriptionMissing int64
}

func RateLimitStep(prev *RateBucket, now, limit, windowMs int64) RateVerdict
func ApplyByDate(ty TermYearLike) time.Time
func IsFresh(f FreshnessFacts) bool
func IsStale(f FreshnessFacts) bool
func IsPublic(roleCount int64, f FreshnessFacts) bool
func IsStaleByLastSeen(lastSeenAt, now time.Time, maxDays ...int64) bool
func NeedsPicker(postingCount, firstPostingLocationCount int64) bool
func QualifySingleTitle(locationCount int64) bool
func SaysOpenings(postingCount, uniqueLocationCount int64) bool
func ShouldRetireJob(run BoardRun, priorEmptyRuns int64, seen JobSighting) bool
func NextEmptyRuns(run BoardRun, priorEmptyRuns int64) int64
func SumFunnels(funnels []FunnelCounts) FunnelTotals
func IsZeroMatchRun(errorText string) bool
func ParsePage(raw ...string) int64
func ParseLimit(raw ...string) int64
func PageOffset(page, limit int64) int64
func JobSlugSegment(base, id string) string
func ParseIdPrefix(slug string) *string
```

Board-run constants are `BoardRunFailed`, `BoardRunEmpty`, `BoardRunListed`, and `BoardRunExpired`. Sighting constants are `JobSightingGone`, `JobSightingRejected`, `JobSightingUnverified`, and `JobSightingMatched`.

Invalid Nat values and unknown enum values panic with `*proven.RangeError`, matching the TypeScript wrapper's programmer-error contract. Bend arithmetic overflow panics with the compiled wrapper's error text.

### `internal/classify`

```go
const MaxAgeDaysNoTerm int64
const LastSeenStaleDays int64
// Compatibility aliases: MAX_AGE_DAYS_NO_TERM, LAST_SEEN_STALE_DAYS.

var TermOrder []string
var RoleOptions []string
var RegionOptions []string
var DurationPresets []int
// Compatibility aliases: TERM_ORDER, ROLE_OPTIONS, REGION_OPTIONS,
// DURATION_PRESETS.

type InternshipTerm = string
type TermYear = model.TermYear
type JobRole = string
type JobRegion = string
type StaleByAgeOptions struct {
    TermYears []model.TermYear
    PostedAt *time.Time
    FirstSeenAt time.Time
    Now *time.Time
    MaxAgeDays *int64
}

func IsTechInternship(title string, description ...string) bool
func LooksRemote(locations []string, title ...string) bool
func ExtractTerms(text string) []string
func ExtractTermYears(text string, cohortYear *int, now ...time.Time) []model.TermYear
func TermYearEndDate(ty model.TermYear) time.Time
func IsStaleByTermYears(termYears []model.TermYear, now ...time.Time) bool
func IsStaleByAge(opts StaleByAgeOptions) bool
func IsStaleByLastSeen(lastSeenAt, now time.Time, maxDays ...int64) bool
func ExtractDurationMonthsList(text string) []int
func ExtractDurationMonths(text string) *int
func ExtractCohortYear(text string, now ...time.Time) *int
func ExtractRoles(title string, description ...string) []string
func ExtractRegions(locations []string, isRemote bool) []string
func ExcerptFromHtml(html string, maximum ...int) *string
func NormalizeTitle(raw string) string
func ShortHash(input string, length ...int) string
func JobFingerprint(companyID, source, externalJobID string) string
func RoleFamilyId(companySlug, normalizedTitle string) string
```

### `internal/jobslug`

```go
func SlugifyTitle(title string) string
func JobSlugSegment(title, id string) string
func JobPath(companySlug, title, id string) string
func ParseIdPrefixFromJobSlug(jobSlug string) *string
```

## In progress

Nothing.

## Blocked on

Nothing.

## Requests

None.

## Parity evidence

- Proven vectors: 675 cases generated through `packages/proven/src/index.ts`, which calls `packages/proven/generated/proven.mjs`. They include exact reset boundaries, 0, 1, `2^48-1`, overflow, invalid values, empty lists, and slug boundaries.
- Laws: 56 separately named Go property tests, one for every law in `LAWS.bend`.
- Mutations: all 19 rows of `packages/proven/mutants.tsv` have Go-native equivalents and are killed. Run `go test -run '^TestMutantsKilled$' -v ./internal/proven`; the runner never edits source.
- Classifier/title goldens: 6,234 cases, comprising all 1,434 cached dump jobs and 4,800 synthetic variants. Every case checks 17 outputs, including classification, all metadata extractors, excerpts, title normalization, hashes, role families, and three freshness decisions.
- Job-slug goldens: 6,234 cases, comprising all 1,434 cached dump jobs and 4,800 synthetic variants. Every case checks slugification, segment construction, path construction, and id-prefix parsing.
- Synthetic cases cover token shuffling, casing, Unicode width forms, emoji, punctuation, and term/year/duration phrases mined from the cached dump.
- Regenerate with:
  - `packages/ingest/node_modules/.bin/tsx internal/proven/testdata/gen-vectors.ts`
  - `packages/ingest/node_modules/.bin/tsx internal/classify/testdata/gen-goldens.ts`
  - `packages/ingest/node_modules/.bin/tsx internal/jobslug/testdata/gen-goldens.ts`
- Clean validation:
  - `go build ./internal/proven/... ./internal/classify/... ./internal/jobslug/...`
  - `go test -count=1 ./internal/proven/... ./internal/classify/... ./internal/jobslug/...`
  - `go vet ./internal/proven/... ./internal/classify/... ./internal/jobslug/...`
  - `gofmt -l internal/proven internal/classify internal/jobslug` returns no files.

## Divergences

None.
