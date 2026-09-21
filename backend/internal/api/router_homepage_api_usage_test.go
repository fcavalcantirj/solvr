package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Aggregate API usage, proven against the real router.
//
// The section is only worth publishing if the numbers behind it come from
// requests that actually arrived, so these tests make real calls and read
// what the public endpoint then says about them. They also hold the line that
// matters most for a self-measuring page: reading the statistics must not
// raise the statistics.

// apiUsageMetrics reads the metric values of the API activity section by key.
func apiUsageMetrics(t *testing.T, section map[string]any) map[string]float64 {
	t.Helper()
	values := map[string]float64{}
	metrics, ok := section["metrics"].([]any)
	require.True(t, ok, "the section must publish metrics")
	for _, entry := range metrics {
		metric, isObject := entry.(map[string]any)
		require.True(t, isObject)
		key, _ := metric["key"].(string)
		value, _ := metric["value"].(float64)
		values[key] = value
	}
	return values
}

// readAPIUsage reads GET /v1/homepage/api-usage with no credentials at all.
func readAPIUsage(t *testing.T, baseURL, query string) map[string]any {
	t.Helper()
	payload, _ := getPublicJSON(t, baseURL+"/v1/homepage/api-usage"+query)
	section, ok := payload["data"].(map[string]any)
	require.True(t, ok, "the endpoint must answer with a data object")
	return section
}

func TestHomepageAPIUsage_IsPublicAndStatesItsWindow(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	section := readAPIUsage(t, ts.URL, "")
	assert.Equal(t, "24h", section["selected_window"], "the default window is 24 hours")

	for _, window := range []string{"7d", "30d"} {
		selected := readAPIUsage(t, ts.URL, "?window="+window)
		assert.Equal(t, window, selected["selected_window"])
	}

	// An unoffered window falls back to the default, and the answer says
	// which window it actually measured.
	fallback := readAPIUsage(t, ts.URL, "?window=all-time")
	assert.Equal(t, "24h", fallback["selected_window"])

	options, ok := section["window_options"].([]any)
	require.True(t, ok)
	assert.Len(t, options, 3)
	assert.NotEmpty(t, section["scope_note"])
	assert.NotEmpty(t, section["series"])
}

// A call that arrived is a call that counts.
func TestHomepageAPIUsage_CountsRequestsThatActuallyArrived(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	before := apiUsageMetrics(t, readAPIUsage(t, ts.URL, ""))

	const calls = 3
	for i := 0; i < calls; i++ {
		resp, err := http.Get(ts.URL + "/v1/rooms") //nolint:gosec,noctx // test server URL
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	waitForRecordedCalls(t)
	after := apiUsageMetrics(t, readAPIUsage(t, ts.URL, ""))

	assert.GreaterOrEqual(t, after["api_calls_succeeded"]-before["api_calls_succeeded"], float64(calls),
		"three successful room reads must reach the published call volume")
	assert.GreaterOrEqual(t, after["passive_poll_calls"]-before["passive_poll_calls"], float64(calls),
		"a read is passive polling, reported apart from create/send/search")
	assert.GreaterOrEqual(t, after["room_knowledge_operations"], float64(1),
		"listing rooms is a room operation")
}

// Reading the statistics may not raise the statistics, and neither may a
// health check.
func TestHomepageAPIUsage_DoesNotCountTheOverviewOrHealthChecks(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	before := apiUsageMetrics(t, readAPIUsage(t, ts.URL, ""))

	for _, path := range []string{
		"/v1/homepage/overview", "/v1/homepage/rooms", "/v1/homepage/search",
		"/v1/homepage/api-usage", "/health", "/health/live",
	} {
		resp, err := http.Get(ts.URL + path) //nolint:gosec,noctx // test server URL
		require.NoError(t, err)
		resp.Body.Close()
	}

	waitForRecordedCalls(t)
	after := apiUsageMetrics(t, readAPIUsage(t, ts.URL, ""))

	assert.Equal(t, before["api_calls_succeeded"], after["api_calls_succeeded"],
		"displaying the statistics must not inflate them")
}

// The published payload carries aggregates and nothing that could be read
// back to a room, an account or one request.
func TestHomepageAPIUsage_PublishesNoRouteOrParticipantBreakdown(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	_, raw := getPublicJSON(t, ts.URL+"/v1/homepage/api-usage")

	for _, forbidden := range []string{
		"request_id", "route_template", "ip_address", "user_agent", "actor_id",
		"room_id", "room_slug", "account_id", "api_key", "authorization",
	} {
		assert.NotContains(t, raw, forbidden,
			"the public API-usage payload must not carry %q", forbidden)
	}
}

// waitForRecordedCalls waits out one flush of the boundary recorder.
//
// Recording is asynchronous on purpose — a statistic may not slow down the
// product it measures — so a test that wants to read what it just caused has
// to wait for the writer rather than assume it already ran.
func waitForRecordedCalls(t *testing.T) {
	t.Helper()
	time.Sleep(1500 * time.Millisecond)
}
