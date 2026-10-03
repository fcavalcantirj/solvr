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

// Task idx 73 step 3, "adapt then retire": the legacy feed (GET /v1/feed, /v1/feed/stuck,
// /v1/feed/unanswered — family "legacy-feed") was an adapter over the canonical GET /v1/posts
// list (idx 71). It is retired: every caller gets 410 ENDPOINT_RETIRED naming GET /v1/posts and
// the exact query that served the route (router_legacy_read_retirement_test.go pins the answer
// for every caller, the OpenAPI document and SPEC.md 26.7). The tests here pin that the query
// each route names lists what the route listed, with every feed item field, so a caller that
// follows the instructions loses nothing.
//
// The adapter tests this file held (idx 71/72) and where their behavior is guarded now:
//   - TestLegacyFeed_ServedByCanonicalPostsList (route == its query; stuck = in_progress or a
//     stuck approach; unanswered; all six) -> TestRetiredFeed_NamedQueryListsWhatTheRouteListed.
//   - TestLegacyFeed_KeepsFeedItemShape (snippet, author, approach and answer counts)
//     -> TestRetiredFeed_NamedQueryCarriesEveryFeedItemField.
//   - TestLegacyFeed_AnnouncesCanonicalSuccessor (Deprecation, Link to the query, registry)
//     -> TestRetiredFeed_NamedQueryListsWhatTheRouteListed (the instructions name the query)
//     and TestLegacyReadRetirements_CoverEveryRetiredReadFamilyAndNameAServedReplacement.
//   - TestLegacyFeed_AcceptsLenientLegacyQueryShape (clamped and defaulted pagination, pinned
//     filters) -> TestRetiredFeed_EveryLegacyQueryShapeGetsTheMigrationError.
//   - TestLegacyFeed_EmptyResultIsValidNotError -> TestRetiredFeed_EmptyNamedQueryIsValidNotError.

// canonicalFeedRow is one row of GET /v1/posts with the fields a legacy feed item came from.
type canonicalFeedRow struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Tags            []string  `json:"tags"`
	Status          string    `json:"status"`
	VoteScore       int       `json:"vote_score"`
	AnswersCount    int       `json:"answers_count"`
	ApproachesCount int       `json:"approaches_count"`
	CommentsCount   int       `json:"comments_count"`
	CreatedAt       time.Time `json:"created_at"`
	Author          struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"author"`
}

type canonicalFeedResp struct {
	Data []canonicalFeedRow `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
	rows []map[string]json.RawMessage // the same data rows, keyed by JSON field name
}

func getCanonicalFeed(t *testing.T, url string) canonicalFeedResp {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", url, string(body))
	var out canonicalFeedResp
	require.NoError(t, json.Unmarshal(body, &out), "decode %s: %s", url, string(body))
	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &raw), "decode %s: %s", url, string(body))
	out.rows = raw.Data
	return out
}

func canonicalFeedIDs(f canonicalFeedResp) []string {
	ids := make([]string, 0, len(f.Data))
	for _, d := range f.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

// readRetirement is the retirement entry the router mounts for a retired legacy read path.
func readRetirement(t *testing.T, path string) LegacyRouteRetirement {
	t.Helper()
	for _, ret := range LegacyReadRetirements {
		if ret.Route == "GET "+path {
			return ret
		}
	}
	require.Failf(t, "not retired", "GET %s has no LegacyReadRetirements entry", path)
	return LegacyRouteRetirement{}
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
			Type:         pt,
			Title:        title,
			Description:  "Legacy feed adapter body for " + title,
			Tags:         []string{s.tag},
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   authorID,
			Status:       status,
			Visibility:   models.VisibilityPublic,
		})
		require.NoError(t, err)
		return p.ID
	}
	s.inProgress = create(models.PostTypePost, models.PostStatusOpen, "Feed problem in progress")
	s.stuckApproach = create(models.PostTypePost, models.PostStatusOpen, "Feed problem with a stuck approach")
	s.openProblem = create(models.PostTypePost, models.PostStatusOpen, "Feed problem progressing fine")
	s.unanswered = create(models.PostTypePost, models.PostStatusOpen, "Feed unanswered question")
	s.answered = create(models.PostTypePost, models.PostStatusOpen, "Feed answered question")
	s.idea = create(models.PostTypePost, models.PostStatusOpen, "Feed idea")

	for _, a := range []struct {
		problem string
		status  string
	}{{s.stuckApproach, "stuck"}, {s.openProblem, "working"}} {
		// A reply shaped like the one the contribution cutover made from a legacy approach:
		// needs_help reads the approach status kept in its provenance (task idx 76).
		_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
			VALUES ($1, 'agent', $2, 'Feed adapter approach', 'approach', gen_random_uuid(), jsonb_build_object('status', $3::text))`,
			a.problem, authorID, a.status)
		require.NoError(t, err)
	}
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

// legacyFeedRoutes are the retired feed routes and the canonical query each adapter served
// (its pinned type and filter, newest first).
var legacyFeedRoutes = []struct {
	path      string
	canonical string
}{
	{"/v1/feed", "/v1/posts?sort=newest"},
	{"/v1/feed/stuck", "/v1/posts?needs_help=true&sort=newest"},
	{"/v1/feed/unanswered", "/v1/posts?has_answer=false&sort=newest"},
}

// legacyFeedFieldMoves maps each legacy feed item field the canonical row renames to that row's
// field; every other feed item field keeps its name.
var legacyFeedFieldMoves = map[string]string{
	"snippet":        "description",
	"answer_count":   "answers_count",
	"approach_count": "approaches_count",
	"comment_count":  "comments_count",
}

// TestRetiredFeed_NamedQueryListsWhatTheRouteListed: each retired route names GET /v1/posts and
// the exact query its adapter served, and that query lists the rows the route listed: stuck =
// problems in progress or with a stuck approach, unanswered = questions without an answer, the
// feed = every post, newest first.
func TestRetiredFeed_NamedQueryListsWhatTheRouteListed(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyFeed(t, pool, agentID)

	listed := map[string]canonicalFeedResp{}
	for _, fr := range legacyFeedRoutes {
		ret := readRetirement(t, fr.path)
		assert.Equal(t, "GET /v1/posts", ret.Replacement, fr.path)
		assert.Contains(t, ret.Instructions, "GET "+fr.canonical, "%s must name the query that served it", fr.path)
		assert.Contains(t, ret.Instructions, "same query parameters", "%s: tags, author and pagination carry over", fr.path)

		got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+fr.path+"?tags="+s.tag, "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusGone, got.status, "%s: %s", fr.path, got.body)
		listed[fr.path] = getCanonicalFeed(t, ts.URL+fr.canonical+"&tags="+s.tag)
	}

	assert.ElementsMatch(t, []string{s.inProgress, s.stuckApproach}, canonicalFeedIDs(listed["/v1/feed/stuck"]))
	assert.Equal(t, []string{s.unanswered}, canonicalFeedIDs(listed["/v1/feed/unanswered"]))
	all := listed["/v1/feed"]
	require.Len(t, all.Data, 6)
	assert.Equal(t, 6, all.Meta.Total)
	for i := 1; i < len(all.Data); i++ {
		assert.False(t, all.Data[i].CreatedAt.After(all.Data[i-1].CreatedAt), "newest first: %v", canonicalFeedIDs(all))
	}
}

// TestRetiredFeed_NamedQueryCarriesEveryFeedItemField: every field of a legacy feed item is in
// the canonical row, under its own name or the one the instructions give, with the same value.
func TestRetiredFeed_NamedQueryCarriesEveryFeedItemField(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup) // LIFO: seed cleanup runs before the pool closes
	agentID, _ := registerRoomTestAgent(t, ts)
	s := seedLegacyFeed(t, pool, agentID)

	for _, fr := range legacyFeedRoutes {
		ret := readRetirement(t, fr.path)
		f := getCanonicalFeed(t, ts.URL+fr.canonical+"&tags="+s.tag)
		require.NotEmpty(t, f.Data, fr.canonical)
		for _, field := range legacyFeedItemFields {
			name := field
			if moved, ok := legacyFeedFieldMoves[field]; ok {
				name = moved
				assert.Contains(t, ret.Instructions, field+" is "+moved, "%s: say where %s went", fr.path, field)
			}
			for _, row := range f.rows {
				_, ok := row[name]
				assert.True(t, ok, "%s: a row has no %q (legacy %q)", fr.canonical, name, field)
			}
		}
	}

	stuck := getCanonicalFeed(t, ts.URL+"/v1/posts?needs_help=true&sort=newest&tags="+s.tag)
	require.Len(t, stuck.Data, 2)
	for _, d := range stuck.Data {
		assert.Equal(t, "problem", d.Type)
		assert.Contains(t, d.Description, "Legacy feed adapter body for", "the snippet came from the description")
		assert.Contains(t, d.Tags, s.tag)
		assert.NotEmpty(t, d.Status)
		assert.Equal(t, "agent", d.Author.Type)
		assert.Equal(t, agentID, d.Author.ID)
		assert.NotEmpty(t, d.Author.DisplayName)
		assert.False(t, d.CreatedAt.IsZero())
		if d.ID == s.stuckApproach {
			assert.Equal(t, 1, d.ApproachesCount)
		}
	}

	all := getCanonicalFeed(t, ts.URL+"/v1/posts?sort=newest&tags="+s.tag)
	for _, d := range all.Data {
		if d.ID == s.answered {
			assert.Equal(t, 1, d.AnswersCount, "questions keep their answer count")
		}
	}
}

// TestRetiredFeed_EveryLegacyQueryShapeGetsTheMigrationError: nothing is read, so the lenient
// legacy query shape (over-cap or invalid pagination, overridden filters) gets the same 410 as
// a bare call. The instructions warn that the canonical list answers 400 for that pagination
// instead of clamping it, and it does.
func TestRetiredFeed_EveryLegacyQueryShapeGetsTheMigrationError(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup)

	for _, fr := range legacyFeedRoutes {
		ret := readRetirement(t, fr.path)
		message, _ := retirementAnswer(ret)
		for _, query := range []string{"", "?per_page=200", "?page=abc&per_page=-1", "?has_answer=true&tags=x"} {
			got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+fr.path+query, "", "")
			require.NoError(t, err)
			require.Equal(t, http.StatusGone, got.status, "%s%s: %s", fr.path, query, got.body)
			assert.Equal(t, ErrCodeEndpointRetired, got.code, "%s%s", fr.path, query)
			assert.Equal(t, message, got.message, "%s%s", fr.path, query)
		}
		assert.Contains(t, ret.Instructions, "per_page above 50 answers 400", fr.path)

		for _, pagination := range []string{"&per_page=200", "&page=abc", "&per_page=-1"} {
			got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+fr.canonical+pagination, "", "")
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, got.status, "%s%s: %s", fr.canonical, pagination, got.body)
			assert.Equal(t, "VALIDATION_ERROR", got.code, "%s%s", fr.canonical, pagination)
		}
	}
}

// TestRetiredFeed_EmptyNamedQueryIsValidNotError: a query that matches nothing returns 200 with
// an empty data array and a valid zero meta (not an error, not a nil body). These empty-result
// guards came from the retired FeedHandler/FeedRepository query stack (idx 72 step 3) through
// the feed adapter, and now sit on the canonical queries the retired routes name.
func TestRetiredFeed_EmptyNamedQueryIsValidNotError(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	t.Cleanup(cleanup)

	noMatch := fmt.Sprintf("legacyfeednone%d", time.Now().UnixNano())
	for _, fr := range legacyFeedRoutes {
		t.Run(fr.path, func(t *testing.T) {
			f := getCanonicalFeed(t, ts.URL+fr.canonical+"&tags="+noMatch)
			assert.NotNil(t, f.Data, "empty data must serialize as [] not null")
			assert.Empty(t, f.Data)
			assert.Equal(t, 0, f.Meta.Total)
			assert.Equal(t, 1, f.Meta.Page)
			assert.Equal(t, 20, f.Meta.PerPage)
			assert.False(t, f.Meta.HasMore)
		})
	}
}
