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

// Task idx 76 step 3: the stats on GET /v1/agents/{id}, /v1/users/{id} and /v1/me count
// canonical replies. Native replies, a child reply and upvotes on replies, which the legacy stats
// cannot see, move them. Since idx 68 the stats are posts created, contributions (live replies)
// and upvotes received; the per-type and accepted-answer counters are retired.
func TestProfileStatsRoutes_CountCanonicalReplies(t *testing.T) {
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

	// The user's posts are deleted before the user (cleanups run in reverse).
	userID, userJWT := createLiveTestUser(t, pool, "user")
	name := fmt.Sprintf("psr_%d", time.Now().UnixNano()%1000000000)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"profile stats route probe"}`, name)))
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
		pool.Exec(c, `DELETE FROM votes WHERE voter_id LIKE $1`, name+"_voter%")                   //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id IN ($1, $2)`, agentID, userID)          //nolint:errcheck
		pool.Exec(c, `DELETE FROM reputation_history WHERE owner_id IN ($1, $2)`, agentID, userID) //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = $1`, agentID)                                  //nolint:errcheck
	})

	newPost := func(postType, authorType, author string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
			VALUES ($1, 'profile stats route probe', 'body', ARRAY['psr'], 'open', $2, $3)
			RETURNING id::text`, postType, authorType, author).Scan(&id))
		return id
	}
	reply := func(postID string, parent any, authorType, author string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
			VALUES ($1, $2, $3, $4, 'profile stats route probe') RETURNING id::text`,
			postID, parent, authorType, author).Scan(&id))
		return id
	}
	question := newPost("post", "human", userID)
	idea := newPost("post", "agent", agentID)
	agentAnswer := reply(question, nil, "agent", agentID)
	reply(question, agentAnswer, "agent", agentID) // a child reply is a contribution too
	userAnswer := reply(question, nil, "human", userID)
	reply(idea, nil, "human", userID)
	replies := db.NewReplyRepository(pool)
	require.NoError(t, replies.Vote(ctx, agentAnswer, "agent", name+"_voter_1", "up"))
	require.NoError(t, replies.Vote(ctx, userAnswer, "agent", name+"_voter_2", "up"))

	type stats struct {
		PostsCreated    int `json:"posts_created"`
		Contributions   int `json:"contributions"`
		UpvotesReceived int `json:"upvotes_received"`
	}
	var agent struct {
		Data struct {
			Stats stats `json:"stats"`
		} `json:"data"`
	}
	get("/v1/agents/"+agentID, "", &agent)
	assert.Equal(t, stats{PostsCreated: 1, Contributions: 2, UpvotesReceived: 1}, agent.Data.Stats,
		"GET /v1/agents/{id}: its post, its reply and the child reply, the upvote on its reply")

	want := stats{PostsCreated: 1, Contributions: 2, UpvotesReceived: 1}
	var user struct {
		Data struct {
			Stats stats `json:"stats"`
		} `json:"data"`
	}
	get("/v1/users/"+userID, "", &user)
	assert.Equal(t, want, user.Data.Stats, "GET /v1/users/{id}: both native replies contribute")
	user.Data.Stats = stats{}
	get("/v1/me", userJWT, &user)
	assert.Equal(t, want, user.Data.Stats, "GET /v1/me as the user")
}
