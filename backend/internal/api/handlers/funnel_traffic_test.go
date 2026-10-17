package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/traffic"
)

// Every funnel step a client reports is classified as it is stored (SPEC.md 27.6): a browser
// step from what the browser's request said about itself, a skill fetch from what the web
// server saw of the request for the skill. The row keeps the class and the reason, never the
// headers they were read from.

const (
	browserUA  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	headlessUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36"
	googleUA   = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
)

func ingestWithHeaders(t *testing.T, h *FunnelHandler, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/analytics/funnel", strings.NewReader(body))
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	h.IngestBrowserEvent(rec, req)
	return rec
}

func TestFunnelIngest_ABrowserStepIsClassifiedFromItsRequest(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	for _, c := range []struct {
		name    string
		headers map[string]string
		class   traffic.Class
		reason  traffic.Reason
	}{
		{"a browser", map[string]string{"User-Agent": browserUA, "Sec-Fetch-Mode": "cors"}, traffic.Human, traffic.ReasonBrowser},
		{"no fetch headers", map[string]string{"User-Agent": browserUA}, traffic.Automation, traffic.ReasonNoBrowserHeaders},
		{"a headless browser", map[string]string{"User-Agent": headlessUA, "Sec-Fetch-Mode": "cors"}, traffic.Automation, traffic.ReasonHeadlessMarker},
		{"a headless client hint", map[string]string{"User-Agent": browserUA, "Sec-Fetch-Mode": "cors", "Sec-CH-UA": `"HeadlessChrome";v="141"`}, traffic.Automation, traffic.ReasonHeadlessMarker},
		{"a crawler", map[string]string{"User-Agent": googleUA, "Sec-Fetch-Mode": "navigate"}, traffic.Crawler, traffic.ReasonDeclaredBot},
	} {
		t.Run(c.name, func(t *testing.T) {
			flow := freshFlowCode(t, getTestPool(t))
			rec := ingestWithHeaders(t, h, `{"event":"connection_started","flow_id":"`+flow+`","entry_surface":"connect_page"}`, c.headers)
			require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
			events, err := repo.ListByFlow(context.Background(), flow)
			require.NoError(t, err)
			require.Len(t, events, 1)
			require.Equal(t, string(c.class), events[0].TrafficClass)
			require.Equal(t, string(c.reason), events[0].TrafficReason)
		})
	}
}

// A skill fetch is judged by the request for the skill, which the web server reports, and
// never by the web server's own report request.
func TestFunnelIngest_ASkillFetchIsClassifiedFromTheFetchOfTheSkill(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	for _, c := range []struct {
		name, requestMode, userAgent string
		surface                      string
		class                        traffic.Class
		reason                       traffic.Reason
	}{
		{"an agent's HTTP client", "", "curl/8.7.1", models.FunnelSurfaceAgentFetch, traffic.Agent, traffic.ReasonHTTPClient},
		{"no user agent", "", "", models.FunnelSurfaceAgentFetch, traffic.Agent, traffic.ReasonHTTPClient},
		{"an assistant", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot", models.FunnelSurfaceAgentFetch, traffic.Agent, traffic.ReasonAssistantUA},
		{"a person opening the link", "navigate", browserUA, models.FunnelSurfaceBrowserVisit, traffic.Human, traffic.ReasonBrowser},
		{"a crawler", "", googleUA, models.FunnelSurfaceBotFetch, traffic.Crawler, traffic.ReasonDeclaredBot},
	} {
		t.Run(c.name, func(t *testing.T) {
			flow := freshFlowCode(t, getTestPool(t))
			body, err := json.Marshal(map[string]string{"event": "skill_fetched", "flow_id": flow, "request_mode": c.requestMode, "user_agent": c.userAgent})
			require.NoError(t, err)
			rec := ingestWithHeaders(t, h, string(body), map[string]string{"User-Agent": "solvr-web/1.0 (skill-fetch report)"})
			require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
			events, err := repo.ListByFlow(context.Background(), flow)
			require.NoError(t, err)
			require.Len(t, events, 1)
			require.Equal(t, c.surface, events[0].EntrySurface)
			require.Equal(t, string(c.class), events[0].TrafficClass)
			require.Equal(t, string(c.reason), events[0].TrafficReason)
		})
	}
}
