package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// spec.json idx 97: a referral stored before an OAuth redirect is credited after it. Run against
// the real router and Postgres, the human signs in the way frontend/app/auth/callback/page.tsx
// does (a login code exchanged for an access token) and then sends the page's claim request
// unchanged. Without a JWT the same request is 401 and records nothing.

// claimReferralFromCallback sends POST /v1/auth/claim-referral exactly as
// frontend/app/auth/callback/page.tsx does: method POST, headers Content-Type: application/json
// and Authorization: Bearer <access_token from the exchange>, body JSON.stringify({ ref: refCode }).
// The browser adds Origin. An empty token leaves Authorization out.
func claimReferralFromCallback(t *testing.T, base, origin, token, refCode string) (int, string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"ref": refCode})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/claim-referral", strings.NewReader(string(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Origin", origin)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestRouterClaimReferral_OAuthSignupRecordsTheReferral(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	t.Setenv("ALLOWED_ORIGINS", "") // the default origins, which include the production site
	const origin = "https://solvr.dev"
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ts := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(ts.Close)

	referrerID, _ := createLiveTestUser(t, pool, "user")
	refCode, err := db.NewReferralRepository(pool).GetReferralCode(ctx, referrerID)
	require.NoError(t, err)
	require.NotEmpty(t, refCode)
	// The account an OAuth signup leaves (a GitHub identity, no password).
	newUserID, _ := createLiveTestUser(t, pool, "user")
	// Registered after both accounts, so it runs before their removal: referrals has no cascade.
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM referrals WHERE referred_id = $1 OR referrer_id = $2", newUserID, referrerID) //nolint:errcheck
	})
	referrals := func() (count int, referrer string) {
		t.Helper()
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT count(*), coalesce(max(referrer_id::text), '') FROM referrals WHERE referred_id = $1", newUserID).
			Scan(&count, &referrer))
		return count, referrer
	}

	// The callback's sign-in: the redirect's login code is exchanged for the access token.
	loginCode, err := db.NewOAuthLoginCodeRepository(pool).Issue(ctx, newUserID, auth.LoginCodeTTL)
	require.NoError(t, err)
	status, exchange := postExchange(t, ts.URL, loginCode)
	require.Equal(t, http.StatusOK, status)
	token := exchange.Data.AccessToken
	require.NotEmpty(t, token)

	// Without a JWT the claim is refused and nothing is recorded.
	status, body := claimReferralFromCallback(t, ts.URL, origin, "", refCode)
	require.Equal(t, http.StatusUnauthorized, status, body)
	var refusal struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &refusal), body)
	require.Equal(t, auth.ErrCodeUnauthorized, refusal.Error.Code, body)
	count, _ := referrals()
	require.Zero(t, count, "an unauthenticated claim records no referral")

	// The browser's preflight for the cross-origin call with an Authorization header.
	preflight, err := http.NewRequest(http.MethodOptions, ts.URL+"/v1/auth/claim-referral", nil)
	require.NoError(t, err)
	preflight.Header.Set("Origin", origin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	presp, err := http.DefaultClient.Do(preflight)
	require.NoError(t, err)
	presp.Body.Close()
	require.True(t, presp.StatusCode >= 200 && presp.StatusCode < 300, "preflight status %d", presp.StatusCode)
	require.Equal(t, origin, presp.Header.Get("Access-Control-Allow-Origin"))

	// The page's claim with the exchanged token credits the referrer.
	status, body = claimReferralFromCallback(t, ts.URL, origin, token, refCode)
	require.Equal(t, http.StatusOK, status, body)
	var claimed struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &claimed), body)
	require.Equal(t, "claimed", claimed.Status, body)
	count, referrer := referrals()
	require.Equal(t, 1, count, "one referral row for the new account")
	require.Equal(t, referrerID, referrer)

	// A repeated claim (a second tab, a retry) still answers 200 and records nothing more.
	status, body = claimReferralFromCallback(t, ts.URL, origin, token, refCode)
	require.Equal(t, http.StatusOK, status, body)
	count, _ = referrals()
	require.Equal(t, 1, count, "a repeated claim adds no row")
}
