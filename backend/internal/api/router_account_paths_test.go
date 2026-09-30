package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W0 on the real router: an agent stops authenticating once the human who claimed
// it is gone, and a claim on a soft-deleted agent is a 404, not a 500.

func TestAgentKey_RefusedOnceItsOwnerIsSoftDeleted(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, agentKey := uniqueTestAgent(t, ts, pool)
	ownerID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	claimAgentToUser(t, pool, agentID, ownerID)
	deletePostsBy(t, pool, agentID)

	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/v1/me", ""},
		{http.MethodPost, "/v1/posts", uniquePostBody()},
		{http.MethodPost, "/v1/rooms", `{"display_name":"Owner deleted room"}`},
	}
	before, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/me", agentKey, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, before.status, "a claimed agent with a live owner authenticates: %s", before.body)

	_, err = pool.Exec(context.Background(), "UPDATE users SET deleted_at = NOW() WHERE id = $1", ownerID)
	require.NoError(t, err)

	for _, rt := range routes {
		answer, err := callStatusContract(http.DefaultClient, rt.method, ts.URL+rt.path, agentKey, rt.body)
		require.NoError(t, err, "%s %s", rt.method, rt.path)
		require.Equal(t, http.StatusUnauthorized, answer.status, "%s %s: %s", rt.method, rt.path, answer.body)
	}
}

func TestAgentClaim_SoftDeletedAgentIsNotFound(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, agentKey := uniqueTestAgent(t, ts, pool)
	_, jwt := createLiveTestUser(t, pool, models.UserRoleUser)

	status, out := claimGenerate(t, ts, agentKey)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	token, _ := out["token"].(string)
	require.NotEmpty(t, token)

	_, err := pool.Exec(context.Background(), "UPDATE agents SET deleted_at = NOW() WHERE id = $1", agentID)
	require.NoError(t, err)

	status, code, err := claimAttempt(http.DefaultClient, ts, jwt, token)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, status, "code %s", code)
	require.Equal(t, "AGENT_NOT_FOUND", code)
}
