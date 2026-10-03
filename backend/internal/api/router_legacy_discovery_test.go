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

// Task idx 73 step 3, "adapt then retire": the legacy typed discovery lists (GET /v1/problems,
// /v1/questions, /v1/ideas — family "legacy-typed-discovery") were adapters over the canonical
// GET /v1/posts list (idx 71). They are retired: every caller gets 410 ENDPOINT_RETIRED naming
// GET /v1/posts and the typed query that served the route (router_legacy_read_retirement_test.go
// pins the answer for every caller, the OpenAPI document and SPEC.md 26.7). The tests here pin
// that the query each route names lists what the route listed, so a caller that follows the
// instructions loses nothing.
//
// The adapter tests this file held (idx 71) and where their behavior is guarded now:
//   - TestLegacyTypedDiscovery_ServedByCanonicalPostsList (route == GET /v1/posts?type=<type>,
//     same ids and meta, every row of the path's type)
//     -> TestRetiredTypedDiscovery_NamedQueryListsWhatTheRouteListed.
//   - TestLegacyTypedDiscovery_AnnouncesCanonicalSuccessor (Deprecation, Link to the typed query,
//     registry, the canonical list not deprecated)
//     -> TestRetiredTypedDiscovery_NamedQueryListsWhatTheRouteListed (the instructions name the
//     query; the canonical list carries no Deprecation) and
//     TestLegacyReadRetirements_CoverEveryRetiredReadFamilyAndNameAServedReplacement.
//   - TestLegacyTypedDiscovery_AcceptsLenientLegacyQueryShape (clamped and defaulted pagination,
//     the path's type over a caller's type)
//     -> TestRetiredTypedDiscovery_EveryLegacyQueryShapeGetsTheMigrationError.
//   - TestPostsList_HasAnswerFilterDefinedOnce keeps its name: the canonical filter is unchanged,
//     and GET /v1/questions?has_answer= now answers the migration error naming it.

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

// legacyDiscoverySeed holds the posts seeded for the typed discovery tests.
type legacyDiscoverySeed struct {
	tag                                 string
	problem, unanswered, answered, idea string
}

// seedLegacyDiscovery seeds one open public post per legacy type under a unique tag, plus a
// second question that has an answer.
func seedLegacyDiscovery(t *testing.T, pool *db.Pool, authorID string) legacyDiscoverySeed {
	t.Helper()
	ctx := context.Background()
	s := legacyDiscoverySeed{tag: fmt.Sprintf("legacydisc%d", time.Now().UnixNano()%1000000000)}
	repo := db.NewPostRepository(pool)
	create := func(pt models.PostType, title string) string {
		p, err := repo.Create(ctx, &models.Post{
			Type:            pt,
			Title:           title,
			Description:     "Legacy discovery adapter body for " + title,
			Tags:            []string{s.tag},
			PostedByType:    models.AuthorTypeAgent,
			PostedByID:      authorID,
			Status:          models.PostStatusOpen,
			Visibility:      models.VisibilityPublic,
			SuccessCriteria: []string{"it works"},
		})
		require.NoError(t, err)
		return p.ID
	}
	s.problem = create(models.PostTypeProblem, "Legacy discovery problem")
	s.unanswered = create(models.PostTypeQuestion, "Legacy discovery unanswered question")
	s.answered = create(models.PostTypeQuestion, "Legacy discovery answered question")
	s.idea = create(models.PostTypeIdea, "Legacy discovery idea")

	// A reply shaped like the one the contribution cutover made from a legacy answer: has_answer
	// and the answer count read replies (task idx 76).
	_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
		VALUES ($1, 'agent', $2, 'An answer so the question counts as answered', 'answer', gen_random_uuid())`,
		s.answered, authorID)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM replies WHERE post_id IN (SELECT id FROM posts WHERE $1 = ANY(tags))", s.tag)
		pool.Exec(ctx, "DELETE FROM posts WHERE $1 = ANY(tags)", s.tag)
	})
	return s
}

var legacyTypedLists = []struct {
	path      string
	postType  string
	canonical string
}{
	{"/v1/problems", "problem", "/v1/posts?type=problem"},
	{"/v1/questions", "question", "/v1/posts?type=question"},
	{"/v1/ideas", "idea", "/v1/posts?type=idea"},
}

// TestRetiredTypedDiscovery_NamedQueryListsWhatTheRouteListed: each retired route names GET
// /v1/posts and the typed query its adapter served, and that query lists exactly the posts of
// the route's type, with the same meta; the canonical list itself is not deprecated.
func TestRetiredTypedDiscovery_NamedQueryListsWhatTheRouteListed(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyDiscovery(t, pool, agentID)

	want := map[string][]string{
		"problem":  {s.problem},
		"question": {s.unanswered, s.answered},
		"idea":     {s.idea},
	}
	for _, lt := range legacyTypedLists {
		t.Run(lt.path, func(t *testing.T) {
			ret := readRetirement(t, lt.path)
			assert.Equal(t, "GET /v1/posts", ret.Replacement, lt.path)
			assert.Contains(t, ret.Instructions, "GET "+lt.canonical+" ", "%s must name the query that served it", lt.path)
			assert.Contains(t, ret.Instructions, "same query parameters", "%s: tags, status, sort and pagination carry over", lt.path)
			assert.Contains(t, ret.Instructions, "data rows and meta are unchanged", "%s: the rows keep their shape", lt.path)

			got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+lt.path+"?tags="+s.tag, "", "")
			require.NoError(t, err)
			require.Equal(t, http.StatusGone, got.status, "%s: %s", lt.path, got.body)

			resp, listed := getLegacyList(t, ts.URL+lt.canonical+"&tags="+s.tag)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.ElementsMatch(t, want[lt.postType], listIDs(listed), "%s lists the route's posts", lt.canonical)
			assert.Equal(t, len(want[lt.postType]), listed.Meta.Total)
			assert.Equal(t, 1, listed.Meta.Page)
			assert.Equal(t, 20, listed.Meta.PerPage)
			assert.False(t, listed.Meta.HasMore)
			for _, d := range listed.Data {
				assert.Equal(t, lt.postType, d.Type)
			}
			assert.Empty(t, resp.Header.Get("Deprecation"), "the canonical list is not deprecated")
		})
	}
}

// TestRetiredTypedDiscovery_EveryLegacyQueryShapeGetsTheMigrationError: nothing is read, so the
// lenient legacy query shape (over-cap or invalid pagination, a caller's type the path used to
// override) gets the same 410 as a bare call. The instructions warn that the canonical list
// answers 400 for that pagination instead of clamping it, and it does.
func TestRetiredTypedDiscovery_EveryLegacyQueryShapeGetsTheMigrationError(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup)

	for _, lt := range legacyTypedLists {
		ret := readRetirement(t, lt.path)
		message, _ := retirementAnswer(ret)
		for _, query := range []string{"", "?per_page=200", "?page=abc&per_page=0", "?type=idea&tags=x", "?has_answer=true"} {
			got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+lt.path+query, "", "")
			require.NoError(t, err)
			require.Equal(t, http.StatusGone, got.status, "%s%s: %s", lt.path, query, got.body)
			assert.Equal(t, ErrCodeEndpointRetired, got.code, "%s%s", lt.path, query)
			assert.Equal(t, message, got.message, "%s%s", lt.path, query)
		}
		assert.Contains(t, ret.Instructions, "per_page above 50 answers 400", lt.path)
		assert.Contains(t, ret.Instructions, "other than type", "%s: the path's type replaces a caller's type", lt.path)

		for _, pagination := range []string{"&per_page=200", "&page=abc", "&per_page=0"} {
			got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+lt.canonical+pagination, "", "")
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, got.status, "%s%s: %s", lt.canonical, pagination, got.body)
			assert.Equal(t, "VALIDATION_ERROR", got.code, "%s%s", lt.canonical, pagination)
		}
	}
}

// TestPostsList_HasAnswerFilterDefinedOnce: has_answer is a canonical GET /v1/posts filter; GET
// /v1/questions?has_answer= reached it through the adapter and now answers the migration error
// that names it.
func TestPostsList_HasAnswerFilterDefinedOnce(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyDiscovery(t, pool, agentID)

	base := "/v1/posts?type=question&"
	resp, unanswered := getLegacyList(t, ts.URL+base+"has_answer=false&tags="+s.tag)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, unanswered.Data, 1, "%s has_answer=false", base)
	assert.Equal(t, s.unanswered, unanswered.Data[0].ID)

	resp, withAnswer := getLegacyList(t, ts.URL+base+"has_answer=true&tags="+s.tag)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, withAnswer.Data, 1, "%s has_answer=true", base)
	assert.Equal(t, s.answered, withAnswer.Data[0].ID)

	ret := readRetirement(t, "/v1/questions")
	assert.Contains(t, ret.Instructions, "has_answer", "GET /v1/questions names its answered filter")
	for _, value := range []string{"false", "true"} {
		got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/questions?has_answer="+value+"&tags="+s.tag, "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusGone, got.status, "has_answer=%s: %s", value, got.body)
		assert.Equal(t, ErrCodeEndpointRetired, got.code)
	}
}
