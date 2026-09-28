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

// Task idx 76 step 3 (feature:briefing, platform sections, recommendations and the
// /me/diff opportunity count): through the real router, GET /v1/me for an agent lists a
// room outcome as a recent victory, counts canonical open posts in the platform pulse and
// leaves posts the agent replied to out of its recommendations; GET /v1/me/diff counts a
// new canonical 'post' in the agent's domain; an agent without declared specialties gets
// them inferred from its replies. The legacy queries see none of it: victories needed a
// succeeded approach, the pulse had no open_posts, recommendations only excluded
// approach/answer interactions, the diff only counted problems, and inference read
// approaches and answers.
func TestAgentBriefingRoutes_ServeCanonicalPlatformSections(t *testing.T) {
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

	type registration struct {
		APIKey string `json:"api_key"`
		Agent  struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	register := func(name string) registration {
		rec := send(http.MethodPost, "/v1/agents/register", fmt.Sprintf(`{"name":"%s","description":"canonical platform briefing probe"}`, name), "")
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var reg registration
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&reg))
		require.NotEmpty(t, reg.APIKey)
		id := reg.Agent.ID
		t.Cleanup(func() {
			c := context.Background()
			pool.Exec(c, `DELETE FROM votes WHERE voter_id = $1`, id)     //nolint:errcheck
			pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = $1`, id) //nolint:errcheck
			pool.Exec(c, `DELETE FROM agents WHERE id = $1`, id)          //nolint:errcheck
		})
		return reg
	}
	name := fmt.Sprintf("cbpl_route_%d", time.Now().UnixNano()%1000000000)
	reg := register(name)
	agentID := reg.Agent.ID
	var roomID string
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = $1::uuid`, roomID) }) //nolint:errcheck
	specialty, liked := "cbpl-route-s-"+agentID, "cbpl-route-t-"+agentID
	_, err = pool.Exec(ctx, `UPDATE agents SET specialties = ARRAY[$2] WHERE id = $1`, agentID, specialty)
	require.NoError(t, err)
	userID, _ := createLiveTestUser(t, pool, "user")
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM posts WHERE posted_by_id = $1`, userID) }) //nolint:errcheck
	diffSince := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	post := func(title, byType, byID string, tags []string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
				publication_state, moderation_state, visibility)
			VALUES ('post', $1, 'Canonical platform briefing route fixture body', $2, 'open', $3, $4,
				'published', 'approved', 'public')
			RETURNING id::text`, title, tags, byType, byID).Scan(&id))
		return id
	}

	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO rooms (slug, display_name, created_at)
		VALUES ($1, $2, NOW() - INTERVAL '2 days') RETURNING id::text`,
		strings.ReplaceAll(name, "_", "-"), name).Scan(&roomID))
	outcome := post("a room outcome the agent saved", "agent", agentID, []string{"probe"})
	_, err = pool.Exec(ctx, `UPDATE posts SET source_room_id = $2::uuid WHERE id = $1`, outcome, roomID)
	require.NoError(t, err)

	upvoted := post("a post the agent upvoted", "human", userID, []string{liked})
	_, err = pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('post', $1, 'agent', $2, 'up', true)`, upvoted, agentID)
	require.NoError(t, err)
	recommended := post("a post in the agent's voted tag", "human", userID, []string{liked})
	replied := post("a post in the voted tag the agent replied to", "human", userID, []string{liked})
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'route reply')`,
		replied, agentID)
	require.NoError(t, err)
	post("a new post in the agent's domain", "human", userID, []string{specialty})

	rec := send(http.MethodGet, "/v1/me", "", reg.APIKey)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var me struct {
		Data struct {
			PlatformPulse *struct {
				OpenPosts int `json:"open_posts"`
			} `json:"platform_pulse"`
			RecentVictories []struct {
				ID          string `json:"id"`
				SolverID    string `json:"solver_id"`
				DaysToSolve int    `json:"days_to_solve"`
			} `json:"recent_victories"`
			YouMightLike []struct {
				ID          string `json:"id"`
				MatchReason string `json:"match_reason"`
			} `json:"you_might_like"`
		} `json:"data"`
	}
	raw := rec.Body.String()
	require.NoError(t, json.Unmarshal([]byte(raw), &me))
	d := me.Data

	require.NotNil(t, d.PlatformPulse, raw)
	require.GreaterOrEqual(t, d.PlatformPulse.OpenPosts, 5, raw)

	require.NotEmpty(t, d.RecentVictories, raw)
	require.Equal(t, outcome, d.RecentVictories[0].ID, raw)
	require.Equal(t, agentID, d.RecentVictories[0].SolverID)
	require.Equal(t, 2, d.RecentVictories[0].DaysToSolve, "room opened 2 days before the outcome")

	recs := map[string]string{}
	for _, r := range d.YouMightLike {
		recs[r.ID] = r.MatchReason
	}
	require.Equal(t, "voted_tags", recs[recommended], raw)
	require.NotContains(t, recs, replied, "a post the agent replied to is not recommended")
	require.NotContains(t, recs, upvoted)

	rec = send(http.MethodGet, "/v1/me/diff?since="+url.QueryEscape(diffSince.Format(time.RFC3339)), "", reg.APIKey)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var diff struct {
		Data struct {
			NewOpportunities int `json:"new_opportunities"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&diff))
	require.Equal(t, 1, diff.Data.NewOpportunities, "the diff counts a canonical post in the agent's domain")

	// No declared specialties: the briefing infers them from the agent's replies.
	quiet := register(name + "_q")
	inferredTag := "cbpl-route-i-" + quiet.Agent.ID
	discussed := post("a post the quiet agent replied to", "human", userID, []string{inferredTag})
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'route reply')`,
		discussed, quiet.Agent.ID)
	require.NoError(t, err)
	rec = send(http.MethodGet, "/v1/me", "", quiet.APIKey)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var inferred struct {
		Data struct {
			Opportunities *struct {
				InferredFrom string `json:"inferred_from"`
				Items        []struct {
					ID string `json:"id"`
				} `json:"items"`
			} `json:"opportunities"`
		} `json:"data"`
	}
	raw = rec.Body.String()
	require.NoError(t, json.Unmarshal([]byte(raw), &inferred))
	require.NotNil(t, inferred.Data.Opportunities, "specialties inferred from a reply select opportunities: %s", raw)
	require.Equal(t, "inferred", inferred.Data.Opportunities.InferredFrom)
	require.Len(t, inferred.Data.Opportunities.Items, 1, raw)
	require.Equal(t, discussed, inferred.Data.Opportunities.Items[0].ID)
}
