package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// POST /v1/analytics/pageview (SPEC.md 27.6, 27.7): the site's web server reports each page it
// serves, consent-free. The API classifies the view and keeps one row of what kind of page it
// was, who it came from and when: never the path, an address, the user agent or a cookie.

func newPageViewHandler(t *testing.T) (*PageViewHandler, *db.Pool) {
	t.Helper()
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	return NewPageViewHandler(db.NewPageViewRepository(pool), ""), pool
}

// countingPageViews is a page view store that counts what it is given.
type countingPageViews struct{ n int }

func (c *countingPageViews) Record(context.Context, string, db.TrafficLabel) error {
	c.n++
	return nil
}

// F6 (SPEC.md 27.6): with WEB_REPORT_SECRET set, a view is taken only from the site's web
// server, which sends that secret as X-Solvr-Web-Report. Anything else is refused before its
// body is read, and nothing reaches the store.
func TestPageView_WithASecretTakesOnlyTheWebServersReports(t *testing.T) {
	const secret = "web-report-secret-for-tests"
	view := `{"path":"/rooms/planner-room","user_agent":"` + browserUA + `","request_mode":"navigate","fetch_dest":"document"}`
	for _, c := range []struct {
		name, header, body, answer string
		status, stored             int
	}{
		{name: "no header", body: view, status: http.StatusUnauthorized},
		{name: "a wrong secret", header: "not-the-secret", body: view, status: http.StatusUnauthorized},
		{name: "a malformed report without the secret", body: `{"path":`, status: http.StatusUnauthorized},
		{name: "the secret", header: secret, body: view, status: http.StatusAccepted, stored: 1, answer: `{"data":{"recorded":true}}`},
		{name: "the secret on a prefetch", header: secret, body: `{"path":"/blog","prefetch":true}`, status: http.StatusAccepted, answer: `{"data":{"recorded":false}}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			store := &countingPageViews{}
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/analytics/pageview", strings.NewReader(c.body))
			if c.header != "" {
				req.Header.Set("X-Solvr-Web-Report", c.header)
			}
			rec := httptest.NewRecorder()
			NewPageViewHandler(store, secret).Record(rec, req)

			require.Equal(t, c.status, rec.Code, rec.Body.String())
			assert.Equal(t, c.stored, store.n, "views given to the store")
			if c.status != http.StatusUnauthorized {
				require.JSONEq(t, c.answer, rec.Body.String())
				return
			}
			var out struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			assert.Equal(t, "UNAUTHORIZED", out.Error.Code)
		})
	}
}

// Without WEB_REPORT_SECRET a view is taken from anyone, as before F6.
func TestPageView_WithoutASecretTakesAnyReport(t *testing.T) {
	store := &countingPageViews{}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/analytics/pageview",
		strings.NewReader(`{"path":"/","user_agent":"curl/8.7.1"}`))
	req.Header.Set("X-Solvr-Web-Report", "anything")
	rec := httptest.NewRecorder()
	NewPageViewHandler(store, "").Record(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"data":{"recorded":true}}`, rec.Body.String())
	assert.Equal(t, 1, store.n)
}

func reportPageView(t *testing.T, h *PageViewHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/analytics/pageview", strings.NewReader(body))
	req.Header.Set("User-Agent", "solvr-web/1.0 (pageview report)")
	rec := httptest.NewRecorder()
	h.Record(rec, req)
	return rec
}

func pageViewCount(t *testing.T, pool *db.Pool, group, class, reason string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT count(*) FROM page_view_events
		 WHERE content_group = $1 AND traffic_class = $2 AND traffic_reason IS NOT DISTINCT FROM $3`,
		group, class, reason).Scan(&n))
	return n
}

func TestPageView_RecordsAClassifiedViewOfAKindOfPage(t *testing.T) {
	h, pool := newPageViewHandler(t)
	for _, c := range []struct {
		name, body, group, class, reason string
	}{
		{"a person opening a room", `{"path":"/rooms/planner-room","user_agent":"` + browserUA + `","request_mode":"navigate","fetch_dest":"document","accept_language":true}`, "room", "human", "browser"},
		{"a client-side navigation", `{"path":"/posts/3f2c1d1e-0000-4000-8000-000000000001","user_agent":"` + browserUA + `","request_mode":"cors","fetch_dest":"empty","rsc":true,"accept_language":true}`, "post", "human", "browser"},
		{"curl", `{"path":"/","user_agent":"curl/8.7.1"}`, "home", "automation", "no_browser_headers"},
		{"a browser user agent without fetch headers", `{"path":"/docs/guides/codex","user_agent":"` + browserUA + `"}`, "guide", "automation", "no_browser_headers"},
		{"a crawler", `{"path":"/rooms","user_agent":"` + googleUA + `","request_mode":"navigate"}`, "collection", "crawler", "declared_bot"},
		{"a headless browser", `{"path":"/connect","user_agent":"` + headlessUA + `","request_mode":"navigate","fetch_dest":"document"}`, "connect", "automation", "headless_marker"},
		{"an assistant", `{"path":"/rooms/planner-room/history/2","user_agent":"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot"}`, "transcript", "agent", "assistant_ua"},
	} {
		t.Run(c.name, func(t *testing.T) {
			before := pageViewCount(t, pool, c.group, c.class, c.reason)
			rec := reportPageView(t, h, c.body)
			require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
			require.JSONEq(t, `{"data":{"recorded":true}}`, rec.Body.String())
			assert.Equal(t, before+1, pageViewCount(t, pool, c.group, c.class, c.reason))
		})
	}
}

// A prefetch is not a view: the page was fetched in case it is opened.
func TestPageView_DropsAPrefetch(t *testing.T) {
	h, pool := newPageViewHandler(t)
	before := pageViewCount(t, pool, "blog", "human", "browser")
	rec := reportPageView(t, h, `{"path":"/blog","user_agent":"`+browserUA+`","request_mode":"no-cors","fetch_dest":"document","prefetch":true}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"data":{"recorded":false}}`, rec.Body.String())
	assert.Equal(t, before, pageViewCount(t, pool, "blog", "human", "browser"))
}

func TestPageView_RefusesAMalformedReport(t *testing.T) {
	h, _ := newPageViewHandler(t)
	long := strings.Repeat("a", 201)
	for name, body := range map[string]string{
		"not JSON":                  `{"path":`,
		"no path":                   `{"user_agent":"curl/8.7.1"}`,
		"a path that is not a path": `{"path":"rooms/x"}`,
		"a path too long":           `{"path":"/` + strings.Repeat("p", 512) + `"}`,
		"a user agent too long":     `{"path":"/","user_agent":"` + long + `"}`,
		"client hints too long":     `{"path":"/","sec_ch_ua":"` + long + `"}`,
		"a request mode too long":   `{"path":"/","request_mode":"` + strings.Repeat("m", 21) + `"}`,
		"a fetch dest too long":     `{"path":"/","fetch_dest":"` + strings.Repeat("d", 21) + `"}`,
	} {
		rec := reportPageView(t, h, body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
		var out struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), name)
		assert.Equal(t, "VALIDATION_ERROR", out.Error.Code, name)
	}
}

// The table keeps no path, no address, no user agent and no cookie: five columns.
func TestPageView_StoresNothingThatNamesAVisitor(t *testing.T) {
	_, pool := newPageViewHandler(t)
	rows, err := pool.Query(context.Background(), `
		SELECT column_name FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = 'page_view_events' ORDER BY ordinal_position`)
	require.NoError(t, err)
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		columns = append(columns, c)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"id", "content_group", "traffic_class", "traffic_reason", "occurred_at"}, columns)
}
