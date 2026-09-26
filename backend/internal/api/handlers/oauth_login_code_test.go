package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 75 step 5: the OAuth callback redirects with a one-time login code, never a JWT, and
// POST /v1/auth/oauth/exchange is the only place the code turns into a token.

const loginCodeTestSecret = "test-jwt-secret-32-chars-long!!"

// fakeLoginCodes is a test double for the interface the handlers depend on. The real repository
// (single-use, expiry, racing redeems, hash-only storage) is exercised against Postgres in
// internal/db/oauth_login_codes_test.go and through the real router in internal/api.
type fakeLoginCodes struct {
	issuedFor  []string
	issueTTL   time.Duration
	issueErr   error
	redeemUser *db.OAuthLoginUser
	redeemErr  error
	redeemed   []string
}

func (f *fakeLoginCodes) Issue(_ context.Context, userID string, ttl time.Duration) (string, error) {
	if f.issueErr != nil {
		return "", f.issueErr
	}
	f.issuedFor = append(f.issuedFor, userID)
	f.issueTTL = ttl
	return "solvr_lc_fakecode", nil
}

func (f *fakeLoginCodes) Redeem(_ context.Context, code string) (*db.OAuthLoginUser, error) {
	f.redeemed = append(f.redeemed, code)
	if f.redeemErr != nil {
		return nil, f.redeemErr
	}
	return f.redeemUser, nil
}

func loginCodeConfig() *OAuthConfig {
	return &OAuthConfig{
		GitHubClientID:     "test-client-id",
		GitHubClientSecret: "test-client-secret",
		GitHubRedirectURI:  "http://localhost:8080/v1/auth/github/callback",
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURI:  "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:          loginCodeTestSecret,
		JWTExpiry:          "15m",
		RefreshExpiry:      "7d",
	}
}

func mockGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_test_token", "token_type": "bearer"})
		case "/user":
			json.NewEncoder(w).Encode(map[string]any{"id": 12345, "login": "someone", "email": "someone@example.com", "name": "Some One"})
		case "/user/emails":
			json.NewEncoder(w).Encode([]map[string]any{{"email": "someone@example.com", "primary": true, "verified": true}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mockGoogle(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "ya29.test", "token_type": "Bearer"})
		case "/userinfo":
			json.NewEncoder(w).Encode(map[string]any{"sub": "g-sub", "email": "someone@gmail.com", "email_verified": true, "name": "Some One"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func loginCodeCallbacks(t *testing.T, codes OAuthLoginCodeStore) map[string]func() *httptest.ResponseRecorder {
	t.Helper()
	gh, gg := mockGitHub(t), mockGoogle(t)
	users := &MockOAuthUserService{users: map[string]*MockUserData{
		"github:12345": {ID: "user-gh", Email: "someone@example.com", Username: "someone"},
	}}
	googleUsers := &MockGoogleOAuthUserService{users: map[string]*MockGoogleUserData{
		"google:g-sub": {ID: "user-google", Email: "someone@gmail.com", Username: "someone"},
	}}
	ghHandler := NewOAuthHandlersWithDeps(loginCodeConfig(), nil, nil, users, gh.URL).WithLoginCodes(codes)
	googleHandler := NewOAuthHandlersWithAllDeps(loginCodeConfig(), nil, nil, googleUsers, "", gg.URL).WithLoginCodes(codes)
	return map[string]func() *httptest.ResponseRecorder{
		"github": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			ghHandler.GitHubCallback(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback?code=valid&state=s", nil))
			return rec
		},
		"google": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			googleHandler.GoogleCallback(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid&state=s", nil))
			return rec
		},
	}
}

func TestOAuthCallbacks_RedirectWithAOneTimeCodeAndNeverAJWT(t *testing.T) {
	for provider, callback := range loginCodeCallbacks(t, &fakeLoginCodes{}) {
		t.Run(provider, func(t *testing.T) {
			rec := callback()
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302; body %s", rec.Code, rec.Body.String())
			}
			location := rec.Header().Get("Location")
			u, err := url.Parse(location)
			if err != nil {
				t.Fatalf("Location %q: %v", location, err)
			}
			if !strings.HasPrefix(location, "http://localhost:3000/auth/callback?") {
				t.Fatalf("redirect must land on the frontend callback page, got %s", location)
			}
			if got := u.Query().Get("code"); got != "solvr_lc_fakecode" {
				t.Errorf("code parameter = %q, want the issued login code", got)
			}
			for _, banned := range []string{"token", "access_token", "refresh_token"} {
				if u.Query().Has(banned) {
					t.Errorf("redirect URL carries %q: a credential in a URL reaches history, logs and analytics", banned)
				}
			}
			if strings.Contains(location, "eyJ") {
				t.Errorf("redirect URL contains a JWT: %s", location)
			}
			if u.Fragment != "" {
				t.Errorf("redirect must carry nothing in the fragment either, got %q", u.Fragment)
			}
		})
	}
}

func TestOAuthCallbacks_IssueTheCodeForTheAuthenticatedUserWithAShortLife(t *testing.T) {
	codes := &fakeLoginCodes{}
	cb := loginCodeCallbacks(t, codes)
	cb["github"]()
	cb["google"]()
	if len(codes.issuedFor) != 2 || codes.issuedFor[0] != "user-gh" || codes.issuedFor[1] != "user-google" {
		t.Fatalf("codes issued for %v, want [user-gh user-google]", codes.issuedFor)
	}
	if codes.issueTTL != auth.LoginCodeTTL || auth.LoginCodeTTL > time.Minute {
		t.Fatalf("ttl = %v (LoginCodeTTL %v), want the short constant, at most a minute", codes.issueTTL, auth.LoginCodeTTL)
	}
}

func TestOAuthCallbacks_FailClosedWithoutACodeStore(t *testing.T) {
	failing := &fakeLoginCodes{issueErr: context.DeadlineExceeded}
	for name, codes := range map[string]OAuthLoginCodeStore{"no store": nil, "store fails": failing} {
		for provider, callback := range loginCodeCallbacks(t, codes) {
			t.Run(name+"/"+provider, func(t *testing.T) {
				rec := callback()
				if rec.Code != http.StatusInternalServerError {
					t.Fatalf("status = %d, want 500 (never fall back to a token in the URL)", rec.Code)
				}
				if loc := rec.Header().Get("Location"); loc != "" {
					t.Fatalf("no redirect may be issued without a code, got Location %q", loc)
				}
			})
		}
	}
}

func exchange(h *OAuthHandlers, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/oauth/exchange", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ExchangeLoginCode(rec, req)
	return rec
}

func TestExchangeLoginCode_MintsAJWTForTheRedeemedAccount(t *testing.T) {
	codes := &fakeLoginCodes{redeemUser: &db.OAuthLoginUser{ID: "user-1", Email: "u1@example.com", Role: "admin"}}
	h := NewOAuthHandlers(loginCodeConfig(), nil, nil).WithLoginCodes(codes)

	rec := exchange(h, `{"login_code":"solvr_lc_abc"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q: a response carrying a token must not be cached", cc)
	}
	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.TokenType != "Bearer" || resp.Data.ExpiresIn != 900 {
		t.Errorf("token_type %q expires_in %d, want Bearer and 900", resp.Data.TokenType, resp.Data.ExpiresIn)
	}
	claims, err := auth.ValidateJWT(loginCodeTestSecret, resp.Data.AccessToken)
	if err != nil {
		t.Fatalf("the access token must be a valid JWT: %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "u1@example.com" || claims.Role != "admin" {
		t.Errorf("claims = %+v, want the redeemed account", claims)
	}
	if len(codes.redeemed) != 1 || codes.redeemed[0] != "solvr_lc_abc" {
		t.Errorf("redeemed %v, want the posted code exactly once", codes.redeemed)
	}
}

func TestExchangeLoginCode_RefusesWhatIsNotAValidCode(t *testing.T) {
	cases := []struct {
		name     string
		store    *fakeLoginCodes
		body     string
		wantHTTP int
		wantCode string
	}{
		{"unknown, used, expired or deleted", &fakeLoginCodes{redeemErr: db.ErrLoginCodeInvalid}, `{"login_code":"solvr_lc_x"}`, http.StatusUnauthorized, "INVALID_LOGIN_CODE"},
		{"missing", &fakeLoginCodes{}, `{}`, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"empty", &fakeLoginCodes{}, `{"login_code":""}`, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"malformed JSON", &fakeLoginCodes{}, `{"login_code":`, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"not an object", &fakeLoginCodes{}, `"solvr_lc_x"`, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"a JWT is not a code", &fakeLoginCodes{redeemErr: db.ErrLoginCodeInvalid}, `{"login_code":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.c2ln"}`, http.StatusUnauthorized, "INVALID_LOGIN_CODE"},
		{"the store is down", &fakeLoginCodes{redeemErr: context.DeadlineExceeded}, `{"login_code":"solvr_lc_x"}`, http.StatusInternalServerError, "INTERNAL_ERROR"},
		{"oversized body", &fakeLoginCodes{}, `{"login_code":"` + strings.Repeat("x", 100000) + `"}`, http.StatusBadRequest, "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewOAuthHandlers(loginCodeConfig(), nil, nil).WithLoginCodes(tc.store)
			rec := exchange(h, tc.body)
			if rec.Code != tc.wantHTTP {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.wantHTTP, rec.Body.String())
			}
			var resp ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Error.Code != tc.wantCode {
				t.Fatalf("error code = %q (err %v), want %s; body %s", resp.Error.Code, err, tc.wantCode, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "eyJ") || strings.Contains(rec.Body.String(), "access_token") {
				t.Errorf("a refusal must not carry a token: %s", rec.Body.String())
			}
		})
	}
}

func TestExchangeLoginCode_WithoutAStoreIsAnInternalError(t *testing.T) {
	rec := exchange(NewOAuthHandlers(loginCodeConfig(), nil, nil), `{"login_code":"solvr_lc_x"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
