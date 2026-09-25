package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/require"
)

// claimAttempt posts one claim and returns (status, error code). It never calls t.FailNow,
// so it is safe inside a goroutine.
func claimAttempt(client *http.Client, ts *httptest.Server, jwt, token string) (int, string, error) {
	req, err := http.NewRequest("POST", ts.URL+"/v1/agents/claim", strings.NewReader(`{"token":"`+token+`"}`))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out.Error.Code, nil
}

// claimBurst opens n connections first (a public read of the claim page), then releases n
// claims at once over them, so no attempt finishes before the others have been sent.
func claimBurst(t *testing.T, client *http.Client, ts *httptest.Server, jwts []string, token string, n int) ([]int, []string) {
	t.Helper()
	statuses := make([]int, n)
	codes := make([]string, n)
	errs := make([]error, n)
	run := func(fn func(i int)) {
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				fn(i)
			}()
		}
		close(start)
		wg.Wait()
	}
	run(func(i int) {
		resp, err := client.Get(ts.URL + "/v1/claim/" + token)
		if err != nil {
			errs[i] = err
			return
		}
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
	})
	run(func(i int) {
		if errs[i] == nil {
			statuses[i], codes[i], errs[i] = claimAttempt(client, ts, jwts[i%len(jwts)], token)
		}
	})
	for _, err := range errs {
		require.NoError(t, err)
	}
	return statuses, codes
}

func claimedAgentState(t *testing.T, pool *db.Pool, agentID string) (humanID string, reputation int) {
	t.Helper()
	var h *string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT human_id::text, reputation FROM agents WHERE id = $1`, agentID).Scan(&h, &reputation))
	if h != nil {
		humanID = *h
	}
	return humanID, reputation
}

// A claim moves an agent from no owner to one human exactly once. Parallel retries by the
// same human and a race between several humans holding the same token end with one 200,
// one owner and one +50 bonus; every other attempt is a 409 (never a 500), and a later
// retry changes nothing.
func TestAgentClaim_RetriesAndRacesLinkTheAgentOnce(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	cases := []struct {
		name   string
		humans int
	}{
		{"same human retries in parallel", 1},
		{"several humans race one token", 4},
	}
	const attempts, rounds = 16, 4
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: attempts}}
	defer client.CloseIdleConnections()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for round := range rounds {
				agentID, agentKey := registerRoomTestAgent(t, ts)
				t.Cleanup(func() {
					pool.Exec(context.Background(), "DELETE FROM claim_tokens WHERE agent_id = $1", agentID) //nolint:errcheck
				})
				status, out := doJSON(t, "POST", ts.URL+"/v1/agents/me/claim", agentKey, "")
				require.Equal(t, http.StatusCreated, status, "generate claim: %v", out)
				token, _ := out["token"].(string)
				require.NotEmpty(t, token)

				userIDs := make([]string, tc.humans)
				jwts := make([]string, tc.humans)
				for i := range jwts {
					userIDs[i], jwts[i] = createRoomTestUser(t, pool)
				}
				owner, repBefore := claimedAgentState(t, pool, agentID)
				require.Empty(t, owner)

				statuses, codes := claimBurst(t, client, ts, jwts, token, attempts)
				winner := -1
				for i := range attempts {
					if statuses[i] == http.StatusOK {
						require.Equal(t, -1, winner, "round %d: exactly one claim succeeds: %v", round, statuses)
						winner = i % tc.humans
						continue
					}
					require.Equal(t, http.StatusConflict, statuses[i],
						"round %d: a losing claim is a conflict, not a failure: %v %v", round, statuses, codes)
					require.Contains(t, []string{"TOKEN_USED", "ALREADY_CLAIMED"}, codes[i])
				}
				require.NotEqual(t, -1, winner, "round %d: one claim succeeds: %v %v", round, statuses, codes)

				owner, repAfter := claimedAgentState(t, pool, agentID)
				require.Equal(t, userIDs[winner], owner, "the agent belongs to the one winning human")
				require.Equal(t, repBefore+handlers.ReputationBonusOnClaim, repAfter, "the claim bonus is granted once")
				var usedBy *string
				require.NoError(t, pool.QueryRow(context.Background(),
					`SELECT used_by_human_id::text FROM claim_tokens WHERE token = $1`, token).Scan(&usedBy))
				require.NotNil(t, usedBy)
				require.Equal(t, userIDs[winner], *usedBy, "the token records the winning human")

				// A retry after the fact, by the winner or anyone else, changes nothing.
				for i := range jwts {
					status, code, err := claimAttempt(client, ts, jwts[i], token)
					require.NoError(t, err)
					require.Equal(t, http.StatusConflict, status)
					require.Equal(t, "TOKEN_USED", code)
				}
				owner, repFinal := claimedAgentState(t, pool, agentID)
				require.Equal(t, userIDs[winner], owner)
				require.Equal(t, repAfter, repFinal)
			}
		})
	}
}
