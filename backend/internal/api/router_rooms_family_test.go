package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// createPrivateRoomWithAgentKey creates a CLOSED (is_private) room via an agent API key.
// The room's owner_id is auto-set to the claimed agent's linked human. Returns slug, token.
func createPrivateRoomWithAgentKey(t *testing.T, ts *httptest.Server, apiKey string) (string, string) {
	t.Helper()
	slug := fmt.Sprintf("test-fam-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Family %s","slug":"%s","is_private":true}`, slug, slug)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create private room via agent: %s", string(raw))
	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	token, _ := result["token"].(string)
	data, _ := result["data"].(map[string]any)
	roomSlug, _ := data["slug"].(string)
	require.NotEmpty(t, roomSlug)
	return roomSlug, token
}

// meRoomSlugs calls GET /v1/me/rooms with the given bearer and returns the room slugs.
func meRoomSlugs(t *testing.T, ts *httptest.Server, bearer string) []string {
	t.Helper()
	status, out := doJSON(t, "GET", ts.URL+"/v1/me/rooms", bearer, "")
	require.Equal(t, http.StatusOK, status)
	data, _ := out["data"].([]any)
	slugs := make([]string, 0, len(data))
	for _, item := range data {
		if room, ok := item.(map[string]any); ok {
			if s, ok := room["slug"].(string); ok {
				slugs = append(slugs, s)
			}
		}
	}
	return slugs
}

// TestRoomFamily_SiblingAgent_AccessesAndHandshakesClosedRoom verifies that an agent
// claimed by the same human as the room owner ("family sibling") can read AND handshake a
// closed room it was never explicitly allowlisted into, and then act on the data plane
// with its OWN per-agent token.
func TestRoomFamily_SiblingAgent_AccessesAndHandshakesClosedRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userA, _ := createRoomTestUser(t, pool)
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	agentBID, agentBKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, userA) // sibling: same human as owner

	// Agent A (claimed to userA) creates a closed room -> owner_id = userA.
	slug, _ := createPrivateRoomWithAgentKey(t, ts, agentAKey)

	// Sibling B is NOT on the allowlist and holds no room token, yet:
	// 1) family read succeeds (was 403 before this feature).
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, agentBKey),
		"sibling agent (same human) should read the closed room")

	// 2) family handshake succeeds and issues B its OWN per-agent token.
	hsStatus, siblingTok := handshake(t, ts.URL, slug, agentBKey, "")
	require.Equal(t, http.StatusCreated, hsStatus, "sibling handshake should succeed")
	require.Contains(t, siblingTok, "solvr_rt_", "expected a per-agent room token")

	// 3) B acts on the data plane with its own token (authoritative authorship).
	st, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", siblingTok,
		`{"agent_name":"sibling","content":"family coordination"}`)
	require.Equal(t, http.StatusCreated, st, "sibling should post with its own room token")
}

// TestRoomFamily_ForeignAndUnclaimedAgents_Still403 is the non-negotiable regression:
// a foreign agent (different human) and an unclaimed agent must STILL be denied a closed
// room — both read and handshake.
func TestRoomFamily_ForeignAndUnclaimedAgents_Still403(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userA, _ := createRoomTestUser(t, pool)
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	slug, _ := createPrivateRoomWithAgentKey(t, ts, agentAKey) // owner_id = userA

	// Foreign agent: claimed to a DIFFERENT human.
	userB, _ := createRoomTestUser(t, pool)
	foreignID, foreignKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, foreignID, userB)
	require.Equal(t, http.StatusForbidden, getStatus(t, ts.URL+"/v1/rooms/"+slug, foreignKey),
		"foreign agent read must stay 403")
	fhs, _ := handshake(t, ts.URL, slug, foreignKey, "")
	require.Equal(t, http.StatusForbidden, fhs, "foreign agent handshake must stay 403")

	// Unclaimed agent: no linked human at all.
	_, unclaimedKey := registerRoomTestAgent(t, ts)
	require.Equal(t, http.StatusForbidden, getStatus(t, ts.URL+"/v1/rooms/"+slug, unclaimedKey),
		"unclaimed agent read must stay 403")
	uhs, _ := handshake(t, ts.URL, slug, unclaimedKey, "")
	require.Equal(t, http.StatusForbidden, uhs, "unclaimed agent handshake must stay 403")
}

// TestRoomFamily_Discovery_ScopedByHuman verifies GET /v1/me/rooms lists the caller's
// human's rooms (including private) for a sibling, and does NOT leak them to a foreign
// agent.
func TestRoomFamily_Discovery_ScopedByHuman(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userA, _ := createRoomTestUser(t, pool)
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	agentBID, agentBKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, userA) // sibling

	slug, _ := createPrivateRoomWithAgentKey(t, ts, agentAKey) // private, owner_id = userA

	// Sibling B discovers A's private room via /me/rooms.
	require.Contains(t, meRoomSlugs(t, ts, agentBKey), slug,
		"sibling should discover the family's private room")

	// Foreign agent (different human) does NOT see it.
	userB, _ := createRoomTestUser(t, pool)
	foreignID, foreignKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, foreignID, userB)
	require.NotContains(t, meRoomSlugs(t, ts, foreignKey), slug,
		"foreign agent must not discover another human's private room")

	// Unclaimed agent gets an empty (but 200) list.
	_, unclaimedKey := registerRoomTestAgent(t, ts)
	require.NotContains(t, meRoomSlugs(t, ts, unclaimedKey), slug,
		"unclaimed agent has no family rooms")
}
