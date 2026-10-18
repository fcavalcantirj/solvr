package api

import (
	"net/http"
	"testing"
)

// F6 (SPEC.md 27.6): WEB_REPORT_SECRET, read when the router is built, makes the page-view
// report take only the site's web server's reports. A prefetch is used so no row is written.
func TestRouter_PageViewReport_RequiresTheWebServersSecretWhenSet(t *testing.T) {
	const secret = "router-web-report-secret"
	t.Setenv("WEB_REPORT_SECRET", secret)
	h := setupTestRouter(t)
	body := `{"path":"/","prefetch":true}`

	if rec := serve(h, http.MethodPost, "/v1/analytics/pageview", body, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("without the header: %d %s, want 401", rec.Code, rec.Body.String())
	}
	if rec := serve(h, http.MethodPost, "/v1/analytics/pageview", body,
		map[string]string{"X-Solvr-Web-Report": secret}); rec.Code != http.StatusAccepted {
		t.Errorf("with the header: %d %s, want 202", rec.Code, rec.Body.String())
	}
}
