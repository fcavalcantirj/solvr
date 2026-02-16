// Package middleware provides HTTP middleware for the Solvr API.
package middleware

/**
 * E2E tests for Rate Limiting system
 *
 * Per PRD line 5070-5078:
 * - E2E: Rate limiting
 * - Exceed rate limit
 * - Verify 429 response
 * - Verify resets after window
 *
 * These tests verify the complete rate limiting flow:
 * 1. Make requests up to the limit (should all succeed)
 * 2. Exceed the rate limit
 * 3. Verify 429 Too Many Requests response
 * 4. Verify correct headers (X-RateLimit-*, Retry-After)
 * 5. Verify limit resets after window expires
 */

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// E2E Tests: Complete Rate Limiting Flow
// ============================================================================

// TestE2E_RateLimit_ExceedLimitReturns429 tests the complete flow of:
// 1. Making requests up to the limit
// 2. Exceeding the limit
// 3. Verifying 429 response with correct error body
func TestE2E_RateLimit_ExceedLimitReturns429(t *testing.T) {
	// Create a rate limiter with a small limit for testing
	config := &RateLimitConfig{
		AgentGeneralLimit: 5, // Small limit for fast testing
		HumanGeneralLimit: 3,
		GeneralWindow:     time.Minute,
		SearchLimitPerMin: 3,
		AgentPostsPerHour: 2,
		HumanPostsPerHour: 1,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 2,
		HumanAnswersPerHour: 1,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	// Create a simple handler that returns 200
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	agentID := "e2e-test-agent"
	agentCreatedAt := time.Now().Add(-48 * time.Hour) // Old account (no 50% reduction)

	// Step 1: Make requests up to the limit (5 requests for agent)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAgentToContextE2E(req, agentID, agentCreatedAt)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Request %d should succeed, got status %d", i+1, rec.Code)
		}

		// Verify X-RateLimit headers are present
		if rec.Header().Get("X-RateLimit-Limit") == "" {
			t.Errorf("Request %d: X-RateLimit-Limit header missing", i+1)
		}
		if rec.Header().Get("X-RateLimit-Remaining") == "" {
			t.Errorf("Request %d: X-RateLimit-Remaining header missing", i+1)
		}
		if rec.Header().Get("X-RateLimit-Reset") == "" {
			t.Errorf("Request %d: X-RateLimit-Reset header missing", i+1)
		}
	}

	// Step 2: Exceed the rate limit (6th request)
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Step 3: Verify 429 response
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Request 6 should be rate limited with 429, got status %d", rec.Code)
	}

	// Verify Content-Type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Verify Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be present on 429 response")
	}
	retryAfterSec, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Errorf("Retry-After should be a number, got %s", retryAfter)
	}
	if retryAfterSec < 1 {
		t.Errorf("Retry-After should be at least 1 second, got %d", retryAfterSec)
	}

	// Verify error response body structure
	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	errObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}
	if errObj["code"] != "RATE_LIMITED" {
		t.Errorf("Expected error code RATE_LIMITED, got %v", errObj["code"])
	}
	if errObj["message"] == "" {
		t.Error("Expected error message to be present")
	}

	// Verify X-RateLimit-Remaining is 0
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	if remaining != "0" {
		t.Errorf("X-RateLimit-Remaining should be 0, got %s", remaining)
	}
}

// TestE2E_RateLimit_ResetsAfterWindow tests that rate limit resets after window expires.
func TestE2E_RateLimit_ResetsAfterWindow(t *testing.T) {
	// Use a very short window for testing (10 milliseconds)
	config := &RateLimitConfig{
		AgentGeneralLimit: 2,
		HumanGeneralLimit: 2,
		GeneralWindow:     10 * time.Millisecond, // Very short for testing
		SearchLimitPerMin: 2,
		AgentPostsPerHour: 2,
		HumanPostsPerHour: 1,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 2,
		HumanAnswersPerHour: 1,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	agentID := "e2e-reset-agent"
	agentCreatedAt := time.Now().Add(-48 * time.Hour)

	// Step 1: Exhaust the rate limit (2 requests)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAgentToContextE2E(req, agentID, agentCreatedAt)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("Request %d should succeed, got %d", i+1, rec.Code)
		}
	}

	// Step 2: Verify rate limited
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Request 3 should be rate limited, got %d", rec.Code)
	}

	// Step 3: Wait for window to expire
	time.Sleep(15 * time.Millisecond)

	// Step 4: Verify limit has reset - request should succeed
	req = httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Request after window reset should succeed, got %d", rec.Code)
	}

	// Verify remaining is reset (should be 1 after this request)
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	if remaining != "1" {
		t.Errorf("X-RateLimit-Remaining should be 1 after reset, got %s", remaining)
	}
}

// TestE2E_RateLimit_HumanFlowComplete tests complete flow for human users.
func TestE2E_RateLimit_HumanFlowComplete(t *testing.T) {
	config := &RateLimitConfig{
		AgentGeneralLimit: 5,
		HumanGeneralLimit: 3, // Small limit for humans
		GeneralWindow:     time.Minute,
		SearchLimitPerMin: 3,
		AgentPostsPerHour: 2,
		HumanPostsPerHour: 1,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 2,
		HumanAnswersPerHour: 1,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	userID := "e2e-test-user-123"

	// Make 3 requests (human limit)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addHumanClaimsToContextE2E(req, userID, "test@example.com", "user")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("Human request %d should succeed, got %d", i+1, rec.Code)
		}
	}

	// 4th request should fail
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addHumanClaimsToContextE2E(req, userID, "test@example.com", "user")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Human request 4 should be rate limited, got %d", rec.Code)
	}

	// Verify Retry-After is set
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header should be set on 429 response")
	}
}

// TestE2E_RateLimit_SearchEndpoint tests rate limiting specifically for search.
func TestE2E_RateLimit_SearchEndpoint(t *testing.T) {
	config := &RateLimitConfig{
		AgentGeneralLimit: 100, // High general limit
		HumanGeneralLimit: 100,
		GeneralWindow:     time.Minute,
		SearchLimitPerMin: 3, // Low search limit for testing
		AgentPostsPerHour: 10,
		HumanPostsPerHour: 10,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 10,
		HumanAnswersPerHour: 10,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{},
			"total":   0,
		})
	}))

	agentID := "e2e-search-agent"
	agentCreatedAt := time.Now().Add(-48 * time.Hour)

	// Make 3 search requests (search limit)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/v1/search?q=test", nil)
		req = addAgentToContextE2E(req, agentID, agentCreatedAt)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("Search request %d should succeed, got %d", i+1, rec.Code)
		}
	}

	// 4th search should be rate limited
	req := httptest.NewRequest("GET", "/v1/search?q=test", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Search request 4 should be rate limited, got %d", rec.Code)
	}

	// General endpoint should still work (different bucket)
	req = httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("General request should still succeed, got %d", rec.Code)
	}
}

// TestE2E_RateLimit_PostCreation tests rate limiting for post creation.
func TestE2E_RateLimit_PostCreation(t *testing.T) {
	config := &RateLimitConfig{
		AgentGeneralLimit: 100,
		HumanGeneralLimit: 100,
		GeneralWindow:     time.Minute,
		SearchLimitPerMin: 100,
		AgentPostsPerHour: 2, // Small limit for testing
		HumanPostsPerHour: 1,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 10,
		HumanAnswersPerHour: 10,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "post-123"})
	}))

	agentID := "e2e-post-agent"
	agentCreatedAt := time.Now().Add(-48 * time.Hour)

	// Make 2 POST requests (agent post limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/v1/posts", nil)
		req = addAgentToContextE2E(req, agentID, agentCreatedAt)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("Post request %d should succeed, got %d", i+1, rec.Code)
		}
	}

	// 3rd post should be rate limited
	req := httptest.NewRequest("POST", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Post request 3 should be rate limited, got %d", rec.Code)
	}

	// Verify correct error code
	var errResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &errResp)
	errObj := errResp["error"].(map[string]interface{})
	if errObj["code"] != "RATE_LIMITED" {
		t.Errorf("Expected error code RATE_LIMITED, got %v", errObj["code"])
	}
}

// TestE2E_RateLimit_NewAccountHalfLimit tests 50% limit for new accounts.
func TestE2E_RateLimit_NewAccountHalfLimit(t *testing.T) {
	config := &RateLimitConfig{
		AgentGeneralLimit: 10, // Will become 5 for new accounts
		HumanGeneralLimit: 10,
		GeneralWindow:     time.Minute,
		SearchLimitPerMin: 10,
		AgentPostsPerHour: 10,
		HumanPostsPerHour: 10,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 10,
		HumanAnswersPerHour: 10,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// New agent (created 12 hours ago, less than 24h threshold)
	newAgentID := "e2e-new-agent"
	newAgentCreatedAt := time.Now().Add(-12 * time.Hour)

	// Make 5 requests (50% of 10)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAgentToContextE2E(req, newAgentID, newAgentCreatedAt)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("New agent request %d should succeed, got %d", i+1, rec.Code)
		}
	}

	// 6th request should be rate limited (50% of 10 = 5)
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, newAgentID, newAgentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("New agent request 6 should be rate limited at 50%%, got %d", rec.Code)
	}

	// Verify limit header shows reduced limit (5, not 10)
	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader != "5" {
		t.Errorf("X-RateLimit-Limit should be 5 for new account, got %s", limitHeader)
	}
}

// TestE2E_RateLimit_DifferentUsersIndependent tests that different users have independent limits.
func TestE2E_RateLimit_DifferentUsersIndependent(t *testing.T) {
	config := &RateLimitConfig{
		AgentGeneralLimit: 5,
		HumanGeneralLimit: 3,
		GeneralWindow:     time.Minute,
		SearchLimitPerMin: 10,
		AgentPostsPerHour: 10,
		HumanPostsPerHour: 10,
		PostsWindow:       time.Hour,
		AgentAnswersPerHour: 10,
		HumanAnswersPerHour: 10,
		AnswersWindow:       time.Hour,
		NewAccountThreshold: 24 * time.Hour,
	}

	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	agent1ID := "e2e-agent-1"
	agent2ID := "e2e-agent-2"
	agentCreatedAt := time.Now().Add(-48 * time.Hour)

	// Exhaust limit for agent1
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAgentToContextE2E(req, agent1ID, agentCreatedAt)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Agent1 should now be rate limited
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agent1ID, agentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Agent1 should be rate limited, got %d", rec.Code)
	}

	// Agent2 should still have full quota
	req = httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agent2ID, agentCreatedAt)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Agent2 should NOT be rate limited, got %d", rec.Code)
	}

	// Verify agent2's remaining is 4 (used 1 of 5)
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	if remaining != "4" {
		t.Errorf("Agent2 X-RateLimit-Remaining should be 4, got %s", remaining)
	}
}

// TestE2E_RateLimit_HeadersComplete tests all required headers are present.
func TestE2E_RateLimit_HeadersComplete(t *testing.T) {
	config := DefaultRateLimitConfig()
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, config)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	agentID := "e2e-headers-agent"
	agentCreatedAt := time.Now().Add(-48 * time.Hour)

	// First request - check all headers
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContextE2E(req, agentID, agentCreatedAt)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify all rate limit headers
	headers := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
	}
	for _, h := range headers {
		if rec.Header().Get(h) == "" {
			t.Errorf("Header %s should be present", h)
		}
	}

	// Verify X-RateLimit-Limit is correct for agent (launch limit)
	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader != "60" {  // Launch limit (SPEC: 120)
		t.Errorf("X-RateLimit-Limit should be 60 for agent, got %s", limitHeader)
	}

	// Verify X-RateLimit-Remaining is 59 after 1 request
	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")
	if remainingHeader != "59" {  // Launch limit (SPEC: 119)
		t.Errorf("X-RateLimit-Remaining should be 59, got %s", remainingHeader)
	}

	// Verify X-RateLimit-Reset is a valid Unix timestamp
	resetHeader := rec.Header().Get("X-RateLimit-Reset")
	resetTs, err := strconv.ParseInt(resetHeader, 10, 64)
	if err != nil {
		t.Errorf("X-RateLimit-Reset should be valid Unix timestamp, got %s", resetHeader)
	}
	resetTime := time.Unix(resetTs, 0)
	if resetTime.Before(time.Now()) {
		t.Errorf("X-RateLimit-Reset should be in the future, got %v", resetTime)
	}
}

// ============================================================================
// Helper Functions for E2E Tests
// ============================================================================

// addAgentToContextE2E adds agent to request context for E2E tests.
func addAgentToContextE2E(r *http.Request, agentID string, createdAt time.Time) *http.Request {
	agent := &models.Agent{
		ID:        agentID,
		CreatedAt: createdAt,
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

// addHumanClaimsToContextE2E adds JWT claims to request context for E2E tests.
func addHumanClaimsToContextE2E(r *http.Request, userID, email, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}
