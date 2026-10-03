package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
)

// The operations report (spec.json idx 79 step 1) is operator material: listed in
// handlers.OperatorReports (so the refusal walk in router_operator_analytics_test.go covers
// it for every product credential) and served from the real tables on the real router.

func TestOpsSLO_IsARegisteredOperatorReport(t *testing.T) {
	found := false
	for _, r := range handlers.OperatorReports {
		if r.Method == http.MethodGet && r.Path == "/admin/ops/slo" {
			found = true
		}
	}
	assert.True(t, found, "/admin/ops/slo is not in handlers.OperatorReports")
}

func TestOpsSLO_ServesTheOperatorFromRealTables(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)

	resp, raw := call(t, http.MethodGet, ts.URL+"/admin/ops/slo",
		map[string]string{handlers.OperatorAccessHeader: operatorTestKey}, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", raw)
	assert.Contains(t, resp.Header.Get("Cache-Control"), "no-store")

	var payload struct {
		Data struct {
			Targets []struct {
				Key    string `json:"key"`
				Status string `json:"status"`
			} `json:"targets"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	require.Len(t, payload.Data.Targets, 4)
	for _, target := range payload.Data.Targets {
		assert.Contains(t, []string{"met", "unmet", "not_yet_measurable"}, target.Status, target.Key)
	}

	resp, _ = call(t, http.MethodGet, ts.URL+"/admin/ops/slo", nil, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
