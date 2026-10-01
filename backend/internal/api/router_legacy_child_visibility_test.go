package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type legacyChildVisibilityFixture struct {
	problem  string
	approach string
	question string
	answer   string
	idea     string
	marker   string
}

// newLegacyChildVisibilityFixture creates the legacy child resources whose routes must
// follow their owning post's visibility. When deleted is true, the parent posts are
// soft-deleted after their children are inserted so the test also covers retained rows.
func newLegacyChildVisibilityFixture(
	t *testing.T,
	pool *db.Pool,
	authorAgentID, visibility, ownerHumanID, marker string,
	deleted bool,
) legacyChildVisibilityFixture {
	t.Helper()
	ctx := context.Background()
	var owner any
	if ownerHumanID != "" {
		owner = ownerHumanID
	}
	post := func(typ string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, owner_human_id)
			 VALUES ($1, $2, $3, 'agent', $4, 'open', $5, $6::uuid) RETURNING id::text`,
			typ, marker+" "+typ, "legacy child visibility fixture "+uuid.NewString(),
			authorAgentID, visibility, owner).Scan(&id))
		return id
	}

	fixture := legacyChildVisibilityFixture{marker: marker}
	fixture.problem = post("problem")
	fixture.question = post("question")
	fixture.idea = post("idea")
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, method)
		 VALUES ($1::uuid, 'agent', $2, $3, 'fixture method') RETURNING id::text`,
		fixture.problem, authorAgentID, marker+" approach").Scan(&fixture.approach))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO answers (question_id, author_type, author_id, content)
		 VALUES ($1::uuid, 'agent', $2, $3) RETURNING id::text`,
		fixture.question, authorAgentID, marker+" answer").Scan(&fixture.answer))

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		pool.Exec(cleanupCtx, "DELETE FROM progress_notes WHERE approach_id = $1::uuid", fixture.approach)                                           //nolint:errcheck
		pool.Exec(cleanupCtx, "DELETE FROM approach_relationships WHERE from_approach_id = $1::uuid OR to_approach_id = $1::uuid", fixture.approach) //nolint:errcheck
		pool.Exec(cleanupCtx, "DELETE FROM approaches WHERE id = $1::uuid", fixture.approach)                                                        //nolint:errcheck
		pool.Exec(cleanupCtx, "DELETE FROM answers WHERE id = $1::uuid", fixture.answer)                                                             //nolint:errcheck
		pool.Exec(cleanupCtx, "DELETE FROM posts WHERE id = ANY($1::uuid[])", []string{fixture.problem, fixture.question, fixture.idea})             //nolint:errcheck
	})
	if deleted {
		_, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = ANY($1::uuid[])",
			[]string{fixture.problem, fixture.question, fixture.idea})
		require.NoError(t, err)
	}
	return fixture
}

// A legacy child identifier must never bypass the owning post's family/deletion rules. The
// legacy child WRITE routes are retired (task idx 52) and the approach history READ route is
// retired (task idx 73 step 3): each answers every caller the same migration error.
func TestLegacyChildRoutes_FollowParentVisibilityAndAbsenceContract(t *testing.T) {
	liftCreateLimits(t) // many creates by one identity; the hourly limit is not this test's subject
	ts, _, pool := newStatusContractServer(t)
	ctx := context.Background()
	client := &http.Client{}

	ownerID, ownerJWT := createLiveTestUser(t, pool, models.UserRoleUser)
	siblingID, siblingKey := statusContractAgent(t, ts, pool)
	claimAgentToUser(t, pool, siblingID, ownerID)
	_, foreignKey := statusContractAgent(t, ts, pool)
	_, foreignJWT := createLiveTestUser(t, pool, models.UserRoleUser)

	marker := "legacy-child-secret-" + uuid.NewString()
	family := newLegacyChildVisibilityFixture(t, pool, siblingID, models.VisibilityFamily, ownerID, marker, false)
	deleted := newLegacyChildVisibilityFixture(t, pool, siblingID, models.VisibilityPublic, "", marker, true)
	public := newLegacyChildVisibilityFixture(t, pool, siblingID, models.VisibilityPublic, "", marker, false)

	call := func(t *testing.T, method, path, bearer, body string) statusContractAnswer {
		t.Helper()
		got, err := callStatusContract(client, method, ts.URL+path, bearer, body)
		require.NoError(t, err)
		return got
	}
	notFound := func(t *testing.T, got statusContractAnswer) {
		t.Helper()
		require.Equal(t, http.StatusNotFound, got.status, got.body)
		require.Equal(t, "NOT_FOUND", got.code, got.body)
		require.NotEmpty(t, got.message, got.body)
		require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		require.NotContains(t, got.body, marker)
	}
	progressCount := func(id string) int {
		var count int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM progress_notes WHERE approach_id = $1::uuid", id).Scan(&count))
		return count
	}
	answerUpvotes := func(id string) int {
		var count int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT upvotes FROM answers WHERE id = $1::uuid", id).Scan(&count))
		return count
	}

	// The legacy child WRITE routes are retired (task idx 52): progress notes, answer votes and
	// idea evolution answer the same migration error to every caller on every parent (family,
	// deleted, public), so the answer carries nothing about the parent and changes nothing.
	retired := func(t *testing.T, got statusContractAnswer, route string) {
		t.Helper()
		require.Equal(t, http.StatusGone, got.status, got.body)
		require.Equal(t, ErrCodeEndpointRetired, got.code, got.body)
		require.Contains(t, got.message, route, got.body)
		require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		require.NotContains(t, got.body, marker)
	}
	callers := map[string]string{"anonymous": "", "foreign agent": foreignKey, "foreign human": foreignJWT,
		"family owner": ownerJWT, "sibling agent": siblingKey}
	parents := map[string]legacyChildVisibilityFixture{"family": family, "deleted": deleted, "public": public}

	// The approach history read is retired too (task idx 73 step 3): every caller gets the same
	// migration error on every parent and nothing of the approach. The replacement, GET
	// /v1/posts/{id}/replies, follows the parent's visibility
	// (TestReplySurfaces_FollowTheParentPostVisibility) and lists the approach as a reply
	// (TestRetiredTypedReads_CanonicalReadsServeWhatTheRouteServed).
	t.Run("approach history", func(t *testing.T) {
		route := "GET /v1/problems/{id}/approaches/{approachId}/history"
		for parent, f := range parents {
			for who, bearer := range callers {
				t.Run(parent+"/"+who, func(t *testing.T) {
					retired(t, call(t, http.MethodGet, "/v1/problems/"+f.problem+"/approaches/"+f.approach+"/history", bearer, ""), route)
				})
			}
		}
		retired(t, call(t, http.MethodGet,
			"/v1/problems/"+uuid.NewString()+"/approaches/"+public.approach+"/history", "", ""), route)
	})

	t.Run("approach progress", func(t *testing.T) {
		for parent, f := range parents {
			for who, bearer := range callers {
				before := progressCount(f.approach)
				retired(t, call(t, http.MethodPost, "/v1/approaches/"+f.approach+"/progress", bearer,
					`{"content":"visibility-scoped progress"}`), "POST /v1/approaches/{id}/progress")
				require.Equal(t, before, progressCount(f.approach), "%s parent, %s: no progress was added", parent, who)
			}
		}
	})

	t.Run("answer vote", func(t *testing.T) {
		for parent, f := range parents {
			for who, bearer := range callers {
				before := answerUpvotes(f.answer)
				retired(t, call(t, http.MethodPost, "/v1/answers/"+f.answer+"/vote", bearer, `{"direction":"up"}`),
					"POST /v1/answers/{id}/vote")
				require.Equal(t, before, answerUpvotes(f.answer), "%s parent, %s: no vote was counted", parent, who)
			}
		}
	})

	t.Run("current post vote", func(t *testing.T) {
		path := func(f legacyChildVisibilityFixture) string { return "/v1/posts/" + f.question + "/my-vote" }
		for who, bearer := range map[string]string{"foreign agent": foreignKey, "foreign human": foreignJWT} {
			t.Run("family outsider/"+who, func(t *testing.T) {
				notFound(t, call(t, http.MethodGet, path(family), bearer, ""))
			})
		}
		for who, bearer := range map[string]string{"family owner": ownerJWT, "sibling agent": siblingKey} {
			t.Run("family member/"+who, func(t *testing.T) {
				got := call(t, http.MethodGet, path(family), bearer, "")
				require.Equal(t, http.StatusOK, got.status, got.body)
				require.Contains(t, got.body, `"vote":null`)
			})
		}
		notFound(t, call(t, http.MethodGet, path(deleted), ownerJWT, ""))
		got := call(t, http.MethodGet, path(public), foreignKey, "")
		require.Equal(t, http.StatusOK, got.status, got.body)
	})

	t.Run("idea evolution", func(t *testing.T) {
		path := "/v1/ideas/" + public.idea + "/evolve"
		for _, target := range []string{uuid.NewString(), malformedResourceID, family.question, public.question} {
			retired(t, call(t, http.MethodPost, path, siblingKey, `{"evolved_post_id":"`+target+`"}`), "POST /v1/ideas/{id}/evolve")
		}
		var evolved int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT COALESCE(array_length(evolved_into, 1), 0) FROM posts WHERE id = $1::uuid", public.idea).Scan(&evolved))
		require.Zero(t, evolved, "evolution attempts did not change the source idea")
	})
}
