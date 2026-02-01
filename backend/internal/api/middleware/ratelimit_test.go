package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockRateLimitStore implements RateLimitStore interface for testing.
type MockRateLimitStore struct {
	records   map[string]*RateLimitRecord
	failGet   bool
	failStore bool
}

func NewMockRateLimitStore() *MockRateLimitStore {
	return &MockRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
}

func (m *MockRateLimitStore) GetRecord(ctx context.Context, key string) (*RateLimitRecord, error) {
	if m.failGet {
		return nil, context.DeadlineExceeded
	}
	record, exists := m.records[key]
	if !exists {
		return nil, nil
	}
	return record, nil
}

func (m *MockRateLimitStore) IncrementAndGet(ctx context.Context, key string, window time.Duration) (*RateLimitRecord, error) {
	if m.failStore {
		return nil, context.DeadlineExceeded
	}
	record, exists := m.records[key]
	now := time.Now()
	if !exists || now.Sub(record.WindowStart) > window {
		// New window
		record = &RateLimitRecord{
			Key:         key,
			Count:       1,
			WindowStart: now,
		}
		m.records[key] = record
		return record, nil
	}
	// Same window
	record.Count++
	return record, nil
}

func (m *MockRateLimitStore) SetRecord(key string, record *RateLimitRecord) {
	m.records[key] = record
}

// Helper to add JWT claims to context
func addClaimsToContext(r *http.Request, userID, email, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// Helper to add agent to context
func addAgentToContext(r *http.Request, agentID string, createdAt time.Time) *http.Request {
	agent := &models.Agent{
		ID:        agentID,
		CreatedAt: createdAt,
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

// Helper function to create a simple handler that returns 200 OK
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

// TestRateLimiter_AgentGeneralLimit tests 120 requests/minute for agents.
func TestRateLimiter_AgentGeneralLimit(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 120 requests (should all succeed)
	for i := 0; i < 120; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 121st request should be rate limited
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 121 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_HumanGeneralLimit tests 60 requests/minute for humans.
func TestRateLimiter_HumanGeneralLimit(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 60 requests (should all succeed)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addClaimsToContext(req, "user-123", "test@example.com", "user")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 61st request should be rate limited
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addClaimsToContext(req, "user-123", "test@example.com", "user")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 61 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_SearchLimit tests 60 searches/minute for agents.
func TestRateLimiter_SearchLimit(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 60 search requests (should all succeed)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/v1/search?q=test", nil)
		req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("search request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 61st search request should be rate limited
	req := httptest.NewRequest("GET", "/v1/search?q=test", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("search request 61 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_PostsLimitAgent tests 10 posts/hour for agents.
func TestRateLimiter_PostsLimitAgent(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 10 post requests (should all succeed)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/v1/posts", nil)
		req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("post request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 11th post request should be rate limited
	req := httptest.NewRequest("POST", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("post request 11 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_PostsLimitHuman tests 5 posts/hour for humans.
func TestRateLimiter_PostsLimitHuman(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 5 post requests (should all succeed)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/v1/posts", nil)
		req = addClaimsToContext(req, "user-123", "test@example.com", "user")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("post request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 6th post request should be rate limited
	req := httptest.NewRequest("POST", "/v1/posts", nil)
	req = addClaimsToContext(req, "user-123", "test@example.com", "user")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("post request 6 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_AnswersLimitAgent tests 30 answers/hour for agents.
func TestRateLimiter_AnswersLimitAgent(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 30 answer requests (should all succeed)
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest("POST", "/v1/questions/123/answers", nil)
		req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("answer request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 31st answer request should be rate limited
	req := httptest.NewRequest("POST", "/v1/questions/123/answers", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("answer request 31 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_AnswersLimitHuman tests 20 answers/hour for humans.
func TestRateLimiter_AnswersLimitHuman(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Make 20 answer requests (should all succeed)
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("POST", "/v1/questions/123/answers", nil)
		req = addClaimsToContext(req, "user-123", "test@example.com", "user")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("answer request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// 21st answer request should be rate limited
	req := httptest.NewRequest("POST", "/v1/questions/123/answers", nil)
	req = addClaimsToContext(req, "user-123", "test@example.com", "user")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("answer request 21 should be rate limited, got status %d", rec.Code)
	}
}

// TestRateLimiter_NewAccount50Percent tests 50% rate limit for new accounts.
func TestRateLimiter_NewAccount50Percent(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// New agent (created 12 hours ago, less than 24h)
	// Should get 60 requests/min instead of 120
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addAgentToContext(req, "new-agent", time.Now().Add(-12*time.Hour))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should succeed for new agent, got status %d", i+1, rec.Code)
		}
	}

	// 61st request should be rate limited (50% of 120)
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "new-agent", time.Now().Add(-12*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 61 should be rate limited for new agent, got status %d", rec.Code)
	}
}

// TestRateLimiter_Returns429 tests that 429 status is returned when rate limited.
func TestRateLimiter_Returns429(t *testing.T) {
	store := NewMockRateLimitStore()
	// Pre-fill the store with at-limit count
	store.SetRecord("agent:test-agent:general", &RateLimitRecord{
		Key:         "agent:test-agent:general",
		Count:       120,
		WindowStart: time.Now(),
	})

	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", rec.Code)
	}
}

// TestRateLimiter_XRateLimitLimitHeader tests X-RateLimit-Limit header is set.
func TestRateLimiter_XRateLimitLimitHeader(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader == "" {
		t.Error("X-RateLimit-Limit header should be set")
	}
	if limitHeader != "120" {
		t.Errorf("X-RateLimit-Limit should be 120 for agent, got %s", limitHeader)
	}
}

// TestRateLimiter_XRateLimitRemainingHeader tests X-RateLimit-Remaining header is set.
func TestRateLimiter_XRateLimitRemainingHeader(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Error("X-RateLimit-Remaining header should be set")
	}
	// After 1 request, should have 119 remaining
	if remainingHeader != "119" {
		t.Errorf("X-RateLimit-Remaining should be 119 after 1 request, got %s", remainingHeader)
	}
}

// TestRateLimiter_XRateLimitResetHeader tests X-RateLimit-Reset header is set.
func TestRateLimiter_XRateLimitResetHeader(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resetHeader := rec.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("X-RateLimit-Reset header should be set")
	}
}

// TestRateLimiter_RetryAfterHeader tests Retry-After header is set on 429 response.
func TestRateLimiter_RetryAfterHeader(t *testing.T) {
	store := NewMockRateLimitStore()
	// Pre-fill the store with at-limit count
	store.SetRecord("agent:test-agent:general", &RateLimitRecord{
		Key:         "agent:test-agent:general",
		Count:       120,
		WindowStart: time.Now(),
	})

	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be set on 429 response")
	}
}

// TestRateLimiter_DifferentUsersIndependent tests different users have independent limits.
func TestRateLimiter_DifferentUsersIndependent(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Exhaust limit for user1
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("GET", "/v1/posts", nil)
		req = addClaimsToContext(req, "user-1", "user1@example.com", "user")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// user1 should be rate limited
	req1 := httptest.NewRequest("GET", "/v1/posts", nil)
	req1 = addClaimsToContext(req1, "user-1", "user1@example.com", "user")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusTooManyRequests {
		t.Errorf("user1 should be rate limited, got status %d", rec1.Code)
	}

	// user2 should NOT be rate limited
	req2 := httptest.NewRequest("GET", "/v1/posts", nil)
	req2 = addClaimsToContext(req2, "user-2", "user2@example.com", "user")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("user2 should NOT be rate limited, got status %d", rec2.Code)
	}
}

// TestRateLimiter_UnauthenticatedAllowed tests unauthenticated requests are allowed.
func TestRateLimiter_UnauthenticatedAllowed(t *testing.T) {
	store := NewMockRateLimitStore()
	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	// Request without auth context
	req := httptest.NewRequest("GET", "/v1/posts", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should be allowed through (auth middleware will handle rejection if needed)
	if rec.Code != http.StatusOK {
		t.Errorf("unauthenticated request should be allowed through, got %d", rec.Code)
	}
}

// TestRateLimiter_KeyGeneration tests rate limit key generation.
func TestRateLimiter_KeyGeneration(t *testing.T) {
	tests := []struct {
		name        string
		isAgent     bool
		identifier  string
		operation   string
		expectedKey string
	}{
		{
			name:        "agent general",
			isAgent:     true,
			identifier:  "my-agent",
			operation:   "general",
			expectedKey: "agent:my-agent:general",
		},
		{
			name:        "human general",
			isAgent:     false,
			identifier:  "user-uuid-123",
			operation:   "general",
			expectedKey: "human:user-uuid-123:general",
		},
		{
			name:        "agent search",
			isAgent:     true,
			identifier:  "search-agent",
			operation:   "search",
			expectedKey: "agent:search-agent:search",
		},
		{
			name:        "human posts",
			isAgent:     false,
			identifier:  "user-456",
			operation:   "posts",
			expectedKey: "human:user-456:posts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GenerateRateLimitKey(tt.isAgent, tt.identifier, tt.operation)
			if key != tt.expectedKey {
				t.Errorf("expected key %s, got %s", tt.expectedKey, key)
			}
		})
	}
}

// TestRateLimiter_OperationDetection tests correct operation is detected from request.
func TestRateLimiter_OperationDetection(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		expectedOp    string
	}{
		{"general GET", "GET", "/v1/posts", "general"},
		{"search", "GET", "/v1/search", "search"},
		{"search with query", "GET", "/v1/search?q=test", "search"},
		{"create post", "POST", "/v1/posts", "posts"},
		{"create problem", "POST", "/v1/problems", "posts"},
		{"create question", "POST", "/v1/questions", "posts"},
		{"create idea", "POST", "/v1/ideas", "posts"},
		{"create answer", "POST", "/v1/questions/123/answers", "answers"},
		{"general PUT", "PUT", "/v1/posts/123", "general"},
		{"general DELETE", "DELETE", "/v1/posts/123", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			op := DetectOperation(req)
			if op != tt.expectedOp {
				t.Errorf("expected operation %s, got %s", tt.expectedOp, op)
			}
		})
	}
}

// TestRateLimitConfig_Default tests default configuration values.
func TestRateLimitConfig_Default(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.AgentGeneralLimit != 120 {
		t.Errorf("AgentGeneralLimit should be 120, got %d", config.AgentGeneralLimit)
	}
	if config.HumanGeneralLimit != 60 {
		t.Errorf("HumanGeneralLimit should be 60, got %d", config.HumanGeneralLimit)
	}
	if config.SearchLimitPerMin != 60 {
		t.Errorf("SearchLimitPerMin should be 60, got %d", config.SearchLimitPerMin)
	}
	if config.AgentPostsPerHour != 10 {
		t.Errorf("AgentPostsPerHour should be 10, got %d", config.AgentPostsPerHour)
	}
	if config.HumanPostsPerHour != 5 {
		t.Errorf("HumanPostsPerHour should be 5, got %d", config.HumanPostsPerHour)
	}
	if config.AgentAnswersPerHour != 30 {
		t.Errorf("AgentAnswersPerHour should be 30, got %d", config.AgentAnswersPerHour)
	}
	if config.HumanAnswersPerHour != 20 {
		t.Errorf("HumanAnswersPerHour should be 20, got %d", config.HumanAnswersPerHour)
	}
	if config.NewAccountThreshold != 24*time.Hour {
		t.Errorf("NewAccountThreshold should be 24h, got %v", config.NewAccountThreshold)
	}
}

// TestRateLimiter_ResponseBody tests 429 response body format.
func TestRateLimiter_ResponseBody(t *testing.T) {
	store := NewMockRateLimitStore()
	store.SetRecord("agent:test-agent:general", &RateLimitRecord{
		Key:         "agent:test-agent:general",
		Count:       120,
		WindowStart: time.Now(),
	})

	rl := NewRateLimiter(store, DefaultRateLimitConfig())
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/v1/posts", nil)
	req = addAgentToContext(req, "test-agent", time.Now().Add(-25*time.Hour))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type should be application/json, got %s", contentType)
	}

	// Check body contains error code
	body := rec.Body.String()
	if body == "" {
		t.Error("response body should not be empty")
	}
}
