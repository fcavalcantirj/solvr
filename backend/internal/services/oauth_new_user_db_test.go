package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// SPEC.md 5.2: POST /v1/auth/oauth/exchange answers is_new_user true ONLY for the sign-in that
// created the account. Run with the real user service, the real login code repository and
// Postgres; only GitHub and Google are faked. One person signs in four times:
//
//  1. GitHub, never seen before         → the account is created      → is_new_user true
//  2. GitHub again                      → the same account            → false
//  3. Google, same e-mail address       → linked to that account      → false
//  4. Google again                      → the same account            → false
//
// and a second person arrives through Google first → true.
func TestOAuthSignIn_ExchangeSaysNewOnlyForTheSignInThatCreatedTheAccount(t *testing.T) {
	pool := oauthTestPool(t)
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	githubID := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "newflag_" + suffix + "@test.solvr.dev"
	otherEmail := "newflag_" + suffix + "_b@test.solvr.dev"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email IN ($1, $2)`, email, otherEmail)
	})

	svc := NewOAuthUserService(db.NewUserRepository(pool), db.NewAuthMethodRepository(pool))
	cfg := &handlers.OAuthConfig{
		FrontendURL:       "http://localhost:3000",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:         "test-jwt-secret-32-chars-long!!",
		JWTExpiry:         "15m",
	}

	type answer struct {
		userID   string
		isNew    bool
		provider string
	}
	// signIn runs one whole sign-in: the provider callback, then the exchange of its code.
	signIn := func(provider, providerID, withEmail string) answer {
		t.Helper()
		fake := fakeOAuthProvider(t, providerID, withEmail)
		h := handlers.NewOAuthHandlersWithAllDeps(cfg, nil, nil, NewOAuthUserServiceAdapter(svc), fake.URL, fake.URL).
			WithLoginCodes(db.NewOAuthLoginCodeRepository(pool))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/auth/%s/callback?code=valid&state=s", provider), nil)
		if provider == models.AuthProviderGitHub {
			h.GitHubCallback(rec, req)
		} else {
			h.GoogleCallback(rec, req)
		}
		if rec.Code != http.StatusFound {
			t.Fatalf("%s callback: status = %d, want 302; body %s", provider, rec.Code, rec.Body.String())
		}
		location, err := url.Parse(rec.Header().Get("Location"))
		if err != nil || location.Query().Get("code") == "" {
			t.Fatalf("%s callback: Location %q carries no login code (err %v)", provider, rec.Header().Get("Location"), err)
		}
		// The redirect itself says nothing about the account: only the exchange does.
		for _, leaked := range []string{"is_new_user", "new", "provider"} {
			if location.Query().Has(leaked) {
				t.Errorf("the redirect URL carries %q: %s", leaked, location)
			}
		}

		body, _ := json.Marshal(map[string]string{"login_code": location.Query().Get("code")})
		exchange := httptest.NewRecorder()
		exchangeReq := httptest.NewRequest(http.MethodPost, "/v1/auth/oauth/exchange", strings.NewReader(string(body)))
		exchangeReq.Header.Set("Content-Type", "application/json")
		h.ExchangeLoginCode(exchange, exchangeReq)
		if exchange.Code != http.StatusOK {
			t.Fatalf("%s exchange: status = %d, want 200; body %s", provider, exchange.Code, exchange.Body.String())
		}
		var reply struct {
			Data struct {
				IsNewUser *bool  `json:"is_new_user"`
				Provider  string `json:"provider"`
			} `json:"data"`
		}
		if err := json.Unmarshal(exchange.Body.Bytes(), &reply); err != nil || reply.Data.IsNewUser == nil {
			t.Fatalf("%s exchange: no is_new_user in %s (err %v)", provider, exchange.Body.String(), err)
		}
		var userID string
		if err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1 AND deleted_at IS NULL`, withEmail).Scan(&userID); err != nil {
			t.Fatalf("%s: the account for %s: %v", provider, withEmail, err)
		}
		return answer{userID: userID, isNew: *reply.Data.IsNewUser, provider: reply.Data.Provider}
	}

	first := signIn(models.AuthProviderGitHub, githubID, email)
	if !first.isNew || first.provider != "github" {
		t.Fatalf("1. the sign-in that created the account answered %+v, want is_new_user true through github", first)
	}
	second := signIn(models.AuthProviderGitHub, githubID, email)
	if second.isNew || second.provider != "github" || second.userID != first.userID {
		t.Fatalf("2. a returning GitHub sign-in answered %+v, want the same account, is_new_user false", second)
	}
	linked := signIn(models.AuthProviderGoogle, "g-"+suffix, email)
	if linked.isNew || linked.provider != "google" || linked.userID != first.userID {
		t.Fatalf("3. Google linked to the existing e-mail answered %+v, want the same account, is_new_user false through google", linked)
	}
	again := signIn(models.AuthProviderGoogle, "g-"+suffix, email)
	if again.isNew || again.userID != first.userID {
		t.Fatalf("4. a returning Google sign-in answered %+v, want is_new_user false", again)
	}

	other := signIn(models.AuthProviderGoogle, "g-"+suffix+"-b", otherEmail)
	if !other.isNew || other.provider != "google" || other.userID == first.userID {
		t.Fatalf("a second person signing up through Google answered %+v, want a new account, is_new_user true", other)
	}

	var accounts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email IN ($1, $2)`, email, otherEmail).Scan(&accounts); err != nil {
		t.Fatalf("count: %v", err)
	}
	if accounts != 2 {
		t.Fatalf("%d accounts exist for the two people, want 2", accounts)
	}
}
