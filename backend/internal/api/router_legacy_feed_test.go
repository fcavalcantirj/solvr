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
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 71 step 3: the legacy feed (GET /v1/feed, /v1/feed/stuck, /v1/feed/unanswered —
// family "legacy-feed") is an ADAPTER over the canonical GET /v1/posts list instead of its
// own query stack. It keeps the legacy feed item shape during the transition and announces
// the exact canonical query that replaces each route.

type legacyFeedResp struct {
	Data []struct {
		ID            string `json:"id"`
		Type          string `json:"type"`
		Title         string `json:"title"`
		Snippet       string `json:"snippet"`
		Status        string `json:"status"`
		VoteScore     int    `json:"vote_score"`
		AnswerCount   int    `json:"answer_count"`
		ApproachCount int    `json:"approach_count"`
		CommentCount  int    `json:"comment_count"`
		Author        struct {
			Type        string `json:"type"`
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

func getLegacyFeed(t *testing.T, url string) (*http.Response, legacyFeedResp) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out legacyFeedResp
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &out), "decode %s: %s", url, string(raw))
	}
	return resp, out
}

func feedIDs(f legacyFeedResp) []string {
	ids := make([]string, 0, len(f.Data))
	for _, d := range f.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

// legacyFeedSeed holds the posts seeded for the feed adapter tests.
type legacyFeedSeed struct {
	tag           string
	inProgress    string // problem with status in_progress (needs help)
	stuckApproach string // open problem with a stuck approach (needs help)
	openProblem   string // open problem with a working approach (does not need help)
	unanswered    string
	answered      string
	idea          string
}

func seedLegacyFeed(t *testing.T, pool *db.Pool, authorID string) legacyFeedSeed {
	t.Helper()
	ctx := context.Background()
	s := legacyFeedSeed{tag: fmt.Sprintf("legacyfeed%d", time.Now().UnixNano()%1000000000)}
	repo := db.NewPostRepository(pool)
	create := func(pt models.PostType, status models.PostStatus, title string) string {
		p, err := repo.Create(ctx, &models.Post{
			Type:            pt,
			Title:           title,
			Description:     "Legacy feed adapter body for " + title,
			Tags:            []string{s.tag},
			PostedByType:    models.AuthorTypeAgent,
			PostedByID:      authorID,
			Status:          status,
			Visibility:      models.VisibilityPublic,
			SuccessCriteria: []string{"it works"},
		})
		require.NoError(t, err)
		return p.ID
	}
	s.inProgress = create(models.PostTypeProblem, models.PostStatusInProgress, "Feed problem in progress")
	s.stuckApproach = create(models.PostTypeProblem, models.PostStatusOpen, "Feed problem with a stuck approach")
	s.openProblem = create(models.PostTypeProblem, models.PostStatusOpen, "Feed problem progressing fine")
	s.unanswered = create(models.PostTypeQuestion, models.PostStatusOpen, "Feed unanswered question")
	s.answered = create(models.PostTypeQuestion, models.PostStatusOpen, "Feed answered question")
	s.idea = create(models.PostTypeIdea, models.PostStatusOpen, "Feed idea")

	approaches := db.NewApproachesRepository(pool)
	for _, a := range []struct {
		problem string
		status  models.ApproachStatus
	}{{s.stuckApproach, models.ApproachStatusStuck}, {s.openProblem, models.ApproachStatusWorking}} {
		approach, err := approaches.CreateApproach(ctx, &models.Approach{
			ProblemID:  a.problem,
			AuthorType: models.AuthorTypeAgent,
			AuthorID:   authorID,
			Angle:      "Feed adapter approach",
			Status:     a.status,
		})
		require.NoError(t, err)
		// The reply the contribution cutover makes from the approach: needs_help reads the
		// approach status kept in its provenance (task idx 76).
		_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
			VALUES ($1, 'agent', $2, 'Feed adapter approach', 'approach', $3, jsonb_build_object('status', $4::text))`,
			a.problem, authorID, approach.ID, string(a.status))
		require.NoError(t, err)
	}
	answer, err := db.NewAnswersRepository(pool).CreateAnswer(ctx, &models.Answer{
		QuestionID: s.answered,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   authorID,
		Content:    "An answer so the question counts as answered",
	})
	require.NoError(t, err)
	// The reply the contribution cutover makes from the answer: has_answer and the answer count
	// read replies (task idx 76).
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
		VALUES ($1, 'agent', $2, $3, 'answer', $4)`, s.answered, authorID, answer.Content, answer.ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM answers WHERE question_id IN (SELECT id FROM posts WHERE $1 = ANY(tags))", s.tag)
		pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id IN (SELECT id FROM posts WHERE $1 = ANY(tags))", s.tag)
		pool.Exec(ctx, "DELETE FROM posts WHERE $1 = ANY(tags)", s.tag)
	})
	return s
}

var legacyFeedRoutes = []struct {
	path      string
	canonical string // the canonical GET /v1/posts query the route is served by
}{
	{"/v1/feed", "/v1/posts?sort=newest"},
	{"/v1/feed/stuck", "/v1/posts?type=problem&needs_help=true"},
	{"/v1/feed/unanswered", "/v1/posts?type=question&has_answer=false"},
}

// TestLegacyFeed_ServedByCanonicalPostsList: each legacy feed route returns exactly what its
// canonical GET /v1/posts query returns for the same filters, in the legacy feed item shape.
func TestLegacyFeed_ServedByCanonicalPostsList(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyFeed(t, pool, agentID)

	for _, fr := range legacyFeedRoutes {
		t.Run(fr.path, func(t *testing.T) {
			resp, feed := getLegacyFeed(t, ts.URL+fr.path+"?tags="+s.tag)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			canonResp, canon := getLegacyList(t, ts.URL+fr.canonical+"&tags="+s.tag)
			require.Equal(t, http.StatusOK, canonResp.StatusCode)

			require.NotEmpty(t, feed.Data)
			assert.Equal(t, listIDs(canon), feedIDs(feed), "legacy feed must equal its canonical query")
			assert.Equal(t, canon.Meta.Total, feed.Meta.Total)
			assert.Equal(t, canon.Meta.Page, feed.Meta.Page)
			assert.Equal(t, canon.Meta.PerPage, feed.Meta.PerPage)
			assert.Equal(t, canon.Meta.HasMore, feed.Meta.HasMore)
		})
	}

	// The seeded rows land in the right routes: stuck = in_progress OR a stuck approach.
	_, stuck := getLegacyFeed(t, ts.URL+"/v1/feed/stuck?tags="+s.tag)
	assert.ElementsMatch(t, []string{s.inProgress, s.stuckApproach}, feedIDs(stuck))
	_, unanswered := getLegacyFeed(t, ts.URL+"/v1/feed/unanswered?tags="+s.tag)
	assert.Equal(t, []string{s.unanswered}, feedIDs(unanswered))
	_, all := getLegacyFeed(t, ts.URL+"/v1/feed?tags="+s.tag)
	assert.Len(t, all.Data, 6)
}

// TestLegacyFeed_KeepsFeedItemShape: the adapter renders canonical rows as legacy feed items
// (snippet, author display name, counts) so the frontend's /v1/feed/stuck consumer keeps working.
func TestLegacyFeed_KeepsFeedItemShape(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyFeed(t, pool, agentID)

	_, stuck := getLegacyFeed(t, ts.URL+"/v1/feed/stuck?tags="+s.tag)
	require.Len(t, stuck.Data, 2)
	for _, d := range stuck.Data {
		assert.Equal(t, "problem", d.Type)
		assert.Contains(t, d.Snippet, "Legacy feed adapter body for")
		assert.Equal(t, "agent", d.Author.Type)
		assert.Equal(t, agentID, d.Author.ID)
		assert.NotEmpty(t, d.Author.DisplayName)
		if d.ID == s.stuckApproach {
			assert.Equal(t, 1, d.ApproachCount)
		}
	}

	_, all := getLegacyFeed(t, ts.URL+"/v1/feed?tags="+s.tag)
	for _, d := range all.Data {
		if d.ID == s.answered {
			assert.Equal(t, 1, d.AnswerCount, "questions keep their answer count")
		}
	}
}

// TestLegacyFeed_AnnouncesCanonicalSuccessor: every legacy feed response carries Deprecation
// and a Link to the exact canonical query that serves it; the registry lists each route.
func TestLegacyFeed_AnnouncesCanonicalSuccessor(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes

	var family *RouteFamily
	for i := range RouteFamilies {
		if RouteFamilies[i].Name == "legacy-feed" {
			family = &RouteFamilies[i]
		}
	}
	require.NotNil(t, family, "the registry must hold the legacy-feed family")
	require.Contains(t, family.Canonical, "GET /v1/posts")

	for _, fr := range legacyFeedRoutes {
		resp, _ := getLegacyFeed(t, ts.URL+fr.path)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "true", resp.Header.Get("Deprecation"), "%s must be marked deprecated", fr.path)
		assert.Equal(t, fmt.Sprintf(`<%s>; rel="successor-version"`, fr.canonical),
			resp.Header.Get("Link"), "%s must link its canonical successor", fr.path)
		assert.Contains(t, family.Routes, "GET "+fr.path)
	}
}

// TestLegacyFeed_AcceptsLenientLegacyQueryShape: the legacy feed keeps clamping per_page to
// 50 and falling back to defaults on invalid pagination, and pins its type.
func TestLegacyFeed_AcceptsLenientLegacyQueryShape(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyFeed(t, pool, agentID)

	resp, f := getLegacyFeed(t, ts.URL+"/v1/feed?per_page=200")
	require.Equal(t, http.StatusOK, resp.StatusCode, "legacy per_page over the cap is clamped")
	assert.Equal(t, 50, f.Meta.PerPage)

	resp, f = getLegacyFeed(t, ts.URL+"/v1/feed/stuck?page=abc&per_page=-1")
	require.Equal(t, http.StatusOK, resp.StatusCode, "legacy invalid pagination falls back to defaults")
	assert.Equal(t, 1, f.Meta.Page)
	assert.Equal(t, 20, f.Meta.PerPage)

	resp, f = getLegacyFeed(t, ts.URL+"/v1/feed/unanswered?type=problem&has_answer=true&tags="+s.tag)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{s.unanswered}, feedIDs(f), "the route pins its own filters")
}

// TestLegacyFeed_EmptyResultIsValidNotError: a feed route that matches nothing returns 200 with
// an empty data array and a valid zero meta (not an error, not a nil body). This transfers the
// empty-result guards from the retired FeedHandler/FeedRepository query stack (idx 72 step 3):
// handlers.TestFeed_RecentActivity_EmptyResult, TestFeed_Stuck_Empty, TestFeed_Unanswered_Empty
// and db.TestFeedRepository_GetUnansweredQuestions_Empty onto the live canonical-posts adapter.
func TestLegacyFeed_EmptyResultIsValidNotError(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes

	noMatch := fmt.Sprintf("legacyfeednone%d", time.Now().UnixNano())
	for _, fr := range legacyFeedRoutes {
		t.Run(fr.path, func(t *testing.T) {
			resp, f := getLegacyFeed(t, ts.URL+fr.path+"?tags="+noMatch)
			require.Equal(t, http.StatusOK, resp.StatusCode, "an empty feed is not an error")
			assert.NotNil(t, f.Data, "empty data must serialize as [] not null")
			assert.Empty(t, f.Data)
			assert.Equal(t, 0, f.Meta.Total)
			assert.Equal(t, 1, f.Meta.Page)
			assert.Equal(t, 20, f.Meta.PerPage)
			assert.False(t, f.Meta.HasMore)
		})
	}
}
