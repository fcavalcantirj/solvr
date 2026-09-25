package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// An optional-auth route is anonymous only when the caller omits credentials.
// Once a caller presents credentials, accepting an invalid value as anonymous
// hides expiry/revocation and violates the public API's 401 contract.
func TestOptionalAuthMiddleware_RejectsPresentedInvalidCredentials(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"
	agentDB := NewMockAgentDB()
	userDB := NewMockUserAPIKeyDB()
	middleware := OptionalAuthMiddleware(
		secret,
		NewAPIKeyValidator(agentDB),
		NewUserAPIKeyValidator(userDB),
		nil,
	)
	expiredJWT, err := GenerateJWT(secret, "expired-user", "expired@example.com", "user", -time.Minute)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantCode   string
		wantCalled bool
	}{
		{"omitted credentials stay anonymous", "", http.StatusOK, "", true},
		{"non-Bearer credentials", "Basic abc123", http.StatusUnauthorized, ErrCodeUnauthorized, false},
		{"malformed bearer token", "Bearer not-a-jwt", http.StatusUnauthorized, ErrCodeInvalidToken, false},
		{"expired JWT", "Bearer " + expiredJWT, http.StatusUnauthorized, ErrCodeTokenExpired, false},
		{"unknown agent API key", "Bearer solvr_unknown123456789012345678901234567890", http.StatusUnauthorized, ErrCodeInvalidAPIKey, false},
		{"unknown user API key", "Bearer solvr_sk_unknown123456789012345678901234567890", http.StatusUnauthorized, ErrCodeInvalidAPIKey, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/public", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rr := httptest.NewRecorder()

			middleware(next).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if called != tt.wantCalled {
				t.Fatalf("next handler called = %v, want %v", called, tt.wantCalled)
			}
			if tt.wantCode == "" {
				return
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error envelope: %v; body = %s", err, rr.Body.String())
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
