package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 85: the weekly SEO baseline is an operator-only report served from the real
// tables. Measured milestones carry values; the ones only Google Search Console can
// tell are listed as unavailable with their source, never as zero.
func TestSEOBaseline_OperatorReportFromRealTables(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)
	auth := map[string]string{handlers.OperatorAccessHeader: operatorTestKey}

	resp, raw := call(t, http.MethodGet, ts.URL+"/admin/seo/baseline?window=7d", auth, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", raw)
	assert.Contains(t, resp.Header.Get("Cache-Control"), "no-store")
	var payload struct {
		Data struct {
			Window     string           `json:"window"`
			Indexable  map[string]any   `json:"indexable"`
			Milestones []map[string]any `json:"milestones"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	assert.Equal(t, "7d", payload.Data.Window)
	assert.Contains(t, payload.Data.Indexable, "room_history_pages")
	assert.Contains(t, payload.Data.Indexable, "users", "people's profiles with content are offered for indexing too (SPEC.md 27.1)")
	status := map[string]string{}
	for _, m := range payload.Data.Milestones {
		status[m["name"].(string)] = m["status"].(string)
	}
	assert.Equal(t, map[string]string{
		"publication": "measured", "sitemap_acceptance": "unavailable", "crawling": "unavailable",
		"indexing": "unavailable", "traffic": "unavailable", "core_web_vitals": "unavailable",
		"activation": "measured",
	}, status)

	resp, _ = call(t, http.MethodGet, ts.URL+"/admin/seo/baseline?window=1y", auth, "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp, _ = call(t, http.MethodGet, ts.URL+"/admin/seo/baseline", nil, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
