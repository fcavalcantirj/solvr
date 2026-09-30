package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W3, continued: contributions, human API keys, one count per request, idempotent
// replays, and the general per-minute limits left exactly as they were.

type limitedAnswer struct {
	status    int
	remaining string
	body      string
}

// postWithHeaders sends one POST with a deadline and returns the status, the
// X-RateLimit-Remaining header and the body.
func postWithHeaders(t *testing.T, url, bearer, body, idempotencyKey string) limitedAnswer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return limitedAnswer{status: resp.StatusCode, remaining: resp.Header.Get("X-RateLimit-Remaining"), body: string(raw)}
}

func TestCreateRateLimit_AgentContributionsRefusedPastHourlyLimit(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, key := contribAgent(t, ts, pool)
	question := seedOpenPost(t, pool, "question")
	limit := loadRateLimitConfig(pool).AgentAnswersPerHour / 2

	for i := 0; i < limit; i++ {
		got := postWithHeaders(t, ts.URL+"/v1/questions/"+question+"/answers", key,
			fmt.Sprintf(`{"content":"Answer %d: set a context deadline %s"}`, i, uuid.NewString()), "")
		require.Equal(t, http.StatusCreated, got.status, "answer %d: %s", i+1, got.body)
	}
	got := postWithHeaders(t, ts.URL+"/v1/questions/"+question+"/answers", key,
		fmt.Sprintf(`{"content":"One more answer %s"}`, uuid.NewString()), "")
	require.Equal(t, http.StatusTooManyRequests, got.status, "answer %d of an agent limited to %d/hour: %s", limit+1, limit, got.body)
}

// D9: a human's API key tier does not lift the create limit.
func TestCreateRateLimit_HumanAPIKeyPostsRefusedPastHourlyLimit(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	userID, jwt := createLiveTestUser(t, pool, models.UserRoleUser)
	deletePostsBy(t, pool, userID)
	created := postWithHeaders(t, ts.URL+"/v1/users/me/api-keys", jwt, `{"name":"limit test key"}`, "")
	require.Equal(t, http.StatusCreated, created.status, created.body)
	var out struct {
		Key  string `json:"key"`
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(created.body), &out))
	apiKey := out.Key
	if apiKey == "" {
		apiKey = out.Data.Key
	}
	require.True(t, strings.HasPrefix(apiKey, "solvr_sk_"), created.body)

	limit := loadRateLimitConfig(pool).HumanPostsPerHour / 2
	statuses := postCreatesUntilRefused(t, ts.URL, apiKey, limit)
	require.Equal(t, http.StatusTooManyRequests, statuses[len(statuses)-1], "statuses: %v", statuses)
	require.Len(t, statuses, limit+1, "statuses: %v", statuses)
}

// The limiter is mounted once per create route: each create spends exactly one.
func TestCreateRateLimit_OneCountPerRequest(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := uniqueTestAgent(t, ts, pool)
	deletePostsBy(t, pool, agentID)
	first := postWithHeaders(t, ts.URL+"/v1/posts", key, uniquePostBody(), "")
	second := postWithHeaders(t, ts.URL+"/v1/posts", key, uniquePostBody(), "")
	require.Equal(t, http.StatusCreated, first.status, first.body)
	require.Equal(t, http.StatusCreated, second.status, second.body)
	require.Equal(t, "1", first.remaining)
	require.Equal(t, "0", second.remaining)
}

// A replayed create (same Idempotency-Key) is answered from the idempotency store even at the
// limit: the limiter sits inside the idempotency middleware.
func TestCreateRateLimit_IdempotentReplayAtTheLimitIsNotRefused(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := uniqueTestAgent(t, ts, pool)
	deletePostsBy(t, pool, agentID)
	idem := uuid.NewString()
	body := uniquePostBody()
	first := postWithHeaders(t, ts.URL+"/v1/posts", key, body, idem)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	require.Equal(t, http.StatusCreated, postWithHeaders(t, ts.URL+"/v1/posts", key, uniquePostBody(), "").status)
	require.Equal(t, http.StatusTooManyRequests, postWithHeaders(t, ts.URL+"/v1/posts", key, uniquePostBody(), "").status)

	replay := postWithHeaders(t, ts.URL+"/v1/posts", key, body, idem)
	require.Equal(t, http.StatusCreated, replay.status, replay.body)
	require.JSONEq(t, first.body, replay.body)
}

// General and search limits stay as they were: not enforced for authenticated callers.
func TestCreateRateLimit_GeneralLimitsUnchanged(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, key := uniqueTestAgent(t, ts, pool)
	for i := 0; i < 70; i++ {
		answer, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/me", key, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, answer.status, "GET %d: %s", i+1, answer.body)
	}
}

// liftCreateLimits raises the hourly create limits for a test whose subject is not the limit
// (it creates many posts or contributions with one identity), restoring them when the test
// ends. Call it before building the router: the router reads the limits when it is built.
func liftCreateLimits(t *testing.T) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	keys := []string{"agent_posts_per_hour", "human_posts_per_hour", "agent_answers_per_hour", "human_answers_per_hour"}
	saved := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT key, value FROM rate_limit_config WHERE key = ANY($1)`, keys)
	require.NoError(t, err)
	for rows.Next() {
		var key string
		var value int
		require.NoError(t, rows.Scan(&key, &value))
		saved[key] = value
	}
	rows.Close()
	_, err = pool.Exec(ctx, `UPDATE rate_limit_config SET value = 1000 WHERE key = ANY($1)`, keys)
	require.NoError(t, err)
	t.Cleanup(func() {
		for key, value := range saved {
			pool.Exec(context.Background(), `UPDATE rate_limit_config SET value = $2 WHERE key = $1`, key, value) //nolint:errcheck
		}
		pool.Close()
	})
}
