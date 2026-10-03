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

// Task idx 76 step 3: the stats routes and the overview read contributions from the canonical
// replies. Native replies, which the legacy statistics cannot see, move the contribution total;
// a system verdict is not a contribution. (The type-specific routes that named a problem's
// solver and a question's answerer are retired, task idx 73 step 3; the repository still
// names them, pinned by internal/db TestCanonicalStats_KeepsLegacyFiguresAcrossTheCutover.)
func TestStatsRoutes_ServeCanonicalReplies(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)
	get := func(path string, out any) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
		require.NoError(t, json.NewDecoder(rec.Body).Decode(out), path)
	}
	type stats struct {
		Data struct {
			TotalContributions int `json:"total_contributions"`
		} `json:"data"`
	}
	type overview struct {
		Data struct {
			Community struct {
				Metrics []struct {
					Key         string `json:"key"`
					Value       int    `json:"value"`
					Definition  string `json:"definition"`
					Unavailable bool   `json:"unavailable"`
				} `json:"metrics"`
			} `json:"community"`
		} `json:"data"`
	}
	contributionsIn := func(o overview) int {
		t.Helper()
		for _, m := range o.Data.Community.Metrics {
			if m.Key == "total_contributions" {
				require.False(t, m.Unavailable, "total_contributions unavailable")
				require.Contains(t, m.Definition, "Replies by people and agents on public posts", "the definition says what is counted")
				return m.Value
			}
		}
		t.Fatal("no total_contributions metric")
		return 0
	}

	var before stats
	get("/v1/stats", &before)
	var overviewBefore overview
	get("/v1/homepage/overview", &overviewBefore)

	agent := fmt.Sprintf("agent_stc_%d", time.Now().UnixNano()%1000000000)
	_, err = pool.Exec(ctx, `INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $2, $3, 'active')`, agent, agent+" solver", "hash_"+agent)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = $1`, agent) //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = $1`, agent)          //nolint:errcheck
	})
	newPost := func() string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, created_at)
			VALUES ('post', 'stats route probe', 'body', ARRAY['stc'], 'open', 'agent', $1, NOW() - INTERVAL '2 hours')
			RETURNING id::text`, agent).Scan(&id))
		return id
	}
	problem, question := newPost(), newPost()
	// A reply as the cutover leaves one migrated from a succeeded approach; no approach row exists.
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
		VALUES ($1, 'agent', $2, 'the fix', 'approach', gen_random_uuid(), '{"legacy_table":"approaches","status":"succeeded"}')`,
		problem, agent)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'agent', $2, 'the answer')`, question, agent)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'system', 'solvr-moderation', 'verdict')`, question)
	require.NoError(t, err)

	var after stats
	get("/v1/stats", &after)
	assert.Equal(t, before.Data.TotalContributions+2, after.Data.TotalContributions, "GET /v1/stats counts native replies, not verdicts")
	var overviewAfter overview
	get("/v1/homepage/overview", &overviewAfter)
	assert.Equal(t, contributionsIn(overviewBefore)+2, contributionsIn(overviewAfter), "GET /v1/homepage/overview")
	var consolidated overview
	get("/v1/overview", &consolidated)
	assert.Equal(t, contributionsIn(overviewAfter), contributionsIn(consolidated), "GET /v1/overview")
}
