package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeAccounts answers IsActiveUser from a fixed set of live user ids, or fails every
// lookup when err is set.
type fakeAccounts struct {
	live  map[string]bool
	err   error
	calls int
}

func (f *fakeAccounts) IsActiveUser(_ context.Context, userID string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.live[userID], nil
}

type accountCase struct {
	name     string
	accounts *fakeAccounts
	subject  string
}

func accountCases() []accountCase {
	return []accountCase{
		{"live account", &fakeAccounts{live: map[string]bool{"live-user": true}}, "live-user"},
		{"deleted or absent account", &fakeAccounts{live: map[string]bool{"live-user": true}}, "gone-user"},
		{"account lookup fails", &fakeAccounts{err: errors.New("connection refused")}, "live-user"},
	}
}

// serveWithJWT runs one request carrying a JWT for subject through mw and reports the
// status, the error code and the claims the handler saw (nil when it never ran).
func serveWithJWT(t *testing.T, mw func(http.Handler) http.Handler, secret, subject string) (int, string, *Claims, bool) {
	t.Helper()
	token, err := GenerateJWT(secret, subject, subject+"@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	var seen *Claims
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		seen = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	return rr.Code, body.Error.Code, seen, called
}

// Routes that require authentication refuse a JWT whose account is gone with 401, and
// answer 503 (not 401) when the account cannot be checked: a transient outage must not
// read as a bad credential.
func TestRequiredAuth_JWTMustNameALiveAccount(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"
	middlewares := map[string]func(AccountChecker) func(http.Handler) http.Handler{
		"JWTMiddleware": func(a AccountChecker) func(http.Handler) http.Handler { return JWTMiddleware(secret, a) },
		"UnifiedAuthMiddleware": func(a AccountChecker) func(http.Handler) http.Handler {
			return UnifiedAuthMiddleware(secret, NewAPIKeyValidator(NewMockAgentDB()), NewUserAPIKeyValidator(NewMockUserAPIKeyDB()), a)
		},
	}
	want := map[string]struct {
		status int
		code   string
	}{
		"live account":              {http.StatusOK, ""},
		"deleted or absent account": {http.StatusUnauthorized, ErrCodeUnauthorized},
		"account lookup fails":      {http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
	}
	for name, build := range middlewares {
		for _, tc := range accountCases() {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				status, code, claims, called := serveWithJWT(t, build(tc.accounts), secret, tc.subject)
				if tc.accounts.calls != 1 {
					t.Errorf("IsActiveUser calls = %d, want 1", tc.accounts.calls)
				}
				if status != want[tc.name].status || code != want[tc.name].code {
					t.Fatalf("got %d %q, want %d %q", status, code, want[tc.name].status, want[tc.name].code)
				}
				if tc.name == "live account" && (claims == nil || claims.UserID != tc.subject) {
					t.Errorf("handler claims = %+v, want subject %q", claims, tc.subject)
				}
				if tc.name != "live account" && called {
					t.Error("handler ran for a JWT whose account could not be confirmed")
				}
			})
		}
	}
}

// Routes where authentication is optional never answer 401 (TestOptionalAuthMiddleware),
// so a JWT whose account is gone, or cannot be checked, is served as an anonymous caller.
func TestOptionalAuth_JWTOfAGoneAccountIsAnonymous(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"
	for _, tc := range accountCases() {
		t.Run(tc.name, func(t *testing.T) {
			mw := OptionalAuthMiddleware(secret, NewAPIKeyValidator(NewMockAgentDB()), NewUserAPIKeyValidator(NewMockUserAPIKeyDB()), tc.accounts)
			status, _, claims, called := serveWithJWT(t, mw, secret, tc.subject)
			if !called || status != http.StatusOK {
				t.Fatalf("handler called = %v, status = %d; optional auth must always continue", called, status)
			}
			if tc.accounts.calls != 1 {
				t.Errorf("IsActiveUser calls = %d, want 1", tc.accounts.calls)
			}
			if tc.name == "live account" {
				if claims == nil || claims.UserID != tc.subject {
					t.Errorf("claims = %+v, want subject %q", claims, tc.subject)
				}
			} else if claims != nil {
				t.Errorf("claims = %+v, want an anonymous caller", claims)
			}
		})
	}
}

// With no account checker the middlewares trust a valid signature alone, as before.
func TestJWTMiddlewares_NilAccountCheckerTrustsTheSignature(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"
	agents := NewAPIKeyValidator(NewMockAgentDB())
	users := NewUserAPIKeyValidator(NewMockUserAPIKeyDB())
	for name, mw := range map[string]func(http.Handler) http.Handler{
		"JWTMiddleware":          JWTMiddleware(secret, nil),
		"UnifiedAuthMiddleware":  UnifiedAuthMiddleware(secret, agents, users, nil),
		"OptionalAuthMiddleware": OptionalAuthMiddleware(secret, agents, users, nil),
	} {
		t.Run(name, func(t *testing.T) {
			status, _, claims, _ := serveWithJWT(t, mw, secret, "any-user")
			if status != http.StatusOK || claims == nil || claims.UserID != "any-user" {
				t.Errorf("status = %d, claims = %+v; want 200 with subject any-user", status, claims)
			}
		})
	}
}
