package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/traffic"
)

// A stored search says how it was entered and who sent it (SPEC.md 27.6). A term a link
// brought (the tag links a crawler walks) is stored as entry link and is not counted as a
// search; a term the visitor typed is typed; a call that names neither is the API's.

// analyticsLog keeps the searches the handler stores, for the tests below.
type analyticsLog struct{ got chan models.SearchQuery }

func newAnalyticsLog() *analyticsLog { return &analyticsLog{got: make(chan models.SearchQuery, 4)} }

func (a *analyticsLog) Insert(_ context.Context, sq models.SearchQuery) error { //nolint:gocritic // hugeParam: the interface takes the record by value
	a.got <- sq
	return nil
}

func (a *analyticsLog) wait(t *testing.T) models.SearchQuery {
	t.Helper()
	select {
	case sq := <-a.got:
		return sq
	case <-time.After(2 * time.Second):
		t.Fatal("no search was stored")
		return models.SearchQuery{}
	}
}

func searchWith(ctx context.Context, t *testing.T, h *SearchHandler, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	h.Search(w, req)
	return w
}

var browserHeaders = map[string]string{"User-Agent": browserUA, "Sec-Fetch-Mode": "cors"}

func TestSearch_StoresHowTheSearchWasEntered(t *testing.T) {
	for param, want := range map[string]string{"&entry=link": "link", "&entry=typed": "typed", "&entry=api": "api", "": "api"} {
		h := NewSearchHandler(NewMockSearchRepository())
		log := newAnalyticsLog()
		h.SetAnalyticsRepo(log)
		w := searchWith(context.Background(), t, h, "/v1/search?q=deadlock"+param, browserHeaders)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body SearchResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Empty(t, body.Meta.Warnings, "entry is a parameter the search reads")
		assert.Equal(t, want, log.wait(t).Entry, "%q", param)
	}
	assert.Contains(t, SearchParamNames(), "entry")
}

func TestSearch_RefusesAnEntryItDoesNotKnow(t *testing.T) {
	repo := NewMockSearchRepository()
	h := NewSearchHandler(repo)
	failures := newFailureLog()
	h.SetFailureRecorder(failures)
	w := searchWith(context.Background(), t, h, "/v1/search?q=deadlock&entry=crawler", browserHeaders)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "VALIDATION_ERROR", searchFailureCode(t, w))
	assert.Empty(t, repo.searchQuery, "the search is not run")
	rows := failures.wait(t, 1)
	assert.Equal(t, "VALIDATION_ERROR", rows[0].ErrorCode)
}

func TestSearch_StoresWhoSentIt(t *testing.T) {
	agentCtx := auth.ContextWithAgent(context.Background(), &models.Agent{ID: "agent_traffic_test"})
	for _, c := range []struct {
		name    string
		headers map[string]string
		ctx     context.Context
		class   traffic.Class
		reason  traffic.Reason
	}{
		{"a browser", browserHeaders, context.Background(), traffic.Human, traffic.ReasonBrowser},
		{"curl", map[string]string{"User-Agent": "curl/7.76.1"}, context.Background(), traffic.Agent, traffic.ReasonHTTPClient},
		{"a browser user agent without fetch headers", map[string]string{"User-Agent": browserUA}, context.Background(), traffic.Agent, traffic.ReasonHTTPClient},
		{"an agent key", map[string]string{"User-Agent": "python-requests/2.32"}, agentCtx, traffic.Agent, traffic.ReasonAgentCredential},
		{"an impossible iPhone", map[string]string{"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/26.5 Mobile/15A372 Safari/604.1", "Sec-Fetch-Mode": "cors"}, context.Background(), traffic.Automation, traffic.ReasonImpossibleUA},
		{"a crawler", map[string]string{"User-Agent": googleUA}, context.Background(), traffic.Crawler, traffic.ReasonDeclaredBot},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := NewSearchHandler(NewMockSearchRepository())
			log := newAnalyticsLog()
			h.SetAnalyticsRepo(log)
			w := searchWith(c.ctx, t, h, "/v1/search?q=deadlock", c.headers)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			sq := log.wait(t)
			assert.Equal(t, string(c.class), sq.TrafficClass)
			assert.Equal(t, string(c.reason), sq.TrafficReason)
		})
	}
}
