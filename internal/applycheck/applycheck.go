// Package applycheck decides whether a job's apply link is dead, and when a
// dead link has been seen often enough to take the job off the board.
//
// A probe gives one of three verdicts:
//
//	alive         the link answered 2xx and still points at the posting
//	dead          404 or 410 (confirmed by a GET when HEAD said so), or a
//	              redirect that lands on a page without the posting's id
//	              (the employer's careers index, Greenhouse's ?error=true)
//	inconclusive  anything else: 401/403 (the site refuses robots), 429,
//	              5xx, other statuses, timeouts, DNS and TLS errors, and
//	              redirects to a login or bot check
//
// Only dead counts against a job. One dead verdict is a strike; a job is
// retired after StrikesToRetire strikes in a row, each at least
// MinStrikeGap after the one before, so a flaky response, a deploy gone
// wrong for an hour, or several ingest runs on one day never retire it. An
// alive verdict clears the strikes; an inconclusive one leaves them as they
// are. Requests go through internal/fetch (honest user-agent only).
package applycheck

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dnexdev/openintern/internal/fetch"
)

// StrikesToRetire is how many dead verdicts in a row retire a job.
const StrikesToRetire = 3

// MinStrikeGap is the least time between two strikes that both count. A
// dead verdict sooner than this after the last strike changes nothing.
const MinStrikeGap = 20 * time.Hour

// ProbeTimeout is the budget for one request of a probe.
const ProbeTimeout = 15 * time.Second

// Verdict is what one probe of an apply link found.
type Verdict string

const (
	Alive        Verdict = "alive"
	Dead         Verdict = "dead"
	Inconclusive Verdict = "inconclusive"
)

// Result is one probe: the verdict, the last HTTP status (0 when there was
// none), where redirects ended, and a short reason for logs.
type Result struct {
	Verdict  Verdict
	Status   int
	FinalURL string
	Reason   string
}

// Strikes is a job's dead-link record: consecutive strikes against its
// current apply URL and when the last one was counted (zero when none).
type Strikes struct {
	Count int
	Last  time.Time
}

// Retired reports whether the strikes take the job off the board.
func (s Strikes) Retired() bool { return s.Count >= StrikesToRetire }

// Next is the strike record after a probe at now. alive clears it;
// inconclusive keeps it; dead adds a strike unless the last one is less than
// MinStrikeGap old (or the count is already at StrikesToRetire, which is
// kept as is so a retired job's record does not change every run).
func Next(s Strikes, v Verdict, now time.Time) Strikes {
	switch v {
	case Alive:
		return Strikes{}
	case Dead:
		if s.Count >= StrikesToRetire {
			return s
		}
		if s.Count > 0 && now.Sub(s.Last) < MinStrikeGap {
			return s
		}
		return Strikes{Count: s.Count + 1, Last: now}
	}
	return s
}

// Due reports whether a job with these strikes may be probed again at now:
// always when it has none, otherwise once MinStrikeGap has passed, so a
// probe that cannot count is never sent.
func (s Strikes) Due(now time.Time) bool {
	return s.Count == 0 || now.Sub(s.Last) >= MinStrikeGap
}

// Probe checks one apply link with the honest client: HEAD, then GET when
// HEAD is not allowed or says 404/410 (some sites answer HEAD wrongly), one
// attempt each, no retries.
func Probe(ctx context.Context, rawURL string) Result {
	res, err := fetch.Request(ctx, rawURL, fetch.Options{Method: "HEAD", Attempts: 1, Timeout: ProbeTimeout})
	if err == nil {
		return Classify(rawURL, res.Status, res.URL)
	}
	if ctx.Err() != nil {
		return Result{Verdict: Inconclusive, Reason: "cancelled"}
	}
	var f *fetch.Failure
	if !errors.As(err, &f) {
		return Result{Verdict: Inconclusive, Reason: "error: " + err.Error()}
	}
	if f.Status != 404 && f.Status != 410 && f.Status != 405 && f.Status != 501 {
		return Classify(rawURL, f.Status, "")
	}
	res, err = fetch.Request(ctx, rawURL, fetch.Options{Method: "GET", Attempts: 1, Timeout: ProbeTimeout})
	if err == nil {
		return Classify(rawURL, res.Status, res.URL)
	}
	if ctx.Err() != nil {
		return Result{Verdict: Inconclusive, Reason: "cancelled"}
	}
	if errors.As(err, &f) {
		return Classify(rawURL, f.Status, "")
	}
	return Result{Verdict: Inconclusive, Reason: "error: " + err.Error()}
}

// Classify turns a final status (0 for a network failure) and the URL the
// redirects ended at ("" when unknown) into a verdict.
func Classify(requested string, status int, final string) Result {
	r := Result{Status: status, FinalURL: final}
	switch {
	case status == 404 || status == 410:
		r.Verdict, r.Reason = Dead, "HTTP "+strconv.Itoa(status)
	case status >= 200 && status < 300:
		r.Verdict, r.Reason = RedirectVerdict(requested, final)
	case status == 0:
		r.Verdict, r.Reason = Inconclusive, "no response"
	default:
		r.Verdict, r.Reason = Inconclusive, "HTTP "+strconv.Itoa(status)
	}
	return r
}

// RedirectVerdict judges where a link that answered 2xx ended up. No
// redirect, or one that keeps the posting's id, is alive. One that drops
// every id the link carried (an index or search page, the company's home
// page) is dead, and so is Greenhouse's closed-job redirect (?error=true).
// A link with no id to look for, or a redirect to a login or bot-check
// page, is inconclusive: there is nothing safe to conclude.
func RedirectVerdict(requested, final string) (Verdict, string) {
	if final == "" || final == requested {
		return Alive, "ok"
	}
	req, err1 := url.Parse(requested)
	fin, err2 := url.Parse(final)
	if err1 != nil || err2 != nil {
		return Inconclusive, "unparseable redirect"
	}
	if sameTarget(req, fin) {
		return Alive, "ok"
	}
	if strings.EqualFold(fin.Query().Get("error"), "true") {
		return Dead, "redirect to " + final
	}
	if gated(fin) {
		return Inconclusive, "redirect to a login or check page"
	}
	ids := IDTokens(req)
	if len(ids) == 0 {
		return Inconclusive, "redirect, no id to compare"
	}
	where := strings.ToLower(fin.EscapedPath() + "?" + fin.RawQuery)
	if dec, err := url.PathUnescape(where); err == nil {
		where += " " + strings.ToLower(dec)
	}
	for _, id := range ids {
		if strings.Contains(where, id) {
			return Alive, "redirect keeps the posting"
		}
	}
	return Dead, "redirect to " + final
}

// sameTarget ignores what redirects commonly change without leaving the
// page: scheme, host case, a www. prefix, a trailing slash and the fragment.
func sameTarget(a, b *url.URL) bool {
	host := func(u *url.URL) string { return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") }
	path := func(u *url.URL) string { return strings.TrimRight(u.EscapedPath(), "/") }
	return host(a) == host(b) && path(a) == path(b) && a.RawQuery == b.RawQuery
}

var gateWords = []string{"login", "signin", "sign-in", "sign_in", "sso", "oauth", "auth", "captcha", "challenge", "consent", "verify", "cdn-cgi"}

// gated reports a redirect to a login, consent or bot-check page.
func gated(u *url.URL) bool {
	p := strings.ToLower(u.Hostname() + u.EscapedPath())
	for _, w := range gateWords {
		if strings.Contains(p, w) {
			return true
		}
	}
	return false
}

// IDTokens are the parts of a link that name one posting, lowercased: runs
// of at least 4 digits, and runs of at least 8 letters and digits that mix
// both (UUIDs, Workable shortcodes), from the path and the query values.
// The title words in a slug are not ids.
func IDTokens(u *url.URL) []string {
	text := u.EscapedPath()
	for _, vs := range u.Query() {
		for _, v := range vs {
			text += "/" + v
		}
	}
	if dec, err := url.PathUnescape(text); err == nil {
		text = dec
	}
	text = strings.ToLower(text)
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	isAlnum := func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' }
	for i := 0; i < len(text); {
		if !isAlnum(text[i]) {
			i++
			continue
		}
		j := i
		digits, letters, run, best := 0, 0, 0, 0
		for j < len(text) && isAlnum(text[j]) {
			if text[j] <= '9' {
				digits++
				run++
				best = max(best, run)
			} else {
				letters++
				run = 0
			}
			j++
		}
		word := text[i:j]
		switch {
		case letters > 0 && digits > 0 && len(word) >= 8:
			add(word)
		case best >= 4:
			// Pure digits, or a short mixed word like r108218: its longest
			// digit run is the id.
			for k := 0; k < len(word); {
				if word[k] > '9' {
					k++
					continue
				}
				m := k
				for m < len(word) && word[m] <= '9' {
					m++
				}
				if m-k >= 4 {
					add(word[k:m])
				}
				k = m
			}
		}
		i = j
	}
	return out
}
