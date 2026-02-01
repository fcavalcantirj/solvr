package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
