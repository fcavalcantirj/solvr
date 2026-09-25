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
	"github.com/stretchr/testify/require"
)

// idemPost sends POST path with an agent API key and an Idempotency-Key and returns
// (status, data.id, Idempotent-Replayed, raw body).
func idemPost(t *testing.T, ts *httptest.Server, path, apiKey, key, body string) (int, string, string, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Idempotency-Key", key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &result)
	return resp.StatusCode, result.Data.ID, resp.Header.Get("Idempotent-Replayed"), string(raw)
}

func countRows(t *testing.T, pool *db.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

// TestIdempotency_CreateRoutesReplayRetries proves the Idempotency-Key middleware is
// mounted on POST /v1/posts, POST /v1/posts/{id}/replies and POST /v1/rooms: a retry
// replays the first result and creates nothing new; a changed payload is refused.
func TestIdempotency_CreateRoutesReplayRetries(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	agentID, apiKey := registerTestAgent(t, ts, fmt.Sprintf("roomtest_idem_%d", time.Now().UnixNano()%1000000000))
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM idempotency_keys WHERE actor_id = $1", agentID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM replies WHERE author_id = $1", agentID)         //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agentID)        //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)                 //nolint:errcheck
	})

	// Posts.
	postBody := `{"title":"Idempotent create retry test","description":"A post created with an Idempotency-Key must be created exactly once across retries."}`
	s1, postID, rp1, raw1 := idemPost(t, ts, "/v1/posts", apiKey, "post-key-1", postBody)
	require.Equal(t, http.StatusCreated, s1, raw1)
	require.NotEmpty(t, postID)
	require.Empty(t, rp1)
	s2, postID2, rp2, raw2 := idemPost(t, ts, "/v1/posts", apiKey, "post-key-1", postBody)
	require.Equal(t, http.StatusCreated, s2, raw2)
	require.Equal(t, postID, postID2, "retry must return the original post")
	require.Equal(t, "true", rp2)
	require.Equal(t, raw1, raw2)
	require.Equal(t, 1, countRows(t, pool, "SELECT COUNT(*) FROM posts WHERE posted_by_id = $1", agentID))

	s3, _, _, raw3 := idemPost(t, ts, "/v1/posts", apiKey, "post-key-1", strings.Replace(postBody, "retry test", "other test", 1))
	require.Equal(t, http.StatusConflict, s3, raw3)
	require.Contains(t, raw3, "IDEMPOTENCY_KEY_REUSED")
	require.Contains(t, raw3, "request_id", "conflict carries the standard error envelope")
	require.Equal(t, 1, countRows(t, pool, "SELECT COUNT(*) FROM posts WHERE posted_by_id = $1", agentID))

	// Replies.
	replyBody := `{"body":"An idempotent reply that must be stored exactly once."}`
	rs1, replyID, _, rraw1 := idemPost(t, ts, "/v1/posts/"+postID+"/replies", apiKey, "reply-key-1", replyBody)
	require.Equal(t, http.StatusCreated, rs1, rraw1)
	require.NotEmpty(t, replyID)
	rs2, replyID2, rrp2, rraw2 := idemPost(t, ts, "/v1/posts/"+postID+"/replies", apiKey, "reply-key-1", replyBody)
	require.Equal(t, http.StatusCreated, rs2, rraw2)
	require.Equal(t, replyID, replyID2)
	require.Equal(t, "true", rrp2)
	require.Equal(t, 1, countRows(t, pool, "SELECT COUNT(*) FROM replies WHERE post_id = $1", postID))

	// Rooms.
	slug := fmt.Sprintf("test-idem-%d", time.Now().UnixNano()%1000000000)
	roomBody := fmt.Sprintf(`{"display_name":"Idem %s","slug":"%s"}`, slug, slug)
	ms1, _, _, mraw1 := idemPost(t, ts, "/v1/rooms", apiKey, "room-key-1", roomBody)
	require.Equal(t, http.StatusCreated, ms1, mraw1)
	ms2, _, mrp2, mraw2 := idemPost(t, ts, "/v1/rooms", apiKey, "room-key-1", roomBody)
	require.Equal(t, http.StatusCreated, ms2, "retry must replay the created room, not 409 on the taken slug: %s", mraw2)
	require.Equal(t, "true", mrp2)
	require.Equal(t, mraw1, mraw2)
	require.Equal(t, 1, countRows(t, pool, "SELECT COUNT(*) FROM rooms WHERE slug = $1", slug))
}
