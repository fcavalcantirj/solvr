package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// Every rate-limited response carries the limiter's state in the IETF headers
// (RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset in seconds) next to the X-RateLimit-*
// the API already sent (X-RateLimit-Reset as a Unix time), and every 429 carries Retry-After.

func assertBothRateLimitFamilies(t *testing.T, h http.Header, limit, remaining int) {
	t.Helper()
	for name, want := range map[string]string{
		"RateLimit-Limit": strconv.Itoa(limit), "X-RateLimit-Limit": strconv.Itoa(limit),
		"RateLimit-Remaining": strconv.Itoa(remaining), "X-RateLimit-Remaining": strconv.Itoa(remaining),
	} {
		if got := h.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if secs, err := strconv.Atoi(h.Get("RateLimit-Reset")); err != nil || secs < 0 || secs > 3600 {
		t.Errorf("RateLimit-Reset = %q, want the seconds until the window resets", h.Get("RateLimit-Reset"))
	}
	if epoch, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err != nil || epoch < time.Now().Unix()-1 {
		t.Errorf("X-RateLimit-Reset = %q, want the reset as a Unix time", h.Get("X-RateLimit-Reset"))
	}
}

func assertRetryAfterSeconds(t *testing.T, h http.Header) {
	t.Helper()
	if secs, err := strconv.Atoi(h.Get("Retry-After")); err != nil || secs < 1 {
		t.Errorf("Retry-After = %q, want whole seconds", h.Get("Retry-After"))
	}
}

func TestRateLimiter_SendsBothHeaderFamilies(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	cfg.AgentGeneralLimit = 2
	rl := NewRateLimiter(NewInMemoryRateLimitStore(), cfg)
	h := rl.Middleware(okHandler())
	send := func() *httptest.ResponseRecorder {
		req := addAgentToContext(httptest.NewRequest(http.MethodGet, "/v1/me", nil), "agent_headers", time.Now().Add(-48*time.Hour))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	for i, remaining := range []int{1, 0} {
		rr := send()
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, rr.Code)
		}
		assertBothRateLimitFamilies(t, rr.Header(), 2, remaining)
	}
	rr := send()
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("third request: %d, want 429", rr.Code)
	}
	assertBothRateLimitFamilies(t, rr.Header(), 2, 0)
	assertRetryAfterSeconds(t, rr.Header())
}

type fixedCreateCounter struct{ count int }

func (c fixedCreateCounter) CountRecentCreates(context.Context, string, string, string, time.Time) (int, time.Time, time.Time, error) {
	return c.count, time.Now().Add(-10 * time.Minute), time.Now().Add(-48 * time.Hour), nil
}

func TestCreateRateLimit_SendsBothHeaderFamilies(t *testing.T) {
	cfg := DefaultRateLimitConfig() // AgentPostsPerHour 5
	for _, tc := range []struct {
		count, status, remaining int
	}{{count: 2, status: http.StatusOK, remaining: 2}, {count: 5, status: http.StatusTooManyRequests, remaining: 0}} {
		h := CreateRateLimit(fixedCreateCounter{tc.count}, cfg, CreateOpPosts)(okHandler())
		req := addAgentToContext(httptest.NewRequest(http.MethodPost, "/v1/posts", nil), "agent_creates", time.Now().Add(-48*time.Hour))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.status {
			t.Fatalf("with %d creates: %d, want %d", tc.count, rr.Code, tc.status)
		}
		assertBothRateLimitFamilies(t, rr.Header(), 5, tc.remaining)
		if tc.status == http.StatusTooManyRequests {
			assertRetryAfterSeconds(t, rr.Header())
		}
	}
}

func TestRegistrationRateLimiter_SendsRateLimitHeadersOnEveryResponse(t *testing.T) {
	rl := NewRegistrationRateLimiter(NewInMemoryRateLimitStore(), &RegistrationRateLimitConfig{MaxPerIP: 2, Window: time.Hour, LogPrefix: "test"})
	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) }))
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.0.2.10:4000"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	for i, remaining := range []int{1, 0} {
		rr := send()
		if rr.Code != http.StatusCreated {
			t.Fatalf("registration %d: %d", i+1, rr.Code)
		}
		assertBothRateLimitFamilies(t, rr.Header(), 2, remaining)
	}
	rr := send()
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("third registration: %d, want 429", rr.Code)
	}
	assertBothRateLimitFamilies(t, rr.Header(), 2, 0)
	assertRetryAfterSeconds(t, rr.Header())
}

func TestLimitByClientIP_SendsBothFamiliesAndRetryAfter(t *testing.T) {
	h := LimitByClientIP(2, time.Minute)(okHandler())
	send := func(remoteAddr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/rooms/r/entries", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	for i, remaining := range []int{1, 0} {
		rr := send("198.51.100.7")
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, rr.Code)
		}
		assertBothRateLimitFamilies(t, rr.Header(), 2, remaining)
	}
	rr := send("198.51.100.7")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("third request: %d, want 429", rr.Code)
	}
	assertBothRateLimitFamilies(t, rr.Header(), 2, 0)
	assertRetryAfterSeconds(t, rr.Header())
	if rr := send("198.51.100.8"); rr.Code != http.StatusOK {
		t.Errorf("another client IP has its own bucket: %d", rr.Code)
	}
}

// One LimitByClientIP mounted on several routes is one bucket per IP across them (the room
// write adapters and the entries route share the agent and human write limits).
func TestLimitByClientIP_MountsShareOneBucket(t *testing.T) {
	limit := LimitByClientIP(2, time.Minute)
	a, b := limit(okHandler()), limit(okHandler())
	send := func(h http.Handler) int {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "198.51.100.9"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	send(a)
	send(b)
	if code := send(a); code != http.StatusTooManyRequests {
		t.Errorf("third request across two mounts: %d, want 429", code)
	}
}
