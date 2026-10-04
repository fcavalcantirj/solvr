package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
)

// What a visitor can read about Solvr, proven against the real router.
//
// Solvr publishes PRODUCT ACTIVITY — rooms, participation, eligible search,
// aggregate API usage and labeled product totals — and keeps WEBSITE TRAFFIC
// AND GROWTH INTELLIGENCE private. The danger is not the homepage drawing a
// traffic chart; it is an internal analytics field being serialized and then
// merely not drawn, where curl still finds it. So these tests read the bytes
// the API actually returns with NO credentials and walk the whole payload.
//
// The allowlist and the excluded vocabulary live in
// handlers/public_overview_allowlist.go, which is the one place either can
// change.

// publicPrivacyEndpoints is every public statistics surface a visitor can call
// without any account at all.
var publicPrivacyEndpoints = []string{
	"/v1/homepage/overview",
	"/v1/homepage/overview?window=7d",
	"/v1/homepage/overview?window=30d",
	"/v1/homepage/rooms",
	"/v1/homepage/search",
	"/v1/homepage/api-usage",
	"/v1/homepage/activity?offset=0&limit=6",
	"/v1/stats",
	"/v1/stats/search",
	"/v1/data/breakdown",
	"/v1/data/trending",
	"/v1/data/categories",
}

// No public statistics surface may carry an audience or growth idea — not in a
// field name anywhere in the payload, and not in a sentence the API wrote.
func TestPublicStatisticsEndpoints_PublishNoAudienceOrGrowthVocabulary(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	for _, endpoint := range publicPrivacyEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			payload, raw := getPublicJSON(t, ts.URL+endpoint)
			assert.NotEmpty(t, raw)
			walkPublicResponse(t, endpoint, payload)
		})
	}
}

// Every number the overview publishes is a number somebody named as
// publishable. A metric that is not on the allowlist never reaches a visitor,
// because the API withholds it rather than trusting the browser not to draw it.
func TestPublicOverview_PublishesOnlyAllowlistedSectionsAndMetrics(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)
	// The featured rooms section is published only when a room is featured (SPEC Part 26).
	featured := hpoSlug("allow")
	hpoSeedRoom(t, pool, featured, "Allowlist Room", "a featured room", false, []string{"the ask", "the outcome"})
	featureOnHomepage(t, pool, featured)

	payload, _ := getPublicJSON(t, ts.URL+"/v1/homepage/overview")
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "the overview must answer with a data object")

	for section := range data {
		purpose, allowed := handlers.PublicOverviewSections[section]
		assert.True(t, allowed, "the overview published section %q, which is not allowlisted", section)
		assert.NotEmpty(t, purpose, "section %q must state what it may carry", section)
	}
	for section := range handlers.PublicOverviewSections {
		_, present := data[section]
		assert.True(t, present, "the allowlist names section %q, which the overview did not publish", section)
	}

	published := collectPublishedMetricKeys(t, data)
	require.NotEmpty(t, published, "the overview must publish some metrics to be worth testing")
	for _, key := range published {
		category, allowed := handlers.PublicOverviewAllowsMetric(key)
		assert.True(t, allowed, "the overview published metric %q, which is not allowlisted", key)
		assert.NotEmpty(t, category, "metric %q must be filed under a public category", key)
	}

	// The same rule on the two sections that are also served on their own.
	for _, endpoint := range []string{"/v1/homepage/rooms", "/v1/homepage/search", "/v1/homepage/api-usage"} {
		sectionPayload, _ := getPublicJSON(t, ts.URL+endpoint)
		sectionData, sectionOK := sectionPayload["data"].(map[string]any)
		require.True(t, sectionOK, "%s must answer with a data object", endpoint)
		for _, key := range collectPublishedMetricKeys(t, sectionData) {
			_, allowed := handlers.PublicOverviewAllowsMetric(key)
			assert.True(t, allowed, "%s published metric %q, which is not allowlisted", endpoint, key)
		}
	}
}

// The registration totals are the figures a reader is most likely to mistake
// for a website audience, so the live payload has to label them as cumulative
// product accounts and say that registering is not using.
func TestPublicOverview_RegistrationTotalsReadAsProductAccountsNotAudience(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	payload, _ := getPublicJSON(t, ts.URL+"/v1/homepage/overview")
	data := payload["data"].(map[string]any)
	community, ok := data["community"].(map[string]any)
	require.True(t, ok, "the overview must publish the all-time section")

	metrics := map[string]map[string]any{}
	for _, entry := range community["metrics"].([]any) {
		metric := entry.(map[string]any)
		metrics[metric["key"].(string)] = metric
	}

	for _, key := range []string{"registered_agents", "registered_humans"} {
		metric, present := metrics[key]
		require.True(t, present, "the all-time section must carry %q", key)
		assert.Equal(t, "all time", metric["window"], "%s window", key)
		assert.NotEmpty(t, metric["qualifier"], "%s must state that a registration is not a measure of use", key)
		for _, field := range []string{"label", "definition", "qualifier"} {
			text, _ := metric[field].(string)
			assert.Empty(t, handlers.PrivateAnalyticsTermIn(text), "%s %s: %q", key, field, text)
		}
	}
}

// getPublicJSON calls an endpoint with no credentials at all and returns the
// decoded body.
func getPublicJSON(t *testing.T, url string) (map[string]any, string) {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec,noctx // test server URL
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s body: %s", url, string(body))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload), "%s body: %s", url, string(body))
	return payload, string(body)
}

// collectPublishedMetricKeys finds every metric object in a payload, wherever
// a section chose to put it.
func collectPublishedMetricKeys(t *testing.T, node any) []string {
	t.Helper()
	var keys []string

	switch v := node.(type) {
	case map[string]any:
		// A metric object is the one shape that carries key + label + display.
		if key, hasKey := v["key"].(string); hasKey {
			_, hasLabel := v["label"]
			_, hasDisplay := v["display"]
			if hasLabel && hasDisplay {
				keys = append(keys, key)
			}
		}
		for _, child := range v {
			keys = append(keys, collectPublishedMetricKeys(t, child)...)
		}
	case []any:
		for _, child := range v {
			keys = append(keys, collectPublishedMetricKeys(t, child)...)
		}
	}
	return keys
}

// walkPublicResponse checks every field name in a response, and the text of
// every field the API itself wrote. Participant content — message excerpts,
// room names, post titles, search terms — is governed by the publishing rules
// that quote it, not by this vocabulary: a public room is allowed to be about
// traffic analysis.
func walkPublicResponse(t *testing.T, path string, node any) {
	t.Helper()

	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			here := key
			if path != "" {
				here = path + "." + key
			}
			assert.Empty(t, handlers.PrivateAnalyticsTermInKey(key), "field %s", here)
			if text, isText := child.(string); isText && handlers.PublicOverviewAuthoredText(key) {
				assert.Empty(t, handlers.PrivateAnalyticsTermIn(text), "field %s: %q", here, text)
			}
			walkPublicResponse(t, here, child)
		}
	case []any:
		for _, child := range v {
			walkPublicResponse(t, path+"[]", child)
		}
	}
}
