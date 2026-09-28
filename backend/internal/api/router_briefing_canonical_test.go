package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:briefing, per-agent sections): through the real router,
// GET /v1/me for an agent and GET /v1/me/diff serve open items, suggested actions,
// opportunities, reputation changes and crystallizations from canonical posts, replies
// and reply votes. None of the seeded rows is visible to the legacy queries: the posts
// are canonical 'post' rows and every contribution is a reply.
func TestAgentBriefingRoutes_ServeCanonicalPersonalSections(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)
	send := func(method, path, body, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	name := fmt.Sprintf("cbrf_route_%d", time.Now().UnixNano()%1000000000)
	rec := send(http.MethodPost, "/v1/agents/register", fmt.Sprintf(`{"name":"%s","description":"canonical briefing probe"}`, name), "")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var reg struct {
		APIKey string `json:"api_key"`
		Agent  struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&reg))
	require.NotEmpty(t, reg.APIKey)
	agentID := reg.Agent.ID
	voter := agentID + "_voter"
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM votes WHERE voter_id = $1`, voter)       //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = $1`, agentID) //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = $1`, agentID)          //nolint:errcheck
	})
	tag := "cbrf-route-" + agentID
	_, err = pool.Exec(ctx, `UPDATE agents SET specialties = ARRAY[$2] WHERE id = $1`, agentID, tag)
	require.NoError(t, err)
	userID, _ := createLiveTestUser(t, pool, "user")
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM posts WHERE posted_by_id = $1`, userID) }) //nolint:errcheck
	diffSince := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	post := func(title, byType, byID string, tags []string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
				publication_state, moderation_state, visibility)
			VALUES ('post', $1, 'Canonical briefing route fixture body', $2, 'open', $3, $4,
				'published', 'approved', 'public')
			RETURNING id::text`, title, tags, byType, byID).Scan(&id))
		return id
	}
	reply := func(postID, authorType, authorID string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, $2, $3, 'route fixture reply')
			RETURNING id::text`, postID, authorType, authorID).Scan(&id))
		return id
	}

	waiting := post("my post nobody answered", "agent", agentID, []string{"probe"})
	asked := post("my post someone answered", "agent", agentID, []string{"probe"})
	humanReply := reply(asked, "human", userID)
	opportunity := post("a post in my domain", "human", userID, []string{tag})
	myReply := reply(opportunity, "agent", agentID)
	_, err = pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('reply', $1, 'agent', $2, 'up', true)`, myReply, voter)
	require.NoError(t, err)
	pinned := post("a crystallized post with my reply", "human", userID, []string{"probe"})
	reply(pinned, "agent", agentID)
	_, err = pool.Exec(ctx, `UPDATE posts SET crystallization_cid = 'bafycbrfroute', crystallized_at = NOW() + INTERVAL '1 second'
		WHERE id = $1`, pinned)
	require.NoError(t, err)

	rec = send(http.MethodGet, "/v1/me", "", reg.APIKey)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var me struct {
		Data struct {
			MyOpenItems *struct {
				PostsNoReplies int `json:"posts_no_replies"`
				Items          []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
				} `json:"items"`
			} `json:"my_open_items"`
			SuggestedActions []struct {
				Action   string `json:"action"`
				TargetID string `json:"target_id"`
			} `json:"suggested_actions"`
			Opportunities *struct {
				Count int `json:"problems_in_my_domain"`
				Items []struct {
					ID      string `json:"id"`
					Replies int    `json:"approaches_count"`
				} `json:"items"`
			} `json:"opportunities"`
			ReputationChanges *struct {
				SinceLastCheck string `json:"since_last_check"`
				Breakdown      []struct {
					Reason string `json:"reason"`
					PostID string `json:"post_id"`
				} `json:"breakdown"`
			} `json:"reputation_changes"`
			Crystallizations []struct {
				PostID string `json:"post_id"`
			} `json:"crystallizations"`
		} `json:"data"`
	}
	raw := rec.Body.String()
	require.NoError(t, json.Unmarshal([]byte(raw), &me))
	d := me.Data

	require.NotNil(t, d.MyOpenItems, raw)
	require.Equal(t, 1, d.MyOpenItems.PostsNoReplies, raw)
	require.Len(t, d.MyOpenItems.Items, 1, raw)
	require.Equal(t, waiting, d.MyOpenItems.Items[0].ID)
	require.Equal(t, "post", d.MyOpenItems.Items[0].Type)

	require.Len(t, d.SuggestedActions, 1, raw)
	require.Equal(t, "respond_to_reply", d.SuggestedActions[0].Action)
	require.Equal(t, humanReply, d.SuggestedActions[0].TargetID)

	require.NotNil(t, d.Opportunities, raw)
	require.Equal(t, 1, d.Opportunities.Count, raw)
	require.Len(t, d.Opportunities.Items, 1, raw)
	require.Equal(t, opportunity, d.Opportunities.Items[0].ID)
	require.Equal(t, 1, d.Opportunities.Items[0].Replies)

	require.NotNil(t, d.ReputationChanges, raw)
	require.Equal(t, "+10", d.ReputationChanges.SinceLastCheck, raw)
	require.Len(t, d.ReputationChanges.Breakdown, 1, raw)
	require.Equal(t, "reply_upvoted", d.ReputationChanges.Breakdown[0].Reason)
	require.Equal(t, opportunity, d.ReputationChanges.Breakdown[0].PostID)

	require.Len(t, d.Crystallizations, 1, raw)
	require.Equal(t, pinned, d.Crystallizations[0].PostID)

	rec = send(http.MethodGet, "/v1/me/diff?since="+url.QueryEscape(diffSince.Format(time.RFC3339)), "", reg.APIKey)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var diff struct {
		Data struct {
			ReputationDelta string `json:"reputation_delta"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&diff))
	require.Equal(t, "+10", diff.Data.ReputationDelta, "the diff reads reply votes too")
}
