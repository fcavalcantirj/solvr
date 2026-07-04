package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// doJSON performs an HTTP request with an optional bearer token and returns status + body.
func doJSON(t *testing.T, method, url, bearer, body string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, url, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
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

// handshake performs a handshake and returns the issued per-agent room token.
func handshake(t *testing.T, baseURL, slug, agentKey, roomToken string) (int, string) {
	t.Helper()
	body := "{}"
	if roomToken != "" {
		body = `{"room_token":"` + roomToken + `"}`
	}
	status, out := doJSON(t, "POST", baseURL+"/v1/rooms/"+slug+"/handshake", agentKey, body)
	data, _ := out["data"].(map[string]any)
	tok, _ := data["room_token"].(string)
	return status, tok
}

func TestRoomHandshake_PublicRoom_IssuesTokenAndAuthoritativeAuthorship(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%hs%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public room
	agentID, agentKey := registerTestAgent(t, ts, uniqName("agent_hs_pub"))

	// Handshake with the agent's own key -> per-agent token.
	status, perAgentTok := handshake(t, ts.URL, slug, agentKey, "")
	require.Equal(t, http.StatusCreated, status)
	require.True(t, strings.HasPrefix(perAgentTok, "solvr_rt_"), "expected per-agent token, got %q", perAgentTok)

	// Posting with the per-agent token stamps an authoritative author_id.
	st, out := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", perAgentTok, `{"agent_name":"display","content":"hello"}`)
	require.Equal(t, http.StatusCreated, st)
	data, _ := out["data"].(map[string]any)
	require.Equal(t, agentID, data["author_id"], "author_id must be the authenticated agent id")
}

func TestRoomHandshake_ClosedRoom_RequiresMembershipOrToken(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%hs%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createClosedRoom(t, ts, jwt)
	_, agentKey := registerTestAgent(t, ts, uniqName("agent_hs_closed"))

	// No membership, no room token -> 403.
	status, _ := handshake(t, ts.URL, slug, agentKey, "")
	require.Equal(t, http.StatusForbidden, status)

	// Bootstrapping with the shared room token -> issued a per-agent token.
	status, perAgentTok := handshake(t, ts.URL, slug, agentKey, roomToken)
	require.Equal(t, http.StatusCreated, status)
	require.NotEmpty(t, perAgentTok)

	// The per-agent token now reads the closed room (it made the agent a member).
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, perAgentTok))
}

func TestRoomMembers_AddRevoke_IsolatesAgents(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%hs%'") }) //nolint:errcheck

	_, ownerJWT := createRoomTestUser(t, pool)
	slug, _ := createClosedRoom(t, ts, ownerJWT)
	agentA, keyA := registerTestAgent(t, ts, uniqName("agent_hs_a"))
	agentB, keyB := registerTestAgent(t, ts, uniqName("agent_hs_b"))

	// Owner adds both agents to the allowlist.
	stA, _ := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+agentA+`"}`)
	require.Equal(t, http.StatusCreated, stA)
	stB, _ := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+agentB+`"}`)
	require.Equal(t, http.StatusCreated, stB)

	// Both handshake (they're members) and both can post via their per-agent tokens.
	sA, tokA := handshake(t, ts.URL, slug, keyA, "")
	require.Equal(t, http.StatusCreated, sA)
	sB, tokB := handshake(t, ts.URL, slug, keyB, "")
	require.Equal(t, http.StatusCreated, sB)
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, tokA))
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, tokB))

	// Owner revokes agent A only.
	stDel, _ := doJSON(t, "DELETE", ts.URL+"/v1/rooms/"+slug+"/members/"+agentA, ownerJWT, "")
	require.Equal(t, http.StatusNoContent, stDel)

	// Agent A's per-agent token is dead (401) and its closed-room read is denied (403)...
	postStatus, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", tokA, `{"agent_name":"a","content":"x"}`)
	require.Equal(t, http.StatusUnauthorized, postStatus, "revoked agent's per-agent token must be rejected")
	require.Equal(t, http.StatusForbidden, getStatus(t, ts.URL+"/v1/rooms/"+slug, keyA), "revoked agent loses closed-room read")

	// ...while agent B is unaffected — no shared-token rotation needed.
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, tokB), "other agent's access must be intact")
	postB, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", tokB, `{"agent_name":"b","content":"y"}`)
	require.Equal(t, http.StatusCreated, postB)
}

// uniqName appends a time-based suffix to keep agent names unique across runs.
func uniqName(base string) string {
	return base + "_" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")
}
