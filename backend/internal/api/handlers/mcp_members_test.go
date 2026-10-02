package handlers

import (
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78: /v1/mcp reads and grows a room's participants, as the npm mcp-server does
// (solvr_room_members and solvr_room_add_member): both present the caller's API key, never a
// room token, and the API decides who may list or admit (the owner) and which roles exist.

// mcpMemberRows are the participants the stub API answers, in the API's order (owners first,
// then by admission): sorting them by agent id would change it.
var mcpMemberRows = []interface{}{
	map[string]interface{}{"room_id": "r-1", "agent_id": "agent_planner", "role": "owner", "added_by": "system", "created_at": "2026-10-02T10:00:00Z"},
	map[string]interface{}{"room_id": "r-1", "agent_id": "zeta_executor", "role": "member", "added_by": "agent_planner", "created_at": "2026-10-02T10:01:00Z"},
	map[string]interface{}{"room_id": "r-1", "agent_id": "alpha_reviewer", "role": "member", "added_by": "agent_planner", "created_at": "2026-10-02T10:02:00Z"},
}

// mcpRoomError answers a dispatched request with a room error envelope and a request id.
func mcpRoomError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("X-Request-ID", "req-members-9")
	writeMCPJSON(w, status, map[string]interface{}{"error": map[string]string{"code": code, "message": message}})
}

func TestMCPRoomMembers_ListsTheParticipantsInTheAPIsOrderWithTheCallersKey(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": mcpMemberRows})
	})
	text, isError := callMCP(t, h, "solvr_room_members", map[string]interface{}{"slug": "a b/c"}, mcpTestKey)
	require.False(t, isError, text)
	assert.Equal(t, strings.Join([]string{
		"3 participants of a b/c:",
		"agent_planner owner (added by system, since 2026-10-02T10:00:00Z)",
		"zeta_executor member (added by agent_planner, since 2026-10-02T10:01:00Z)",
		"alpha_reviewer member (added by agent_planner, since 2026-10-02T10:02:00Z)",
	}, "\n"), text)

	sent := rec.requests()
	require.Len(t, sent, 1)
	assert.Equal(t, http.MethodGet, sent[0].method)
	assert.Equal(t, "/v1/rooms/a%20b%2Fc/members", sent[0].path)
	assert.Equal(t, mcpTestKey, sent[0].auth)
	assert.Empty(t, sent[0].query)
	assert.Empty(t, sent[0].body)
}

func TestMCPRoomMemberTools_PresentTheCallersKeyNeverARoomToken(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": mcpMemberRows})
			return
		}
		writeMCPJSON(w, http.StatusCreated, map[string]interface{}{"data": mcpMemberRows[1]})
	})
	_, isError := callMCP(t, h, "solvr_room_members", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x"}, mcpTestKey)
	require.False(t, isError)
	_, isError = callMCP(t, h, "solvr_room_add_member", map[string]interface{}{"slug": "demo", "agent_id": "zeta_executor", "room_token": "solvr_rt_x"}, mcpTestKey)
	require.False(t, isError)
	sent := rec.requests()
	require.Len(t, sent, 2)
	for _, s := range sent {
		assert.Equal(t, mcpTestKey, s.auth, "%s %s", s.method, s.path)
	}
	assert.JSONEq(t, `{"agent_id":"zeta_executor"}`, sent[1].body, "a room_token argument is not part of the request")
}

func TestMCPRoomMemberTools_WithoutAKeySendNoneAndReportTheAPIsRefusal(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		mcpRoomError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	})
	for _, call := range []struct {
		tool string
		args map[string]interface{}
	}{
		{"solvr_room_members", map[string]interface{}{"slug": "demo"}},
		{"solvr_room_add_member", map[string]interface{}{"slug": "demo", "agent_id": "zeta_executor"}},
	} {
		text, isError := callMCP(t, h, call.tool, call.args, "")
		assert.True(t, isError, call.tool)
		assert.Equal(t, "Error executing "+call.tool+": API request failed: 401 Unauthorized: UNAUTHORIZED: authentication required\nrequest id: req-members-9", text)
	}
	sent := rec.requests()
	require.Len(t, sent, 2)
	for _, s := range sent {
		assert.Empty(t, s.auth)
	}
}

func TestMCPRoomAddMember_SendsOnlyTheGivenFieldsAndNamesTheJoin(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusCreated, map[string]interface{}{"data": map[string]interface{}{
			"room_id": "r-1", "agent_id": "zeta_executor", "role": "member", "added_by": "agent_planner", "created_at": "2026-10-02T10:01:00Z"}})
	})
	text, isError := callMCP(t, h, "solvr_room_add_member", map[string]interface{}{"slug": "demo", "agent_id": "zeta_executor"}, mcpTestKey)
	require.False(t, isError, text)
	assert.Equal(t, "zeta_executor is in demo as member (added by agent_planner)\nIt joins with its own API key: solvr_room_join with slug demo.", text)

	callMCP(t, h, "solvr_room_add_member", map[string]interface{}{"slug": "a b/c", "agent_id": "zeta_executor", "role": "owner"}, mcpTestKey)
	sent := rec.requests()
	require.Len(t, sent, 2)
	assert.Equal(t, http.MethodPost, sent[0].method)
	assert.Equal(t, "/v1/rooms/demo/members", sent[0].path)
	assert.Equal(t, `{"agent_id":"zeta_executor"}`, strings.TrimSpace(sent[0].body), "no role is sent when none was given")
	assert.Equal(t, "application/json", sent[0].header.Get("Content-Type"))
	assert.Empty(t, sent[0].header.Get("Idempotency-Key"))
	assert.Equal(t, "/v1/rooms/a%20b%2Fc/members", sent[1].path)
	assert.JSONEq(t, `{"agent_id":"zeta_executor","role":"owner"}`, sent[1].body)
}

// The tool does not decide which roles exist: an unknown role reaches the API, which refuses it.
func TestMCPRoomAddMember_LeavesTheRoleToTheAPI(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		mcpRoomError(w, http.StatusBadRequest, "VALIDATION_ERROR", "role must be 'member' or 'owner'")
	})
	text, isError := callMCP(t, h, "solvr_room_add_member", map[string]interface{}{"slug": "demo", "agent_id": "zeta_executor", "role": "admin"}, mcpTestKey)
	assert.True(t, isError)
	assert.Contains(t, text, "400 Bad Request: VALIDATION_ERROR: role must be 'member' or 'owner'")
	sent := rec.requests()
	require.Len(t, sent, 1)
	assert.JSONEq(t, `{"agent_id":"zeta_executor","role":"admin"}`, sent[0].body)
}

func TestMCPRoomMemberTools_ReportEachAPIErrorWithItsRequestID(t *testing.T) {
	for _, tc := range []struct {
		tool, code, message, want string
		status                    int
		args                      map[string]interface{}
	}{
		{"solvr_room_add_member", "INVALID_AGENT", "agent_id does not reference an existing agent", "400 Bad Request", http.StatusBadRequest,
			map[string]interface{}{"slug": "demo", "agent_id": "agent_missing"}},
		{"solvr_room_add_member", "FORBIDDEN", "only the room owner can manage members", "403 Forbidden", http.StatusForbidden,
			map[string]interface{}{"slug": "demo", "agent_id": "zeta_executor", "role": "owner"}},
		{"solvr_room_add_member", "LAST_OWNER", "a room must keep at least one owner; add another owner first or delete the room", "409 Conflict", http.StatusConflict,
			map[string]interface{}{"slug": "demo", "agent_id": "agent_planner", "role": "member"}},
		{"solvr_room_members", "FORBIDDEN", "only the room owner can manage members", "403 Forbidden", http.StatusForbidden,
			map[string]interface{}{"slug": "demo"}},
	} {
		h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) { mcpRoomError(w, tc.status, tc.code, tc.message) })
		text, isError := callMCP(t, h, tc.tool, tc.args, mcpTestKey)
		assert.True(t, isError, tc.code)
		assert.Equal(t, "Error executing "+tc.tool+": API request failed: "+tc.want+": "+tc.code+": "+tc.message+"\nrequest id: req-members-9", text)
		assert.Len(t, rec.requests(), 1, "%s %s is sent once", tc.tool, tc.code)
	}
}

func TestMCPRoomMemberTools_RequireTheirArgumentsBeforeAnyRequest(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for _, tc := range []struct {
		tool, missing string
		args          map[string]interface{}
	}{
		{"solvr_room_members", "slug", map[string]interface{}{}},
		{"solvr_room_add_member", "slug", map[string]interface{}{"agent_id": "zeta_executor"}},
		{"solvr_room_add_member", "agent_id", map[string]interface{}{"slug": "demo"}},
		{"solvr_room_add_member", "agent_id", map[string]interface{}{"slug": "demo", "agent_id": ""}},
	} {
		text, isError := callMCP(t, h, tc.tool, tc.args, mcpTestKey)
		assert.True(t, isError, tc.tool)
		assert.Equal(t, "Error executing "+tc.tool+": "+tc.missing+" is required", text)
	}
	assert.Empty(t, rec.requests(), "nothing is sent without a required argument")
}

func TestMCPRoomMemberTools_DeclareTheirArguments(t *testing.T) {
	schemas, names := mcpToolSchemas(t)
	join := -1
	for i, name := range names {
		if name == "solvr_room_join" {
			join = i
		}
	}
	require.GreaterOrEqual(t, join, 0)
	require.Greater(t, len(names), join+2)
	assert.Equal(t, []string{"solvr_room_members", "solvr_room_add_member"}, names[join+1:join+3], "listed right after solvr_room_join")
	require.Contains(t, schemas, "solvr_room_members")
	require.Contains(t, schemas, "solvr_room_add_member")

	members := schemas["solvr_room_members"]
	assert.Equal(t, []string{"slug"}, schemaKeys(members))
	assert.Equal(t, []string{"slug"}, schemaRequired(members))

	add := schemas["solvr_room_add_member"]
	assert.Equal(t, []string{"agent_id", "role", "slug"}, schemaKeys(add))
	assert.Equal(t, []string{"agent_id", "slug"}, schemaRequired(add))
	role := add["properties"].(map[string]interface{})["role"].(map[string]interface{})
	assert.Equal(t, []interface{}{"owner", "member"}, role["enum"])
}

// Both first-party MCP servers name the member operations' tools alike. They are not contract
// operations yet (contract/openapi-examples.json has no listRoomMembers or addRoomMember
// example), so neither server lists them among its operation tools.
func TestMCPMemberTools_MatchTheNpmServer(t *testing.T) {
	src, err := os.ReadFile("../../../../mcp-server/src/tools.ts")
	require.NoError(t, err)
	block := regexp.MustCompile(`(?s)export const MEMBER_TOOLS[^{]*\{(.*?)\n\};`).FindSubmatch(src)
	require.NotNil(t, block, "mcp-server/src/tools.ts has no MEMBER_TOOLS")
	npm := map[string]string{}
	for _, m := range regexp.MustCompile(`(\w+): '(solvr_\w+)'`).FindAllStringSubmatch(string(block[1]), -1) {
		npm[m[1]] = m[2]
	}
	want := map[string]string{"listRoomMembers": "solvr_room_members", "addRoomMember": "solvr_room_add_member"}
	assert.True(t, reflect.DeepEqual(want, MCPMemberTools), "/v1/mcp %v", MCPMemberTools)
	assert.True(t, reflect.DeepEqual(npm, MCPMemberTools), "npm %v, /v1/mcp %v", npm, MCPMemberTools)

	served := MCPToolNames()
	for operation, tool := range MCPMemberTools {
		assert.Contains(t, served, tool)
		_, isOperationTool := MCPOperationTools[operation]
		assert.False(t, isOperationTool, "%s is not a contract operation yet", operation)
	}
}
