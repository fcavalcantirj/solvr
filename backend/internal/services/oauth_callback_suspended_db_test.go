package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Anti-abuse W0/W4: the browser OAuth callbacks end a banned or tombstoned sign-in on the
// frontend callback page with ?error=account_suspended, create nothing and issue no login
// code. The real OAuthUserService runs on the database; only GitHub and Google are faked,
// because the router hardcodes their base URLs.

type countingLoginCodes struct{ issued atomic.Int64 }

func (c *countingLoginCodes) Issue(context.Context, string, time.Duration) (string, error) {
	c.issued.Add(1)
	return "solvr_lc_test", nil
}

func (c *countingLoginCodes) Redeem(context.Context, string) (*db.OAuthLoginUser, error) {
	return nil, db.ErrLoginCodeInvalid
}

// fakeOAuthProvider answers the token and profile calls of GitHub and Google with one
// identity.
func fakeOAuthProvider(t *testing.T, providerID, email string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_test", "token_type": "bearer"}) //nolint:errcheck
		case "/user":
			id, _ := strconv.ParseInt(providerID, 10, 64)
			json.NewEncoder(w).Encode(map[string]any{"id": id, "login": "returning", "email": email, "name": "Returning"}) //nolint:errcheck
		case "/user/emails":
			json.NewEncoder(w).Encode([]map[string]any{{"email": email, "primary": true, "verified": true}}) //nolint:errcheck
		case "/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "ya29.test", "token_type": "Bearer", "expires_in": 3600}) //nolint:errcheck
		case "/userinfo":
			json.NewEncoder(w).Encode(map[string]any{"sub": providerID, "email": email, "email_verified": true, "name": "Returning"}) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOAuthCallbacks_TombstonedIdentityRedirectsAccountSuspended(t *testing.T) {
	pool := oauthTestPool(t)
	cases := []struct {
		name     string
		provider string
		newEmail bool // sign in with the tombstoned OAuth id but an email no account holds
	}{
		{"github, same email", models.AuthProviderGitHub, false},
		{"github, new email", models.AuthProviderGitHub, true},
		{"google, same email", models.AuthProviderGoogle, false},
		{"google, new email", models.AuthProviderGoogle, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
			providerID := strconv.FormatInt(time.Now().UnixNano(), 10)
			_, email := seedTombstonedUser(t, pool, tc.provider, providerID, suffix)
			signInEmail := email
			if tc.newEmail {
				signInEmail = "tomb_" + suffix + "_new@test.solvr.dev"
			}
			provider := fakeOAuthProvider(t, providerID, signInEmail)

			svc := NewOAuthUserService(db.NewUserRepository(pool), db.NewAuthMethodRepository(pool))
			svc.WithIdentityGate(db.NewBannedIdentityRepository(pool))
			codes := &countingLoginCodes{}
			cfg := &handlers.OAuthConfig{FrontendURL: "http://localhost:3000", GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback"}
			h := handlers.NewOAuthHandlersWithAllDeps(cfg, nil, nil, NewOAuthUserServiceAdapter(svc), provider.URL, provider.URL).WithLoginCodes(codes)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/auth/%s/callback?code=valid&state=s", tc.provider), nil)
			if tc.provider == models.AuthProviderGitHub {
				h.GitHubCallback(rec, req)
			} else {
				h.GoogleCallback(rec, req)
			}

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
			}
			if want := "http://localhost:3000/auth/callback?error=account_suspended"; rec.Header().Get("Location") != want {
				t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
			}
			if n := codes.issued.Load(); n != 0 {
				t.Errorf("%d login code(s) issued; want none", n)
			}
			var live int
			if err := pool.QueryRow(context.Background(),
				"SELECT count(*) FROM users WHERE email = $1 AND deleted_at IS NULL", signInEmail).Scan(&live); err != nil {
				t.Fatalf("count users: %v", err)
			}
			if live != 0 {
				t.Errorf("%d live users row(s) for %s; want none", live, signInEmail)
			}
		})
	}
}
