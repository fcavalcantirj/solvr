package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// idx 75 step 5: the claim link must not expose the credential it carries. On the real router
// and Postgres: the token is in the URL fragment (never sent to a server), the API takes it
// from a request body (never a URL), the database holds no clear copy, and a repeat request
// for the link still gets the same link back from any API instance.

func claimGenerate(t *testing.T, ts *httptest.Server, agentKey string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, "POST", ts.URL+"/v1/agents/me/claim", agentKey, "")
}

func TestClaimLink_OnTheRealRouterTheTokenIsNeverInAURLOrStoredInTheClear(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM claim_tokens WHERE agent_id = $1", agentID) //nolint:errcheck
	})

	status, out := claimGenerate(t, ts, agentKey)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	token, _ := out["token"].(string)
	claimURL, _ := out["claim_url"].(string)
	require.NotEmpty(t, token)
	u, err := url.Parse(claimURL)
	require.NoError(t, err)
	require.Equal(t, "/claim", u.Path, "the token is not a path segment: %s", claimURL)
	require.Empty(t, u.RawQuery, "the token is not in a query: %s", claimURL)
	require.Equal(t, "token="+token, u.Fragment)

	// Storage: a hash and a sealed copy, never the token.
	var hash, rowJSON string
	var sealed []byte
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT token_hash, token_sealed, row_to_json(c)::text FROM claim_tokens c WHERE agent_id = $1`, agentID).
		Scan(&hash, &sealed, &rowJSON))
	sum := sha256.Sum256([]byte(token))
	require.Equal(t, hex.EncodeToString(sum[:]), hash)
	require.NotEmpty(t, sealed, "the router's repository must seal so a repeat request can return the link")
	require.NotContains(t, rowJSON, token, "the clear token is stored")

	// A repeat request, on this instance and on another one, returns the same link.
	status, again := claimGenerate(t, ts, agentKey)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, claimURL, again["claim_url"])
	other := httptest.NewServer(NewRouter(pool, nil, nil))
	t.Cleanup(other.Close)
	status, viaOther := claimGenerate(t, other, agentKey)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, claimURL, viaOther["claim_url"], "another API instance must open the same sealed token")

	// The page asks the API about the token with a body; a URL carries nothing.
	status, info := doJSON(t, "POST", ts.URL+"/v1/agents/claim/lookup", "", `{"token":"`+token+`"}`)
	require.Equal(t, http.StatusOK, status, "%v", info)
	require.Equal(t, true, info["token_valid"])
	status, _ = doJSON(t, "POST", ts.URL+"/v1/agents/claim/lookup?token="+token, "", `{}`)
	require.Equal(t, http.StatusBadRequest, status, "a token only in the URL must not be read")
	status, _ = doJSON(t, "GET", ts.URL+"/v1/claim/"+token, "", "")
	require.Equal(t, http.StatusNotFound, status, "the path-token lookup route is retired")

	// The link still claims the agent once, for the human who holds it.
	_, humanJWT := createRoomTestUser(t, pool)
	status, claimed := doJSON(t, "POST", ts.URL+"/v1/agents/claim", humanJWT, `{"token":"`+token+`"}`)
	require.Equal(t, http.StatusOK, status, "%v", claimed)
	status, info = doJSON(t, "POST", ts.URL+"/v1/agents/claim/lookup", "", `{"token":"`+token+`"}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, false, info["token_valid"], "a used link is no longer valid")
}

// A row from before the migration has a hash and no sealed copy. The agent's next request
// replaces it with a sealed one instead of answering with a blank link.
func TestClaimLink_ALiveTokenWithNoSealedCopyIsReplacedOnTheNextRequest(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM claim_tokens WHERE agent_id = $1", agentID) //nolint:errcheck
	})

	legacy := "legacy_live_token_" + agentID
	sum := sha256.Sum256([]byte(legacy))
	_, err := pool.Exec(context.Background(),
		`INSERT INTO claim_tokens (token_hash, agent_id, expires_at) VALUES ($1, $2, NOW() + INTERVAL '2 hours')`,
		hex.EncodeToString(sum[:]), agentID)
	require.NoError(t, err)

	status, out := claimGenerate(t, ts, agentKey)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	token, _ := out["token"].(string)
	require.NotEmpty(t, token)
	require.NotEqual(t, legacy, token)
	require.Contains(t, out["claim_url"], "#token="+token)

	// The old value no longer resolves, the new one does, and the next repeat is stable.
	status, info := doJSON(t, "POST", ts.URL+"/v1/agents/claim/lookup", "", `{"token":"`+legacy+`"}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, false, info["token_valid"])
	status, again := claimGenerate(t, ts, agentKey)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, out["claim_url"], again["claim_url"])
}

// A failed request logs its (redacted) body; the token in a claim lookup or claim must not
// reach the access log, and neither may the link.
func TestClaimLink_TheTokenNeverReachesTheAccessLog(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, humanJWT := createRoomTestUser(t, pool)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	canary := "CLAIMLEAKCANARY" + strings.Repeat("z", 20)
	// A lookup that fails after the body is read (over the size cap), and a claim that fails.
	status, _ := doJSON(t, "POST", ts.URL+"/v1/agents/claim/lookup", "",
		`{"token":"`+canary+`","padding":"`+strings.Repeat("p", 8192)+`"}`)
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = doJSON(t, "POST", ts.URL+"/v1/agents/claim", humanJWT, `{"token":"`+canary+`"}`)
	require.Equal(t, http.StatusNotFound, status)

	time.Sleep(50 * time.Millisecond)
	require.Contains(t, logs.String(), "/v1/agents/claim", "the capture must see the requests, or this test proves nothing")
	require.NotContains(t, logs.String(), "CLAIMLEAKCANARY", "the claim token reached the access log")
}
