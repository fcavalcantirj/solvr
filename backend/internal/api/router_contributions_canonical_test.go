package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: through the real router, GET /v1/users/{id}/contributions and
// GET /v1/me/contributions list the reply migrated from a user's answer, under the reply's id,
// and not a legacy answers row that was never migrated: the routes read replies, not the
// answers table.
func TestUserContributions_ListTheMigratedReplies(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userID, jwt := createLiveTestUser(t, pool, "user")
	var questionID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, visibility)
		VALUES ('question', 'Contributions probe question', 'A question the probe user answered',
			ARRAY['probe'], 'open', 'human', $1, 'published', 'approved', 'public')
		RETURNING id::text`, userID).Scan(&questionID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM answers WHERE question_id = $1", questionID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", questionID)
	})
	var replyID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
		VALUES ($1, 'human', $2, 'the migrated answer', 'answer', gen_random_uuid(), '{"legacy_table":"answers"}')
		RETURNING id::text`, questionID, userID).Scan(&replyID))
	_, err = pool.Exec(ctx, `
		INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'human', $2, 'a legacy answer that was never migrated')`, questionID, userID)
	require.NoError(t, err)

	router := NewRouter(pool, nil, nil, nil)
	get := func(path, bearer string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return body
	}

	for _, c := range []struct{ path, bearer string }{
		{"/v1/users/" + userID + "/contributions", ""},
		{"/v1/me/contributions", jwt},
	} {
		body := get(c.path, c.bearer)
		data, ok := body["data"].([]any)
		require.True(t, ok, "%s: %v", c.path, body)
		require.Len(t, data, 1, "%s: only the migrated reply is listed", c.path)
		item := data[0].(map[string]any)
		assert.Equal(t, replyID, item["id"], "%s: the item is the reply", c.path)
		assert.Equal(t, "answer", item["type"], c.path)
		assert.Equal(t, questionID, item["parent_id"], c.path)
		assert.Equal(t, "question", item["parent_type"], c.path)
		assert.Equal(t, "Contributions probe question", item["parent_title"], c.path)
		assert.Equal(t, "the migrated answer", item["content_preview"], c.path)
		meta := body["meta"].(map[string]any)
		assert.Equal(t, float64(1), meta["total"], c.path)
	}
}
