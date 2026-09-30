package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Anti-abuse W2, T-M9 (D5a): an author cannot publish a post moderation has not approved by
// editing only its status. The edit sends it back to pending_review and through moderation.
func TestModeration_PatchStatusCannotPublishAnUnapprovedPost(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := gateAgent(t, ts, pool)

	for _, seed := range []struct{ status, publication, moderation string }{
		{"rejected", "draft", "rejected"},
		{"pending_review", "draft", "pending"},
	} {
		var postID string
		require.NoError(t, pool.QueryRow(context.Background(), `
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state)
			VALUES ('question', 'Self-approval probe post title', 'A description long enough for the post validation rules to accept it.', 'agent', $1, $2, $3, $4)
			RETURNING id::text`, agentID, seed.status, seed.publication, seed.moderation).Scan(&postID))

		calls := mod.GetCalls()
		mod.QueueResults(approved())
		answer, err := callStatusContract(http.DefaultClient, http.MethodPatch, ts.URL+"/v1/posts/"+postID, key, `{"status":"open"}`)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, answer.status, answer.body)
		var out struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(answer.body), &out))
		require.Equal(t, "pending_review", out.Data.Status, "from %s: an author edit must not publish: %s", seed.status, answer.body)

		// Published only once moderation approves.
		waitForValue(t, pool, "published", `SELECT publication_state FROM posts WHERE id = $1::uuid`, postID)
		require.Equal(t, calls+1, mod.GetCalls(), "from %s: moderation runs once", seed.status)
	}
}
