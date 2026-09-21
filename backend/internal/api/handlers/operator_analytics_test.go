package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Who may read what Solvr measures about itself.
//
// These tests are about the gate, not about any one report. The gate is the
// only thing standing between an operator report and a visitor, so it is
// tested the way an attacker would probe it: with every credential the product
// issues, with a key that is nearly right, and with the server configured
// badly.

const testOperatorKey = "operator-key-for-tests-0123456789"

// okHandler records whether the protected handler was ever reached.
func okHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"summary":{"total_searches":4210}}`))
	})
}

// Every report says what it reports. A surface nobody can describe is a
// surface nobody reviewed.
func TestOperatorReports_NameWhatTheyPublishAndWhereTheyLive(t *testing.T) {
	require.NotEmpty(t, OperatorReports, "the operator reports must be registered somewhere the router and the tests both read")

	seen := map[string]bool{}
	for _, report := range OperatorReports {
		assert.Contains(t, []string{http.MethodGet, http.MethodPost}, report.Method,
			"report %q must name its HTTP method", report.Path)
		assert.True(t, strings.HasPrefix(report.Path, "/admin/"),
			"report %q must live under /admin, away from every public route", report.Path)
		assert.NotEmpty(t, report.Reports, "report %q must say what it publishes", report.Path)

		key := report.Method + " " + report.Path
		assert.False(t, seen[key], "report %q is registered twice", key)
		seen[key] = true
	}
}

// Not one credential the product issues opens an operator report. A human
// account, an agent key, a user key and a room token are all credentials for
// using Solvr; none of them is a credential for reading Solvr's own numbers.
func TestRequireOperatorAccess_RefusesEveryCredentialThatIsNotTheOperatorKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", testOperatorKey)

	callers := []struct {
		name    string
		headers map[string]string
	}{
		{"anonymous", nil},
		{"a human account", map[string]string{"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.human.signature"}},
		{"an agent API key", map[string]string{"Authorization": "Bearer solvr_agentkeyagentkeyagentkey"}},
		{"an agent API key in its own header", map[string]string{"X-API-Key": "solvr_agentkeyagentkeyagentkey"}},
		{"a user API key", map[string]string{"Authorization": "Bearer solvr_sk_userkeyuserkeyuserkey"}},
		{"a room-scoped token", map[string]string{"Authorization": "Bearer room_tokenroomtokenroomtoken"}},
		{"a room owner who also holds a room token", map[string]string{
			"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.owner.signature",
			"X-Room-Token":  "room_tokenroomtokenroomtoken",
		}},
		{"an empty operator header", map[string]string{OperatorAccessHeader: ""}},
		{"a wrong operator key", map[string]string{OperatorAccessHeader: "not-the-key"}},
		{"a key that is a prefix of the real one", map[string]string{OperatorAccessHeader: testOperatorKey[:10]}},
		{"a key that differs only in the last byte", map[string]string{OperatorAccessHeader: testOperatorKey[:len(testOperatorKey)-1] + "X"}},
		{"a key that differs only in the first byte", map[string]string{OperatorAccessHeader: "X" + testOperatorKey[1:]}},
		{"a key with trailing whitespace", map[string]string{OperatorAccessHeader: testOperatorKey + " "}},
	}

	for _, caller := range callers {
		t.Run(caller.name, func(t *testing.T) {
			reached := false
			req := httptest.NewRequest(http.MethodGet, "/admin/search-analytics/summary", nil)
			for name, value := range caller.headers {
				req.Header.Set(name, value)
			}
			rec := httptest.NewRecorder()

			RequireOperatorAccess(okHandler(&reached)).ServeHTTP(rec, req)

			assert.False(t, reached, "the report ran for %s", caller.name)
			assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rec.Code,
				"%s must be refused, got %d", caller.name, rec.Code)
			assert.NotContains(t, rec.Body.String(), "total_searches",
				"the refusal leaked the report to %s", caller.name)
		})
	}
}

// The operator's own key is the one that works, and the report runs unchanged.
func TestRequireOperatorAccess_ServesTheOperator(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", testOperatorKey)

	reached := false
	req := httptest.NewRequest(http.MethodGet, "/admin/search-analytics/summary", nil)
	req.Header.Set(OperatorAccessHeader, testOperatorKey)
	rec := httptest.NewRecorder()

	RequireOperatorAccess(okHandler(&reached)).ServeHTTP(rec, req)

	assert.True(t, reached, "the operator's own key must reach the report")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "total_searches")
}

// A server with no operator key configured is CLOSED, not open. An empty
// secret must never compare equal to an empty header.
func TestRequireOperatorAccess_WithNoKeyConfiguredRefusesEveryone(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "")

	for _, header := range []string{"", "anything"} {
		t.Run("header="+header, func(t *testing.T) {
			reached := false
			req := httptest.NewRequest(http.MethodGet, "/admin/search-analytics/summary", nil)
			req.Header.Set(OperatorAccessHeader, header)
			rec := httptest.NewRecorder()

			RequireOperatorAccess(okHandler(&reached)).ServeHTTP(rec, req)

			assert.False(t, reached, "an unconfigured server ran the report anyway")
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		})
	}
}

// A refusal explains itself in the shape the rest of the API uses, and says
// nothing about the report it refused.
func TestRequireOperatorAccess_RefusalIsAReadableError(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", testOperatorKey)

	cases := []struct {
		name   string
		key    string
		status int
		code   string
	}{
		{"no credential at all", "", http.StatusUnauthorized, "MISSING_API_KEY"},
		{"the wrong credential", "not-the-key", http.StatusForbidden, "INVALID_API_KEY"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			req := httptest.NewRequest(http.MethodGet, "/admin/search-analytics/trending", nil)
			if tc.key != "" {
				req.Header.Set(OperatorAccessHeader, tc.key)
			}
			rec := httptest.NewRecorder()

			RequireOperatorAccess(okHandler(&reached)).ServeHTTP(rec, req)

			require.Equal(t, tc.status, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			errObj, ok := body["error"].(map[string]any)
			require.True(t, ok, "a refusal must carry an error object")
			assert.Equal(t, tc.code, errObj["code"])
			assert.NotEmpty(t, errObj["message"])
		})
	}
}

// An operator report must never become a cache entry. Whether it was served or
// refused, the answer is unstorable, private, and keyed on the credential — so
// no shared cache in front of the API can hold one and hand it to a visitor.
func TestRequireOperatorAccess_AnswersAreNeverStorableAndNeverShared(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", testOperatorKey)

	cases := []struct {
		name string
		key  string
	}{
		{"served to the operator", testOperatorKey},
		{"refused to a visitor", ""},
		{"refused to a wrong key", "not-the-key"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			req := httptest.NewRequest(http.MethodGet, "/admin/search-analytics/summary", nil)
			if tc.key != "" {
				req.Header.Set(OperatorAccessHeader, tc.key)
			}
			rec := httptest.NewRecorder()

			RequireOperatorAccess(okHandler(&reached)).ServeHTTP(rec, req)

			cacheControl := rec.Header().Get("Cache-Control")
			assert.Contains(t, cacheControl, "no-store", "an operator answer must not be stored")
			assert.Contains(t, cacheControl, "private", "an operator answer belongs to one caller")
			assert.NotContains(t, cacheControl, "public", "an operator answer must never be publicly cacheable")
			assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))

			vary := rec.Header().Get("Vary")
			assert.Contains(t, vary, OperatorAccessHeader, "the cache key must include the operator credential")
			assert.Contains(t, vary, "Authorization", "the cache key must include the caller's credential")
		})
	}
}

// The same policy is available to any handler that answers with operator
// material outside the middleware.
func TestApplyOperatorReportCachePolicy_SetsThePrivatePolicy(t *testing.T) {
	rec := httptest.NewRecorder()
	ApplyOperatorReportCachePolicy(rec)

	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	assert.NotContains(t, rec.Header().Get("Cache-Control"), "public")
	assert.Contains(t, rec.Header().Get("Vary"), OperatorAccessHeader)
}

// Google Analytics and Search Console are the operator's own instruments. The
// tag that collects a page view is public by design — it runs in the browser —
// but the credentials, the reporting property and the exports are not, and
// naming them is how the public payload tests find them.
func TestOperatorCredentialTermIn_NamesReportingCredentialsNotTheCollectionTag(t *testing.T) {
	// A field name is API-owned, so snake_case is read as words.
	fields := []struct {
		name string
		key  string
	}{
		{"a GA4 property", "ga4_property_id"},
		{"a service account", "service_account"},
		{"an analytics export", "analytics_export"},
	}
	for _, tc := range fields {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, OperatorCredentialTermInKey(tc.key),
				"the field %q is operator material and must be named", tc.key)
		})
	}

	found := []struct {
		name string
		text string
	}{
		{"a reporting property resource", "properties/123456789"},
		{"Google Analytics reporting", "google analytics data api"},
		{"Search Console", "search console impressions report"},
		{"the webmasters API", "webmasters.googleapis.com"},
		{"a service account", "service_account"},
		{"a client secret", "client_secret"},
		{"a reporting key pasted in", "-----BEGIN PRIVATE KEY-----"},
		{"an analytics export", "analytics_export.csv"},
		{"a traffic export", "traffic export"},
		{"a warehouse copy", "bigquery"},
	}
	for _, tc := range found {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, OperatorCredentialTermIn(tc.text),
				"%q is operator material and must be named", tc.text)
		})
	}

	allowed := []struct {
		name string
		text string
	}{
		{"a public room metric", "rooms_with_conversation"},
		{"aggregate API usage", "api_calls_succeeded"},
		{"eligible search activity", "agent_searches"},
		{"exporting a post, which is a product feature", "export this problem as markdown"},
		{"a key that belongs to a caller, not to an operator", "api_key"},
		{"the private key an agent holds for its own identity", "only the agent holding the private key can decrypt"},
		{"the refresh token Solvr issues to a person", "refresh_token"},
		{"the word property in ordinary prose", "a property of the answer"},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, OperatorCredentialTermIn(tc.text),
				"%q is not operator material and must not be flagged", tc.text)
		})
	}
}
