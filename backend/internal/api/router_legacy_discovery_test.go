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

// Task idx 71 step 3: the legacy typed discovery lists (GET /v1/problems, /v1/questions,
// /v1/ideas) are ADAPTERS over the canonical GET /v1/posts list. They share its filters,
// ordering, pagination and visibility instead of running an independent query stack, keep
// accepting the lenient legacy query shape during the transition, and announce their
// canonical successor with Deprecation and Link headers.

type legacyListResp struct {
	Data []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

func getLegacyList(t *testing.T, url string) (*http.Response, legacyListResp) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out legacyListResp
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &out), "decode %s: %s", url, string(raw))
	}
	return resp, out
}

func listIDs(l legacyListResp) []string {
	ids := make([]string, 0, len(l.Data))
	for _, d := range l.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

// seedLegacyDiscovery seeds one open public post per legacy type under a unique tag, plus a
// second question that has an answer. Returns the tag and the answered question's ID.
func seedLegacyDiscovery(t *testing.T, pool *db.Pool, authorID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	tag := fmt.Sprintf("legacydisc%d", time.Now().UnixNano()%1000000000)
	repo := db.NewPostRepository(pool)
	create := func(pt models.PostType, title string) string {
		p, err := repo.Create(ctx, &models.Post{
			Type:            pt,
			Title:           title,
			Description:     "Legacy discovery adapter body for " + title,
			Tags:            []string{tag},
			PostedByType:    models.AuthorTypeAgent,
			PostedByID:      authorID,
			Status:          models.PostStatusOpen,
			Visibility:      models.VisibilityPublic,
			SuccessCriteria: []string{"it works"},
		})
		require.NoError(t, err)
		return p.ID
	}
	create(models.PostTypeProblem, "Legacy discovery problem")
	create(models.PostTypeQuestion, "Legacy discovery unanswered question")
	answered := create(models.PostTypeQuestion, "Legacy discovery answered question")
	create(models.PostTypeIdea, "Legacy discovery idea")

	answer, err := db.NewAnswersRepository(pool).CreateAnswer(ctx, &models.Answer{
		QuestionID: answered,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   authorID,
		Content:    "An answer so the question counts as answered",
	})
	require.NoError(t, err)
	// The reply the contribution cutover makes from the answer: has_answer and the answer count
	// read replies (task idx 76).
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
		VALUES ($1, 'agent', $2, $3, 'answer', $4)`, answered, authorID, answer.Content, answer.ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM answers WHERE question_id IN (SELECT id FROM posts WHERE $1 = ANY(tags))", tag)
		pool.Exec(ctx, "DELETE FROM posts WHERE $1 = ANY(tags)", tag)
	})
	return tag, answered
}

var legacyTypedLists = []struct {
	path     string
	postType string
}{
	{"/v1/problems", "problem"},
	{"/v1/questions", "question"},
	{"/v1/ideas", "idea"},
}

// TestLegacyTypedDiscovery_ServedByCanonicalPostsList: every legacy typed list returns
// exactly what GET /v1/posts?type=<type> returns for the same filters.
func TestLegacyTypedDiscovery_ServedByCanonicalPostsList(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	agentID, _ := registerRoomTestAgent(t, ts)
	tag, _ := seedLegacyDiscovery(t, pool, agentID)

	for _, lt := range legacyTypedLists {
		t.Run(lt.path, func(t *testing.T) {
			legacyResp, legacy := getLegacyList(t, ts.URL+lt.path+"?tags="+tag)
			require.Equal(t, http.StatusOK, legacyResp.StatusCode)
			canonResp, canon := getLegacyList(t, ts.URL+"/v1/posts?type="+lt.postType+"&tags="+tag)
			require.Equal(t, http.StatusOK, canonResp.StatusCode)

			require.NotEmpty(t, legacy.Data, "the seeded %s must be listed", lt.postType)
			assert.Equal(t, listIDs(canon), listIDs(legacy), "legacy list must equal the canonical list")
			assert.Equal(t, canon.Meta, legacy.Meta, "legacy meta must equal the canonical meta")
			for _, d := range legacy.Data {
				assert.Equal(t, lt.postType, d.Type)
			}
		})
	}
}

// TestLegacyTypedDiscovery_AnnouncesCanonicalSuccessor: legacy responses carry Deprecation
// and a successor Link to the canonical destination the route registry publishes; the
// canonical list itself is not deprecated.
func TestLegacyTypedDiscovery_AnnouncesCanonicalSuccessor(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	var family *RouteFamily
	for i := range RouteFamilies {
		if RouteFamilies[i].Name == "legacy-typed-discovery" {
			family = &RouteFamilies[i]
		}
	}
	require.NotNil(t, family, "the registry must hold the legacy-typed-discovery family")
	require.Contains(t, family.Canonical, "GET /v1/posts")

	for _, lt := range legacyTypedLists {
		resp, _ := getLegacyList(t, ts.URL+lt.path)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "true", resp.Header.Get("Deprecation"), "%s must be marked deprecated", lt.path)
		assert.Equal(t, fmt.Sprintf(`</v1/posts?type=%s>; rel="successor-version"`, lt.postType),
			resp.Header.Get("Link"), "%s must link its canonical successor", lt.path)
		assert.Contains(t, family.Routes, "GET "+lt.path)
	}

	resp, _ := getLegacyList(t, ts.URL+"/v1/posts")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Deprecation"), "the canonical list is not deprecated")
}

// TestLegacyTypedDiscovery_AcceptsLenientLegacyQueryShape: during the transition the
// adapter keeps the old lenient pagination (clamp, default) and pins the type, while the
// canonical endpoint keeps its strict validation.
func TestLegacyTypedDiscovery_AcceptsLenientLegacyQueryShape(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	agentID, _ := registerRoomTestAgent(t, ts)
	tag, _ := seedLegacyDiscovery(t, pool, agentID)

	resp, l := getLegacyList(t, ts.URL+"/v1/problems?per_page=200")
	require.Equal(t, http.StatusOK, resp.StatusCode, "legacy per_page over the cap is clamped, not rejected")
	assert.Equal(t, 50, l.Meta.PerPage)

	resp, l = getLegacyList(t, ts.URL+"/v1/problems?page=abc&per_page=0")
	require.Equal(t, http.StatusOK, resp.StatusCode, "legacy invalid pagination falls back to defaults")
	assert.Equal(t, 1, l.Meta.Page)
	assert.Equal(t, 20, l.Meta.PerPage)

	resp, l = getLegacyList(t, ts.URL+"/v1/problems?type=idea&tags="+tag)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, l.Data)
	for _, d := range l.Data {
		assert.Equal(t, "problem", d.Type, "the legacy path pins its type over a caller-supplied type")
	}

	resp, _ = getLegacyList(t, ts.URL+"/v1/posts?per_page=200")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "canonical validation is unchanged")
}

// TestPostsList_HasAnswerFilterDefinedOnce: has_answer is a canonical GET /v1/posts filter,
// and GET /v1/questions?has_answer= reaches it through the adapter.
func TestPostsList_HasAnswerFilterDefinedOnce(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	agentID, _ := registerRoomTestAgent(t, ts)
	tag, answered := seedLegacyDiscovery(t, pool, agentID)

	for _, base := range []string{"/v1/posts?type=question&", "/v1/questions?"} {
		resp, unanswered := getLegacyList(t, ts.URL+base+"has_answer=false&tags="+tag)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Len(t, unanswered.Data, 1, "%s has_answer=false", base)
		assert.NotEqual(t, answered, unanswered.Data[0].ID)

		resp, withAnswer := getLegacyList(t, ts.URL+base+"has_answer=true&tags="+tag)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Len(t, withAnswer.Data, 1, "%s has_answer=true", base)
		assert.Equal(t, answered, withAnswer.Data[0].ID)
	}
}
