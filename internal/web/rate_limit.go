package web

import (
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dnexdev/openintern/internal/proven"
)

const defaultLimitPerMinute int64 = 60

type rateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]proven.RateBucket
	limit       int64
	windowMS    int64
	nextCleanup int64
	now         func() time.Time
}

type rateDecision struct {
	OK        bool
	Remaining int64
	ResetAt   int64
}

func environmentRateLimit() int64 {
	raw, present := os.LookupEnv("API_RATE_LIMIT_PER_MINUTE")
	if !present {
		return defaultLimitPerMinute
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	number, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number < 0 || number >= float64(proven.NatLimit) {
		return defaultLimitPerMinute
	}
	return int64(number)
}

func newRateLimiter(limit int64, now func() time.Time) *rateLimiter {
	if limit < 0 || limit >= proven.NatLimit {
		limit = defaultLimitPerMinute
	}
	return &rateLimiter{
		buckets:  make(map[string]proven.RateBucket),
		limit:    limit,
		windowMS: 60_000,
		now:      now,
	}
}

func (l *rateLimiter) step(key string) (decision rateDecision) {
	now := l.now().UnixMilli()
	defer func() {
		if recover() != nil {
			decision = rateDecision{OK: true, ResetAt: now + l.windowMS}
		}
	}()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now >= l.nextCleanup {
		for key, bucket := range l.buckets {
			if bucket.ResetAt <= now {
				delete(l.buckets, key)
			}
		}
		l.nextCleanup = now + l.windowMS
	}
	var previous *proven.RateBucket
	if bucket, ok := l.buckets[key]; ok {
		copy := bucket
		previous = &copy
	}
	verdict := proven.RateLimitStep(previous, now, l.limit, l.windowMS)
	l.buckets[key] = verdict.Bucket
	return rateDecision{OK: verdict.OK, Remaining: verdict.Remaining, ResetAt: verdict.Bucket.ResetAt}
}

func clientIP(request *http.Request) string {
	if forwarded, ok := request.Header[http.CanonicalHeaderKey("X-Forwarded-For")]; ok && len(forwarded) > 0 {
		first := strings.Split(forwarded[0], ",")[0]
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
		return "unknown"
	}
	if real, ok := request.Header[http.CanonicalHeaderKey("X-Real-Ip")]; ok && len(real) > 0 {
		return real[0]
	}
	return "unknown"
}
