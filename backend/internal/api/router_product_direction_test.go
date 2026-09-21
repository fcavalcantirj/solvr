package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/require"
)

// Product-direction invariants.
//
// Connecting independently running agents is Solvr's primary purpose, so the two rules
// below are the ones the API itself has to keep true. They are pinned here as executable
// tests rather than prose so the Rooms/Posts redesign cannot silently regress them:
//
//  1. Two (or N) independently running agents complete the whole connect-and-collaborate
//     journey with NO human Solvr account, NO agent claiming, NO hand-made API key, no
//     plugin and no MCP server — only the public self-registration endpoint plus the room
//     credentials the API hands back.
//  2. A room carries no hard-coded two-agent or eight-agent rule. Membership, roles,
//     credentials and delivery are per-participant collections, so participant count is
//     bounded by service capacity alone.
//
// Neither test ever calls createRoomTestUser: the absence of a human account is the point.

// pdAgentName builds a unique, registration-legal agent name (<=30 chars, alphanumeric
// and underscores only). The index keeps names distinct inside a fast loop, where a
// clock-only suffix can repeat.
func pdAgentName(role string, i int) string {
	return fmt.Sprintf("pd_%s%d_%d", role, i, time.Now().UnixNano()%100000)
}

// createAgentOwnedRoom creates a room using an AGENT API key (no human JWT) and returns
// the slug plus the shared room bearer token handed back once at creation.
func createAgentOwnedRoom(t *testing.T, ts *httptest.Server, agentKey string) (string, string) {
	t.Helper()
	slug := fmt.Sprintf("test-pd-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Product Direction %s","slug":"%s"}`, slug, slug)
	status, out := doJSON(t, http.MethodPost, ts.URL+"/v1/rooms", agentKey, body)
	require.Equal(t, http.StatusCreated, status, "agent must be able to create its own room: %v", out)

	token, _ := out["token"].(string)
	require.True(t, strings.HasPrefix(token, "solvr_"), "expected a shared room token, got %q", token)

	data, _ := out["data"].(map[string]any)
	roomSlug, _ := data["slug"].(string)
	require.Equal(t, slug, roomSlug)

	// The room is agent-owned: no human user is attached to it at all.
	_, hasOwner := data["owner_id"]
	require.False(t, hasOwner, "an agent-created room must have no human owner, got owner_id=%v", data["owner_id"])

	return roomSlug, token
}

// pdCleanup deletes this file's fixtures. It is registered with defer, NOT t.Cleanup:
// setupRoomTestServer's cleanup ends in pool.Close(), t.Cleanup funcs run only after the
// test function (and its defers) have returned, so a t.Cleanup delete would execute
// against a closed pool and silently leave agent_pd_% rows behind. Deferred funcs run
// LIFO, so registering this AFTER `defer cleanup()` makes it run BEFORE the pool closes.
// setupRoomTestServer's own sweep covers rooms/messages/presence under 'test-%' but only
// deletes agents matching 'agent_roomtest_%', so the agent rows are ours to clear.
func pdCleanup(t *testing.T, pool *db.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		"DELETE FROM messages WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-pd-%')",
		"DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-pd-%')",
		"DELETE FROM rooms WHERE slug LIKE 'test-pd-%'",
		"DELETE FROM agents WHERE id LIKE 'agent_pd_%'",
	}
	for _, q := range stmts {
		if _, err := pool.Exec(ctx, q); err != nil {
			// Never fail the test on cleanup, but never swallow it either: a silent
			// failure here is exactly how leaked fixtures go unnoticed.
			t.Logf("pdCleanup: %q failed: %v", q, err)
		}
	}
}

// TestProductDirection_AgentsConnectWithoutHumanAccount walks the defining journey:
// a planner agent self-registers, creates and owns a real room, an executor agent
// self-registers and joins that same room, and the two exchange messages whose
// authorship the server stamps. No human account, claim, or MCP server is involved.
func TestProductDirection_AgentsConnectWithoutHumanAccount(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer pdCleanup(t, pool) // LIFO: must run BEFORE cleanup() closes the pool
	roomPreCleanup(t, pool)

	// 1. Planner agent self-registers. Public endpoint, no Authorization header.
	plannerID, plannerKey := registerTestAgent(t, ts, pdAgentName("plan", 0))

	// The registered agent is unclaimed: no human owns it, and it is immediately usable.
	status, agentOut := doJSON(t, http.MethodGet, ts.URL+"/v1/agents/"+plannerID, "", "")
	require.Equal(t, http.StatusOK, status)
	agentData, _ := agentOut["data"].(map[string]any)
	require.Nil(t, agentData["human_id"], "a self-registered agent must need no human owner")

	// 2. Planner creates and owns a real room with its own key.
	slug, roomToken := createAgentOwnedRoom(t, ts, plannerKey)

	// 3. The ownerless room is still manageable: its creator holds an owner membership.
	status, membersOut := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug+"/members", plannerKey, "")
	require.Equal(t, http.StatusOK, status, "creator agent must be able to manage its room: %v", membersOut)
	members, _ := membersOut["data"].([]any)
	require.True(t, pdHasMemberWithRole(members, plannerID, "owner"),
		"creator agent %s must hold an owner membership, got %v", plannerID, members)

	// 4. A second, independently running agent self-registers and joins the SAME room
	//    with nothing but its own key. The shared room token is accepted as the invite.
	executorID, executorKey := registerTestAgent(t, ts, pdAgentName("exec", 0))
	status, executorRoomToken := handshake(t, ts.URL, slug, executorKey, roomToken)
	require.Equal(t, http.StatusCreated, status)
	require.True(t, strings.HasPrefix(executorRoomToken, "solvr_rt_"),
		"executor must get its OWN per-agent room token, got %q", executorRoomToken)

	// The planner takes its own per-agent token the same way — credentials are per agent,
	// never shared between them.
	status, plannerRoomToken := handshake(t, ts.URL, slug, plannerKey, roomToken)
	require.Equal(t, http.StatusCreated, status)
	require.NotEqual(t, plannerRoomToken, executorRoomToken, "each agent must hold a distinct credential")

	// 5. Both announce presence and collaborate on the one room timeline.
	pdJoin(t, ts.URL, slug, plannerRoomToken, "planner")
	pdJoin(t, ts.URL, slug, executorRoomToken, "executor")

	status, plannerMsg := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", plannerRoomToken,
		`{"agent_name":"planner","content":"Step 1: build the thing."}`)
	require.Equal(t, http.StatusCreated, status, "%v", plannerMsg)
	plannerMsgData, _ := plannerMsg["data"].(map[string]any)
	require.Equal(t, plannerID, plannerMsgData["author_id"], "authorship must be stamped by the server")

	status, executorMsg := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", executorRoomToken,
		`{"agent_name":"executor","content":"Step 1 done."}`)
	require.Equal(t, http.StatusCreated, status, "%v", executorMsg)
	executorMsgData, _ := executorMsg["data"].(map[string]any)
	require.Equal(t, executorID, executorMsgData["author_id"], "authorship must be stamped by the server")

	// 6. Each agent reads the shared conversation, including the other's message.
	for name, tok := range map[string]string{"planner": plannerRoomToken, "executor": executorRoomToken} {
		status, listed := doJSON(t, http.MethodGet, ts.URL+"/r/"+slug+"/messages", tok, "")
		require.Equal(t, http.StatusOK, status, "%s must read the room timeline", name)
		msgs, _ := listed["data"].([]any)
		require.True(t, pdHasMessage(msgs, "Step 1: build the thing."), "%s missing planner message", name)
		require.True(t, pdHasMessage(msgs, "Step 1 done."), "%s missing executor message", name)
	}

	// 7. The collaboration is publicly observable with no credential at all.
	status, public := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, "", "")
	require.Equal(t, http.StatusOK, status, "a public room must be readable logged out")
	publicData, _ := public["data"].(map[string]any)
	publicMsgs, _ := publicData["recent_messages"].([]any)
	require.True(t, pdHasMessage(publicMsgs, "Step 1 done."), "public view must show the exchange")
}

// TestProductDirection_NoHardCodedParticipantLimit puts nine independently registered
// agents in one room. Nine clears both the two-agent shape of the planner/executor
// example and any eight-agent ceiling: the planner/executor pair is the default example,
// not the product limit.
func TestProductDirection_NoHardCodedParticipantLimit(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer pdCleanup(t, pool) // LIFO: must run BEFORE cleanup() closes the pool
	roomPreCleanup(t, pool)

	const participants = 9

	// The first agent creates the room; the rest join it exactly like any other agent.
	hostID, hostKey := registerTestAgent(t, ts, pdAgentName("n", 0))
	slug, roomToken := createAgentOwnedRoom(t, ts, hostKey)

	agentIDs := []string{hostID}
	keys := []string{hostKey}
	for i := 1; i < participants; i++ {
		id, key := registerTestAgent(t, ts, pdAgentName("n", i))
		agentIDs = append(agentIDs, id)
		keys = append(keys, key)
	}

	for i, key := range keys {
		name := fmt.Sprintf("worker_%d", i)

		status, roomTok := handshake(t, ts.URL, slug, key, roomToken)
		require.Equal(t, http.StatusCreated, status,
			"participant %d (%s) must be admitted — no hard-coded participant cap", i, agentIDs[i])
		require.True(t, strings.HasPrefix(roomTok, "solvr_rt_"))

		pdJoin(t, ts.URL, slug, roomTok, name)

		status, posted := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", roomTok,
			fmt.Sprintf(`{"agent_name":%q,"content":"report from %s"}`, name, name))
		require.Equal(t, http.StatusCreated, status, "participant %d must be able to post: %v", i, posted)
		postedData, _ := posted["data"].(map[string]any)
		require.Equal(t, agentIDs[i], postedData["author_id"])
	}

	// Membership, presence and the timeline all hold every participant.
	status, membersOut := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug+"/members", hostKey, "")
	require.Equal(t, http.StatusOK, status)
	members, _ := membersOut["data"].([]any)
	require.Len(t, members, participants, "every participant must hold a membership row")
	for _, id := range agentIDs {
		require.True(t, pdHasMember(members, id), "agent %s missing from the membership collection", id)
	}

	status, presenceOut := doJSON(t, http.MethodGet, ts.URL+"/r/"+slug+"/agents", roomToken, "")
	require.Equal(t, http.StatusOK, status)
	present, _ := presenceOut["data"].([]any)
	require.Len(t, present, participants, "every participant must be visible in presence")

	status, listed := doJSON(t, http.MethodGet, ts.URL+"/r/"+slug+"/messages?limit=100", roomToken, "")
	require.Equal(t, http.StatusOK, status)
	msgs, _ := listed["data"].([]any)
	for i := 0; i < participants; i++ {
		require.True(t, pdHasMessage(msgs, fmt.Sprintf("report from worker_%d", i)),
			"timeline is missing participant %d's message", i)
	}
}

// pdJoin announces presence on /r/{slug}/join using a per-agent room token.
func pdJoin(t *testing.T, baseURL, slug, roomToken, agentName string) {
	t.Helper()
	status, out := doJSON(t, http.MethodPost, baseURL+"/r/"+slug+"/join", roomToken,
		fmt.Sprintf(`{"agent_name":%q}`, agentName))
	require.Equal(t, http.StatusOK, status, "%s failed to join: %v", agentName, out)
}

// pdHasMember reports whether agentID appears in a members payload.
func pdHasMember(members []any, agentID string) bool {
	for _, m := range members {
		entry, ok := m.(map[string]any)
		if ok && entry["agent_id"] == agentID {
			return true
		}
	}
	return false
}

// pdHasMemberWithRole reports whether agentID appears with the given role.
func pdHasMemberWithRole(members []any, agentID, role string) bool {
	for _, m := range members {
		entry, ok := m.(map[string]any)
		if ok && entry["agent_id"] == agentID && entry["role"] == role {
			return true
		}
	}
	return false
}

// pdHasMessage reports whether a message with the exact content is in the payload.
func pdHasMessage(messages []any, content string) bool {
	for _, m := range messages {
		entry, ok := m.(map[string]any)
		if ok && entry["content"] == content {
			return true
		}
	}
	return false
}
