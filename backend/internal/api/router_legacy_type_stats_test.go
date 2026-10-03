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

// Task idx 72 step 3: the knowledge counts are defined once, by the aggregate GET /v1/overview
// publishes, and that aggregate counts exactly what an anonymous GET /v1/posts lists. The
// type-specific statistics routes that adapted it are retired (task idx 73 step 3;
// router_legacy_read_retirement_test.go pins their 410 naming GET /v1/overview), and since the
// legacy types were retired (idx 68) the aggregate has one entry, "post".

type overviewKnowledgeType struct {
	Type              string         `json:"type"`
	Label             string         `json:"label"`
	Total             int            `json:"total"`
	ByStatus          map[string]int `json:"by_status"`
	WithReplies       int            `json:"with_replies"`
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

// seedTypeStats inserts visible and hidden posts plus canonical replies, tagged for cleanup.
func seedTypeStats(t *testing.T, pool *db.Pool) {
	t.Helper()
	ctx := context.Background()
	tag := fmt.Sprintf("typestats%d", time.Now().UnixNano()%1000000000)
	replier, _ := createLiveTestUser(t, pool, "user") // a reply (000116) and a post (000117) name their author
	insert := func(postType, status, visibility string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, visibility)
			VALUES ($1, $2, 'Type statistics adapter fixture body.', ARRAY[$3], $4, 'human', $6, $5)
			RETURNING id::text`,
			postType, "Type stats "+postType+" "+status, tag, status, visibility, replier).Scan(&id))
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

	insert("post", "open", "public")
	insert("post", "closed", "public")
	insert("post", "draft", "public")
	insert("post", "open", "family")
	reply(insert("post", "open", "public"))
	withTwo := insert("post", "open", "public")
	reply(withTwo)
	reply(withTwo)
	insert("post", "pending_review", "public")
	insert("post", "rejected", "public")

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
	require.Equal(t, []string{"post"}, types)

	for _, kt := range ov.Data.Knowledge.Types {
		assert.NotEmpty(t, kt.Label, kt.Type)
		_, canon := getLegacyList(t, ts.URL+"/v1/posts")
		assert.Equal(t, canon.Meta.Total, kt.Total, "%s total must equal GET /v1/posts", kt.Type)
		sum := 0
		for status, n := range kt.ByStatus {
			sum += n
			_, byStatus := getLegacyList(t, ts.URL+"/v1/posts?status="+status)
			assert.Equal(t, byStatus.Meta.Total, n, "%s/%s must equal the canonical status filter", kt.Type, status)
		}
		assert.Equal(t, kt.Total, sum, "%s status split must add up to its total", kt.Type)
		for _, hidden := range []string{"draft", "pending_review", "rejected"} {
			assert.NotContains(t, kt.ByStatus, hidden, "%s: hidden status %s is never counted", kt.Type, hidden)
		}
	}
	k := knowledgeByType(ov)
	assert.GreaterOrEqual(t, k["post"].WithReplies, 2)
	assert.GreaterOrEqual(t, k["post"].Replies, 3)
}
