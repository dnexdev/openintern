// Package fetch is the only way OpenIntern talks to job boards: an honest
// user-agent, bounded retries that respect Retry-After, and typed failures so
// a refusal is never mistaken for an empty board.
//
// Requests identify as OpenIntern. Callers cannot set User-Agent, Referer,
// Origin or Cookie: pretending to be a browser or another site is not allowed.
// Every other package must call this one; nothing else builds an HTTP client.
package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UserAgent is the only identity OpenIntern sends.
const UserAgent = "OpenIntern/0.1 (+https://github.com/dnexdev/openintern)"

// FailureKind is why a board read failed. Ingest uses it to tell a fetched
// empty board from a fetch that never succeeded (job-retirement laws).
type FailureKind string

const (
	KindRefused       FailureKind = "refused"        // 401/403, the site said no
	KindThrottled     FailureKind = "throttled"      // 429 after retries
	KindNotFound      FailureKind = "not-found"      // 404/410, usually a wrong token
	KindHTTP          FailureKind = "http"           // any other non-2xx
	KindNetwork       FailureKind = "network"        // DNS, TLS, reset, timeout
	KindInvalid       FailureKind = "invalid"        // a 2xx body we could not read
	KindManualExpired FailureKind = "manual-expired" // a committed capture is too old
)

// Failure is a typed board-read error. Callers inspect it with errors.As.
// Error() is "<kind>: <message>", the same text the TypeScript FetchFailure
// had, because it ends up in runs.jsonl.
type Failure struct {
	Kind FailureKind
	// Message is the detail without the kind prefix.
	Message string
	// Status is the HTTP status, or 0 when there was none.
	Status int
	// Cause is the underlying Go error for network failures (DNS, TLS,
	// timeout), kept for logs; Message carries Node's wording.
	Cause error
}

func (f *Failure) Error() string {
	if f == nil {
		return "<nil>"
	}
	return string(f.Kind) + ": " + f.Message
}

func (f *Failure) Unwrap() error { return f.Cause }

// NewFailure builds a typed failure, for sources that are not HTTP reads
// (manual captures) or that check a body after it arrived.
func NewFailure(kind FailureKind, message string, status int) *Failure {
	return &Failure{Kind: kind, Message: message, Status: status}
}

func fail(kind FailureKind, message string, status int) *Failure {
	return &Failure{Kind: kind, Message: message, Status: status}
}

// Options configures one call. Zero values mean GET (or POST when Body is
// set), three attempts and a 30s timeout per attempt.
type Options struct {
	Method string
	// Body is sent as-is when it is a string. Anything else is JSON-encoded
	// (no HTML escaping). Nil means no body.
	Body any
	// Headers the caller may set, except User-Agent, Referer, Origin and Cookie.
	Headers map[string]string
	// Attempts is the total number of tries, including the first. <= 0 means 3.
	Attempts int
	// Timeout is the budget for one attempt. <= 0 means 30s.
	Timeout time.Duration
}

const (
	defaultAttempts = 3
	defaultTimeout  = 30 * time.Second
	maxRetryAfter   = 30 * time.Second
)

var forbiddenHeaders = map[string]struct{}{
	"user-agent": {},
	"referer":    {},
	"origin":     {},
	"cookie":     {},
}

// Response is a finished HTTP response with the body already read.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	// URL is where the response came from after redirects, for resolving
	// relative links in an HTML body.
	URL string
}

type transportKey struct{}

// WithRoundTripper returns a context whose requests use rt instead of the
// default transport. Tests use it to serve recorded fixtures. A nil rt
// selects the default transport.
func WithRoundTripper(ctx context.Context, rt http.RoundTripper) context.Context {
	return context.WithValue(ctx, transportKey{}, rt)
}

func transportFrom(ctx context.Context) http.RoundTripper {
	if rt, ok := ctx.Value(transportKey{}).(http.RoundTripper); ok && rt != nil {
		return rt
	}
	return http.DefaultTransport
}

// RetryAfterMs parses a Retry-After header (delta-seconds or an HTTP date)
// into a delay capped at 30s. ok is false when the header is missing or
// unusable. now is the clock the date form is measured against.
func RetryAfterMs(value string, now time.Time) (ms int, ok bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	if digitsOnly(trimmed) {
		sec := 0
		for _, c := range trimmed {
			sec = sec*10 + int(c-'0')
			if sec > int(maxRetryAfter/time.Second) {
				return int(maxRetryAfter / time.Millisecond), true
			}
		}
		return sec * 1000, true
	}
	at, err := http.ParseTime(trimmed)
	if err != nil {
		return 0, false
	}
	d := at.Sub(now)
	if d < 0 {
		d = 0
	}
	if d > maxRetryAfter {
		d = maxRetryAfter
	}
	return int(d / time.Millisecond), true
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func failureFor(status int, rawURL string) *Failure {
	msg := fmt.Sprintf("HTTP %d for %s", status, rawURL)
	switch status {
	case 401, 403:
		return fail(KindRefused, msg, status)
	case 429:
		return fail(KindThrottled, msg, status)
	case 404, 410:
		return fail(KindNotFound, msg, status)
	default:
		return fail(KindHTTP, msg, status)
	}
}

func backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 16 {
		attempt = 16
	}
	return 400 * time.Millisecond * time.Duration(uint(1)<<uint(attempt))
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func headerForbidden(name string) bool {
	_, ok := forbiddenHeaders[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func hasHeader(h map[string]string, name string) bool {
	for k := range h {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

func prepare(opts Options) (method string, headers map[string]string, body *string, err error) {
	headers = make(map[string]string, len(opts.Headers)+2)
	for k, v := range opts.Headers {
		if headerForbidden(k) {
			return "", nil, nil, fmt.Errorf("ingest may not set the %s header (honest requests only)", k)
		}
		headers[k] = v
	}
	headers["User-Agent"] = UserAgent

	if opts.Body != nil {
		var encoded string
		switch b := opts.Body.(type) {
		case string:
			encoded = b
		case []byte:
			encoded = string(b)
		default:
			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(b); err != nil {
				return "", nil, nil, err
			}
			encoded = strings.TrimRight(buf.String(), "\n")
		}
		body = &encoded
		if !hasHeader(headers, "Content-Type") {
			headers["Content-Type"] = "application/json"
		}
	}
	method = opts.Method
	if method == "" {
		if body == nil {
			method = http.MethodGet
		} else {
			method = http.MethodPost
		}
	}
	return method, headers, body, nil
}

// Request performs one honest call and returns the response body.
// It retries 429, 5xx and network errors. A 401/403/404/410 fails immediately.
func Request(ctx context.Context, rawURL string, opts Options) (*Response, error) {
	method, headers, body, err := prepare(opts)
	if err != nil {
		return nil, err
	}
	attempts := opts.Attempts
	if attempts <= 0 {
		attempts = defaultAttempts
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	var last *Failure
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res, err := roundTrip(ctx, method, rawURL, headers, body, timeout)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			last = networkFailure(err, rawURL)
			if attempt == attempts-1 {
				break
			}
			if err := sleepCtx(ctx, backoff(attempt)); err != nil {
				return nil, err
			}
			continue
		}
		if res.Status >= 200 && res.Status < 300 {
			return res, nil
		}
		last = failureFor(res.Status, rawURL)
		if res.Status != 429 && res.Status < 500 {
			return nil, last
		}
		if attempt == attempts-1 {
			break // no point waiting after the last try
		}
		wait := backoff(attempt)
		if ms, ok := RetryAfterMs(res.Header.Get("Retry-After"), time.Now()); ok {
			wait = time.Duration(ms) * time.Millisecond
		}
		if err := sleepCtx(ctx, wait); err != nil {
			return nil, err
		}
	}
	if last == nil {
		last = fail(KindNetwork, "no response for "+rawURL, 0)
	}
	return nil, last
}

// networkFailure words a transport error the way Node's fetch did ("fetch
// failed", or the AbortSignal.timeout message), so run errors read the same
// as before the rewrite. The Go error stays available as Cause.
func networkFailure(err error, rawURL string) *Failure {
	msg := "fetch failed"
	if errors.Is(err, context.DeadlineExceeded) {
		msg = "The operation was aborted due to timeout"
	}
	return &Failure{Kind: KindNetwork, Message: msg + " for " + rawURL, Cause: err}
}

func roundTrip(ctx context.Context, method, rawURL string, headers map[string]string, body *string, timeout time.Duration) (*Response, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var rdr io.Reader
	if body != nil {
		rdr = strings.NewReader(*body)
	}
	req, err := http.NewRequestWithContext(attemptCtx, method, rawURL, rdr)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Transport: transportFrom(ctx)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: b, URL: resp.Request.URL.String()}, nil
}

// InvalidJSON is the failure for a 2xx body that is not JSON.
func InvalidJSON(rawURL string, err error) *Failure {
	return fail(KindInvalid, fmt.Sprintf("unreadable JSON from %s: %s", rawURL, err.Error()), 0)
}

// JSONBody performs Request with Accept: application/json (unless the caller
// set Accept) and returns the raw body, for callers that decode it
// themselves. A body that does not parse should be reported with InvalidJSON.
func JSONBody(ctx context.Context, rawURL string, opts Options) ([]byte, error) {
	opts.Headers = mergeHeaders(map[string]string{"Accept": "application/json"}, opts.Headers)
	res, err := Request(ctx, rawURL, opts)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// TextBody performs Request with the HTML Accept header (unless the caller
// set Accept) and returns the raw body.
func TextBody(ctx context.Context, rawURL string, opts Options) ([]byte, error) {
	opts.Headers = mergeHeaders(map[string]string{"Accept": "text/html,application/xhtml+xml"}, opts.Headers)
	res, err := Request(ctx, rawURL, opts)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// JSON performs Request and decodes a JSON body into dest. dest may be nil
// to only check that the body is JSON.
func JSON(ctx context.Context, rawURL string, opts Options, dest any) error {
	body, err := JSONBody(ctx, rawURL, opts)
	if err != nil {
		return err
	}
	res := Response{Body: body}
	if dest == nil {
		if !json.Valid(res.Body) {
			return InvalidJSON(rawURL, fmt.Errorf("invalid JSON"))
		}
		return nil
	}
	if err := json.Unmarshal(res.Body, dest); err != nil {
		return InvalidJSON(rawURL, err)
	}
	return nil
}

// Text performs Request and returns the body as a string.
func Text(ctx context.Context, rawURL string, opts Options) (string, error) {
	body, err := TextBody(ctx, rawURL, opts)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// mergeHeaders is `{...base, ...extra}` where a caller's header replaces a
// default of the same name in any case.
func mergeHeaders(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		if !hasHeader(extra, k) {
			out[k] = v
		}
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
