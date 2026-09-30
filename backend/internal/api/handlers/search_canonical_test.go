package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonicalSearchResult is a post found through one of its replies: the result is the
// canonical post, and the reply travels with it as an anchor.
func canonicalSearchResult() models.SearchResult {
	legacyType, legacyStatus := "approach", "failed"
	sim := 0.71
	return models.SearchResult{
		ID: "post-1", Type: "post", Title: "a post", Source: "post",
		AuthorID: "agent-1", AuthorType: "agent", AuthorName: "Agent One",
		MatchedReplies: []models.SearchReplyMatch{{
			ID: "reply-1", PostID: "post-1", URL: "/posts/post-1#reply-1",
			Snippet:    "the <mark>failed</mark> attempt",
			Author:     models.SearchAuthor{ID: "agent-2", Type: "agent", DisplayName: "Agent Two"},
			LegacyType: &legacyType, LegacyStatus: &legacyStatus,
			Score: 0.4, Similarity: &sim, CreatedAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		}},
	}
}

// Task idx 53 steps 2-3: GET /v1/search returns canonical posts; a match inside a reply comes
// back as a reply anchor on its post (id, post, link, excerpt, original author and origin),
// and an agent gets it with no type or content_types filter.
func TestSearch_ResultCarriesReplyAnchors(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{canonicalSearchResult()}, 1)
	w := httptest.NewRecorder()
	NewSearchHandler(repo).Search(w, httptest.NewRequest(http.MethodGet, "/v1/search?q=failed", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Empty(t, repo.searchOpts.Type, "no type filter is needed")
	assert.Empty(t, repo.searchOpts.ContentTypes, "no content_types filter is needed")

	var body struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.JSONEq(t, `"post-1"`, string(body.Data[0]["id"]))
	assert.JSONEq(t, `[{
		"id": "reply-1",
		"post_id": "post-1",
		"url": "/posts/post-1#reply-1",
		"snippet": "the <mark>failed</mark> attempt",
		"author": {"id": "agent-2", "type": "agent", "display_name": "Agent Two"},
		"legacy_type": "approach",
		"legacy_status": "failed",
		"score": 0.4,
		"similarity": 0.71,
		"created_at": "2026-03-04T05:06:07Z"
	}]`, string(body.Data[0]["matched_replies"]))
}

// A post found by its own text carries no anchors, and a native reply has no legacy origin.
func TestSearch_ResultWithoutReplyMatchesOmitsAnchors(t *testing.T) {
	native := canonicalSearchResult()
	native.MatchedReplies[0].LegacyType, native.MatchedReplies[0].LegacyStatus = nil, nil
	native.MatchedReplies[0].Similarity = nil
	plain := models.SearchResult{ID: "post-2", Type: "post", Title: "another post", Source: "post"}
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{native, plain}, 2)
	w := httptest.NewRecorder()
	NewSearchHandler(repo).Search(w, httptest.NewRequest(http.MethodGet, "/v1/search?q=post", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 2)
	var anchors []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body.Data[0]["matched_replies"], &anchors))
	require.Len(t, anchors, 1)
	for _, key := range []string{"legacy_type", "legacy_status", "similarity"} {
		assert.NotContains(t, anchors[0], key, "a native keyword-only reply match has no %s", key)
	}
	assert.NotContains(t, body.Data[1], "matched_replies", "a post matched by its own text has no anchors")
}

// Task idx 53 step 3: the MCP tool text an agent reads names each reply anchor with its link,
// original author, origin and excerpt.
func TestFormatSearchResults_ShowsReplyAnchors(t *testing.T) {
	text := formatSearchResults([]models.SearchResult{canonicalSearchResult()}, 1, true)
	for _, want := range []string{
		"ID: post-1",
		"Matched reply: /posts/post-1#reply-1 by Agent Two (approach, failed)",
		"the <mark>failed</mark> attempt",
	} {
		assert.Contains(t, text, want)
	}
	plain := formatSearchResults([]models.SearchResult{{ID: "post-2", Type: "post", Title: "plain"}}, 1, true)
	assert.False(t, strings.Contains(plain, "Matched reply"), "no anchor line without a reply match")
}

// Task idx 53 step 5 (counts): a search result carries the post's reply_count, the same
// server-computed total the posts list gives (answers + approaches + comments, which
// partition the post's live replies), so a result card never shows 0 replies for a post that
// has some.
func TestSearch_ResultCarriesReplyCount(t *testing.T) {
	withReplies := canonicalSearchResult()
	withReplies.AnswersCount, withReplies.ApproachesCount, withReplies.CommentsCount = 2, 3, 4
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{withReplies, {ID: "post-2", Type: "post", Title: "no replies", Source: "post"}}, 2)
	w := httptest.NewRecorder()
	NewSearchHandler(repo).Search(w, httptest.NewRequest(http.MethodGet, "/v1/search?q=post", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 2)
	assert.JSONEq(t, `9`, string(body.Data[0]["reply_count"]))
	assert.JSONEq(t, `0`, string(body.Data[1]["reply_count"]), "reply_count is always present")
}

// Task idx 53 step 3: an agent searches through MCP with a query alone. solvr_search offers
// query and limit, and neither its description nor its schema names a legacy post type.
func TestMCPHandler_SearchToolTakesNoLegacyType(t *testing.T) {
	schemas, _ := mcpToolSchemas(t)
	search, ok := schemas["solvr_search"]
	require.True(t, ok, "solvr_search is listed")
	assert.Equal(t, "limit,query", strings.Join(schemaKeys(search), ","))
	assert.Equal(t, "query", strings.Join(schemaRequired(search), ","))

	tools, _ := mcpRPC(t, "tools/list", nil)["tools"].([]interface{})
	for _, raw := range tools {
		tool := raw.(map[string]interface{})
		if tool["name"] != "solvr_search" {
			continue
		}
		text, _ := json.Marshal(tool)
		for _, legacy := range []string{"problem", "question", "idea", "approach", "answer"} {
			assert.NotContains(t, string(text), legacy, "solvr_search still mentions %q", legacy)
		}
	}
}

// A legacy type argument from an older client is ignored: it no longer narrows the search to
// posts migrated from that type, so the agent still gets every post and its reply anchors.
func TestMCPExecuteSearch_IgnoresALegacyTypeArgument(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{canonicalSearchResult()}, 1)
	res, err := NewMCPHandler(repo, nil).executeSearch(context.Background(),
		map[string]interface{}{"query": "failed", "type": "problem"})
	require.NoError(t, err)
	assert.Empty(t, repo.searchOpts.Type, "the legacy type argument is not a filter")
	assert.Empty(t, repo.searchOpts.ContentTypes)
	assert.Contains(t, mcpResultText(t, res), "Matched reply: /posts/post-1#reply-1 by Agent Two (approach, failed)")
}
