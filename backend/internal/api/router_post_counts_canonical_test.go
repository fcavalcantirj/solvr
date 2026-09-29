package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the posts list and detail count a post's canonical replies. A native
// top-level reply answers the post (answers_count, has_answer), its child reply and a system
// verdict are comments, a deleted reply counts nowhere, and reply_count equals the reply list's
// total on GET /v1/posts/{id}/replies.
func TestPostCountsRoute_CountCanonicalReplies(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)

	userID, _ := createLiveTestUser(t, pool, "user")
	tag := fmt.Sprintf("pcr%d", time.Now().UnixNano()%1000000000)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM posts WHERE $1 = ANY(tags)`, tag) //nolint:errcheck
	})
	newQuestion := func(title string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
				visibility, publication_state, moderation_state)
			VALUES ('question', $1, 'post counts route probe', ARRAY[$2], 'open', 'human', $3,
				'public', 'published', 'approved')
			RETURNING id::text`, title, tag, userID).Scan(&id))
		return id
	}
	reply := func(postID string, parent any, authorType, author string, deleted bool) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body, deleted_at)
			VALUES ($1, $2, $3, $4, 'post counts route reply', CASE WHEN $5 THEN NOW() END)
			RETURNING id::text`, postID, parent, authorType, author, deleted).Scan(&id))
		return id
	}
	answered := newQuestion("post counts route: answered")
	bare := newQuestion("post counts route: bare")
	top := reply(answered, nil, "human", userID, false)
	reply(answered, top, "human", userID, false)
	reply(answered, nil, "system", "moderation", false)
	reply(answered, nil, "human", userID, true)

	get := func(path string, out any) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NoError(t, json.NewDecoder(rec.Body).Decode(out))
	}
	type counts struct {
		ID         string `json:"id"`
		Answers    int    `json:"answers_count"`
		Approaches int    `json:"approaches_count"`
		Comments   int    `json:"comments_count"`
		Replies    int    `json:"reply_count"`
	}

	var detail struct{ Data counts }
	get("/v1/posts/"+answered, &detail)
	assert.Equal(t, counts{ID: answered, Answers: 1, Comments: 2, Replies: 3}, detail.Data)

	var page struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	get("/v1/posts/"+answered+"/replies", &page)
	assert.Equal(t, detail.Data.Replies, page.Meta.Total, "reply_count is the reply list total")

	var list struct{ Data []counts }
	get("/v1/posts?type=question&has_answer=true&tags="+tag, &list)
	assert.Equal(t, []counts{detail.Data}, list.Data, "the native reply answers the question")
	get("/v1/posts?type=question&has_answer=false&tags="+tag, &list)
	assert.Equal(t, []counts{{ID: bare}}, list.Data)
}
