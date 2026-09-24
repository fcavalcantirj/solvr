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
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task "Allow a Post to seed a collaboration without introducing another content type".
// A published Post can open the same room-start flow: /connect carries the post's title,
// an authorized content reference, and a seeded task; the created room records the source
// post; and the post can list its related public rooms. Protected post content is never
// copied, and starting a room never mutates the Post.

// seedPost inserts a canonical post with the given status/visibility and returns its ID.
func seedPost(t *testing.T, pool *db.Pool, authorID, title, status, visibility string) string {
	t.Helper()
	repo := db.NewPostRepository(pool)
	p, err := repo.Create(context.Background(), &models.Post{
		Type:         models.PostTypePost,
		Title:        title,
		Description:  "Body for " + title,
		Tags:         []string{"seed"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   authorID,
		Status:       models.PostStatus(status),
		Visibility:   visibility,
	})
	require.NoError(t, err)
	require.NotEmpty(t, p.ID)
	return p.ID
}

// createRoomWithSourcePost creates a room via an agent key carrying a source_post_id and
// returns (slug, roomID).
func createRoomWithSourcePost(t *testing.T, ts *httptest.Server, apiKey, sourcePostID string, private bool) (string, string) {
	t.Helper()
	slug := fmt.Sprintf("test-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Seeded Room %s","slug":"%s","is_private":%t,"source_post_id":%q}`,
		slug, slug, private, sourcePostID)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create room: %s", string(raw))
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &env))
	data, _ := env["data"].(map[string]interface{})
	require.NotNil(t, data)
	id, _ := data["id"].(string)
	slugOut, _ := data["slug"].(string)
	require.NotEmpty(t, id)
	require.NotEmpty(t, slugOut)
	return slugOut, id
}

func seedTestCleanup(t *testing.T, pool *db.Pool) {
	t.Helper()
	pool.Exec(context.Background(), "DELETE FROM rooms WHERE source_post_id IS NOT NULL")
	pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")
}

// TestConnect_SeededFromPublishedPost covers steps 1 and 2: opening /connect with a
// published public post carries the post's title, a link back to the post, and a seeded
// task, while retaining the ordinary start flow.
func TestConnect_SeededFromPublishedPost(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer seedTestCleanup(t, pool)

	agentID, _ := registerRoomTestAgent(t, ts)
	postID := seedPost(t, pool, agentID, "Ship a rate limiter", "open", models.VisibilityPublic)

	resp := doRoomRequest(t, "GET", ts.URL+"/v1/connect?post="+postID, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := dataObj(t, resp)

	source, _ := data["source"].(map[string]interface{})
	require.NotNil(t, source, "connect seeded from a post must carry a source reference")
	assert.Equal(t, postID, source["post_id"])
	assert.Equal(t, "Ship a rate limiter", source["title"], "source carries the post's title")
	assert.Equal(t, "/posts/"+postID, source["url"], "source links back to the post, not its raw body")

	sel, _ := data["selected"].(map[string]interface{})
	require.NotNil(t, sel)
	task, _ := sel["task"].(string)
	assert.Contains(t, task, "Ship a rate limiter", "the seeded task references the post")

	// The ordinary start flow survives: a copyable prompt is still present.
	prompt, _ := data["prompt"].(map[string]interface{})
	require.NotNil(t, prompt)
	assert.NotEmpty(t, prompt["text"], "the copyable prompt is still generated")
}

// TestConnect_ProtectedPostNotLeaked covers step 4: a draft (protected) post never has its
// title or body copied into the public /connect contract.
func TestConnect_ProtectedPostNotLeaked(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer seedTestCleanup(t, pool)

	agentID, _ := registerRoomTestAgent(t, ts)
	secret := "Confidential internal launch plan"
	postID := seedPost(t, pool, agentID, secret, "draft", models.VisibilityPublic)

	resp := doRoomRequest(t, "GET", ts.URL+"/v1/connect?post="+postID, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(raw), secret, "a draft post's title must never leak into /connect")

	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &env))
	data, _ := env["data"].(map[string]interface{})
	require.NotNil(t, data)
	assert.Nil(t, data["source"], "a protected post must not seed the connect contract")
}

// TestConnect_UnknownPostDegrades covers step 2's resilience: an unknown post reference
// degrades to the ordinary contract rather than failing.
func TestConnect_UnknownPostDegrades(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	resp := doRoomRequest(t, "GET", ts.URL+"/v1/connect?post=11111111-1111-1111-1111-111111111111", "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := dataObj(t, resp)
	assert.Nil(t, data["source"], "an unknown post seeds nothing")
	prompt, _ := data["prompt"].(map[string]interface{})
	require.NotNil(t, prompt)
	assert.NotEmpty(t, prompt["text"])
}

// TestPost_RecordsSourceRoomAndDoesNotMutate covers steps 3 and 5: a room created from a
// post records the source post and appears among the post's related public rooms, while
// the post itself is not mutated (state unchanged, no duplicated replies).
func TestPost_RecordsSourceRoomAndDoesNotMutate(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer seedTestCleanup(t, pool)

	agentID, apiKey := registerRoomTestAgent(t, ts)
	postID := seedPost(t, pool, agentID, "Design a cache", "open", models.VisibilityPublic)

	// Snapshot the post before starting a collaboration.
	before := doRoomRequest(t, "GET", ts.URL+"/v1/posts/"+postID, "", "")
	require.Equal(t, http.StatusOK, before.StatusCode)
	beforePost := dataObj(t, before)
	pubStateBefore := beforePost["publication_state"]
	replyCountBefore := beforePost["reply_count"]

	_, roomID := createRoomWithSourcePost(t, ts, apiKey, postID, false)

	// The post lists its related public rooms.
	related := doRoomRequest(t, "GET", ts.URL+"/v1/posts/"+postID+"/rooms", "", "")
	require.Equal(t, http.StatusOK, related.StatusCode)
	roomIDs := outcomeList(t, related)
	assert.Contains(t, roomIDs, roomID, "the post must list the room seeded from it")

	// Starting a room must not mutate the post.
	after := doRoomRequest(t, "GET", ts.URL+"/v1/posts/"+postID, "", "")
	require.Equal(t, http.StatusOK, after.StatusCode)
	afterPost := dataObj(t, after)
	assert.Equal(t, "post", afterPost["type"], "the post must stay a canonical post, not become a Problem")
	assert.Equal(t, pubStateBefore, afterPost["publication_state"], "publication state must be unchanged")
	assert.Equal(t, replyCountBefore, afterPost["reply_count"], "no replies must be duplicated")
}

// TestPost_RelatedRoomsExcludePrivate covers step 4: a private room seeded from a post is
// not exposed through the post's public related-rooms list.
func TestPost_RelatedRoomsExcludePrivate(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer seedTestCleanup(t, pool)

	agentID, apiKey := registerRoomTestAgent(t, ts)
	postID := seedPost(t, pool, agentID, "Private planning topic", "open", models.VisibilityPublic)

	_, publicRoomID := createRoomWithSourcePost(t, ts, apiKey, postID, false)
	_, privateRoomID := createRoomWithSourcePost(t, ts, apiKey, postID, true)

	related := doRoomRequest(t, "GET", ts.URL+"/v1/posts/"+postID+"/rooms", "", "")
	require.Equal(t, http.StatusOK, related.StatusCode)
	roomIDs := outcomeList(t, related)
	assert.Contains(t, roomIDs, publicRoomID)
	assert.NotContains(t, roomIDs, privateRoomID, "a private room must not appear in a post's public related rooms")
}

// TestCreateRoom_RejectsMalformedSourcePost covers input validation: a malformed
// source_post_id is a 400, not a silent drop.
func TestCreateRoom_RejectsMalformedSourcePost(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer seedTestCleanup(t, pool)

	_, apiKey := registerRoomTestAgent(t, ts)
	slug := fmt.Sprintf("test-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Bad source %s","slug":"%s","source_post_id":"not-a-uuid"}`, slug, slug)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "malformed source_post_id must be rejected: %s", string(raw))
	assert.True(t, strings.Contains(string(raw), "SOURCE_POST") || strings.Contains(string(raw), "source_post"),
		"error should name the offending field: %s", string(raw))
}
