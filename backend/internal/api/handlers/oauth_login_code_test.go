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
	origins    []db.LoginCodeOrigin
	issueTTL   time.Duration
	issueErr   error
	redeemUser *db.OAuthLoginUser
	redeemErr  error
	redeemed   []string
}

func (f *fakeLoginCodes) Issue(_ context.Context, userID string, ttl time.Duration, origin db.LoginCodeOrigin) (string, error) {
	if f.issueErr != nil {
		return "", f.issueErr
	}
	f.issuedFor = append(f.issuedFor, userID)
	f.origins = append(f.origins, origin)
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

// SPEC.md 5.2: the code carries what the callback knew, so the exchange can tell a sign-up from
// a login. The account is new only when THIS sign-in created it.
func TestOAuthCallbacks_IssueTheCodeWithTheProviderAndWhetherTheSignInCreatedTheAccount(t *testing.T) {
	gh, gg := mockGitHub(t), mockGoogle(t)
	known := map[string]OAuthUserServiceInterface{
		"github": &MockOAuthUserService{users: map[string]*MockUserData{"github:12345": {ID: "user-gh", Email: "someone@example.com", Username: "someone"}}},
		"google": &MockGoogleOAuthUserService{users: map[string]*MockGoogleUserData{"google:g-sub": {ID: "user-google", Email: "someone@gmail.com", Username: "someone"}}},
	}
	unknown := map[string]OAuthUserServiceInterface{
		"github": &MockOAuthUserService{users: map[string]*MockUserData{}},
		"google": &MockGoogleOAuthUserService{users: map[string]*MockGoogleUserData{}},
	}
	callback := func(provider string, users OAuthUserServiceInterface, codes OAuthLoginCodeStore) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/"+provider+"/callback?code=valid&state=s", nil)
		if provider == "github" {
			NewOAuthHandlersWithDeps(loginCodeConfig(), nil, nil, users, gh.URL).WithLoginCodes(codes).GitHubCallback(rec, req)
		} else {
			NewOAuthHandlersWithAllDeps(loginCodeConfig(), nil, nil, users, "", gg.URL).WithLoginCodes(codes).GoogleCallback(rec, req)
		}
		return rec
	}
	for _, provider := range []string{"github", "google"} {
		for name, tc := range map[string]struct {
			users   OAuthUserServiceInterface
			wantNew bool
		}{
			"an account that already existed": {known[provider], false},
			"an account this sign-in created": {unknown[provider], true},
		} {
			t.Run(provider+"/"+name, func(t *testing.T) {
				codes := &fakeLoginCodes{}
				if rec := callback(provider, tc.users, codes); rec.Code != http.StatusFound {
					t.Fatalf("status = %d, want 302; body %s", rec.Code, rec.Body.String())
				}
				if len(codes.origins) != 1 {
					t.Fatalf("%d codes issued, want 1", len(codes.origins))
				}
				if got, want := codes.origins[0], (db.LoginCodeOrigin{Provider: provider, IsNewUser: tc.wantNew}); got != want {
					t.Errorf("origin = %+v, want %+v", got, want)
				}
			})
		}
	}
}

// Without a user service the callback creates nothing, so it never claims a new account.
func TestOAuthCallbacks_WithoutAUserServiceNeverSayTheAccountIsNew(t *testing.T) {
	codes := &fakeLoginCodes{}
	gh := mockGitHub(t)
	rec := httptest.NewRecorder()
	NewOAuthHandlersWithDeps(loginCodeConfig(), nil, nil, nil, gh.URL).WithLoginCodes(codes).
		GitHubCallback(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback?code=valid&state=s", nil))
	if rec.Code != http.StatusFound || len(codes.origins) != 1 {
		t.Fatalf("status %d, %d codes issued; want 302 and 1", rec.Code, len(codes.origins))
	}
	if got, want := codes.origins[0], (db.LoginCodeOrigin{Provider: "github"}); got != want {
		t.Errorf("origin = %+v, want %+v", got, want)
	}
}

func TestOAuthCallbacks_FailClosedWithoutACodeStore(t *testing.T) {
	failing := &fakeLoginCodes{issueErr: context.DeadlineExceeded}
	// Without a login code the callback ends on the error page, never falling back to a token in
	// the URL (every callback error redirects; Felipe, 2026-09-30).
	for name, tc := range map[string]struct {
		codes OAuthLoginCodeStore
		want  string
	}{"no store": {nil, OAuthErrorLoginUnavailable}, "store fails": {failing, OAuthErrorLoginFailed}} {
		for provider, callback := range loginCodeCallbacks(t, tc.codes) {
			t.Run(name+"/"+provider, func(t *testing.T) {
				requireCallbackErrorRedirect(t, callback(), tc.want)
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

// SPEC.md 5.2: the answer says whether the sign-in created the account, and through which
// provider. Both are always present, and neither changes the token.
func TestExchangeLoginCode_SaysWhetherTheSignInCreatedTheAccountAndThroughWhichProvider(t *testing.T) {
	for name, tc := range map[string]struct {
		redeemed     db.OAuthLoginUser
		wantNew      bool
		wantProvider string
	}{
		"a sign-up through GitHub":      {db.OAuthLoginUser{ID: "u1", Email: "u1@example.com", Role: "user", IsNewUser: true, Provider: "github"}, true, "github"},
		"a login through Google":        {db.OAuthLoginUser{ID: "u2", Email: "u2@example.com", Role: "user", Provider: "google"}, false, "google"},
		"a code that carries no origin": {db.OAuthLoginUser{ID: "u3", Email: "u3@example.com", Role: "user"}, false, ""},
	} {
		t.Run(name, func(t *testing.T) {
			h := NewOAuthHandlers(loginCodeConfig(), nil, nil).WithLoginCodes(&fakeLoginCodes{redeemUser: &tc.redeemed})
			rec := exchange(h, `{"login_code":"solvr_lc_abc"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
			}
			var resp struct {
				Data map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			// is_new_user is a JSON boolean, never absent and never a string.
			if got, want := string(resp.Data["is_new_user"]), map[bool]string{true: "true", false: "false"}[tc.wantNew]; got != want {
				t.Errorf("is_new_user = %s, want %s; body %s", got, want, rec.Body.String())
			}
			if got, want := string(resp.Data["provider"]), `"`+tc.wantProvider+`"`; got != want {
				t.Errorf("provider = %s, want %s; body %s", got, want, rec.Body.String())
			}
			// The five documented fields, and nothing about the person.
			if len(resp.Data) != 5 {
				t.Errorf("answer has %d fields, want access_token, token_type, expires_in, is_new_user, provider: %s", len(resp.Data), rec.Body.String())
			}
			for _, private := range []string{tc.redeemed.Email, `"email"`, `"user_id"`, `"id"`} {
				if strings.Contains(rec.Body.String(), private) {
					t.Errorf("answer names the person outside the token (%s): %s", private, rec.Body.String())
				}
			}
		})
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
