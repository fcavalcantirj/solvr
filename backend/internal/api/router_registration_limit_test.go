package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// Agent registration is limited per client IP (spec.json idx 79 step 5: rate limits that
// actually reject). The limit is apimiddleware.DefaultRegistrationsPerIPPerHour, overridable
// by RATE_LIMIT_REGISTRATIONS_PER_IP_HOUR when the router is built. The registration after the
// limit, from the same IP, gets the documented 429; a different IP is not affected.

type registrationAnswer struct {
	status     int
	retryAfter string
	body       map[string]any
}

func registerFrom(t *testing.T, ts *httptest.Server, pool *db.Pool, ip string) registrationAnswer {
	t.Helper()
	name := "rl_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register", strings.NewReader(fmt.Sprintf(`{"name":%q}`, name)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", ip)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if resp.StatusCode == http.StatusCreated {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM agents WHERE id = 'agent_' || $1`, name) }) //nolint:errcheck
	}
	return registrationAnswer{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After"), body: body}
}

func assertRegistrationRefused(t *testing.T, got registrationAnswer, limit int) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, got.status, "%v", got.body)
	assert.NotEmpty(t, got.retryAfter, "a refused registration says when to retry")
	errBody, _ := got.body["error"].(map[string]any)
	assert.Equal(t, "RATE_LIMITED", errBody["code"])
	details, _ := errBody["details"].(map[string]any)
	assert.Equal(t, float64(limit), details["limit"])
	assert.Equal(t, "1h0m0s", details["window"])
}

func TestRegistrationLimit_TheRegistrationAfterTheDefaultLimitFromOneIPIsRefused(t *testing.T) {
	t.Setenv(registrationLimitEnv, "") // the default in code, whatever the test process set
	ts, pool, _ := newBillingFreeServer(t)

	limit := apimiddleware.DefaultRegistrationsPerIPPerHour
	for i := 0; i < limit; i++ {
		got := registerFrom(t, ts, pool, "203.0.113.7")
		require.Equal(t, http.StatusCreated, got.status, "registration %d of %d: %v", i+1, limit, got.body)
	}
	assertRegistrationRefused(t, registerFrom(t, ts, pool, "203.0.113.7"), limit)

	other := registerFrom(t, ts, pool, "203.0.113.8")
	assert.Equal(t, http.StatusCreated, other.status, "another IP is not limited by the first: %v", other.body)
}

func TestRegistrationLimit_TheEnvironmentSetsTheLimit(t *testing.T) {
	t.Setenv(registrationLimitEnv, "3")
	ts, pool, _ := newBillingFreeServer(t)

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusCreated, registerFrom(t, ts, pool, "198.51.100.4").status)
	}
	assertRegistrationRefused(t, registerFrom(t, ts, pool, "198.51.100.4"), 3)
}

func TestRegistrationLimit_AnUnusableEnvironmentValueKeepsTheDefault(t *testing.T) {
	for _, v := range []string{"", "0", "-4", "twenty"} {
		t.Setenv(registrationLimitEnv, v)
		assert.Equal(t, apimiddleware.DefaultRegistrationsPerIPPerHour, registrationLimitPerIPPerHour(), "%q", v)
	}
	t.Setenv(registrationLimitEnv, "7")
	assert.Equal(t, 7, registrationLimitPerIPPerHour())
}
