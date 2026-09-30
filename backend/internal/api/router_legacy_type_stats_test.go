package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 72 step 3: type-specific statistics (GET /v1/stats/problems|questions|ideas —
// family "type-specific-statistics") are ADAPTERS whose counts come from the one knowledge
// aggregate GET /v1/overview publishes, and that aggregate counts exactly what an anonymous
// GET /v1/posts lists. The typed sidebar lists they still carry are legacy-only fields.

type overviewKnowledgeType struct {
	Type              string         `json:"type"`
	Label             string         `json:"label"`
	Total             int            `json:"total"`
	ByStatus          map[string]int `json:"by_status"`
	WithReplies       int            `json:"with_replies"`
	WithAcceptedReply int            `json:"with_accepted_reply"`
	Replies           int            `json:"replies"`
}

type overviewKnowledgeResp struct {
	Data struct {
		Knowledge struct {
			Heading    string                  `json:"heading"`
			Definition string                  `json:"definition"`
			Types      []overviewKnowledgeType `json:"types"`
		} `json:"knowledge"`
	} `json:"data"`
	Meta struct {
		SourceAvailability map[string]bool `json:"source_availability"`
	} `json:"meta"`
}

func getJSON(t *testing.T, url string, out any) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", url, string(raw))
	require.NoError(t, json.Unmarshal(raw, out), "decode %s: %s", url, string(raw))
	return resp
}

func knowledgeByType(k overviewKnowledgeResp) map[string]overviewKnowledgeType {
	out := map[string]overviewKnowledgeType{}
	for _, kt := range k.Data.Knowledge.Types {
		out[kt.Type] = kt
	}
	return out
}

// seedTypeStats inserts visible and hidden posts of every legacy type plus canonical
// replies, tagged for cleanup.
func seedTypeStats(t *testing.T, pool *db.Pool) {
	t.Helper()
	ctx := context.Background()
	tag := fmt.Sprintf("typestats%d", time.Now().UnixNano()%1000000000)
	replier, _ := createLiveTestUser(t, pool, "user") // a reply names its author (000116)
	insert := func(postType, status, visibility string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, visibility)
			VALUES ($1, $2, 'Type statistics adapter fixture body.', ARRAY[$3], $4, 'human', 'typestats-user', $5)
			RETURNING id::text`,
			postType, "Type stats "+postType+" "+status, tag, status, visibility).Scan(&id))
		return id
	}
	reply := func(postID string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body)
			VALUES ($1, 'human', $2, 'Type statistics reply fixture.') RETURNING id::text`,
			postID, replier).Scan(&id))
		return id
	}

	insert("problem", "open", "public")
	insert("problem", "solved", "public")
	insert("problem", "draft", "public")
	insert("problem", "open", "family")
	q := insert("question", "open", "public")
	reply(q)
	solvedQ := insert("question", "solved", "public")
	_, err := pool.Exec(ctx, "UPDATE posts SET accepted_answer_id = $2 WHERE id = $1", solvedQ, reply(solvedQ))
	require.NoError(t, err)
	insert("question", "pending_review", "public")
	insert("idea", "open", "public")
	insert("idea", "active", "public")
	insert("post", "open", "public")

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM posts WHERE $1 = ANY(tags)", tag)
	})
}

// TestOverviewKnowledge_DefinedOnceByTheCanonicalList: every knowledge figure the overview
// publishes equals the total of the matching anonymous GET /v1/posts query.
func TestOverviewKnowledge_DefinedOnceByTheCanonicalList(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	seedTypeStats(t, pool)

	var ov overviewKnowledgeResp
	getJSON(t, ts.URL+"/v1/overview", &ov)
	assert.True(t, ov.Meta.SourceAvailability["knowledge"], "knowledge source must be reported available")
	assert.NotEmpty(t, ov.Data.Knowledge.Heading)
	assert.NotEmpty(t, ov.Data.Knowledge.Definition)

	var types []string
	for _, kt := range ov.Data.Knowledge.Types {
		types = append(types, kt.Type)
	}
	require.Equal(t, []string{"problem", "question", "idea", "post"}, types)

	for _, kt := range ov.Data.Knowledge.Types {
		assert.NotEmpty(t, kt.Label, kt.Type)
		_, canon := getLegacyList(t, ts.URL+"/v1/posts?type="+kt.Type)
		assert.Equal(t, canon.Meta.Total, kt.Total, "%s total must equal GET /v1/posts?type=%s", kt.Type, kt.Type)
		sum := 0
		for status, n := range kt.ByStatus {
			sum += n
			_, byStatus := getLegacyList(t, ts.URL+"/v1/posts?type="+kt.Type+"&status="+status)
			assert.Equal(t, byStatus.Meta.Total, n, "%s/%s must equal the canonical status filter", kt.Type, status)
		}
		assert.Equal(t, kt.Total, sum, "%s status split must add up to its total", kt.Type)
		for _, hidden := range []string{"draft", "pending_review", "rejected"} {
			assert.NotContains(t, kt.ByStatus, hidden, "%s: hidden status %s is never counted", kt.Type, hidden)
		}
	}
	k := knowledgeByType(ov)
	assert.GreaterOrEqual(t, k["question"].WithReplies, 2)
	assert.GreaterOrEqual(t, k["question"].Replies, 2)
	assert.GreaterOrEqual(t, k["question"].WithAcceptedReply, 1)
}

// TestLegacyTypeStats_CountsComeFromOverviewKnowledge: each legacy route's counts are the
// overview knowledge figures for its type, read at the same database state, and move by
// exactly what was seeded (hidden and family posts never count).
func TestLegacyTypeStats_CountsComeFromOverviewKnowledge(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes

	var beforeP, beforeQ, beforeI struct {
		Data map[string]any `json:"data"`
	}
	getJSON(t, ts.URL+"/v1/stats/problems", &beforeP)
	getJSON(t, ts.URL+"/v1/stats/questions", &beforeQ)
	getJSON(t, ts.URL+"/v1/stats/ideas", &beforeI)

	seedTypeStats(t, pool)

	var ov overviewKnowledgeResp
	getJSON(t, ts.URL+"/v1/overview", &ov) // fresh server: first read is not cached
	k := knowledgeByType(ov)

	var p, q, i struct {
		Data map[string]any `json:"data"`
	}
	getJSON(t, ts.URL+"/v1/stats/problems", &p)
	getJSON(t, ts.URL+"/v1/stats/questions", &q)
	getJSON(t, ts.URL+"/v1/stats/ideas", &i)

	num := func(m map[string]any, key string) float64 {
		v, ok := m[key].(float64)
		require.True(t, ok, "field %s missing or not a number: %v", key, m[key])
		return v
	}

	assert.Equal(t, float64(k["problem"].Total), num(p.Data, "total_problems"))
	assert.Equal(t, float64(k["problem"].ByStatus["solved"]), num(p.Data, "solved_count"))
	assert.Equal(t, num(beforeP.Data, "total_problems")+2, num(p.Data, "total_problems"), "open+solved public only")
	assert.Equal(t, num(beforeP.Data, "solved_count")+1, num(p.Data, "solved_count"))

	assert.Equal(t, float64(k["question"].Total), num(q.Data, "total_questions"))
	assert.Equal(t, float64(k["question"].WithAcceptedReply), num(q.Data, "answered_count"))
	assert.InDelta(t, float64(k["question"].WithAcceptedReply)*100/float64(k["question"].Total),
		num(q.Data, "response_rate"), 1e-9)
	assert.Equal(t, num(beforeQ.Data, "total_questions")+2, num(q.Data, "total_questions"))
	assert.Equal(t, num(beforeQ.Data, "answered_count")+1, num(q.Data, "answered_count"))

	counts, ok := i.Data["counts_by_status"].(map[string]any)
	require.True(t, ok, "counts_by_status must be an object: %v", i.Data["counts_by_status"])
	assert.Equal(t, float64(k["idea"].Total), num(counts, "total"))
	for status, n := range k["idea"].ByStatus {
		assert.Equal(t, float64(n), num(counts, status), "idea status %s", status)
	}
	beforeCounts := beforeI.Data["counts_by_status"].(map[string]any)
	assert.Equal(t, num(beforeCounts, "total")+2, num(counts, "total"))

	// The typed sidebar lists stay served as legacy-only fields during the transition.
	for _, f := range []string{"active_approaches", "avg_solve_time_days", "recently_solved", "top_solvers"} {
		assert.Contains(t, p.Data, f)
	}
	for _, f := range []string{"avg_response_time_hours", "recently_answered", "top_answerers"} {
		assert.Contains(t, q.Data, f)
	}
	for _, f := range []string{"fresh_sparks", "ready_to_develop", "top_sparklers", "trending_tags", "pipeline_stats", "recently_realized"} {
		assert.Contains(t, i.Data, f)
	}
}

// TestLegacyTypeStats_AnnouncesCanonicalSuccessor: every type-specific statistics response
// is marked deprecated and links GET /v1/overview; the registry lists each route.
func TestLegacyTypeStats_AnnouncesCanonicalSuccessor(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup)

	var family *RouteFamily
	for i := range RouteFamilies {
		if RouteFamilies[i].Name == "type-specific-statistics" {
			family = &RouteFamilies[i]
		}
	}
	require.NotNil(t, family)
	require.Equal(t, "GET /v1/overview", family.Canonical)

	for _, path := range []string{"/v1/stats/problems", "/v1/stats/questions", "/v1/stats/ideas"} {
		var body map[string]any
		resp := getJSON(t, ts.URL+path, &body)
		assert.Equal(t, "true", resp.Header.Get("Deprecation"), "%s must be marked deprecated", path)
		assert.Equal(t, `</v1/overview>; rel="successor-version"`, resp.Header.Get("Link"), path)
		assert.Equal(t, "public, max-age=30", resp.Header.Get("Cache-Control"), path)
		assert.Contains(t, family.Routes, "GET "+path)
	}
}
