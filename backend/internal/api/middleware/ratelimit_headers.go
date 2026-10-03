package middleware

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
)

// The rate-limit state every limited response carries, in two header families: the IETF
// RateLimit-Limit / RateLimit-Remaining / RateLimit-Reset (draft-ietf-httpapi-ratelimit-headers;
// Reset is the seconds until the window resets) and the X-RateLimit-* the API already sent
// (X-RateLimit-Reset is the reset as a Unix time). A 429 also carries Retry-After in seconds.
const (
	HeaderRateLimitLimit      = "RateLimit-Limit"
	HeaderRateLimitRemaining  = "RateLimit-Remaining"
	HeaderRateLimitReset      = "RateLimit-Reset"
	HeaderXRateLimitLimit     = "X-RateLimit-Limit"
	HeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderXRateLimitReset     = "X-RateLimit-Reset"
)

// SetRateLimitHeaders writes one limiter's state in both families.
func SetRateLimitHeaders(h http.Header, limit, remaining int, reset, now time.Time) {
	if remaining < 0 {
		remaining = 0
	}
	h.Set(HeaderRateLimitLimit, strconv.Itoa(limit))
	h.Set(HeaderXRateLimitLimit, strconv.Itoa(limit))
	h.Set(HeaderRateLimitRemaining, strconv.Itoa(remaining))
	h.Set(HeaderXRateLimitRemaining, strconv.Itoa(remaining))
	h.Set(HeaderRateLimitReset, strconv.Itoa(secondsUntil(reset, now)))
	h.Set(HeaderXRateLimitReset, strconv.FormatInt(reset.Unix(), 10))
}

func secondsUntil(reset, now time.Time) int {
	secs := int(math.Ceil(reset.Sub(now).Seconds()))
	if secs < 0 {
		return 0
	}
	return secs
}

// mirrorRateLimitHeaders copies the X-RateLimit-* httprate wrote into the RateLimit-* names.
func mirrorRateLimitHeaders(h http.Header, now time.Time) {
	if v := h.Get(HeaderXRateLimitLimit); v != "" {
		h.Set(HeaderRateLimitLimit, v)
	}
	if v := h.Get(HeaderXRateLimitRemaining); v != "" {
		h.Set(HeaderRateLimitRemaining, v)
	}
	if v := h.Get(HeaderXRateLimitReset); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			h.Set(HeaderRateLimitReset, strconv.Itoa(secondsUntil(time.Unix(epoch, 0), now)))
		}
	}
}

// LimitByClientIP limits requests per client IP (ClientIP, which RealClientIP has already
// written into RemoteAddr) to requests per window, answering with both header families.
// The limiter is built once, so every route the returned middleware is mounted on shares
// one bucket per IP, as httprate.LimitByIP does. httprate writes its X-RateLimit-* before it
// calls the next handler or its limit handler, so both mirror them; the limit handler keeps
// httprate's plain 429 (with its Retry-After), which ErrorEnvelope turns into the JSON
// RATE_LIMITED envelope.
func LimitByClientIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	limit := httprate.Limit(requests, window,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			mirrorRateLimitHeaders(w.Header(), time.Now())
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}),
	)
	return func(next http.Handler) http.Handler {
		return limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mirrorRateLimitHeaders(w.Header(), time.Now())
			next.ServeHTTP(w, r)
		}))
	}
}
