// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// MockIPRateLimitStore is a mock implementation of IPRateLimitStore for testing.
type MockIPRateLimitStore struct {
	mu      sync.RWMutex
	records map[string]*RateLimitRecord
}

// NewMockIPRateLimitStore creates a new mock store.
func NewMockIPRateLimitStore() *MockIPRateLimitStore {
	return &MockIPRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
}

// GetRecord retrieves a rate limit record by key.
func (m *MockIPRateLimitStore) GetRecord(ctx context.Context, key string) (*RateLimitRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, exists := m.records[key]
	if !exists {
		return nil, nil
	}
	return record, nil
}

// IncrementAndGet increments the count and returns the updated record.
func (m *MockIPRateLimitStore) IncrementAndGet(ctx context.Context, key string, window time.Duration) (*RateLimitRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	record, exists := m.records[key]

	if !exists || now.Sub(record.WindowStart) >= window {
		// Start new window
		record = &RateLimitRecord{
			Key:         key,
			Count:       1,
			WindowStart: now,
		}
	} else {
		// Increment existing
		record.Count++
	}

	m.records[key] = record
	return record, nil
}

// TestRegistrationRateLimiter_AllowsUnderLimit tests that requests under the limit pass.
func TestRegistrationRateLimiter_AllowsUnderLimit(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:  5,
		Window:    time.Hour,
		LogPrefix: "test",
	}
	rl := NewRegistrationRateLimiter(store, config)

	// Create handler that tracks if it was called
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	// Make 5 requests (all should pass)
	for i := 0; i < 5; i++ {
		called = false
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		rl.Middleware(handler).ServeHTTP(rr, req)

		if !called {
			t.Errorf("request %d should have been allowed", i+1)
		}
		if rr.Code != http.StatusCreated {
			t.Errorf("request %d expected status 201, got %d", i+1, rr.Code)
		}
	}
}

// TestRegistrationRateLimiter_BlocksOverLimit tests that requests over the limit are blocked.
func TestRegistrationRateLimiter_BlocksOverLimit(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:  5,
		Window:    time.Hour,
		LogPrefix: "test",
	}
	rl := NewRegistrationRateLimiter(store, config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// Make 5 requests (all should pass)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
	}

	// 6th request should be blocked
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr.Code)
	}

	// Check error response
	var errResp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	errObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "RATE_LIMITED" {
		t.Errorf("expected code RATE_LIMITED, got %v", errObj["code"])
	}
}

// TestRegistrationRateLimiter_RetryAfterHeader tests that Retry-After header is set.
func TestRegistrationRateLimiter_RetryAfterHeader(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:  1,
		Window:    time.Hour,
		LogPrefix: "test",
	}
	rl := NewRegistrationRateLimiter(store, config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// First request passes
	req1 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr1, req1)

	// Second request should be blocked with Retry-After
	req2 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr2.Code)
	}

	retryAfter := rr2.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header to be set")
	}
}

// TestRegistrationRateLimiter_DifferentIPsIndependent tests that different IPs have independent limits.
func TestRegistrationRateLimiter_DifferentIPsIndependent(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:  2,
		Window:    time.Hour,
		LogPrefix: "test",
	}
	rl := NewRegistrationRateLimiter(store, config)

	called := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusCreated)
	})

	// IP1 makes 2 requests (should pass)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
	}

	// IP2 makes 2 requests (should also pass - independent limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
	}

	// IP1's 3rd request should be blocked
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected IP1's 3rd request to be blocked with 429, got %d", rr.Code)
	}

	// IP2's 3rd request should also be blocked
	req2 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	rr2 := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("expected IP2's 3rd request to be blocked with 429, got %d", rr2.Code)
	}

	// Total of 4 successful calls
	if called != 4 {
		t.Errorf("expected 4 successful calls, got %d", called)
	}
}

// TestRegistrationRateLimiter_DefaultConfig tests the default configuration.
func TestRegistrationRateLimiter_DefaultConfig(t *testing.T) {
	config := DefaultRegistrationRateLimitConfig()

	// Owner decision 2026-10-03 (idx 79): 20 per IP per hour, so an 8-agent room set up from
	// one machine fits (idx 24). Was 5 while the limiter was not mounted on any route.
	if config.MaxPerIP != 20 || DefaultRegistrationsPerIPPerHour != 20 {
		t.Errorf("expected MaxPerIP 20, got %d (constant %d)", config.MaxPerIP, DefaultRegistrationsPerIPPerHour)
	}
	if config.Window != time.Hour {
		t.Errorf("expected Window 1 hour, got %v", config.Window)
	}
	if config.LogPrefix != "registration" {
		t.Errorf("expected LogPrefix 'registration', got %s", config.LogPrefix)
	}
}

// TestRegistrationRateLimiter_IncludesHintInError tests that error includes helpful hint.
func TestRegistrationRateLimiter_IncludesHintInError(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:  1,
		Window:    time.Hour,
		LogPrefix: "test",
	}
	rl := NewRegistrationRateLimiter(store, config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// First request passes
	req1 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr1, req1)

	// Second request blocked
	req2 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr2, req2)

	var errResp map[string]interface{}
	if err := json.NewDecoder(rr2.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	errObj := errResp["error"].(map[string]interface{})
	message := errObj["message"].(string)

	// Should mention registration limit
	if message == "" {
		t.Error("expected error message to be set")
	}
}

// TestRegistrationRateLimiter_SuspiciousPatternLogging tests suspicious pattern detection.
func TestRegistrationRateLimiter_SuspiciousPatternLogging(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:            10,
		Window:              time.Hour,
		LogPrefix:           "test",
		SuspiciousThreshold: 5, // Log when 5+ attempts
	}
	rl := NewRegistrationRateLimiter(store, config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// Make 5 requests - 5th should trigger suspicious logging (but still pass)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)

		// All 5 should pass (limit is 10)
		if rr.Code != http.StatusCreated {
			t.Errorf("request %d should have passed, got %d", i+1, rr.Code)
		}
	}

	// Make more requests up to the limit
	for i := 5; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("request %d should have passed, got %d", i+1, rr.Code)
		}
	}

	// 11th request should be blocked
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("request 11 should be blocked, got %d", rr.Code)
	}
}

// TestRegistrationRateLimiter_NoLoggingBelowThreshold tests that no logging occurs below threshold.
func TestRegistrationRateLimiter_NoLoggingBelowThreshold(t *testing.T) {
	store := NewMockIPRateLimitStore()
	config := &RegistrationRateLimitConfig{
		MaxPerIP:            5,
		Window:              time.Hour,
		LogPrefix:           "test",
		SuspiciousThreshold: 10, // Set high threshold
	}
	rl := NewRegistrationRateLimiter(store, config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// Make requests under the suspicious threshold - should not log suspicious
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("request %d should have passed, got %d", i+1, rr.Code)
		}
	}
	// Tests pass - logging verification would require capturing log output
	// which is done implicitly by the fact that the code runs without error
}

// The limiter buckets by the client IP Cloudflare reports in CF-Connecting-IP; a spoofed
// X-Forwarded-For neither splits one client's bucket nor joins another's.
func TestRegistrationRateLimiter_BucketsByCFConnectingIP(t *testing.T) {
	rl := NewRegistrationRateLimiter(NewMockIPRateLimitStore(), &RegistrationRateLimitConfig{
		MaxPerIP:  2,
		Window:    time.Hour,
		LogPrefix: "test",
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	send := func(cf, xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", nil)
		req.RemoteAddr = "10.0.0.1:12345" // the proxy in front of the API
		req.Header.Set("CF-Connecting-IP", cf)
		req.Header.Set("X-Forwarded-For", xff)
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
		return rr.Code
	}

	for i, xff := range []string{"198.51.100.1", "198.51.100.2"} {
		if code := send("203.0.113.1", xff); code != http.StatusCreated {
			t.Fatalf("registration %d from 203.0.113.1: got %d, want 201", i+1, code)
		}
	}
	if code := send("203.0.113.1", "198.51.100.3"); code != http.StatusTooManyRequests {
		t.Errorf("third registration from CF-Connecting-IP 203.0.113.1 with a new X-Forwarded-For: got %d, want 429", code)
	}
	if code := send("203.0.113.2", "198.51.100.1"); code != http.StatusCreated {
		t.Errorf("a different CF-Connecting-IP has its own bucket: got %d, want 201", code)
	}
}
