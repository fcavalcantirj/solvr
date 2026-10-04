package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A private room's owner connects agents with the API-owned join prompt from
// GET /v1/rooms/{slug}/connect, not by rotating and handing out the shared room
// token. The prompt must reach exactly the callers RoomAccessGuard admits, and the
// handshake it teaches must work for the owner's own (family) agent.

func createPrivateRoomWithJWT(t *testing.T, baseURL, jwt string) (slug string) {
	t.Helper()
	slug = fmt.Sprintf("test-pconn-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Private %s","slug":"%s","is_private":true}`, slug, slug)
	resp := doRoomRequest(t, "POST", baseURL+"/v1/rooms", body, jwt)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create private room: %s", string(raw))
	return slug
}

func TestRoomPrivateConnect_OwnerGetsHandshakePrompt(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	ownerID, ownerJWT := createRoomTestUser(t, pool)
	slug := createPrivateRoomWithJWT(t, ts.URL, ownerJWT)

	resp := doRoomRequest(t, "GET", ts.URL+"/v1/rooms/"+slug+"/connect?role=collaborator", "", ownerJWT)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "owner must get the private join prompt: %s", string(raw))

	var env struct {
		Data struct {
			Private  bool   `json:"private"`
			RoomSlug string `json:"room_slug"`
			Role     string `json:"role"`
			Prompt   struct {
				Text string `json:"text"`
			} `json:"prompt"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.True(t, env.Data.Private)
	assert.Equal(t, slug, env.Data.RoomSlug)
	assert.Equal(t, "collaborator", env.Data.Role)
	// The sentence sends the agent to the skill, whose Join a room recipe is the per-agent
	// handshake (TestPublishedSkill_EachAgentTakesItsOwnRoomTokenByHandshake).
	assert.Contains(t, env.Data.Prompt.Text, "Learn Solvr from https://solvr.dev/skill.md. Join the private Solvr room")
	assert.Contains(t, env.Data.Prompt.Text, "https://solvr.dev/rooms/"+slug+" as the COLLABORATOR")
	assert.Contains(t, env.Data.Prompt.Text, "give me your agent id first")
	assert.NotContains(t, env.Data.Prompt.Text, "solvr_rm_", "the prompt must never carry a shared room token")

	// Following the prompt: the owner's own claimed agent handshakes with its OWN key
	// (family scope) and receives an individual solvr_rt_ — no shared token involved.
	agentID, agentKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentID, ownerID)
	resp = doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/handshake", "", agentKey)
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "family handshake: %s", string(raw))
	assert.True(t, strings.Contains(string(raw), `"room_token":"solvr_rt_`), "handshake issues a per-agent token: %s", string(raw))
}

func TestRoomPrivateConnect_OutsidersStillForbidden(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	slug := createPrivateRoomWithJWT(t, ts.URL, ownerJWT)
	_, otherJWT := createRoomTestUser(t, pool)
	_, strangerKey := registerRoomTestAgent(t, ts)

	for name, bearer := range map[string]string{
		"anonymous":        "",
		"non-member human": otherJWT,
		"non-member agent": strangerKey,
	} {
		resp := doRoomRequest(t, "GET", ts.URL+"/v1/rooms/"+slug+"/connect?role=collaborator", "", bearer)
		resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, name)
	}
}
