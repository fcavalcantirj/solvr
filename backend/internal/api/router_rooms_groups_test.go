package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// This file verifies task 22 — "Handle late joins, departures, review, and
// completion in groups" — end to end against a real database. Each test maps to a
// step of that task and exercises the composed room lifecycle (late join prompt,
// individual revocation, addressed review, done-does-not-archive, owner-only
// finish, and idempotent reconnect) rather than a single handler in isolation.

// postAgentMessage posts a message on /r/{slug}/message with a per-agent room token
// and returns the created message id. replyTo, when non-nil, sets reply_to_entry_id.
func postAgentMessage(t *testing.T, baseURL, slug, roomToken, agentName, content string, replyTo *int64) int64 {
	t.Helper()
	body := fmt.Sprintf(`{"agent_name":%q,"content":%q`, agentName, content)
	if replyTo != nil {
		body += fmt.Sprintf(`,"reply_to_entry_id":%d`, *replyTo)
	}
	body += "}"
	st, out := doJSON(t, "POST", baseURL+"/r/"+slug+"/message", roomToken, body)
	require.Equal(t, http.StatusCreated, st, "post message failed: %v", out)
	data, _ := out["data"].(map[string]any)
	idF, _ := data["id"].(float64)
	require.NotZero(t, idF, "expected a message id in the response")
	return int64(idF)
}

// listAgentMessages reads /r/{slug}/messages with a per-agent room token. When after
// is non-empty it is passed as ?after= so the caller resumes from its own cursor.
func listAgentMessages(t *testing.T, baseURL, slug, roomToken, after string) []map[string]any {
	t.Helper()
	url := baseURL + "/r/" + slug + "/messages"
	if after != "" {
		url += "?after=" + after
	}
	st, out := doJSON(t, "GET", url, roomToken, "")
	require.Equal(t, http.StatusOK, st)
	raw, _ := out["data"].([]any)
	var res []map[string]any
	for _, m := range raw {
		if mm, ok := m.(map[string]any); ok {
			res = append(res, mm)
		}
	}
	return res
}

// --- Step 1: a late joiner's prompt retrieves the task and recent context before acting. ---

func TestGroupLifecycle_LateJoinPromptRetrievesContextBeforeActing(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%grp%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public room

	// The planner opens the room with the task, then posts a later directive so
	// there is real "work already started" for a late joiner to catch up on.
	_, plannerKey := registerTestAgent(t, ts, uniqName("grpplanner"))
	stHS, plannerTok := handshake(t, ts.URL, slug, plannerKey, "")
	require.Equal(t, http.StatusCreated, stHS)
	task := "Build the account settings page"
	postAgentMessage(t, ts.URL, slug, plannerTok, "planner", task, nil)
	postAgentMessage(t, ts.URL, slug, plannerTok, "planner", "Directive: use the existing design tokens", nil)

	// A reviewer joining after work started asks the room-specific connect endpoint
	// for its join prompt (public read, no auth).
	st, out := doJSON(t, "GET", ts.URL+"/v1/rooms/"+slug+"/connect?role=reviewer", "", "")
	require.Equal(t, http.StatusOK, st)
	data, _ := out["data"].(map[string]any)
	require.Equal(t, task, data["task"], "connect envelope must expose the initial task (first message)")

	promptObj, _ := data["prompt"].(map[string]any)
	prompt, _ := promptObj["text"].(string)
	require.NotEmpty(t, prompt)
	// The sentence names the room and tells the joiner to read it; the skill's Join a room
	// recipe reads the history through the canonical entries contract (idx 70 step 4).
	require.Contains(t, prompt, "https://solvr.dev/rooms/"+slug+" as the REVIEWER, read it", "the sentence names the room and says to read it")
	require.Contains(t, skillHTTPBlock(t, "Join a room"), "GET https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries", "the skill tells the joiner how to retrieve the room history")
	st, hist := doJSON(t, "GET", ts.URL+"/v1/rooms/"+slug+"/entries?kind=message", "", "")
	require.Equal(t, http.StatusOK, st, "the taught history read must work: %v", hist)
	rows, _ := hist["data"].([]any)
	require.Len(t, rows, 2, "the taught history read returns the work already started")
	require.Equal(t, task, rows[0].(map[string]any)["body"])
	require.Contains(t, skillRooms(t), "before you post", "the skill tells the joiner to read context before acting")
}

// --- Step 3: a review addresses one specific result, not all room work. ---

func TestGroupLifecycle_ReviewAddressesSpecificResult(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%grp%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public room

	_, keyExec1 := registerTestAgent(t, ts, uniqName("grpexec1"))
	_, keyExec2 := registerTestAgent(t, ts, uniqName("grpexec2"))
	_, keyReviewer := registerTestAgent(t, ts, uniqName("grpreviewer"))
	_, tokExec1 := handshake(t, ts.URL, slug, keyExec1, "")
	_, tokExec2 := handshake(t, ts.URL, slug, keyExec2, "")
	_, tokReviewer := handshake(t, ts.URL, slug, keyReviewer, "")

	// Two executors post separate results.
	id1 := postAgentMessage(t, ts.URL, slug, tokExec1, "exec1", "Result A: implemented via approach one", nil)
	id2 := postAgentMessage(t, ts.URL, slug, tokExec2, "exec2", "Result B: implemented via approach two", nil)
	require.NotEqual(t, id1, id2)

	// The reviewer approves ONLY the second result by addressing that entry.
	postAgentMessage(t, ts.URL, slug, tokReviewer, "reviewer", "Approved — ship Result B", &id2)

	// The stored review message references exactly id2, so the approval is scoped to
	// a specific result rather than blanket-approving every message in the room.
	msgs := listAgentMessages(t, ts.URL, slug, tokReviewer, "")
	var review map[string]any
	for _, m := range msgs {
		if c, _ := m["content"].(string); c == "Approved — ship Result B" {
			review = m
		}
	}
	require.NotNil(t, review, "review message must be present in the timeline")
	replyF, ok := review["reply_to_entry_id"].(float64)
	require.True(t, ok, "review must carry a reply_to_entry_id identifying which result it concerns")
	require.Equal(t, id2, int64(replyF), "review must reference the second result, not all room work")
	require.NotEqual(t, id1, int64(replyF), "review must not reference the unrelated first result")
}

// --- Step 4: an agent saying "done" does not end the room; finishing is owner-only. ---

func TestGroupLifecycle_AgentDoneDoesNotEndRoomAndFinishIsOwnerOnly(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%grp%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public room, owned by the human

	agentA, keyA := registerTestAgent(t, ts, uniqName("grpdoneA"))
	_, keyB := registerTestAgent(t, ts, uniqName("grpdoneB"))
	_, tokA := handshake(t, ts.URL, slug, keyA, "")

	// Agent A declares it is finished. This is an ordinary message.
	postAgentMessage(t, ts.URL, slug, tokA, "agentA", "done", nil)

	// The room is NOT archived/ended by that message: it is still readable and still
	// accepts new work from another agent.
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, ""),
		"a 'done' message must not end the room")
	_, tokB := handshake(t, ts.URL, slug, keyB, "")
	postAgentMessage(t, ts.URL, slug, tokB, "agentB", "picking up the next piece", nil)

	// A non-owner participant cannot finish (delete) the shared room.
	stMember, _ := doJSON(t, "DELETE", ts.URL+"/v1/rooms/"+slug, keyA, "")
	require.Equal(t, http.StatusForbidden, stMember,
		"a participant agent must not be able to end the shared room")
	_ = agentA

	// Finishing requires the explicit owner-authorized action.
	stOwner, _ := doJSON(t, "DELETE", ts.URL+"/v1/rooms/"+slug, jwt, "")
	require.Equal(t, http.StatusNoContent, stOwner, "the owner may end the room")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/rooms/"+slug, ""),
		"an ended room no longer resolves publicly")
}

// --- Step 5: reconnecting an identity resumes its cursor and role without duplication. ---

func TestGroupLifecycle_ReconnectResumesCursorWithoutDuplicateMembership(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%grp%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public room; owner (JWT) can list members

	agentA, keyA := registerTestAgent(t, ts, uniqName("grprecA"))
	_, keyB := registerTestAgent(t, ts, uniqName("grprecB"))

	// Agent A joins and posts one message, establishing its cursor.
	_, tokA := handshake(t, ts.URL, slug, keyA, "")
	id1 := postAgentMessage(t, ts.URL, slug, tokA, "agentA", "message before disconnect", nil)

	require.Equal(t, 1, countMemberEntries(t, ts.URL, slug, jwt, agentA), "one membership after first join")

	// While A is away, agent B posts.
	_, tokB := handshake(t, ts.URL, slug, keyB, "")
	id2 := postAgentMessage(t, ts.URL, slug, tokB, "agentB", "message while A was gone", nil)

	// Agent A reconnects with the SAME identity: handshake is idempotent, so it is not
	// counted as a new participant and keeps its member role.
	stHS, tokA2 := handshake(t, ts.URL, slug, keyA, "")
	require.Equal(t, http.StatusCreated, stHS)
	require.Equal(t, 1, countMemberEntries(t, ts.URL, slug, jwt, agentA),
		"reconnecting the same identity must not create a duplicate membership")
	require.Equal(t, "member", memberRole(t, ts.URL, slug, jwt, agentA), "role must be preserved on reconnect")

	// Resuming from its last cursor returns only what it missed — no repeated work.
	missed := listAgentMessages(t, ts.URL, slug, tokA2, strconv.FormatInt(id1, 10))
	require.Len(t, missed, 1, "resuming after the cursor returns only the missed message")
	gotID, _ := missed[0]["id"].(float64)
	require.Equal(t, id2, int64(gotID), "resumed read returns the message posted after the cursor")
}

// --- Step 2: revoking one participant leaves the others working (public room). ---

func TestGroupLifecycle_RevokeOneParticipantOthersContinue(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%grp%'") }) //nolint:errcheck

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt) // public room

	agentA, keyA := registerTestAgent(t, ts, uniqName("grprevA"))
	_, keyB := registerTestAgent(t, ts, uniqName("grprevB"))
	_, tokA := handshake(t, ts.URL, slug, keyA, "")
	_, tokB := handshake(t, ts.URL, slug, keyB, "")
	postAgentMessage(t, ts.URL, slug, tokA, "agentA", "A working", nil)
	postAgentMessage(t, ts.URL, slug, tokB, "agentB", "B working", nil)

	// Owner revokes agent A only.
	stDel, _ := doJSON(t, "DELETE", ts.URL+"/v1/rooms/"+slug+"/members/"+agentA, jwt, "")
	require.Equal(t, http.StatusNoContent, stDel)

	// Agent A's per-agent token is dead; agent B is entirely unaffected and continues.
	stA, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", tokA, `{"agent_name":"agentA","content":"still here?"}`)
	require.Equal(t, http.StatusUnauthorized, stA, "revoked participant's token must stop working")
	postAgentMessage(t, ts.URL, slug, tokB, "agentB", "B keeps going", nil)
}

// countMemberEntries returns how many membership rows the room lists for agentID.
func countMemberEntries(t *testing.T, baseURL, slug, ownerJWT, agentID string) int {
	t.Helper()
	st, out := doJSON(t, "GET", baseURL+"/v1/rooms/"+slug+"/members", ownerJWT, "")
	require.Equal(t, http.StatusOK, st)
	raw, _ := out["data"].([]any)
	n := 0
	for _, m := range raw {
		mm, _ := m.(map[string]any)
		if id, _ := mm["agent_id"].(string); id == agentID {
			n++
		}
	}
	return n
}

// memberRole returns the role the room records for agentID (empty if absent).
func memberRole(t *testing.T, baseURL, slug, ownerJWT, agentID string) string {
	t.Helper()
	st, out := doJSON(t, "GET", baseURL+"/v1/rooms/"+slug+"/members", ownerJWT, "")
	require.Equal(t, http.StatusOK, st)
	raw, _ := out["data"].([]any)
	for _, m := range raw {
		mm, _ := m.(map[string]any)
		if id, _ := mm["agent_id"].(string); id == agentID {
			role, _ := mm["role"].(string)
			return role
		}
	}
	return ""
}
