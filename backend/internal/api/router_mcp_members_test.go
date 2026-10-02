package api

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78: through POST /v1/mcp alone, the owner of a private room admits a third and a fourth
// agent to the same room (solvr_room_add_member) and reads who is in it (solvr_room_members);
// every admitted agent then joins with its own key and works the room. No second room is made.
func TestMCP_OwnerAdmitsAThirdAndFourthAgentToTheSameRoomThroughTheRealRouter(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()
	base := ts.URL

	type agent struct{ id, key, token string }
	roles := []string{"planner", "executor", "reviewer", "tester"}
	agents := map[string]*agent{}
	for _, role := range roles {
		id, key := registerRoomTestAgent(t, ts)
		agents[role] = &agent{id: id, key: key}
	}
	planner, executor, reviewer, tester := agents["planner"], agents["executor"], agents["reviewer"], agents["tester"]

	slug := fmt.Sprintf("test-mcp-members-%d", time.Now().UnixNano())
	created := mcpOK(t, base, planner.key, "solvr_room_create", map[string]any{"display_name": "MCP members", "slug": slug, "is_private": true})
	assert.Contains(t, created, "Room "+slug+" created (private)")

	text, isError := mcpHTTP(t, base, executor.key, "solvr_room_join", map[string]any{"slug": slug})
	require.True(t, isError, "a closed room refuses an agent its owner has not admitted: %s", text)
	assert.Contains(t, text, "API request failed: 403 Forbidden: FORBIDDEN")
	assert.Regexp(t, `request id: \S+`, text)

	for _, a := range []*agent{executor, reviewer} {
		added := mcpOK(t, base, planner.key, "solvr_room_add_member", map[string]any{"slug": slug, "agent_id": a.id})
		assert.Equal(t, a.id+" is in "+slug+" as member (added by "+planner.id+")\nIt joins with its own API key: solvr_room_join with slug "+slug+".", added)
	}
	three := mcpOK(t, base, planner.key, "solvr_room_members", map[string]any{"slug": slug})
	again := mcpOK(t, base, planner.key, "solvr_room_add_member", map[string]any{"slug": slug, "agent_id": reviewer.id})
	assert.Contains(t, again, reviewer.id+" is in "+slug+" as member")
	assert.Equal(t, three, mcpOK(t, base, planner.key, "solvr_room_members", map[string]any{"slug": slug}), "re-adding a participant changes nothing")

	mcpOK(t, base, planner.key, "solvr_room_add_member", map[string]any{"slug": slug, "agent_id": tester.id, "role": "member"})
	members := mcpOK(t, base, planner.key, "solvr_room_members", map[string]any{"slug": slug})
	lines := strings.Split(members, "\n")
	require.Len(t, lines, 5, members)
	assert.Equal(t, "4 participants of "+slug+":", lines[0])
	assert.True(t, strings.HasPrefix(lines[1], planner.id+" owner (added by "), "the owner is listed first: %s", members)
	for i, a := range []*agent{executor, reviewer, tester} {
		assert.True(t, strings.HasPrefix(lines[i+2], a.id+" member (added by "+planner.id+", since "), "in admission order: %s", members)
	}

	tokens := map[string]bool{}
	for _, role := range roles {
		a := agents[role]
		joined := mcpOK(t, base, a.key, "solvr_room_join", map[string]any{"slug": slug})
		a.token = mcpCapture(t, joined, `Room token: (\S+)`)
		tokens[a.token] = true
	}
	assert.Len(t, tokens, 4, "each participant holds its own room token")

	plan := mcpOK(t, base, "", "solvr_room_send", map[string]any{"slug": slug, "body": "Plan: build, review, test.",
		"room_token": planner.token, "addressed_member_ids": []string{executor.id, reviewer.id, tester.id}})
	planID := mcpCapture(t, plan, `Message (\d+) sent`)
	for _, a := range []*agent{executor, reviewer, tester} {
		mcpOK(t, base, "", "solvr_room_send", map[string]any{"slug": slug, "body": "On it: " + a.id, "room_token": a.token, "reply_to_entry_id": planID})
	}
	for _, role := range roles {
		read := mcpOK(t, base, "", "solvr_room_read", map[string]any{"slug": slug, "room_token": agents[role].token, "kind": "message"})
		assert.Len(t, strings.Split(read, "\n"), 4, "%s reads the four messages: %s", role, read)
		assert.Contains(t, read, "(id "+planID+") "+planner.id+" (to "+executor.id+", "+reviewer.id+", "+tester.id+"): Plan: build, review, test.", role)
		for _, a := range []*agent{executor, reviewer, tester} {
			assert.Contains(t, read, a.id+" (reply to "+planID+"): On it: "+a.id, role)
		}
	}

	// The API decides who may list or admit, which agents exist and that an owner remains.
	for _, refused := range []struct {
		key, tool, want string
		args            map[string]any
	}{
		{tester.key, "solvr_room_members", "403 Forbidden: FORBIDDEN", map[string]any{"slug": slug}},
		{tester.key, "solvr_room_add_member", "403 Forbidden: FORBIDDEN", map[string]any{"slug": slug, "agent_id": tester.id, "role": "owner"}},
		{planner.key, "solvr_room_add_member", "400 Bad Request: INVALID_AGENT", map[string]any{"slug": slug, "agent_id": "agent_roomtest_missing_" + slug}},
		{planner.key, "solvr_room_add_member", "409 Conflict: LAST_OWNER", map[string]any{"slug": slug, "agent_id": planner.id, "role": "member"}},
		{"", "solvr_room_members", "401 Unauthorized", map[string]any{"slug": slug}},
	} {
		text, isError := mcpHTTP(t, base, refused.key, refused.tool, refused.args)
		assert.True(t, isError, "%s %v: %s", refused.tool, refused.args, text)
		assert.Contains(t, text, "Error executing "+refused.tool+": API request failed: "+refused.want)
		assert.Regexp(t, `request id: \S+`, text)
	}
	assert.Equal(t, members, mcpOK(t, base, planner.key, "solvr_room_members", map[string]any{"slug": slug}), "the refusals changed no participant")
}
