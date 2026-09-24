package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shared room bearer token (solvr_rm_..., rooms.token_hash) is retired: room
// membership (room_members) is the only authority, and every /r/{slug}/* caller holds
// its OWN per-agent token (solvr_rt_...) issued by POST /v1/rooms/{slug}/handshake.

func TestRoomSharedTokenRetired_CreateReturnsNoToken(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	_, agentKey := registerRoomTestAgent(t, ts)

	for name, body := range map[string]struct{ bearer, body string }{
		"human public":  {jwt, `{"display_name":"Retired Token H","slug":"test-rtk-h"}`},
		"human private": {jwt, `{"display_name":"Retired Token HP","slug":"test-rtk-hp","is_private":true}`},
		"agent private": {agentKey, `{"display_name":"Retired Token AP","slug":"test-rtk-ap","is_private":true}`},
	} {
		resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body.body, body.bearer)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode, "%s: %s", name, string(raw))
		assert.NotContains(t, string(raw), `"token"`, "%s: create must not return a shared room token", name)
		assert.NotContains(t, string(raw), "solvr_rm_", name)
	}
}

func TestRoomSharedTokenRetired_RotateTokenRouteGone(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, agentKey := registerRoomTestAgent(t, ts)
	slug, _ := createTestRoomWithAgentKey(t, ts, agentKey)

	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/rotate-token", "", agentKey)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, resp.StatusCode,
		"rotate-token must no longer exist: %d %s", resp.StatusCode, string(raw))
	assert.NotContains(t, string(raw), "solvr_rm_")
}

func TestRoomSharedTokenRetired_HandshakeRoomTokenGrantsNothing(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createClosedRoom(t, ts, jwt)
	_, strangerKey := registerRoomTestAgent(t, ts)

	// A non-member, non-family agent presenting any room_token is still refused.
	status, tok := handshake(t, ts.URL, slug, strangerKey, "solvr_rm_legacy-shared-token")
	assert.Equal(t, http.StatusForbidden, status)
	assert.Empty(t, tok)
}

func TestRoomSharedTokenRetired_A2ARejectsNonAgentBearer(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, agentKey := registerRoomTestAgent(t, ts)
	slug, perAgentTok := createTestRoomWithAgentKey(t, ts, agentKey)
	require.True(t, strings.HasPrefix(perAgentTok, "solvr_rt_"), "room helpers hand out per-agent tokens: %q", perAgentTok)

	// The creator's per-agent token works on /r/.
	assert.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/r/"+slug+"/messages", perAgentTok))

	// Anything that is not a live per-agent token is 401 (no shared-token fallback).
	for _, bearer := range []string{"solvr_rm_legacy-shared-token", agentKey, "garbage"} {
		assert.Equal(t, http.StatusUnauthorized, getStatus(t, ts.URL+"/r/"+slug+"/messages", bearer), bearer)
	}
}

func TestRoomSharedTokenRetired_SchemaHasNoTokenHash(t *testing.T) {
	_, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'rooms' AND column_name = 'token_hash'`).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "rooms.token_hash must be dropped")
}
