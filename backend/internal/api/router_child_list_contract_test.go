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

// childContractPost inserts a post by agentID (visibility "family" is owned by ownerID)
// and removes it when the test ends. deleted soft-deletes it after insertion.
func childContractPost(t *testing.T, pool *db.Pool, agentID, visibility, ownerID string, deleted bool) string {
	t.Helper()
	ctx := context.Background()
	var owner any
	if ownerID != "" {
		owner = ownerID
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, owner_human_id)
		 VALUES ('question', $1, $2, 'agent', $3, 'open', $4, $5::uuid) RETURNING id::text`,
		"child contract "+visibility+" "+uuid.NewString(), "child list contract fixture "+uuid.NewString(),
		agentID, visibility, owner).Scan(&id))
	t.Cleanup(func() {
		pool.Exec(context.Background(), //nolint:errcheck
			"DELETE FROM votes WHERE target_type = 'reply' AND target_id IN (SELECT id FROM replies WHERE post_id = $1::uuid)", id)
		pool.Exec(context.Background(), "DELETE FROM replies WHERE post_id = $1::uuid", id) //nolint:errcheck
		pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1::uuid", id)        //nolint:errcheck
	})
	if deleted {
		_, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1::uuid", id)
		require.NoError(t, err)
	}
	return id
}

// childContractReply inserts a reply by agentID on postID.
func childContractReply(t *testing.T, pool *db.Pool, postID, agentID, body string) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1::uuid, 'agent', $2, $3) RETURNING id::text`,
		postID, agentID, body).Scan(&id))
	return id
}

// A list under a parent resource answers what the parent's own GET answers for the same
// id and caller: 200 when the parent is readable, and the parent's refusal (404 for an
// absent, deleted or family-only parent; the parent's malformed-id answer) otherwise —
// never an empty 200 list for something that does not exist or that the caller may not see.
func TestStatusContract_ChildListsAnswerLikeTheirParent(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	callers := statusContractIdentities(t, ts, pool)
	ownerID, ownerJWT := createLiveTestUser(t, pool, models.UserRoleUser)
	callers["family owner JWT"] = ownerJWT
	liveUserID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	liveAgentID, _ := statusContractAgent(t, ts, pool)
	client := &http.Client{}

	postIDs := map[string]string{
		"live":        childContractPost(t, pool, liveAgentID, models.VisibilityPublic, "", false),
		"family-only": childContractPost(t, pool, liveAgentID, models.VisibilityFamily, ownerID, false),
		"deleted":     childContractPost(t, pool, liveAgentID, models.VisibilityPublic, "", true),
		"malformed":   malformedResourceID,
		"absent":      uuid.NewString(),
	}
	families := []struct {
		parent   string
		children []string
		ids      map[string]string
	}{
		{"/v1/posts/{id}", []string{"/v1/posts/{id}/replies", "/v1/posts/{id}/rooms"}, postIDs},
		{"/v1/users/{id}", []string{"/v1/users/{id}/agents", "/v1/users/{id}/badges"}, map[string]string{
			"live": liveUserID, "malformed": malformedResourceID, "absent": uuid.NewString(),
		}},
		{"/v1/agents/{id}", []string{"/v1/agents/{id}/badges", "/v1/agents/{id}/checkpoints"}, map[string]string{
			"live": liveAgentID, "malformed": malformedResourceID, "absent": uuid.NewString(),
		}},
	}

	for _, fam := range families {
		for kind, id := range fam.ids {
			for who, bearer := range callers {
				parentURL := ts.URL + strings.Replace(fam.parent, "{id}", id, 1)
				parent, err := callStatusContract(client, "GET", parentURL, bearer, "")
				require.NoError(t, err)
				// The parent itself must answer as documented, so a broken parent cannot
				// make a child pass by accident.
				switch {
				case kind == "live", kind == "family-only" && who == "family owner JWT":
					require.Equal(t, http.StatusOK, parent.status, "%s (%s, %s): %s", fam.parent, kind, who, parent.body)
				case kind == "malformed":
					require.GreaterOrEqual(t, parent.status, 400, "%s (%s, %s): %s", fam.parent, kind, who, parent.body)
					require.Less(t, parent.status, 500, "%s (%s, %s): %s", fam.parent, kind, who, parent.body)
				default:
					require.Equal(t, http.StatusNotFound, parent.status, "%s (%s, %s): %s", fam.parent, kind, who, parent.body)
				}
				for _, child := range fam.children {
					t.Run(child+"/"+kind+"/"+who, func(t *testing.T) {
						got, err := callStatusContract(client, "GET", ts.URL+strings.Replace(child, "{id}", id, 1), bearer, "")
						require.NoError(t, err)
						require.Equal(t, parent.status, got.status, "parent %s answered %d: %s", fam.parent, parent.status, got.body)
						if got.status == http.StatusOK {
							return
						}
						require.Equal(t, parent.code, got.code, got.body)
						require.NotEmpty(t, got.message, got.body)
						require.NotEmpty(t, got.headerID)
						require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
					})
				}
			}
		}
	}
}

// The canonical reply surfaces follow their post's visibility: a family-only post's
// replies are 404 to anyone outside the family (the same answer GET /v1/posts/{id} gives
// them), a deleted post's replies are 404 to everyone, and nobody outside the family can
// reply to or vote on them. Family members still read, reply and vote.
func TestReplySurfaces_FollowTheParentPostVisibility(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	ctx := context.Background()
	client := &http.Client{}

	ownerID, ownerJWT := createLiveTestUser(t, pool, models.UserRoleUser)
	siblingID, siblingKey := statusContractAgent(t, ts, pool)
	claimAgentToUser(t, pool, siblingID, ownerID)
	_, foreignKey := statusContractAgent(t, ts, pool)
	_, foreignJWT := createLiveTestUser(t, pool, models.UserRoleUser)

	familyPost := childContractPost(t, pool, siblingID, models.VisibilityFamily, ownerID, false)
	secret := "family-only reply " + uuid.NewString()
	familyReply := childContractReply(t, pool, familyPost, siblingID, secret)
	goneBody := "reply on a deleted post " + uuid.NewString()
	gonePost := childContractPost(t, pool, siblingID, models.VisibilityPublic, "", false)
	goneReply := childContractReply(t, pool, gonePost, siblingID, goneBody)
	_, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1::uuid", gonePost)
	require.NoError(t, err)

	outsiders := map[string]string{"anonymous": "", "foreign agent": foreignKey, "foreign human": foreignJWT}
	members := map[string]string{"family owner": ownerJWT, "sibling agent": siblingKey}

	notFound := func(t *testing.T, got statusContractAnswer, leaked string) {
		t.Helper()
		require.Equal(t, http.StatusNotFound, got.status, got.body)
		require.Equal(t, "NOT_FOUND", got.code, got.body)
		require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		require.NotContains(t, got.body, leaked)
	}
	replyCount := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM replies WHERE post_id = $1::uuid", familyPost).Scan(&n))
		return n
	}
	upvotes := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT upvotes FROM replies WHERE id = $1::uuid", familyReply).Scan(&n))
		return n
	}
	newReply := `{"body":"a reply written by the visibility contract test"}`

	for who, bearer := range outsiders {
		t.Run("outsider reads/"+who, func(t *testing.T) {
			got, err := callStatusContract(client, "GET", ts.URL+"/v1/replies/"+familyReply, bearer, "")
			require.NoError(t, err)
			notFound(t, got, secret)
			got, err = callStatusContract(client, "GET", ts.URL+"/v1/posts/"+familyPost+"/replies", bearer, "")
			require.NoError(t, err)
			notFound(t, got, secret)
		})
	}
	for who, bearer := range map[string]string{"foreign agent": foreignKey, "foreign human": foreignJWT} {
		t.Run("outsider writes/"+who, func(t *testing.T) {
			before, votesBefore := replyCount(), upvotes()
			got, err := callStatusContract(client, "POST", ts.URL+"/v1/posts/"+familyPost+"/replies", bearer, newReply)
			require.NoError(t, err)
			notFound(t, got, secret)
			got, err = callStatusContract(client, "POST", ts.URL+"/v1/replies/"+familyReply+"/vote", bearer, `{"direction":"up"}`)
			require.NoError(t, err)
			notFound(t, got, secret)
			require.Equal(t, before, replyCount(), "no reply was written")
			require.Equal(t, votesBefore, upvotes(), "no vote was counted")
		})
	}
	for who, bearer := range map[string]string{"anonymous": "", "foreign agent": foreignKey, "family owner": ownerJWT, "sibling agent": siblingKey} {
		t.Run("deleted post/"+who, func(t *testing.T) {
			got, err := callStatusContract(client, "GET", ts.URL+"/v1/replies/"+goneReply, bearer, "")
			require.NoError(t, err)
			notFound(t, got, goneBody)
		})
	}
	for who, bearer := range members {
		t.Run("member reads/"+who, func(t *testing.T) {
			got, err := callStatusContract(client, "GET", ts.URL+"/v1/replies/"+familyReply, bearer, "")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, got.status, got.body)
			require.Contains(t, got.body, secret)
			got, err = callStatusContract(client, "GET", ts.URL+"/v1/posts/"+familyPost+"/replies", bearer, "")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, got.status, got.body)
			require.Contains(t, got.body, secret)
		})
	}
	t.Run("member writes", func(t *testing.T) {
		before := replyCount()
		got, err := callStatusContract(client, "POST", ts.URL+"/v1/posts/"+familyPost+"/replies", siblingKey, newReply)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, got.status, got.body)
		require.Equal(t, before+1, replyCount())
		got, err = callStatusContract(client, "POST", ts.URL+"/v1/replies/"+familyReply+"/vote", ownerJWT, `{"direction":"up"}`)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.status, got.body)
		require.Equal(t, 1, upvotes())
	})
}
