package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
)

// Growth reports are operator material (spec.json idx 86 step 7, idx 89 step 7): the
// participant counts, the one-million target, stage gates and acquisition planning are read
// with the operator key on the real router, are refused to every product credential (the
// OperatorReports walk in router_operator_analytics_test.go covers each path listed here), and
// their vocabulary is caught by the same check that keeps the public statistics clean.

var growthReportPaths = []string{
	"/admin/growth/participants",
	"/admin/growth/stages",
}

func TestGrowthReports_AreRegisteredOperatorReports(t *testing.T) {
	registered := map[string]bool{}
	for _, r := range handlers.OperatorReports {
		registered[r.Method+" "+r.Path] = true
	}
	for _, path := range growthReportPaths {
		assert.True(t, registered[http.MethodGet+" "+path], "%s is not in handlers.OperatorReports", path)
		assert.True(t, strings.HasPrefix(path, "/admin/"), "%s must live under /admin", path)
	}
}

func TestGrowthReports_ServeTheOperatorFromRealTables(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)

	for _, path := range growthReportPaths {
		t.Run(path, func(t *testing.T) {
			resp, raw := call(t, http.MethodGet, ts.URL+path,
				map[string]string{handlers.OperatorAccessHeader: operatorTestKey}, "")
			require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", raw)
			assert.Contains(t, resp.Header.Get("Cache-Control"), "no-store")

			var payload struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal([]byte(raw), &payload))
			assert.NotEmpty(t, payload.Data)

			resp, _ = call(t, http.MethodGet, ts.URL+path, nil, "")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// The public statistics walk (TestPublicStatisticsEndpoints_PublishNoAudienceOrGrowthVocabulary)
// refuses any field or sentence the private vocabulary names. These are the growth report's
// own words, so a leak of any of them onto a public surface fails that walk.
func TestGrowthReports_VocabularyIsPrivate(t *testing.T) {
	for _, key := range []string{
		"monthly_active_participant_target",
		"monthly active participants",
		"stage_gates",
		"participant goal",
	} {
		assert.NotEmpty(t, handlers.PrivateAnalyticsTermIn(strings.ReplaceAll(key, "_", " ")),
			"%q is growth planning and must be private vocabulary", key)
	}
}
