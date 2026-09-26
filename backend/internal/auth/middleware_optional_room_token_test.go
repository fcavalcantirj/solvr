package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A per-agent room token (solvr_rt_) is no account credential. Optional auth must let it
// through untouched and anonymous: the room guard resolves it on the routes that accept it,
// and every other optional route (search, posts) has always answered it as anonymous.
// Only account credentials that fail to validate are rejected.
func TestOptionalAuthMiddleware_DoesNotJudgeRoomTokens(t *testing.T) {
	middleware := OptionalAuthMiddleware(
		"test-secret-key-for-testing-purposes-only",
		NewAPIKeyValidator(NewMockAgentDB()),
		NewUserAPIKeyValidator(NewMockUserAPIKeyDB()),
		nil,
	)

	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantCalled bool
	}{
		{"a room token passes", "Bearer solvr_rt_abcdefghijklmnopqrstuvwxyz0123456789", http.StatusOK, true},
		{"a bare room-token prefix is an unknown agent key", "Bearer solvr_rt_", http.StatusUnauthorized, false},
		{"an unknown agent API key is still rejected", "Bearer solvr_unknown123456789012345678901234567890", http.StatusUnauthorized, false},
		{"an unknown user API key is still rejected", "Bearer solvr_sk_unknown123456789012345678901234567890", http.StatusUnauthorized, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			var sawAgent, sawClaims bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				sawAgent = AgentFromContext(r.Context()) != nil
				sawClaims = ClaimsFromContext(r.Context()) != nil
				w.WriteHeader(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/v1/search", nil)
			req.Header.Set("Authorization", tt.header)
			rr := httptest.NewRecorder()

			middleware(next).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if called != tt.wantCalled {
				t.Fatalf("next handler called = %v, want %v", called, tt.wantCalled)
			}
			if sawAgent || sawClaims {
				t.Errorf("a room token must not create an account identity (agent=%v claims=%v)", sawAgent, sawClaims)
			}
		})
	}
}
