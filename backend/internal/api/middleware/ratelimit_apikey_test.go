package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// Helper to add user API key context for rate limiting per key.
// This simulates authentication via user API key.
func addAPIKeyToContext(r *http.Request, userID, apiKeyID string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	ctx = auth.ContextWithAPIKeyID(ctx, apiKeyID)
	return r.WithContext(ctx)
}

// TestRateLimiter_PerAPIKey_IndependentLimits tests that different API keys for the same user
// have independent rate limits.
func TestRateLimiter_PerAPIKey_IndependentLimits(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Exhaust limit for API key 1 (60 requests for human)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAPIKeyToContext(req, "user-123", "api-key-1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d with api-key-1 should succeed, got status %d", i+1, rec.Code)
		}
	}

	// API key 1 should be rate limited
	req1 := httptest.NewRequest("GET", "/v1/posts", nil)
	req1 = addAPIKeyToContext(req1, "user-123", "api-key-1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusTooManyRequests {
		t.Errorf("api-key-1 should be rate limited, got status %d", rec1.Code)
	}

	// API key 2 (same user) should NOT be rate limited - proves per-key limiting
	req2 := httptest.NewRequest("GET", "/v1/posts", nil)
	req2 = addAPIKeyToContext(req2, "user-123", "api-key-2")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("api-key-2 should NOT be rate limited, got status %d", rec2.Code)
	}
}

// TestRateLimiter_PerAPIKey_KeyGeneration tests that rate limit keys include API key ID.
func TestRateLimiter_PerAPIKey_KeyGeneration(t *testing.T) {
	tests := []struct {
		name        string
		isAgent     bool
		identifier  string
		apiKeyID    string
		operation   string
		expectedKey string
	}{
		{
			name:        "human with API key - general",
			isAgent:     false,
			identifier:  "user-123",
			apiKeyID:    "apikey-abc",
			operation:   "general",
			expectedKey: "apikey:apikey-abc:general",
		},
		{
			name:        "human with API key - search",
			isAgent:     false,
			identifier:  "user-456",
			apiKeyID:    "apikey-xyz",
			operation:   "search",
			expectedKey: "apikey:apikey-xyz:search",
		},
		{
			name:        "human without API key - general",
			isAgent:     false,
			identifier:  "user-123",
			apiKeyID:    "",
			operation:   "general",
			expectedKey: "human:user-123:general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GenerateRateLimitKeyWithAPIKey(tt.isAgent, tt.identifier, tt.apiKeyID, tt.operation)
			if key != tt.expectedKey {
				t.Errorf("expected key %s, got %s", tt.expectedKey, key)
			}
		})
	}
}

// TestRateLimiter_PerAPIKey_UsesAPIKeyIDFromContext tests that the rate limiter
// correctly extracts and uses the API key ID from context.
func TestRateLimiter_PerAPIKey_UsesAPIKeyIDFromContext(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make a request with API key context
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAPIKeyToContext(req, "user-123", "test-apikey-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check that the store has a record with the API key ID in the key
	expectedKey := "apikey:test-apikey-id:general"
	record, err := store.GetRecord(context.Background(), expectedKey)
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}
	if record == nil {
		t.Errorf("expected record for key %s to exist", expectedKey)
	}
}

// TestRateLimiter_PerAPIKey_DifferentTiers tests that API keys can have different rate limit tiers.
// This tests the Allow different limits per key tier requirement.
func TestRateLimiter_PerAPIKey_DifferentTiers(t *testing.T) {
	store := NewMockRateLimitStore()
	config := DefaultRateLimitConfig()
	config.APIKeyDefaultLimit = 60       // Default API key limit
	config.APIKeyTierLimits = map[string]int{
		"premium": 180, // Premium tier gets 3x requests
		"basic":   60,  // Basic tier is default
	}
	rl := NewRateLimiter(store, config)
	handler := rl.Middleware(okHandler())

	// Create helper that includes tier information
	addAPIKeyWithTier := func(r *http.Request, userID, apiKeyID, tier string) *http.Request {
		claims := &auth.Claims{
			UserID: userID,
			Email:  "test@example.com",
			Role:   "user",
		}
		ctx := auth.ContextWithClaims(r.Context(), claims)
		ctx = auth.ContextWithAPIKeyID(ctx, apiKeyID)
		ctx = auth.ContextWithAPIKeyTier(ctx, tier)
		return r.WithContext(ctx)
	}

	// Premium key should have 180 request limit
	for i := 0; i < 180; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAPIKeyWithTier(req, "user-123", "premium-key", "premium")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("premium key request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 181st request should be rate limited
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAPIKeyWithTier(req, "user-123", "premium-key", "premium")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("premium key request 181 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_PerAPIKey_FallbackToUser tests that without API key ID,
// rate limiting falls back to user-based limiting.
func TestRateLimiter_PerAPIKey_FallbackToUser(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Use regular user claims without API key context
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addClaimsToContext(req, "user-123", "test@example.com", "user")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 61st should be rate limited (human limit is 60)
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addClaimsToContext(req, "user-123", "test@example.com", "user")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 61 should be rate limited, got status %d", rec.Code)
	}

	// Verify the key used was user-based, not API key based
	userKey := "human:user-123:general"
	record, _ := store.GetRecord(context.Background(), userKey)
	if record == nil {
		t.Errorf("expected user-based rate limit record to exist")
	}
}

// TestRateLimiter_PerAPIKey_Headers tests that rate limit headers reflect per-key limits.
func TestRateLimiter_PerAPIKey_Headers(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAPIKeyToContext(req, "user-123", "my-api-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check rate limit headers
	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader == "" {
		t.Error("X-RateLimit-Limit header should be set")
	}

	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Error("X-RateLimit-Remaining header should be set")
	}

	resetHeader := rec.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("X-RateLimit-Reset header should be set")
	}
}

// TestRateLimiter_PerAPIKey_TrackUsage tests that usage is tracked per API key.
func TestRateLimiter_PerAPIKey_TrackUsage(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 5 requests with key-1
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAPIKeyToContext(req, "user-123", "key-1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Make 3 requests with key-2
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAPIKeyToContext(req, "user-123", "key-2")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Verify independent tracking
	key1Record, _ := store.GetRecord(context.Background(), "apikey:key-1:general")
	if key1Record == nil {
		t.Fatal("key-1 record should exist")
	}
	if key1Record.Count != 5 {
		t.Errorf("key-1 should have count 5, got %d", key1Record.Count)
	}

	key2Record, _ := store.GetRecord(context.Background(), "apikey:key-2:general")
	if key2Record == nil {
		t.Fatal("key-2 record should exist")
	}
	if key2Record.Count != 3 {
		t.Errorf("key-2 should have count 3, got %d", key2Record.Count)
	}
}
