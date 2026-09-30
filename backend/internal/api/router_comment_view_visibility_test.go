package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// commentTargets is one post of each legacy type with one contribution each, keyed by the
// comment route's path segment: posts -> the question, approaches/answers/responses -> the
// contribution. views is the post whose view routes are checked.
type commentTargets struct {
	ids   map[string]string
	views string
}

// commentVisibilityFixture inserts a problem, a question and an idea by agentID with an
// approach, an answer and a response, and one comment holding secret on each of the four
// targets. deleted soft-deletes the three posts afterwards (their children stay).
func commentVisibilityFixture(t *testing.T, pool *db.Pool, agentID, visibility, ownerID, secret string, deleted bool) commentTargets {
	t.Helper()
	ctx := context.Background()
	var owner any
	if ownerID != "" {
		owner = ownerID
	}
	post := func(typ string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, owner_human_id)
			 VALUES ($1, $2, $3, 'agent', $4, 'open', $5, $6::uuid) RETURNING id::text`,
			typ, "comment visibility "+typ+" "+uuid.NewString(), "comment visibility fixture "+uuid.NewString(),
			agentID, visibility, owner).Scan(&id))
		return id
	}
	problem, question, idea := post("problem"), post("question"), post("idea")
	var approach, answer, response string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, method) VALUES ($1::uuid, 'agent', $2, 'fixture angle', 'fixture method') RETURNING id::text`,
		problem, agentID).Scan(&approach))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO answers (question_id, author_type, author_id, content) VALUES ($1::uuid, 'agent', $2, 'fixture answer') RETURNING id::text`,
		question, agentID).Scan(&answer))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO responses (idea_id, author_type, author_id, content, response_type) VALUES ($1::uuid, 'agent', $2, 'fixture response', 'support') RETURNING id::text`,
		idea, agentID).Scan(&response))
	ids := map[string]string{"posts": question, "approaches": approach, "answers": answer, "responses": response}
	singular := map[string]string{"posts": "post", "approaches": "approach", "answers": "answer", "responses": "response"}
	for route, id := range ids {
		_, err := pool.Exec(ctx,
			`INSERT INTO comments (target_type, target_id, author_type, author_id, content) VALUES ($1, $2::uuid, 'agent', $3, $4)`,
			singular[route], id, agentID, secret+" on "+route)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, "DELETE FROM comments WHERE target_id = ANY($1::uuid[])", []string{question, approach, answer, response}) //nolint:errcheck
		pool.Exec(c, "DELETE FROM approaches WHERE id = $1::uuid", approach)                                                   //nolint:errcheck
		pool.Exec(c, "DELETE FROM answers WHERE id = $1::uuid", answer)                                                        //nolint:errcheck
		pool.Exec(c, "DELETE FROM responses WHERE id = $1::uuid", response)                                                    //nolint:errcheck
		pool.Exec(c, "DELETE FROM posts WHERE id = ANY($1::uuid[])", []string{problem, question, idea})                        //nolint:errcheck
	})
	if deleted {
		_, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = ANY($1::uuid[])", []string{problem, question, idea})
		require.NoError(t, err)
	}
	return commentTargets{ids: ids, views: question}
}

// Comments and view counts live under a post, so they answer what GET /v1/posts/{id}
// answers: a family-only post's comments (on the post or on any of its contributions) and
// its view routes are 404 to anyone outside the family, a deleted post's are 404 to
// everyone, and nobody who may not read the post can comment on it or count a view.
// Family members read (with a total that matches what they see), comment and count views.
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
	comments := func(id string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM comments WHERE target_id = $1::uuid", id).Scan(&n))
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
	refused := func(t *testing.T, set commentTargets, bearer string, write bool) {
		t.Helper()
		for route, id := range set.ids {
			notFound(t, call(t, "GET", "/v1/"+route+"/"+id+"/comments", bearer, ""))
			if write {
				before := comments(id)
				notFound(t, call(t, "POST", "/v1/"+route+"/"+id+"/comments", bearer, newComment))
				require.Equal(t, before, comments(id), "no comment was written on %s", route)
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
				got := call(t, "GET", "/v1/"+route+"/"+id+"/comments", bearer, "")
				require.Equal(t, http.StatusOK, got.status, "%s: %s", route, got.body)
				require.Contains(t, got.body, secret, route)
				var list struct {
					Data []json.RawMessage `json:"data"`
					Meta struct {
						Total int `json:"total"`
					} `json:"meta"`
				}
				require.NoError(t, json.Unmarshal([]byte(got.body), &list))
				require.Equal(t, len(list.Data), list.Meta.Total, "%s: the total counts what the caller sees", route)
				before := comments(id)
				// A distinct body per target: the anti-abuse gate refuses an author's repeated comment.
				got = call(t, "POST", "/v1/"+route+"/"+id+"/comments", bearer,
					`{"content":"a comment written by the visibility contract test on `+route+` `+id+`"}`)
				require.Equal(t, http.StatusCreated, got.status, "%s: %s", route, got.body)
				require.Equal(t, before+1, comments(id))
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
			got := call(t, "GET", "/v1/"+route+"/"+id+"/comments", "", "")
			require.Equal(t, http.StatusOK, got.status, "%s: %s", route, got.body)
			require.Contains(t, got.body, secret, route)
			got = call(t, "POST", "/v1/"+route+"/"+id+"/comments", foreignKey,
				`{"content":"a comment written by the visibility contract test on `+route+` `+id+`"}`)
			require.Equal(t, http.StatusCreated, got.status, "%s: %s", route, got.body)
		}
		got := call(t, "GET", "/v1/posts/"+public.views+"/views", "", "")
		require.Equal(t, http.StatusOK, got.status, got.body)
		got = call(t, "POST", "/v1/posts/"+public.views+"/view", foreignJWT, "")
		require.Equal(t, http.StatusOK, got.status, got.body)
		require.True(t, strings.Contains(got.body, `"view_count":1`), got.body)
	})
}
