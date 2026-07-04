package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/require"
)

// createClosedRoom creates a private (closed) room via the API and returns slug, token.
func createClosedRoom(t *testing.T, ts *httptest.Server, jwt string) (string, string) {
	t.Helper()
	slug := fmt.Sprintf("test-closed-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Closed %s","slug":"%s","is_private":true}`, slug, slug)
	req, _ := http.NewRequest("POST", ts.URL+"/v1/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create closed room: %s", string(raw))
	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	token, _ := result["token"].(string)
	data, _ := result["data"].(map[string]any)
	roomSlug, _ := data["slug"].(string)
	require.NotEmpty(t, token)
	require.NotEmpty(t, roomSlug)
	return roomSlug, token
}

// registerTestAgent registers an agent via the public endpoint and returns (agentID, apiKey).
func registerTestAgent(t *testing.T, ts *httptest.Server, name string) (string, string) {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"description":"acl test agent"}`, name)
	resp, err := http.Post(ts.URL+"/v1/agents/register", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "register agent: %s", string(raw))
	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	apiKey, _ := result["api_key"].(string)
	agent, _ := result["agent"].(map[string]any)
	agentID, _ := agent["id"].(string)
	require.NotEmpty(t, apiKey, "expected api_key")
	require.NotEmpty(t, agentID, "expected agent id")
	return agentID, apiKey
}

// getStatus performs GET url with an optional bearer token and returns the status code.
func getStatus(t *testing.T, url, bearer string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	return resp.StatusCode
}

func TestRoomACL_ClosedRoom_ReadsAreMembersOnly(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%acl%'") //nolint:errcheck
	})

	_, ownerJWT := createRoomTestUser(t, pool)
	slug, roomToken := createClosedRoom(t, ts, ownerJWT)
	detailURL := ts.URL + "/v1/rooms/" + slug
	msgURL := ts.URL + "/v1/rooms/" + slug + "/messages"

	// Anonymous caller: 403 on every read of a closed room.
	require.Equal(t, http.StatusForbidden, getStatus(t, detailURL, ""), "anon detail should be 403")
	require.Equal(t, http.StatusForbidden, getStatus(t, msgURL, ""), "anon messages should be 403")

	// Shared room bearer token grants access (backward compat).
	require.Equal(t, http.StatusOK, getStatus(t, detailURL, roomToken), "shared token detail should be 200")
	require.Equal(t, http.StatusOK, getStatus(t, msgURL, roomToken), "shared token messages should be 200")

	// Human room owner (JWT) has access.
	require.Equal(t, http.StatusOK, getStatus(t, detailURL, ownerJWT), "owner detail should be 200")

	// A registered agent that is NOT on the allowlist is denied.
	agentID, agentKey := registerTestAgent(t, ts, fmt.Sprintf("agent_acl_stranger_%d", time.Now().UnixNano()%100000))
	require.Equal(t, http.StatusForbidden, getStatus(t, detailURL, agentKey), "non-member agent should be 403")

	// Add the agent to the allowlist -> it now reads the closed room.
	addMember(t, pool, slug, agentID)
	require.Equal(t, http.StatusOK, getStatus(t, detailURL, agentKey), "member agent detail should be 200")
	require.Equal(t, http.StatusOK, getStatus(t, msgURL, agentKey), "member agent messages should be 200")
}

func TestRoomACL_PublicRoom_StaysOpen(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public (is_private defaults false)

	// Anonymous reads of a public room remain 200.
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, ""), "anon detail of public room should be 200")
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug+"/messages", ""), "anon messages of public room should be 200")
}

// TestRoomACL_AgentCreatedRoom_IsManageableByCreator proves the owner fix: an
// unclaimed agent that creates a room becomes its owner-member and can manage it
// (previously such rooms were ownerless and unmanageable).
func TestRoomACL_AgentCreatedRoom_IsManageableByCreator(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%acl%'") //nolint:errcheck
	})

	// Register an UNCLAIMED agent (no linked human).
	_, agentKey := registerTestAgent(t, ts, fmt.Sprintf("agent_aclowner_%d", time.Now().UnixNano()%100000))

	// Create a room using the agent API key.
	slug := fmt.Sprintf("test-agentowned-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Agent Owned %s","slug":"%s"}`, slug, slug)
	req, _ := http.NewRequest("POST", ts.URL+"/v1/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agentKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "agent create room: %s", string(raw))

	// The creating agent can now DELETE its own room (owner via room_members).
	delReq, _ := http.NewRequest("DELETE", ts.URL+"/v1/rooms/"+slug, nil)
	delReq.Header.Set("Authorization", "Bearer "+agentKey)
	delResp, err := http.DefaultClient.Do(delReq)
	require.NoError(t, err)
	delResp.Body.Close()
	require.Equal(t, http.StatusNoContent, delResp.StatusCode, "creating agent should be able to delete its own room")
}

// addMember inserts a room_members row directly (the add-member endpoint is mission #3).
func addMember(t *testing.T, pool *db.Pool, slug, agentID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO room_members (room_id, agent_id, role, added_by)
		 VALUES ((SELECT id FROM rooms WHERE slug = $1), $2, 'member', 'test')
		 ON CONFLICT (room_id, agent_id) DO NOTHING`, slug, agentID)
	require.NoError(t, err)
}
