package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// commentTargets is one post with a migrated reply of each legacy contribution kind, keyed
// by the retired comment route's path segment: posts -> the post, approaches/answers/responses
// -> the legacy id the contribution's reply carries (the id an old client still holds). views
// is the post itself, whose view routes are checked.
type commentTargets struct {
	ids   map[string]string
	views string
}

// commentVisibilityFixture inserts a post by agentID with the replies the cutover made from an
// approach, an answer and a response on it, and one reply migrated from a comment holding
// secret on each of the four targets: the legacy tables are archived (000138). deleted
// soft-deletes the post afterwards (its replies stay).
func commentVisibilityFixture(t *testing.T, pool *db.Pool, agentID, visibility, ownerID, secret string, deleted bool) commentTargets {
	t.Helper()
	ctx := context.Background()
	var owner any
	if ownerID != "" {
		owner = ownerID
	}
	var post string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, owner_human_id)
		 VALUES ('post', $1, $2, 'agent', $3, 'open', $4, $5::uuid) RETURNING id::text`,
		"comment visibility "+uuid.NewString(), "comment visibility fixture "+uuid.NewString(),
		agentID, visibility, owner).Scan(&post))
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1::uuid", post) //nolint:errcheck
	})
	// reply inserts a reply on the post migrated from a legacy row of legacyType, under parent
	// when it is set, and returns its id and its legacy id.
	reply := func(parent any, legacyType, body, provenance string) (string, string) {
		var id, legacyID string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
			 VALUES ($1::uuid, $2::uuid, 'agent', $3, $4, $5, gen_random_uuid(), $6::jsonb) RETURNING id::text, legacy_id::text`,
			post, parent, agentID, body, legacyType, provenance).Scan(&id, &legacyID))
		return id, legacyID
	}
	singular := map[string]string{"posts": "post", "approaches": "approach", "answers": "answer", "responses": "response"}
	ids := map[string]string{"posts": post}
	parents := map[string]any{"posts": nil}
	for _, route := range []string{"approaches", "answers", "responses"} {
		replyID, legacyID := reply(nil, singular[route], "fixture "+singular[route], `{"legacy_table":"`+route+`"}`)
		ids[route], parents[route] = legacyID, replyID
	}
	for route, id := range ids {
		reply(parents[route], "comment", secret+" on "+route,
			`{"legacy_table":"comments","target_type":"`+singular[route]+`","target_id":"`+id+`"}`)
	}
	if deleted {
		_, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1::uuid", post)
		require.NoError(t, err)
	}
	return commentTargets{ids: ids, views: post}
}

// View counts live under a post, so they answer what GET /v1/posts/{id} answers: a
// family-only post's view routes are 404 to anyone outside the family, a deleted post's are
// 404 to everyone, and nobody who may not read the post can count a view. Family members
// count views. The comment routes are retired: every caller, member or not, gets the same
// migration error on every parent and never a comment's text — the creates since task idx 52
// (and no comment is written), the lists since idx 73 step 3. A post's comments are its
// replies now; their visibility (404 outside the family and on a deleted post, members read
// them) is pinned by TestReplySurfaces_FollowTheParentPostVisibility and
// TestStatusContract_ChildListsAnswerLikeTheirParent, and that the replies are the
// comments by TestRetiredCommentLists_RepliesListWhatTheRouteListed.
func TestCommentAndViewSurfaces_FollowTheParentPostVisibility(t *testing.T) {
	liftCreateLimits(t) // many creates by one identity; the hourly limit is not this test's subject
	ts, _, pool := newStatusContractServer(t)
	ctx := context.Background()
	client := &http.Client{}

	ownerID, ownerJWT := createLiveTestUser(t, pool, models.UserRoleUser)
	siblingID, siblingKey := statusContractAgent(t, ts, pool)
	claimAgentToUser(t, pool, siblingID, ownerID)
	_, foreignKey := statusContractAgent(t, ts, pool)
	_, foreignJWT := createLiveTestUser(t, pool, models.UserRoleUser)

	secret := "comment secret " + uuid.NewString()
	family := commentVisibilityFixture(t, pool, siblingID, models.VisibilityFamily, ownerID, secret, false)
	gone := commentVisibilityFixture(t, pool, siblingID, models.VisibilityPublic, "", secret, true)
	public := commentVisibilityFixture(t, pool, siblingID, models.VisibilityPublic, "", secret, false)

	outsiders := map[string]string{"anonymous": "", "foreign agent": foreignKey, "foreign human": foreignJWT}
	writers := map[string]string{"foreign agent": foreignKey, "foreign human": foreignJWT}
	members := map[string]string{"family owner": ownerJWT, "sibling agent": siblingKey}
	everyone := map[string]string{"anonymous": "", "foreign agent": foreignKey, "family owner": ownerJWT, "sibling agent": siblingKey}
	newComment := `{"content":"a comment written by the visibility contract test"}`

	notFound := func(t *testing.T, got statusContractAnswer) {
		t.Helper()
		require.Equal(t, http.StatusNotFound, got.status, got.body)
		require.Equal(t, "NOT_FOUND", got.code, got.body)
		require.NotEmpty(t, got.message, got.body)
		require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		require.NotContains(t, got.body, secret)
		require.NotContains(t, got.body, "view_count")
	}
	replies := func(post string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM replies WHERE post_id = $1::uuid", post).Scan(&n))
		return n
	}
	views := func(id string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT view_count FROM posts WHERE id = $1::uuid", id).Scan(&n))
		return n
	}
	call := func(t *testing.T, method, path, bearer, body string) statusContractAnswer {
		t.Helper()
		got, err := callStatusContract(client, method, ts.URL+path, bearer, body)
		require.NoError(t, err)
		return got
	}
	retiredList := func(t *testing.T, route, id, bearer string) {
		t.Helper()
		got := call(t, "GET", "/v1/"+route+"/"+id+"/comments", bearer, "")
		require.Equal(t, http.StatusGone, got.status, "%s: %s", route, got.body)
		require.Equal(t, ErrCodeEndpointRetired, got.code, got.body)
		require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		require.NotContains(t, got.body, secret)
	}
	retiredComment := func(t *testing.T, set commentTargets, route, bearer, body string) {
		t.Helper()
		before := replies(set.views)
		got := call(t, "POST", "/v1/"+route+"/"+set.ids[route]+"/comments", bearer, body)
		require.Equal(t, http.StatusGone, got.status, "%s: %s", route, got.body)
		require.Equal(t, ErrCodeEndpointRetired, got.code, got.body)
		require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		require.NotContains(t, got.body, secret)
		require.Equal(t, before, replies(set.views), "no comment was written on %s", route)
	}
	refused := func(t *testing.T, set commentTargets, bearer string, write bool) {
		t.Helper()
		for route, id := range set.ids {
			retiredList(t, route, id, bearer)
			if write {
				retiredComment(t, set, route, bearer, newComment)
			}
		}
		notFound(t, call(t, "GET", "/v1/posts/"+set.views+"/views", bearer, ""))
		before := views(set.views)
		notFound(t, call(t, "POST", "/v1/posts/"+set.views+"/view", bearer, ""))
		require.Equal(t, before, views(set.views), "no view was counted")
	}

	for who, bearer := range outsiders {
		t.Run("family post, outsider/"+who, func(t *testing.T) {
			_, canWrite := writers[who]
			refused(t, family, bearer, canWrite)
		})
	}
	for who, bearer := range everyone {
		t.Run("deleted post/"+who, func(t *testing.T) {
			refused(t, gone, bearer, bearer != "")
		})
	}
	for who, bearer := range members {
		t.Run("family post, member/"+who, func(t *testing.T) {
			for route, id := range family.ids {
				retiredList(t, route, id, bearer)
				retiredComment(t, family, route, bearer, newComment)
			}
			got := call(t, "GET", "/v1/posts/"+family.views+"/views", bearer, "")
			require.Equal(t, http.StatusOK, got.status, got.body)
			before := views(family.views)
			got = call(t, "POST", "/v1/posts/"+family.views+"/view", bearer, "")
			require.Equal(t, http.StatusOK, got.status, got.body)
			require.Equal(t, before+1, views(family.views), "the member's own view is counted once")
			require.Contains(t, got.body, `"view_count":`)
		})
	}
	t.Run("public post, anyone", func(t *testing.T) {
		for route, id := range public.ids {
			retiredList(t, route, id, "")
			retiredComment(t, public, route, foreignKey, newComment)
		}
		got := call(t, "GET", "/v1/posts/"+public.views+"/views", "", "")
		require.Equal(t, http.StatusOK, got.status, got.body)
		got = call(t, "POST", "/v1/posts/"+public.views+"/view", foreignJWT, "")
		require.Equal(t, http.StatusOK, got.status, got.body)
		require.True(t, strings.Contains(got.body, `"view_count":1`), got.body)
	})
}
