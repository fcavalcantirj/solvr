package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestJWTMiddleware(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Generate valid token
	validToken, _ := GenerateJWT(secret, "user-123", "test@example.com", "user", 15*time.Minute)

	// Generate expired token
	expiredToken, _ := GenerateJWT(secret, "user-123", "test@example.com", "user", -1*time.Minute)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
		wantClaims     bool
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken,
			wantStatusCode: http.StatusOK,
			wantClaims:     true,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "missing Bearer prefix",
			authHeader:     validToken,
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "empty token after Bearer",
			authHeader:     "Bearer ",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "expired token",
			authHeader:     "Bearer " + expiredToken,
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "wrong auth scheme",
			authHeader:     "Basic " + validToken,
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler that checks for claims in context
			var gotClaims *Claims
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotClaims = ClaimsFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			// Create the middleware
			middleware := JWTMiddleware(secret)(nextHandler)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Execute
			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			// Check claims presence
			if tt.wantClaims && gotClaims == nil {
				t.Error("expected claims in context but got nil")
			}
			if !tt.wantClaims && gotClaims != nil {
				t.Error("expected no claims but got some")
			}

			// Verify claims data if expected
			if tt.wantClaims && gotClaims != nil {
				if gotClaims.UserID != "user-123" {
					t.Errorf("claims.UserID = %v, want user-123", gotClaims.UserID)
				}
				if gotClaims.Email != "test@example.com" {
					t.Errorf("claims.Email = %v, want test@example.com", gotClaims.Email)
				}
			}
		})
	}
}

func TestClaimsFromContext(t *testing.T) {
	// Test with claims in context
	claims := &Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "admin",
	}

	ctx := ContextWithClaims(context.Background(), claims)
	gotClaims := ClaimsFromContext(ctx)

	if gotClaims == nil {
		t.Fatal("ClaimsFromContext() returned nil for context with claims")
	}
	if gotClaims.UserID != claims.UserID {
		t.Errorf("UserID = %v, want %v", gotClaims.UserID, claims.UserID)
	}
	if gotClaims.Email != claims.Email {
		t.Errorf("Email = %v, want %v", gotClaims.Email, claims.Email)
	}
	if gotClaims.Role != claims.Role {
		t.Errorf("Role = %v, want %v", gotClaims.Role, claims.Role)
	}

	// Test with no claims in context
	emptyCtx := context.Background()
	emptyClaims := ClaimsFromContext(emptyCtx)
	if emptyClaims != nil {
		t.Error("ClaimsFromContext() should return nil for context without claims")
	}
}

func TestOptionalJWTMiddleware(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Generate valid token
	validToken, _ := GenerateJWT(secret, "user-123", "test@example.com", "user", 15*time.Minute)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
		wantClaims     bool
	}{
		{
			name:           "valid token sets claims",
			authHeader:     "Bearer " + validToken,
			wantStatusCode: http.StatusOK,
			wantClaims:     true,
		},
		{
			name:           "no token still allows request",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
			wantClaims:     false,
		},
		{
			name:           "invalid token still allows request but no claims",
			authHeader:     "Bearer invalid",
			wantStatusCode: http.StatusOK,
			wantClaims:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotClaims *Claims
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotClaims = ClaimsFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := OptionalJWTMiddleware(secret)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			if tt.wantClaims && gotClaims == nil {
				t.Error("expected claims in context but got nil")
			}
			if !tt.wantClaims && gotClaims != nil {
				t.Error("expected no claims but got some")
			}
		})
	}
}

func TestAPIKeyMiddleware(t *testing.T) {
	// Create mock database with test agent
	db := NewMockAgentDB()
	testKey := "solvr_testkey123456789012345678901234567890"
	_, err := db.AddTestAgent("test_agent", "Test Agent", testKey)
	if err != nil {
		t.Fatalf("failed to add test agent: %v", err)
	}

	validator := NewAPIKeyValidator(db)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
		wantAgent      bool
	}{
		{
			name:           "valid API key",
			authHeader:     "Bearer " + testKey,
			wantStatusCode: http.StatusOK,
			wantAgent:      true,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
			wantAgent:      false,
		},
		{
			name:           "invalid API key",
			authHeader:     "Bearer solvr_invalidkey1234567890123456789012345",
			wantStatusCode: http.StatusUnauthorized,
			wantAgent:      false,
		},
		{
			name:           "non-API key token (JWT format)",
			authHeader:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzIn0.abcd",
			wantStatusCode: http.StatusUnauthorized,
			wantAgent:      false,
		},
		{
			name:           "missing Bearer prefix",
			authHeader:     testKey,
			wantStatusCode: http.StatusUnauthorized,
			wantAgent:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAgent *models.Agent
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAgent = AgentFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := APIKeyMiddleware(validator)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			if tt.wantAgent && gotAgent == nil {
				t.Error("expected agent in context but got nil")
			}
			if !tt.wantAgent && gotAgent != nil {
				t.Error("expected no agent but got some")
			}

			if tt.wantAgent && gotAgent != nil {
				if gotAgent.ID != "test_agent" {
					t.Errorf("agent ID = %v, want test_agent", gotAgent.ID)
				}
			}
		})
	}
}

func TestAgentFromContext(t *testing.T) {
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
	}

	ctx := ContextWithAgent(context.Background(), agent)
	gotAgent := AgentFromContext(ctx)

	if gotAgent == nil {
		t.Fatal("AgentFromContext() returned nil for context with agent")
	}
	if gotAgent.ID != agent.ID {
		t.Errorf("ID = %v, want %v", gotAgent.ID, agent.ID)
	}

	// Test with no agent in context
	emptyCtx := context.Background()
	emptyAgent := AgentFromContext(emptyCtx)
	if emptyAgent != nil {
		t.Error("AgentFromContext() should return nil for context without agent")
	}
}

func TestCombinedAuthMiddleware(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Create mock database with test agent
	db := NewMockAgentDB()
	testAPIKey := "solvr_testkey123456789012345678901234567890"
	_, err := db.AddTestAgent("test_agent", "Test Agent", testAPIKey)
	if err != nil {
		t.Fatalf("failed to add test agent: %v", err)
	}
	validator := NewAPIKeyValidator(db)

	// Generate valid JWT
	validJWT, _ := GenerateJWT(secret, "user-123", "test@example.com", "user", 15*time.Minute)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
		wantClaims     bool
		wantAgent      bool
	}{
		{
			name:           "valid JWT authentication",
			authHeader:     "Bearer " + validJWT,
			wantStatusCode: http.StatusOK,
			wantClaims:     true,
			wantAgent:      false,
		},
		{
			name:           "valid API key authentication",
			authHeader:     "Bearer " + testAPIKey,
			wantStatusCode: http.StatusOK,
			wantClaims:     false,
			wantAgent:      true,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
			wantAgent:      false,
		},
		{
			name:           "invalid token and invalid API key",
			authHeader:     "Bearer invalid_token",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
			wantAgent:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotClaims *Claims
			var gotAgent *models.Agent
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotClaims = ClaimsFromContext(r.Context())
				gotAgent = AgentFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := CombinedAuthMiddleware(secret, validator)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			if tt.wantClaims && gotClaims == nil {
				t.Error("expected claims in context but got nil")
			}
			if !tt.wantClaims && gotClaims != nil {
				t.Error("expected no claims but got some")
			}

			if tt.wantAgent && gotAgent == nil {
				t.Error("expected agent in context but got nil")
			}
			if !tt.wantAgent && gotAgent != nil {
				t.Error("expected no agent but got some")
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name           string
		claimsRole     string
		requiredRole   string
		wantStatusCode int
	}{
		{
			name:           "admin accessing admin route",
			claimsRole:     "admin",
			requiredRole:   "admin",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "user accessing user route",
			claimsRole:     "user",
			requiredRole:   "user",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "admin accessing user route",
			claimsRole:     "admin",
			requiredRole:   "user",
			wantStatusCode: http.StatusOK, // admin can access user routes
		},
		{
			name:           "user accessing admin route",
			claimsRole:     "user",
			requiredRole:   "admin",
			wantStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := RequireRole(tt.requiredRole)(nextHandler)

			// Create request with claims in context
			claims := &Claims{
				UserID: "user-123",
				Email:  "test@example.com",
				Role:   tt.claimsRole,
			}
			ctx := ContextWithClaims(context.Background(), claims)
			req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}
		})
	}

	// Test without claims
	t.Run("no claims returns 401", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RequireRole("admin")(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusUnauthorized)
		}
	})
}

// TestUnifiedAuthMiddleware tests the middleware that accepts all three auth types:
// 1. User API keys (solvr_sk_...)
// 2. Agent API keys (solvr_...)
// 3. JWT tokens
func TestUnifiedAuthMiddleware(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Create mock database for agents
	agentDB := NewMockAgentDB()
	testAgentAPIKey := "solvr_testkey123456789012345678901234567890"
	_, err := agentDB.AddTestAgent("test_agent", "Test Agent", testAgentAPIKey)
	if err != nil {
		t.Fatalf("failed to add test agent: %v", err)
	}
	agentValidator := NewAPIKeyValidator(agentDB)

	// Create mock database for user API keys
	userDB := NewMockUserAPIKeyDB()
	testUserID := "user-456"
	testUserKeyID := "key-789"
	testUserAPIKey := "solvr_sk_userkey123456789012345678901234567890"
	userDB.AddTestUser(testUserID, "testuser", "test@example.com")
	_, err = userDB.AddTestUserAPIKey(testUserKeyID, testUserID, "Test User Key", testUserAPIKey)
	if err != nil {
		t.Fatalf("failed to add test user API key: %v", err)
	}
	userValidator := NewUserAPIKeyValidator(userDB)

	// Generate valid JWT
	validJWT, _ := GenerateJWT(secret, "jwt-user-123", "jwt@example.com", "user", 15*time.Minute)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
		wantClaims     bool
		wantAgent      bool
		expectUserID   string // Expected user ID in claims
		expectAgentID  string // Expected agent ID
	}{
		{
			name:           "valid JWT authentication",
			authHeader:     "Bearer " + validJWT,
			wantStatusCode: http.StatusOK,
			wantClaims:     true,
			wantAgent:      false,
			expectUserID:   "jwt-user-123",
		},
		{
			name:           "valid agent API key authentication",
			authHeader:     "Bearer " + testAgentAPIKey,
			wantStatusCode: http.StatusOK,
			wantClaims:     false,
			wantAgent:      true,
			expectAgentID:  "test_agent",
		},
		{
			name:           "valid user API key authentication",
			authHeader:     "Bearer " + testUserAPIKey,
			wantStatusCode: http.StatusOK,
			wantClaims:     true,  // User API keys should populate claims
			wantAgent:      false,
			expectUserID:   testUserID,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
			wantAgent:      false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid_token",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
			wantAgent:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotClaims *Claims
			var gotAgent *models.Agent
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotClaims = ClaimsFromContext(r.Context())
				gotAgent = AgentFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := UnifiedAuthMiddleware(secret, agentValidator, userValidator)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			if tt.wantClaims && gotClaims == nil {
				t.Error("expected claims in context but got nil")
			}
			if !tt.wantClaims && gotClaims != nil {
				t.Error("expected no claims but got some")
			}

			if tt.wantAgent && gotAgent == nil {
				t.Error("expected agent in context but got nil")
			}
			if !tt.wantAgent && gotAgent != nil {
				t.Error("expected no agent but got some")
			}

			// Verify specific identities
			if tt.expectUserID != "" && gotClaims != nil {
				if gotClaims.UserID != tt.expectUserID {
					t.Errorf("claims.UserID = %v, want %v", gotClaims.UserID, tt.expectUserID)
				}
			}
			if tt.expectAgentID != "" && gotAgent != nil {
				if gotAgent.ID != tt.expectAgentID {
					t.Errorf("agent.ID = %v, want %v", gotAgent.ID, tt.expectAgentID)
				}
			}
		})
	}
}

// TestOptionalAuthMiddleware tests the middleware that tries all three auth types
// (user API key, agent API key, JWT) but NEVER returns 401.
// If auth succeeds → context populated. If fails → request continues without auth.
func TestOptionalAuthMiddleware(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Create mock database for agents
	agentDB := NewMockAgentDB()
	testAgentAPIKey := "solvr_testkey123456789012345678901234567890"
	_, err := agentDB.AddTestAgent("test_agent", "Test Agent", testAgentAPIKey)
	if err != nil {
		t.Fatalf("failed to add test agent: %v", err)
	}
	agentValidator := NewAPIKeyValidator(agentDB)

	// Create mock database for user API keys
	userDB := NewMockUserAPIKeyDB()
	testUserID := "user-456"
	testUserKeyID := "key-789"
	testUserAPIKey := "solvr_sk_userkey123456789012345678901234567890"
	userDB.AddTestUser(testUserID, "testuser", "test@example.com")
	_, err = userDB.AddTestUserAPIKey(testUserKeyID, testUserID, "Test User Key", testUserAPIKey)
	if err != nil {
		t.Fatalf("failed to add test user API key: %v", err)
	}
	userValidator := NewUserAPIKeyValidator(userDB)

	// Generate valid JWT
	validJWT, _ := GenerateJWT(secret, "jwt-user-123", "jwt@example.com", "user", 15*time.Minute)

	// Generate expired JWT
	expiredJWT, _ := GenerateJWT(secret, "user-123", "test@example.com", "user", -1*time.Minute)

	tests := []struct {
		name          string
		authHeader    string
		wantClaims    bool
		wantAgent     bool
		expectUserID  string
		expectAgentID string
	}{
		{
			name:         "valid JWT sets claims and continues",
			authHeader:   "Bearer " + validJWT,
			wantClaims:   true,
			wantAgent:    false,
			expectUserID: "jwt-user-123",
		},
		{
			name:          "valid agent API key sets agent and continues",
			authHeader:    "Bearer " + testAgentAPIKey,
			wantClaims:    false,
			wantAgent:     true,
			expectAgentID: "test_agent",
		},
		{
			name:         "valid user API key sets claims and continues",
			authHeader:   "Bearer " + testUserAPIKey,
			wantClaims:   true,
			wantAgent:    false,
			expectUserID: testUserID,
		},
		{
			name:       "no auth header continues without auth (no 401)",
			authHeader: "",
			wantClaims: false,
			wantAgent:  false,
		},
		{
			name:       "invalid token continues without auth (no 401)",
			authHeader: "Bearer invalid_token_here",
			wantClaims: false,
			wantAgent:  false,
		},
		{
			name:       "expired JWT continues without auth (no 401)",
			authHeader: "Bearer " + expiredJWT,
			wantClaims: false,
			wantAgent:  false,
		},
		{
			name:       "invalid API key format continues without auth (no 401)",
			authHeader: "Bearer solvr_invalidkey1234567890123456789012345",
			wantClaims: false,
			wantAgent:  false,
		},
		{
			name:       "missing Bearer prefix continues without auth (no 401)",
			authHeader: validJWT,
			wantClaims: false,
			wantAgent:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotClaims *Claims
			var gotAgent *models.Agent
			handlerCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				gotClaims = ClaimsFromContext(r.Context())
				gotAgent = AgentFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := OptionalAuthMiddleware(secret, agentValidator, userValidator)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			// CRITICAL: handler must ALWAYS be called (never 401)
			if !handlerCalled {
				t.Error("handler was not called — OptionalAuthMiddleware must never block requests")
			}

			// CRITICAL: status must ALWAYS be 200 (never 401)
			if rr.Code != http.StatusOK {
				t.Errorf("status code = %v, want %v — OptionalAuthMiddleware must never return 401", rr.Code, http.StatusOK)
			}

			if tt.wantClaims && gotClaims == nil {
				t.Error("expected claims in context but got nil")
			}
			if !tt.wantClaims && gotClaims != nil {
				t.Error("expected no claims but got some")
			}

			if tt.wantAgent && gotAgent == nil {
				t.Error("expected agent in context but got nil")
			}
			if !tt.wantAgent && gotAgent != nil {
				t.Error("expected no agent but got some")
			}

			// Verify specific identities
			if tt.expectUserID != "" && gotClaims != nil {
				if gotClaims.UserID != tt.expectUserID {
					t.Errorf("claims.UserID = %v, want %v", gotClaims.UserID, tt.expectUserID)
				}
			}
			if tt.expectAgentID != "" && gotAgent != nil {
				if gotAgent.ID != tt.expectAgentID {
					t.Errorf("agent.ID = %v, want %v", gotAgent.ID, tt.expectAgentID)
				}
			}
		})
	}
}
