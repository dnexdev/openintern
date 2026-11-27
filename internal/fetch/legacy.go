package fetch

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf16"
)

// LegacyPortalPostJSON is the ByteDance/TikTok search request exactly as it
// ran before the rewrite: one attempt, no retries, and the portal's own
// Origin and Referer.
//
// PENDING OWNER DECISION (plan: "ByteDance/TikTok: test first"). This is the
// only request that still sends a portal's own origin/Referer. It stays until
// the shadow test (OPENINTERN_SHADOW_HONEST=1) shows whether honest requests
// work; then it is removed (honest) or the source moves to a manual capture.
// The honest-requests test allows calls from internal/ats/bytedance.go only.
//
// origin is sent as Origin and origin+"/search" as Referer. User-Agent and
// Cookie stay forbidden in headers. The status mapping is the legacy one:
// 401/403 refused, 429 throttled, anything else (404 included) http, with
// up to 120 characters of the response body in the message.
func LegacyPortalPostJSON(ctx context.Context, rawURL, origin string, body any, headers map[string]string) ([]byte, error) {
	for k := range headers {
		if lk := strings.ToLower(strings.TrimSpace(k)); lk == "user-agent" || lk == "cookie" {
			return nil, fmt.Errorf("ingest may not set the %s header (honest requests only)", k)
		}
	}
	_, prepared, encoded, err := prepare(Options{Body: body})
	if err != nil {
		return nil, err
	}
	prepared["Accept"] = "application/json"
	for k, v := range headers {
		prepared[k] = v
	}
	prepared["Origin"] = origin
	prepared["Referer"] = origin + "/search"
	res, err := roundTrip(ctx, http.MethodPost, rawURL, prepared, encoded, defaultTimeout)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, networkFailure(err, rawURL)
	}
	if res.Status >= 200 && res.Status < 300 {
		return res.Body, nil
	}
	kind := KindHTTP
	switch res.Status {
	case 401, 403:
		kind = KindRefused
	case 429:
		kind = KindThrottled
	}
	msg := fmt.Sprintf("HTTP %d for %s", res.Status, rawURL)
	if text := head16(string(res.Body), 120); text != "" {
		msg += ": " + text
	}
	return nil, fail(kind, msg, res.Status)
}

// head16 is text.slice(0, n) on a JavaScript string (UTF-16 code units).
func head16(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	cut := u[:n]
	if last := cut[n-1]; last >= 0xD800 && last <= 0xDBFF {
		cut = cut[:n-1]
	}
	return string(utf16.Decode(cut))
}
