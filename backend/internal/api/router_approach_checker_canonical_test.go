package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: through the real router, PATCH /v1/posts/{id} marks a problem solved
// when a reply migrated from a succeeded approach exists, and refuses when the only succeeded
// approach is a legacy row that was never migrated: the route checks replies, not the
// approaches table.
func TestPostsPatchSolved_ChecksTheCanonicalSucceededApproach(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userID, jwt := createLiveTestUser(t, pool, "user")
	problem := func(title string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
				publication_state, moderation_state, visibility)
			VALUES ('problem', $1, 'A problem whose solved status the canonical checker gates',
				ARRAY['probe'], 'open', 'human', $2, 'published', 'approved', 'public')
			RETURNING id::text`, title, userID).Scan(&id))
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), "DELETE FROM approaches WHERE problem_id = $1", id)
			_, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", id)
		})
		return id
	}
	migrated := problem("Solved check probe with a migrated succeeded approach")
	_, err = pool.Exec(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
		VALUES ($1, 'human', $2, 'the approach that worked', 'approach', gen_random_uuid(), '{"status":"succeeded"}')`,
		migrated, userID)
	require.NoError(t, err)
	unmigrated := problem("Solved check probe with an unmigrated succeeded approach")
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		VALUES ($1, 'human', $2, 'a legacy approach that worked', 'succeeded')`, unmigrated, userID)
	require.NoError(t, err)

	router := NewRouter(pool, nil, nil, nil)
	patchSolved := func(postID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/v1/posts/"+postID, strings.NewReader(`{"status":"solved"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+jwt)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	status := func(postID string) string {
		t.Helper()
		var s string
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM posts WHERE id = $1`, postID).Scan(&s))
		return s
	}

	rec := patchSolved(migrated)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "solved", status(migrated), "the migrated succeeded approach lets the owner mark it solved")

	rec = patchSolved(unmigrated)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "no succeeded approach exists")
	assert.Equal(t, "open", status(unmigrated), "a legacy approach row alone is not a succeeded approach")
}
