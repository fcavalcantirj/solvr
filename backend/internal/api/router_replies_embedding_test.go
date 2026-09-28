package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/require"
)

// axisEmbedding is a deterministic embedding service: every text maps to the same
// 1024-dim unit vector on its axis, so the stored vector identifies the service.
type axisEmbedding struct{ axis int }

func (e axisEmbedding) GenerateEmbedding(context.Context, string) ([]float32, error) {
	v := make([]float32, 1024)
	v[e.axis] = 1
	return v, nil
}

func (e axisEmbedding) GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return e.GenerateEmbedding(ctx, text)
}

// Task idx 76 step 3 (feature:embedding-workers): through the real router, a reply
// written with POST /v1/posts/{id}/replies and edited with PATCH /v1/replies/{id} gets
// its body embedded into replies.embedding by the configured embedding service.
func TestRepliesRoute_EmbedsTheBodyOnCreateAndEdit(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userID, jwt := createLiveTestUser(t, pool, "user")
	var postID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, visibility)
		VALUES ('post', 'Reply embedding route probe', 'A public post whose replies are embedded',
			ARRAY['probe'], 'open', 'human', $1, 'published', 'approved', 'public')
		RETURNING id::text`, userID).Scan(&postID))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", postID) })

	router := NewRouter(pool, nil, nil, axisEmbedding{axis: 5})
	send := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+jwt)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	storedAxis := func(replyID string) int {
		t.Helper()
		var vec *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT embedding::text FROM replies WHERE id = $1`, replyID).Scan(&vec))
		require.NotNil(t, vec, "the reply has no stored embedding")
		parts := strings.Split(strings.Trim(*vec, "[]"), ",")
		require.Len(t, parts, 1024)
		for i, p := range parts {
			if p == "1" {
				return i
			}
		}
		return -1
	}

	rec := send(http.MethodPost, "/v1/posts/"+postID+"/replies", `{"body":"the route embeds this reply"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	require.Equal(t, 5, storedAxis(created.Data.ID), "create must store the service's vector")

	// Swap the service's vector by editing through a router built with another axis.
	router = NewRouter(pool, nil, nil, axisEmbedding{axis: 9})
	rec = send(http.MethodPatch, "/v1/replies/"+created.Data.ID, `{"body":"the route re-embeds this edit"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 9, storedAxis(created.Data.ID), "an edit must replace the vector with the new body's")
}
