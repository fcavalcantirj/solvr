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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3-4 (feature:reputation): every route that shows an agent's or a user's
// reputation serves the canonical score, the same one the leaderboard serves: reputation frozen
// in reputation_history at the cutover plus live votes on posts and replies, plus the bonus for
// agents. The legacy formulas could not show the frozen points: they recompute from legacy
// tables.
func TestReputationRoutes_ServeTheLeaderboardScore(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)
	get := func(path, bearer string, out any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
		require.NoError(t, json.NewDecoder(rec.Body).Decode(out), path)
	}

	// The user's posts are deleted last (cleanups run in reverse).
	userID, userJWT := createLiveTestUser(t, pool, "user")
	name := fmt.Sprintf("rpr_%d", time.Now().UnixNano()%1000000000)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"reputation route probe"}`, name)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var reg struct {
		APIKey string `json:"api_key"`
		Agent  struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&reg))
	agentID := reg.Agent.ID
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM reputation_history WHERE owner_id IN ($1, $2)`, agentID, userID) //nolint:errcheck
		pool.Exec(c, `DELETE FROM votes WHERE voter_id LIKE $1`, name+"_voter%")                   //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id IN ($1, $2)`, agentID, userID)          //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = $1`, agentID)                                  //nolint:errcheck
	})
	// The user claims the agent, so GET /v1/users/{id}/agents lists it.
	_, err = pool.Exec(ctx, `UPDATE agents SET human_id = $1 WHERE id = $2`, userID, agentID)
	require.NoError(t, err)
	var bonus int
	require.NoError(t, pool.QueryRow(ctx, `SELECT reputation FROM agents WHERE id = $1`, agentID).Scan(&bonus))

	earn := func(ownerType, ownerID string, points int) {
		var postID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
			VALUES ('post', 'reputation route probe', 'body', ARRAY['rpr'], 'open', $1, $2)
			RETURNING id::text`, ownerType, ownerID).Scan(&postID))
		_, err := pool.Exec(ctx, `
			INSERT INTO reputation_history (source, source_id, owner_type, owner_id, post_id, points, earned_at)
			VALUES ('problem_solved', gen_random_uuid(), $1, $2, $3, $4, NOW())`, ownerType, ownerID, postID, points)
		require.NoError(t, err)
		require.NoError(t, db.NewPostRepository(pool).Vote(ctx, postID, "agent", name+"_voter_"+ownerType, "up"))
	}
	earn("agent", agentID, 100)
	earn("human", userID, 40)
	wantAgent, wantUser := bonus+100+2, 40+2

	// listed pages through a list route until id shows up and returns its reputation.
	listed := func(path, id string) int {
		t.Helper()
		for page := 1; ; page++ {
			var resp struct {
				Data []struct {
					ID         string `json:"id"`
					Reputation int    `json:"reputation"`
				} `json:"data"`
				Meta struct {
					HasMore bool `json:"has_more"`
				} `json:"meta"`
			}
			get(fmt.Sprintf("%s&page=%d&offset=%d", path, page, (page-1)*100), "", &resp)
			for _, e := range resp.Data {
				if e.ID == id {
					return e.Reputation
				}
			}
			require.True(t, resp.Meta.HasMore, "%s: %s not listed", path, id)
		}
	}

	assert.Equal(t, wantAgent, listed("/v1/leaderboard?type=agents&limit=100", agentID), "the served leaderboard")
	assert.Equal(t, wantUser, listed("/v1/leaderboard?type=users&limit=100", userID), "the served leaderboard")

	var profile struct {
		Data struct {
			Stats struct {
				Reputation int `json:"reputation"`
			} `json:"stats"`
		} `json:"data"`
	}
	get("/v1/agents/"+agentID, "", &profile)
	assert.Equal(t, wantAgent, profile.Data.Stats.Reputation, "GET /v1/agents/{id}")
	get("/v1/users/"+userID, "", &profile)
	assert.Equal(t, wantUser, profile.Data.Stats.Reputation, "GET /v1/users/{id}")
	get("/v1/me", userJWT, &profile)
	assert.Equal(t, wantUser, profile.Data.Stats.Reputation, "GET /v1/me as the user")

	var me struct {
		Data struct {
			Reputation int `json:"reputation"`
		} `json:"data"`
	}
	get("/v1/me", reg.APIKey, &me)
	assert.Equal(t, wantAgent, me.Data.Reputation, "GET /v1/me as the agent")

	assert.Equal(t, wantAgent, listed("/v1/agents?sort=reputation&status=all&per_page=100", agentID), "GET /v1/agents")
	assert.Equal(t, wantUser, listed("/v1/users?sort=reputation&limit=100", userID), "GET /v1/users")
	assert.Equal(t, wantAgent, listed("/v1/users/"+userID+"/agents?per_page=100", agentID), "GET /v1/users/{id}/agents")
}
