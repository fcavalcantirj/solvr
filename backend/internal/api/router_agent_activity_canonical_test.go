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

// Task idx 76 step 3: GET /v1/agents/{id}/activity lists canonical replies. A native reply and a
// child reply, which the legacy activity cannot see, are listed; a family post is not, and the
// total and has_more count exactly the listed items. A native reply carries no accepted status:
// accepted_answer_id is retired (idx 68), only an answer migrated as accepted is labeled so.
func TestAgentActivityRoute_ListsCanonicalReplies(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)

	// The user's posts are deleted before the user (cleanups run in reverse).
	userID, _ := createLiveTestUser(t, pool, "user")
	name := fmt.Sprintf("aar_%d", time.Now().UnixNano()%1000000000)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"agent activity route probe"}`, name)))
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
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id IN ($1, $2)`, agentID, userID) //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = $1`, agentID)                         //nolint:errcheck
	})

	newPost := func(postType, authorType, author, visibility string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, visibility)
			VALUES ($1, 'agent activity route probe', 'body', ARRAY['aar'], 'open', $2, $3, $4)
			RETURNING id::text`, postType, authorType, author, visibility).Scan(&id))
		return id
	}
	reply := func(postID string, parent any, body string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
			VALUES ($1, $2, 'agent', $3, $4) RETURNING id::text`, postID, parent, agentID, body).Scan(&id))
		return id
	}
	idea := newPost("post", "agent", agentID, "public")
	newPost("post", "agent", agentID, "family")
	question := newPost("post", "human", userID, "public")
	answer := reply(question, nil, "native answer")
	child := reply(question, answer, "native child")

	type item struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Action      string `json:"action"`
		Title       string `json:"title"`
		Status      string `json:"status"`
		TargetID    string `json:"target_id"`
		TargetTitle string `json:"target_title"`
	}
	var resp struct {
		Data []item `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	get := func(query string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/activity"+query, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		resp.Data = nil
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	}

	get("")
	assert.Equal(t, []item{
		{ID: child, Type: "reply", Action: "replied", Title: "native child", TargetID: question, TargetTitle: "agent activity route probe"},
		{ID: answer, Type: "reply", Action: "replied", Title: "native answer", TargetID: question, TargetTitle: "agent activity route probe"},
		{ID: idea, Type: "post", Action: "created", Title: "agent activity route probe", Status: "open"},
	}, resp.Data, "newest first: both replies and the public idea, not the family idea")
	assert.Equal(t, 3, resp.Meta.Total, "the family idea is not counted")
	assert.False(t, resp.Meta.HasMore)

	get("?page=2&per_page=2")
	require.Len(t, resp.Data, 1)
	assert.Equal(t, idea, resp.Data[0].ID)
	assert.Equal(t, 3, resp.Meta.Total)
	assert.False(t, resp.Meta.HasMore, "the last page has no more")
}
