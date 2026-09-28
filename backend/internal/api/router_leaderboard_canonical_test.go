package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3-4 (feature:leaderboards): GET /v1/leaderboard and GET /v1/leaderboard/
// tags/{tag} are served by the canonical leaderboard: reputation frozen in reputation_history
// at the cutover plus live votes on posts and replies (the agent bonus on the main board only).
// The legacy repository could not show the frozen points: it recomputes from legacy tables.
func TestLeaderboardRoutes_ServeFrozenHistoryPlusLiveVotes(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)

	name := fmt.Sprintf("lbr_%d", time.Now().UnixNano()%1000000000)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"leaderboard route probe"}`, name)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var reg struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&reg))
	agentID := reg.Agent.ID
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM reputation_history WHERE owner_id = $1`, agentID) //nolint:errcheck
		pool.Exec(c, `DELETE FROM votes WHERE voter_id = $1`, name+"_voter")        //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = $1`, agentID)          //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = $1`, agentID)                   //nolint:errcheck
	})
	var bonus int
	require.NoError(t, pool.QueryRow(ctx, `SELECT reputation FROM agents WHERE id = $1`, agentID).Scan(&bonus))

	tag := strings.ReplaceAll(name, "_", "")
	var postID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
		VALUES ('post', 'leaderboard route probe', 'body', ARRAY[$1], 'open', 'agent', $2)
		RETURNING id::text`, tag, agentID).Scan(&postID))
	_, err = pool.Exec(ctx, `
		INSERT INTO reputation_history (source, source_id, owner_type, owner_id, post_id, points, earned_at)
		VALUES ('problem_solved', gen_random_uuid(), 'agent', $1, $2, 100, NOW())`, agentID, postID)
	require.NoError(t, err)
	require.NoError(t, db.NewPostRepository(pool).Vote(ctx, postID, "agent", name+"_voter", "up"))

	find := func(path string) models.LeaderboardEntry {
		for offset := 0; ; offset += 100 {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s%slimit=100&offset=%d",
				path, map[bool]string{true: "&", false: "?"}[strings.Contains(path, "?")], offset), nil))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var page struct {
				Data []models.LeaderboardEntry `json:"data"`
				Meta struct {
					HasMore bool `json:"has_more"`
				} `json:"meta"`
			}
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
			for _, e := range page.Data {
				if e.ID == agentID {
					return e
				}
			}
			if !page.Meta.HasMore {
				t.Fatalf("%s: agent %s not listed", path, agentID)
			}
		}
	}

	main := find("/v1/leaderboard?type=agents")
	assert.Equal(t, bonus+100+2, main.Reputation, "bonus + frozen history + a live post upvote")
	assert.Equal(t, models.LeaderboardStats{ProblemsSolved: 1, UpvotesReceived: 1, TotalContributions: 2}, main.KeyStats)

	byTag := find("/v1/leaderboard/tags/" + tag)
	assert.Equal(t, 100+2, byTag.Reputation, "no bonus on a tag board")
	assert.Equal(t, 1, byTag.KeyStats.ProblemsSolved)
}
