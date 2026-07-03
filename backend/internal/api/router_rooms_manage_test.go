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

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerRoomTestAgent registers an agent via the API and returns its ID and API key.
// Names start with "roomtest_" so IDs are "agent_roomtest_%" for cleanup.
func registerRoomTestAgent(t *testing.T, ts *httptest.Server) (string, string) {
	t.Helper()
	name := fmt.Sprintf("roomtest_%d", time.Now().UnixNano()%100000000)
	body := fmt.Sprintf(`{"name":"%s","description":"room management integration test agent"}`, name)
	resp, err := http.Post(ts.URL+"/v1/agents/register", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to register agent: %s", string(respBody))

	var result struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal(respBody, &result))
	require.NotEmpty(t, result.Agent.ID)
	require.NotEmpty(t, result.APIKey)
	return result.Agent.ID, result.APIKey
}

// claimAgentToUser links an agent to a human user directly in the database.
func claimAgentToUser(t *testing.T, pool *db.Pool, agentID, userID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"UPDATE agents SET human_id = $1::uuid WHERE id = $2", userID, agentID)
	require.NoError(t, err, "failed to claim agent to user")
}

// createTestRoomWithAgentKey creates a room via the API using an agent API key.
// Returns slug and room bearer token.
func createTestRoomWithAgentKey(t *testing.T, ts *httptest.Server, apiKey string) (string, string) {
	t.Helper()
	slug := fmt.Sprintf("test-%d", time.Now().UnixNano()%1000000)
	body := fmt.Sprintf(`{"display_name":"Agent Test Room %s","slug":"%s"}`, slug, slug)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create room with agent key: %s", string(respBody))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	token, _ := result["token"].(string)
	require.NotEmpty(t, token)
	data, _ := result["data"].(map[string]interface{})
	roomSlug, _ := data["slug"].(string)
	require.NotEmpty(t, roomSlug)
	return roomSlug, token
}

// doRoomRequest sends an HTTP request with an optional bearer credential.
func doRoomRequest(t *testing.T, method, url, body, bearer string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// --- CreateRoom with agent keys ---

func TestRoomRoutes_CreateRoom_AgentKey(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, _ := createRoomTestUser(t, pool)
	agentID, apiKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentID, userID)

	slug := fmt.Sprintf("test-%d", time.Now().UnixNano()%1000000)
	body := fmt.Sprintf(`{"display_name":"Claimed Agent Room","slug":"%s"}`, slug)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(respBody))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.NotEmpty(t, result["token"], "expected room token in response")
	data, _ := result["data"].(map[string]interface{})
	assert.Equal(t, userID, data["owner_id"], "expected owner to be the agent's linked human")
}

func TestRoomRoutes_CreateRoom_UnclaimedAgent(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, apiKey := registerRoomTestAgent(t, ts)

	slug := fmt.Sprintf("test-%d", time.Now().UnixNano()%1000000)
	body := fmt.Sprintf(`{"display_name":"Unclaimed Agent Room","slug":"%s"}`, slug)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(respBody))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	data, _ := result["data"].(map[string]interface{})
	assert.Nil(t, data["owner_id"], "expected ownerless room for unclaimed agent")
}

// --- UpdateRoom ---

func TestRoomRoutes_UpdateRoom_AgentOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, _ := createRoomTestUser(t, pool)
	agentID, apiKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentID, userID)
	slug, _ := createTestRoomWithAgentKey(t, ts, apiKey)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Renamed by Agent"}`, apiKey)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(respBody))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	data, _ := result["data"].(map[string]interface{})
	assert.Equal(t, "Renamed by Agent", data["display_name"])
}

func TestRoomRoutes_UpdateRoom_AgentNonOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	// Agent A (claimed to user1) creates the room.
	user1ID, _ := createRoomTestUser(t, pool)
	agentAID, apiKeyA := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, user1ID)
	slug, _ := createTestRoomWithAgentKey(t, ts, apiKeyA)

	// Agent B (claimed to user2) tries to update it.
	user2ID, _ := createRoomTestUser(t, pool)
	agentBID, apiKeyB := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, user2ID)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Hijacked"}`, apiKeyB)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestRoomRoutes_UpdateRoom_UnclaimedAgentOwnerlessRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, apiKey := registerRoomTestAgent(t, ts)
	slug, _ := createTestRoomWithAgentKey(t, ts, apiKey)

	// Even the creating agent cannot manage an ownerless room.
	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Nope"}`, apiKey)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestRoomRoutes_UpdateRoom_HumanOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Renamed by Human"}`, jwt)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRoomRoutes_UpdateRoom_Admin(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)

	adminJWT, err := auth.GenerateJWT(roomTestJWTSecret, uuid.New().String(), "admin@test.solvr.dev", "admin", time.Hour)
	require.NoError(t, err)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Renamed by Admin"}`, adminJWT)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRoomRoutes_UpdateRoom_NoAuth(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Anon"}`, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- DeleteRoom ---

func TestRoomRoutes_DeleteRoom_AgentOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, _ := createRoomTestUser(t, pool)
	agentID, apiKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentID, userID)
	slug, _ := createTestRoomWithAgentKey(t, ts, apiKey)

	resp := doRoomRequest(t, "DELETE", ts.URL+"/v1/rooms/"+slug, "", apiKey)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Room is gone.
	getResp, err := http.Get(ts.URL + "/v1/rooms/" + slug)
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func TestRoomRoutes_DeleteRoom_AgentNonOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	user1ID, _ := createRoomTestUser(t, pool)
	agentAID, apiKeyA := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, user1ID)
	slug, _ := createTestRoomWithAgentKey(t, ts, apiKeyA)

	user2ID, _ := createRoomTestUser(t, pool)
	agentBID, apiKeyB := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, user2ID)

	resp := doRoomRequest(t, "DELETE", ts.URL+"/v1/rooms/"+slug, "", apiKeyB)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestRoomRoutes_DeleteRoom_HumanOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)

	resp := doRoomRequest(t, "DELETE", ts.URL+"/v1/rooms/"+slug, "", jwt)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// --- RotateToken ---

func TestRoomRoutes_RotateToken_AgentOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, _ := createRoomTestUser(t, pool)
	agentID, apiKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentID, userID)
	slug, oldToken := createTestRoomWithAgentKey(t, ts, apiKey)

	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/rotate-token", "", apiKey)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(respBody))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	data, _ := result["data"].(map[string]interface{})
	newToken, _ := data["token"].(string)
	require.NotEmpty(t, newToken)
	assert.NotEqual(t, oldToken, newToken)

	// Old token no longer works on the A2A route; new one does.
	joinBody := `{"agent_name":"rotate-test-agent"}`
	oldResp := doRoomRequest(t, "POST", ts.URL+"/r/"+slug+"/join", joinBody, oldToken)
	defer oldResp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, oldResp.StatusCode)

	newResp := doRoomRequest(t, "POST", ts.URL+"/r/"+slug+"/join", joinBody, newToken)
	defer newResp.Body.Close()
	assert.Equal(t, http.StatusOK, newResp.StatusCode)
}

func TestRoomRoutes_RotateToken_AgentNonOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	user1ID, _ := createRoomTestUser(t, pool)
	agentAID, apiKeyA := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, user1ID)
	slug, _ := createTestRoomWithAgentKey(t, ts, apiKeyA)

	user2ID, _ := createRoomTestUser(t, pool)
	agentBID, apiKeyB := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, user2ID)

	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/rotate-token", "", apiKeyB)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestRoomRoutes_RotateToken_HumanOwner(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)

	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/rotate-token", "", jwt)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRoomRoutes_RotateToken_Admin(t *testing.T) {
	// D-25 amendment: admin can rotate — otherwise ownerless rooms have
	// tokens nobody can rotate.
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)

	adminJWT, err := auth.GenerateJWT(roomTestJWTSecret, uuid.New().String(), "admin@test.solvr.dev", "admin", time.Hour)
	require.NoError(t, err)

	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/rotate-token", "", adminJWT)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
