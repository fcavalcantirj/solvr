package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// idx 78 step 7: a room's participants and their roles are a published collection. The
// membership operations (list, admit, remove, revoke one participant's token) are in the same
// OpenAPI document as the other room operations, the member schemas are pinned to what the
// handlers serialize, and a third and fourth agent join and work in the same room as the first
// two, with timeline entries addressed to several of them at once, without another room.

var roomMemberOperations = []struct{ method, path, id string }{
	{"get", "/rooms/{slug}/members", "listRoomMembers"},
	{"post", "/rooms/{slug}/members", "addRoomMember"},
	{"delete", "/rooms/{slug}/members/{agent_id}", "removeRoomMember"},
	{"delete", "/rooms/{slug}/members/{agent_id}/token", "revokeRoomMemberToken"},
}

func TestOpenAPIMembers_MembershipOperationsArePublished(t *testing.T) {
	spec := servedSpec(t)
	served := servedRoutes(t)
	names := map[string]string{
		"400": "BadRequest", "401": "Unauthorized", "403": "Forbidden", "404": "NotFound", "409": "Conflict",
	}
	rows := map[string][]string{
		"listRoomMembers":       {"401", "403", "404"},
		"addRoomMember":         {"400", "401", "403", "404", "409"},
		"removeRoomMember":      {"401", "403", "404", "409"},
		"revokeRoomMemberToken": {"401", "403", "404"},
	}
	for _, o := range roomMemberOperations {
		key := o.method + " " + o.path
		op := operation(t, spec, o.method, o.path)
		assert.Equal(t, o.id, op["operationId"], key)
		assert.Contains(t, op["tags"], "Rooms", key)
		assert.True(t, served[strings.ToUpper(o.method)+" /v1"+o.path], "%s is documented but not served", key)

		sec, _ := op["security"].([]interface{})
		require.Len(t, sec, 1, "%s requires a credential", key)
		assert.NotEmpty(t, sec[0], "%s requires a credential", key)
		assert.Contains(t, op["description"], "owner", "%s: says who may call it", key)

		for _, status := range rows[o.id] {
			assert.Equal(t, "#/components/responses/"+names[status], refName(at(t, op, "responses", status)), "%s: %s", key, status)
		}
		// Demoting or removing the last owner is the conflict these writes answer; their shared
		// 409 row names it, as it names the handshake's TOKEN_LIMIT_REACHED.
		if conflict, ok := op["responses"].(map[string]interface{})["409"]; ok {
			assert.Contains(t, deref(t, spec, conflict).(map[string]interface{})["description"], "LAST_OWNER",
				"%s: the 409 row names LAST_OWNER", key)
		}
		declared := map[string]bool{}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			id := p["in"].(string) + ":" + p["name"].(string)
			assert.False(t, declared[id], "%s declares parameter %s twice", key, id)
			declared[id] = true
		}
		assert.True(t, declared["path:slug"], "%s: slug", key)
		if strings.Contains(o.path, "{agent_id}") {
			assert.True(t, declared["path:agent_id"], "%s: agent_id", key)
		}
		for status, raw := range op["responses"].(map[string]interface{}) {
			assert.NotEmpty(t, deref(t, spec, raw).(map[string]interface{})["description"], "%s: response %s", key, status)
		}
	}

	// A retried admission or removal replays its first result (router_rooms.go Idempotency).
	for _, method := range []string{"post", "delete"} {
		path := map[string]string{"post": "/rooms/{slug}/members", "delete": "/rooms/{slug}/members/{agent_id}"}[method]
		found := false
		for _, raw := range operation(t, spec, method, path)["parameters"].([]interface{}) {
			found = found || refName(raw) == "#/components/parameters/IdempotencyKey"
		}
		assert.True(t, found, "%s %s takes Idempotency-Key", method, path)
	}

	list := operation(t, spec, "get", "/rooms/{slug}/members")
	assert.Equal(t, "#/components/schemas/RoomMemberList", refName(at(t, list, "responses", "200", "content", "application/json", "schema")))
	add := operation(t, spec, "post", "/rooms/{slug}/members")
	assert.Equal(t, "#/components/schemas/RoomMemberResponse", refName(at(t, add, "responses", "201", "content", "application/json", "schema")))
	assert.Equal(t, "#/components/schemas/AddRoomMemberRequest", refName(at(t, add, "requestBody", "content", "application/json", "schema")))
	for _, path := range []string{"/rooms/{slug}/members/{agent_id}", "/rooms/{slug}/members/{agent_id}/token"} {
		assert.NotNil(t, at(t, operation(t, spec, "delete", path), "responses", "204"), "delete %s answers 204", path)
	}

	// The member schemas describe the JSON the handlers serialize.
	sameSet(t, "RoomMember properties", propertyNames(t, spec, "RoomMember"), jsonFields(reflect.TypeOf(models.RoomMember{})))
	member := deref(t, spec, ref("schemas", "RoomMember")).(map[string]interface{})
	assert.ElementsMatch(t, []interface{}{models.RoleOwner, models.RoleMember}, at(t, member, "properties", "role", "enum"))
	request := deref(t, spec, ref("schemas", "AddRoomMemberRequest")).(map[string]interface{})
	sameSet(t, "AddRoomMemberRequest properties", propertyNames(t, spec, "AddRoomMemberRequest"), []string{"agent_id", "role"})
	assert.Equal(t, []interface{}{"agent_id"}, request["required"])
	assert.ElementsMatch(t, []interface{}{models.RoleOwner, models.RoleMember}, at(t, request, "properties", "role", "enum"))
	members := deref(t, spec, ref("schemas", "RoomMemberList")).(map[string]interface{})
	assert.Equal(t, "array", at(t, members, "properties", "data", "type"))
	assert.Equal(t, "#/components/schemas/RoomMember", refName(at(t, members, "properties", "data", "items")))
	assert.Contains(t, at(t, deref(t, spec, ref("schemas", "PostEntryRequest")).(map[string]interface{}),
		"properties", "addressed_member_ids", "description"), "agent_id", "the recipients are members, named by agent_id")
}

// A planner opens a public room; an executor joins it by itself; the planner admits a reviewer
// and a tester. All four hold their own room token, write into the one timeline, and one entry
// is addressed to three of them. The documented answers and errors are what the API answers.
func TestOpenAPIMembers_AThirdAndFourthAgentJoinAndWorkInTheSameRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	spec := servedSpec(t)

	run := time.Now().UnixNano() % 1000000000
	ids, keys := map[string]string{}, map[string]string{}
	roles := []string{"planner", "executor", "reviewer", "tester"}
	for _, role := range roles {
		ids[role], keys[role] = registerTestAgent(t, ts, fmt.Sprintf("roomtest_m%s_%d", role[:4], run%1000000))
	}
	slug := fmt.Sprintf("test-members-%d", run)
	base := ts.URL + "/v1/rooms/" + slug
	call := func(method, target, bearer string, body interface{}) (int, interface{}) {
		t.Helper()
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			require.NoError(t, err)
		}
		return sendMemberRequest(t, method, target, bearer, raw)
	}
	data := func(answer interface{}) map[string]interface{} {
		d, _ := answer.(map[string]interface{})["data"].(map[string]interface{})
		return d
	}

	status, answer := call("POST", ts.URL+"/v1/rooms", keys["planner"], map[string]interface{}{"slug": slug, "display_name": "Plan, build, review, test"})
	require.Equal(t, 201, status, "create: %v", answer)

	tokens := map[string]string{}
	handshake := func(role string) {
		t.Helper()
		status, answer := call("POST", base+"/handshake", keys[role], map[string]interface{}{})
		require.Equal(t, 201, status, "%s handshake: %v", role, answer)
		tokens[role], _ = data(answer)["room_token"].(string)
		require.NotEmpty(t, tokens[role])
	}
	handshake("executor") // joins by itself: a public room admits any agent

	for _, role := range []string{"reviewer", "tester"} {
		body := map[string]interface{}{"agent_id": ids[role]}
		if role == "reviewer" {
			body["role"] = models.RoleMember
		}
		status, answer := call("POST", base+"/members", keys["planner"], body)
		require.Equal(t, 201, status, "admit %s: %v", role, answer)
		assert.Empty(t, schemaProblems(spec, answer, ref("schemas", "RoomMemberResponse"), "addRoomMember answer"))
		assert.Equal(t, ids[role], data(answer)["agent_id"])
		assert.Equal(t, models.RoleMember, data(answer)["role"])
	}

	status, answer = call("GET", base+"/members", keys["planner"], nil)
	require.Equal(t, 200, status, "list: %v", answer)
	assert.Empty(t, schemaProblems(spec, answer, ref("schemas", "RoomMemberList"), "listRoomMembers answer"))
	got := map[string]string{}
	for _, item := range answer.(map[string]interface{})["data"].([]interface{}) {
		m := item.(map[string]interface{})
		got[m["agent_id"].(string)] = m["role"].(string)
	}
	assert.Equal(t, map[string]string{
		ids["planner"]: models.RoleOwner, ids["executor"]: models.RoleMember,
		ids["reviewer"]: models.RoleMember, ids["tester"]: models.RoleMember,
	}, got, "the room's participants and their roles")

	for _, role := range []string{"planner", "reviewer", "tester"} {
		handshake(role)
	}
	assert.Len(t, map[string]bool{tokens["planner"]: true, tokens["executor"]: true, tokens["reviewer"]: true, tokens["tester"]: true}, 4,
		"each participant holds its own room token")

	sent := []string{}
	for _, role := range roles {
		body := map[string]interface{}{"body": role + " checking in"}
		if role == "reviewer" {
			body["addressed_member_ids"] = []string{ids["planner"], ids["executor"], ids["tester"]}
		}
		status, answer := call("POST", base+"/entries", tokens[role], body)
		require.Equal(t, 201, status, "%s entry: %v", role, answer)
		sent = append(sent, fmt.Sprint(data(answer)["id"]))
	}

	for _, reader := range roles {
		status, answer := call("GET", base+"/entries?kind=message", tokens[reader], nil)
		require.Equal(t, 200, status, "%s reads: %v", reader, answer)
		var order, authors []string
		for _, item := range answer.(map[string]interface{})["data"].([]interface{}) {
			e := item.(map[string]interface{})
			order = append(order, fmt.Sprint(e["id"]))
			authors = append(authors, e["author_id"].(string))
			if e["author_id"] == ids["reviewer"] {
				assert.Equal(t, []interface{}{ids["planner"], ids["executor"], ids["tester"]}, e["addressed_member_ids"],
					"%s sees the reviewer's entry addressed to three participants", reader)
			}
		}
		assert.Equal(t, sent, order, "%s reads every participant's entry in order", reader)
		assert.Equal(t, []string{ids["planner"], ids["executor"], ids["reviewer"], ids["tester"]}, authors, reader)
	}

	var rooms int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM room_members WHERE agent_id = $1 AND revoked_at IS NULL`, ids["planner"]).Scan(&rooms))
	assert.Equal(t, 1, rooms, "the planner is in one room: no other room was created for the third or fourth agent")

	// Documented failures: each answers a status the operation lists, in the error envelope.
	failures := []struct {
		what, op, method, target, bearer string
		body                             interface{}
		status                           int
		code                             string
	}{
		{"a participant who does not own the room lists its members", "listRoomMembers", "GET", base + "/members", keys["executor"], nil, 403, "FORBIDDEN"},
		{"a participant who does not own the room admits an agent", "addRoomMember", "POST", base + "/members", keys["reviewer"], map[string]interface{}{"agent_id": ids["executor"]}, 403, "FORBIDDEN"},
		{"the agent does not exist", "addRoomMember", "POST", base + "/members", keys["planner"], map[string]interface{}{"agent_id": "agent_no_such_agent_" + fmt.Sprint(run)}, 400, "INVALID_AGENT"},
		{"the role is neither owner nor member", "addRoomMember", "POST", base + "/members", keys["planner"], map[string]interface{}{"agent_id": ids["tester"], "role": "admin"}, 400, "VALIDATION_ERROR"},
		{"the only owner is demoted", "addRoomMember", "POST", base + "/members", keys["planner"], map[string]interface{}{"agent_id": ids["planner"], "role": "member"}, 409, "LAST_OWNER"},
		{"the only owner removes itself", "removeRoomMember", "DELETE", base + "/members/" + ids["planner"], keys["planner"], nil, 409, "LAST_OWNER"},
		{"the agent is not a member", "removeRoomMember", "DELETE", base + "/members/agent_not_here", keys["planner"], nil, 404, "NOT_FOUND"},
		{"the agent is not a member", "revokeRoomMemberToken", "DELETE", base + "/members/agent_not_here/token", keys["planner"], nil, 404, "NOT_FOUND"},
		{"the room does not exist", "listRoomMembers", "GET", ts.URL + "/v1/rooms/test-members-missing-" + fmt.Sprint(run) + "/members", keys["planner"], nil, 404, "NOT_FOUND"},
	}
	documented := map[string]map[string]interface{}{}
	for _, o := range roomMemberOperations {
		documented[o.id] = operation(t, spec, o.method, o.path)["responses"].(map[string]interface{})
	}
	for _, f := range failures {
		status, answer := call(f.method, f.target, f.bearer, f.body)
		require.Equal(t, f.status, status, "%s: %v", f.what, answer)
		assert.Equal(t, f.code, errorCode(answer), f.what)
		assert.Contains(t, documented[f.op], fmt.Sprint(f.status), "%s: %d is a documented row of %s", f.what, f.status, f.op)
		if row, ok := documented[f.op][fmt.Sprint(f.status)]; ok {
			assert.Contains(t, deref(t, spec, row).(map[string]interface{})["description"], f.code,
				"%s: the %d row of %s names %s", f.what, f.status, f.op, f.code)
		}
		assert.Empty(t, schemaProblems(spec, answer, ref("schemas", "Error"), f.what), f.what)
	}

	// Revoking one participant's token leaves it a member and its peers untouched; removing it
	// ends its access, and the other three keep working in the room.
	status, _ = call("DELETE", base+"/members/"+ids["reviewer"]+"/token", keys["planner"], nil)
	require.Equal(t, 204, status)
	status, _ = call("GET", base+"/entries", tokens["reviewer"], nil)
	assert.Equal(t, 401, status, "the revoked token stops working")
	handshake("reviewer")

	status, _ = call("DELETE", base+"/members/"+ids["tester"], keys["planner"], nil)
	require.Equal(t, 204, status)
	status, _ = call("POST", base+"/entries", tokens["tester"], map[string]interface{}{"body": "still here?"})
	assert.Equal(t, 401, status, "a removed participant's token stops working")
	for _, role := range []string{"planner", "executor", "reviewer"} {
		status, answer := call("POST", base+"/entries", tokens[role], map[string]interface{}{"body": role + " continues"})
		assert.Equal(t, 201, status, "%s keeps working after a peer left: %v", role, answer)
	}
	status, answer = call("GET", base+"/members", keys["planner"], nil)
	require.Equal(t, 200, status)
	left := []string{}
	for _, item := range answer.(map[string]interface{})["data"].([]interface{}) {
		left = append(left, item.(map[string]interface{})["agent_id"].(string))
	}
	assert.ElementsMatch(t, []string{ids["planner"], ids["executor"], ids["reviewer"]}, left)
}

// sendMemberRequest is sendExample for a route that may answer 204 with no body.
func sendMemberRequest(t *testing.T, method, target, bearer string, body []byte) (int, interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if resp.StatusCode == http.StatusNoContent {
		require.Empty(t, raw, "%s %s: 204 with a body", method, target)
		return resp.StatusCode, nil
	}
	var answer interface{}
	require.NoError(t, json.Unmarshal(raw, &answer), "%s %s: %s", method, target, raw)
	return resp.StatusCode, answer
}
