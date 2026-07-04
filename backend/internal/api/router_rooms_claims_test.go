package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// postClaim POSTs to an /r/{slug}/... claim endpoint with the room bearer token and
// returns the HTTP status and decoded body.
func postClaim(t *testing.T, url, roomToken, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+roomToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

func claimOutcome(body map[string]any) string {
	data, _ := body["data"].(map[string]any)
	outcome, _ := data["outcome"].(string)
	return outcome
}

func TestRoomClaims_AcquireRenewRelease(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)
	claimURL := ts.URL + "/r/" + slug + "/claim"

	// w1 acquires APP-185.
	status, body := postClaim(t, claimURL, roomToken, `{"key":"APP-185","agent":"w1","ttl_seconds":300}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "won", claimOutcome(body))

	// w2 attempts the same key -> held by w1.
	status, body = postClaim(t, claimURL, roomToken, `{"key":"APP-185","agent":"w2","ttl_seconds":300}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "held", claimOutcome(body))
	heldClaim, _ := body["data"].(map[string]any)
	claim, _ := heldClaim["claim"].(map[string]any)
	require.Equal(t, "w1", claim["holder"])

	// GET /claims lists the live holder.
	req, _ := http.NewRequest("GET", ts.URL+"/r/"+slug+"/claims", nil)
	req.Header.Set("Authorization", "Bearer "+roomToken)
	listResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	var listBody map[string]any
	json.NewDecoder(listResp.Body).Decode(&listBody)
	listResp.Body.Close()
	claims, _ := listBody["data"].([]any)
	require.Len(t, claims, 1)

	// w1 renews; w2 cannot.
	status, _ = postClaim(t, claimURL+"/renew", roomToken, `{"key":"APP-185","agent":"w1","ttl_seconds":600}`)
	require.Equal(t, http.StatusOK, status)
	status, _ = postClaim(t, claimURL+"/renew", roomToken, `{"key":"APP-185","agent":"w2"}`)
	require.Equal(t, http.StatusConflict, status)

	// w2 cannot release; w1 can.
	status, _ = postClaim(t, claimURL+"/release", roomToken, `{"key":"APP-185","agent":"w2"}`)
	require.Equal(t, http.StatusConflict, status)
	status, _ = postClaim(t, claimURL+"/release", roomToken, `{"key":"APP-185","agent":"w1"}`)
	require.Equal(t, http.StatusOK, status)

	// After release, w2 can win it.
	status, body = postClaim(t, claimURL, roomToken, `{"key":"APP-185","agent":"w2","ttl_seconds":300}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "won", claimOutcome(body))
}

// TestRoomClaims_ConcurrentAcquire_ExactlyOneWins verifies atomicity end-to-end
// through the HTTP layer (mission #2 AC).
func TestRoomClaims_ConcurrentAcquire_ExactlyOneWins(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)
	claimURL := ts.URL + "/r/" + slug + "/claim"

	const n = 16
	var wins int32
	var mu sync.Mutex
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			body := fmt.Sprintf(`{"key":"BUILD-1","agent":"w%d","ttl_seconds":300}`, idx)
			status, out := postClaim(t, claimURL, roomToken, body)
			if status != http.StatusOK {
				t.Errorf("claim status = %d", status)
				return
			}
			if claimOutcome(out) == "won" {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	require.EqualValues(t, 1, wins, "exactly one concurrent claim must win")
}
