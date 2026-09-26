package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 75 step 5: the browser OAuth flow no longer carries a JWT in a URL. Run against the real
// router and Postgres: a login code minted for a live account redeems once into a JWT that the
// rest of the API accepts, and a code (used, unknown, a JWT, or one of a deleted account) never
// does. The GitHub/Google callbacks that issue the code are covered against mock providers in
// handlers/oauth_login_code_test.go; this test stands where they leave off.

type exchangeReply struct {
	Data struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	} `json:"data"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func postExchange(t *testing.T, base, code string) (int, exchangeReply) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"login_code": code})
	require.NoError(t, err)
	status, raw := rawRequest(t, "POST", base+"/v1/auth/oauth/exchange", "", string(body))
	var reply exchangeReply
	_ = json.Unmarshal([]byte(raw), &reply)
	return status, reply
}

func TestOAuthLoginCode_ExchangeOnTheRealRouterMintsOneUsableToken(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ts := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(ts.Close)
	userID, _ := createLiveTestUser(t, pool, "user")
	codes := db.NewOAuthLoginCodeRepository(pool)

	code, err := codes.Issue(ctx, userID, auth.LoginCodeTTL)
	require.NoError(t, err)

	status, reply := postExchange(t, ts.URL, code)
	require.Equal(t, 200, status)
	require.Equal(t, "Bearer", reply.Data.TokenType)
	require.NotEmpty(t, reply.Data.AccessToken)

	// The token is a real session: the rest of the API accepts it for that account.
	meStatus, me := rawRequest(t, "GET", ts.URL+"/v1/me", reply.Data.AccessToken, "")
	require.Equal(t, 200, meStatus, me)
	require.Contains(t, me, userID)

	// One code, one token: a copy of the redirect URL that leaks afterwards opens nothing.
	status, reply = postExchange(t, ts.URL, code)
	require.Equal(t, 401, status)
	require.Equal(t, "INVALID_LOGIN_CODE", reply.Error.Code)

	// Neither a token nor a made-up code is accepted as a code.
	status, reply = postExchange(t, ts.URL, "solvr_lc_"+strings.Repeat("Z", 43))
	require.Equal(t, 401, status)
	require.Equal(t, "INVALID_LOGIN_CODE", reply.Error.Code)
	status, reply = postExchange(t, ts.URL, "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.c2ln")
	require.Equal(t, 401, status)
	require.Equal(t, "INVALID_LOGIN_CODE", reply.Error.Code)
}

func TestOAuthLoginCode_ACodeIsNotACredentialOnAnyOtherRoute(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ts := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(ts.Close)
	userID, _ := createLiveTestUser(t, pool, "user")
	code, err := db.NewOAuthLoginCodeRepository(pool).Issue(ctx, userID, auth.LoginCodeTTL)
	require.NoError(t, err)

	status, body := rawRequest(t, "GET", ts.URL+"/v1/me", code, "")
	require.Equal(t, 401, status, "a login code presented as a bearer credential must be refused: %s", body)
}

func TestOAuthLoginCode_ADeletedAccountsCodeMintsNothing(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ts := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(ts.Close)
	userID, _ := createLiveTestUser(t, pool, "user")
	code, err := db.NewOAuthLoginCodeRepository(pool).Issue(ctx, userID, auth.LoginCodeTTL)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, userID)
	require.NoError(t, err)

	status, reply := postExchange(t, ts.URL, code)
	require.Equal(t, 401, status)
	require.Equal(t, "INVALID_LOGIN_CODE", reply.Error.Code)
	require.Empty(t, reply.Data.AccessToken)
}

// Two API instances share one database: racing exchanges of one code, spread over both, mint
// exactly one token.
func TestOAuthLoginCode_RacingExchangesAcrossTwoInstancesMintOneToken(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	a := httptest.NewServer(NewRouter(pool, nil, nil))
	b := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	userID, _ := createLiveTestUser(t, pool, "user")
	code, err := db.NewOAuthLoginCodeRepository(pool).Issue(ctx, userID, auth.LoginCodeTTL)
	require.NoError(t, err)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			base := a.URL
			if i%2 == 1 {
				base = b.URL
			}
			if status, _ := postExchange(t, base, code); status == 200 {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, wins.Load(), "exactly one of 10 racing exchanges over two instances may mint a token")
}

// A failed exchange logs its request body (redacted); the code in it must not reach the log.
func TestOAuthLoginCode_TheCodeNeverReachesTheAccessLog(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ts := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(ts.Close)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	distinctive := "solvr_lc_LOGLEAKCANARY" + strings.Repeat("q", 20)
	status, _ := postExchange(t, ts.URL, distinctive)
	require.Equal(t, 401, status)
	time.Sleep(50 * time.Millisecond)
	require.Contains(t, logs.String(), "/v1/auth/oauth/exchange", "the capture must see the request, or this test proves nothing")
	require.NotContains(t, logs.String(), "LOGLEAKCANARY", "the login code reached the access log")
}
