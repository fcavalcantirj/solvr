package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task "Offer one canonical room API with thin adapters for the existing transport
// routes", step 4: the join prompt from GET /v1/rooms/{slug}/connect teaches the
// canonical entries contract, and following it literally works end to end with the
// agent's own room token — including the recovery it teaches (handshake again, resend
// with the same client_entry_id).
func TestRoomConnectPrompt_CanonicalEntriesFlowWorksAsTaught(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerKey := registerRoomTestAgent(t, ts)
	slug := entriesTestRoom(t, ts.URL, ownerKey, false)

	status, out := doJSON(t, "GET", ts.URL+"/v1/rooms/"+slug+"/connect?role=executor", "", "")
	require.Equal(t, http.StatusOK, status, "connect prompt: %v", out)
	promptObj, _ := entryData(t, out)["prompt"].(map[string]any)
	prompt, _ := promptObj["text"].(string)
	require.Contains(t, prompt, "Learn Solvr from https://solvr.dev/skill.md. Join the public Solvr room")
	require.Contains(t, prompt, "https://solvr.dev/rooms/"+slug+" as the EXECUTOR")
	// The skill's Join a room recipe is the canonical entries contract it teaches.
	join := skillHTTPBlock(t, "Join a room")
	require.Contains(t, join, "POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries")
	require.Contains(t, join, "GET https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries")
	require.NotContains(t, join, "/message")

	// Step by step, as the prompt teaches: own key -> handshake -> join -> post entry.
	executorID, executorKey := registerRoomTestAgent(t, ts)
	tok := handshakeRoomToken(t, ts, slug, executorKey)
	status, out = doJSON(t, "POST", ts.URL+"/r/"+slug+"/join", tok, `{"agent_name":"executor"}`)
	require.Equal(t, http.StatusOK, status, "join: %v", out)

	post := `{"body": "my plan", "client_entry_id": "executor-plan-1"}`
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", tok, post)
	require.Equal(t, http.StatusCreated, status, "first post: %v", out)
	first := entryID(t, out)

	// Recovery: a lost room token is replaced by handshaking again, and a post whose
	// outcome was unknown is resent with the same client_entry_id — stored once.
	tok2 := handshakeRoomToken(t, ts, slug, executorKey)
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", tok2, post)
	require.Equal(t, http.StatusOK, status, "resend: %v", out)
	assert.Equal(t, first, entryID(t, out))
	meta, _ := out["meta"].(map[string]any)
	assert.Equal(t, true, meta["idempotent_replay"])

	// Reading as taught: the one stored entry, attributed to the executor's identity.
	status, out = doJSON(t, "GET", ts.URL+"/v1/rooms/"+slug+"/entries", tok2, "")
	require.Equal(t, http.StatusOK, status, "read: %v", out)
	rows, _ := out["data"].([]any)
	var mine int
	for _, raw := range rows {
		row := raw.(map[string]any)
		if strings.Contains(row["body"].(string), "my plan") {
			mine++
			assert.Equal(t, "agent", row["author_type"])
			assert.Equal(t, executorID, row["author_id"])
		}
	}
	assert.Equal(t, 1, mine, "exactly one stored entry for the idempotent resend")
	_, hasCursor := out["meta"].(map[string]any)["next_cursor"]
	assert.True(t, hasCursor, "the read response carries meta.next_cursor as the prompt says")
}
