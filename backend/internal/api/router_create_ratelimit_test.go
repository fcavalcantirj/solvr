package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W3: the hourly create limits are enforced for authenticated callers, through the
// real router (authentication and limiter together, no injected identity). Every account made
// here is younger than the new-account threshold, so its limit is halved.

// uniquePostBody is a post that no content gate can mistake for a repeat or a template: the
// title carries a fresh UUID, and nothing in it reads as a day counter or a status report.
func uniquePostBody() string {
	marker := uuid.NewString()
	return fmt.Sprintf(`{"type":"question","title":"How should a Go service bound retries %s","description":"The client retries forever when the upstream answers slowly; what is a sane way to cap it %s?"}`, marker, marker)
}

// postCreatesUntilRefused sends up to limit+1 post creates with bearer and returns the status
// of each one, so a failure shows exactly where the limit did or did not bite.
func postCreatesUntilRefused(t *testing.T, url, bearer string, limit int) []int {
	t.Helper()
	statuses := make([]int, 0, limit+1)
	for i := 0; i <= limit; i++ {
		answer, err := callStatusContract(http.DefaultClient, http.MethodPost, url+"/v1/posts", bearer, uniquePostBody())
		require.NoError(t, err, "create %d", i+1)
		statuses = append(statuses, answer.status)
		if answer.status == http.StatusTooManyRequests {
			require.Equal(t, "RATE_LIMITED", answer.code)
			break
		}
		require.Equal(t, http.StatusCreated, answer.status, "create %d: %s", i+1, answer.body)
	}
	return statuses
}

func deletePostsBy(t *testing.T, pool *db.Pool, authorID string) {
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id = $1", authorID) //nolint:errcheck
	})
}

func TestCreateRateLimit_AgentPostsRefusedPastHourlyLimit(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, agentKey := uniqueTestAgent(t, ts, pool)
	deletePostsBy(t, pool, agentID)

	limit := loadRateLimitConfig(pool).AgentPostsPerHour / 2
	require.Positive(t, limit)

	statuses := postCreatesUntilRefused(t, ts.URL, agentKey, limit)
	require.Len(t, statuses, limit+1, "statuses: %v", statuses)
	require.Equal(t, http.StatusTooManyRequests, statuses[limit],
		"create %d of an agent limited to %d posts/hour must be refused; statuses: %v", limit+1, limit, statuses)
}

func TestCreateRateLimit_HumanPostsRefusedPastHourlyLimit(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	userID, jwt := createLiveTestUser(t, pool, models.UserRoleUser)
	deletePostsBy(t, pool, userID)

	limit := loadRateLimitConfig(pool).HumanPostsPerHour / 2
	require.Positive(t, limit)

	statuses := postCreatesUntilRefused(t, ts.URL, jwt, limit)
	require.Len(t, statuses, limit+1, "statuses: %v", statuses)
	require.Equal(t, http.StatusTooManyRequests, statuses[limit],
		"create %d of a human limited to %d posts/hour must be refused; statuses: %v", limit+1, limit, statuses)
}
