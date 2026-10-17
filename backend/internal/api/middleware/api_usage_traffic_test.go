package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The request log says which error answered and what kind of client asked (SPEC.md 27.6):
// error_code is read from the error envelope the answer carried, and ua_class from the
// User-Agent. Neither stores what identifies anybody.

func trafficUsageRouter(t *testing.T, spy *recordingSpy) *chi.Mux {
	t.Helper()
	r := chi.NewRouter()
	r.Use(APIUsage(spy))
	write := func(w http.ResponseWriter, body string) {
		if _, err := w.Write([]byte(body)); err != nil {
			t.Error(err)
		}
	}
	r.Get("/v1/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch chi.URLParam(r, "id") {
		case "missing":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			write(w, `{"error":{"code":"NOT_FOUND","message":"post not found"}}`)
		case "plain":
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		case "lowercase":
			w.WriteHeader(http.StatusConflict)
			write(w, `{"error":{"code":"conflict!","message":"x"}}`)
		case "closed":
			w.WriteHeader(499)
			write(w, `{"error":{"code":"CLIENT_CLOSED_REQUEST","message":"the client closed the request"}}`)
		case "long":
			w.WriteHeader(http.StatusBadRequest)
			write(w, `{"error":{"code":"VALIDATION_ERROR","message":"`+strings.Repeat("x", 6000)+`"}}`)
		case "empty":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			write(w, `{"data":{"id":"ok"}}`)
		}
	})
	r.Post("/v1/analytics/funnel", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	r.Post("/v1/analytics/pageview", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	return r
}

func TestAPIUsage_RecordsTheErrorCodeOfTheAnswer(t *testing.T) {
	for id, want := range map[string]string{
		"missing":   "NOT_FOUND",
		"plain":     "RATE_LIMITED",          // a plain-text error gets the code the envelope gives it
		"lowercase": "CONFLICT",              // a code outside the stored form falls back to the status's
		"closed":    "CLIENT_CLOSED_REQUEST", // 499 is recorded, though no client reads it
		"long":      "VALIDATION_ERROR",      // read from the first 4 KB of a longer answer
		"empty":     "INTERNAL_ERROR",
		"found":     "",
	} {
		spy := &recordingSpy{}
		rec := httptest.NewRecorder()
		trafficUsageRouter(t, spy).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/posts/"+id, http.NoBody))
		require.Len(t, spy.events, 1, id)
		assert.Equal(t, want, spy.events[0].ErrorCode, id)
	}
}

// Sniffing the code changes nothing the client receives.
func TestAPIUsage_TheAnswerIsUntouched(t *testing.T) {
	spy := &recordingSpy{}
	rec := httptest.NewRecorder()
	trafficUsageRouter(t, spy).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/posts/long", http.NoBody))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Len(t, rec.Body.String(), len(`{"error":{"code":"VALIDATION_ERROR","message":""}}`)+6000)
}

func TestAPIUsage_RecordsTheClassOfClient(t *testing.T) {
	for _, c := range []struct{ ua, hints, want string }{
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36", "", "browser"},
		{"solvr-web/1.0 (ssr)", "", "web_server"},
		{"curl/8.7.1", "", "http_client"},
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "", "crawler"},
		{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36", `"HeadlessChrome";v="141"`, "headless"},
		{"", "", "none"},
	} {
		spy := &recordingSpy{}
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/posts/found", http.NoBody)
		req.Header.Set("User-Agent", c.ua)
		if c.hints != "" {
			req.Header.Set("Sec-CH-UA", c.hints)
		}
		trafficUsageRouter(t, spy).ServeHTTP(httptest.NewRecorder(), req)
		require.Len(t, spy.events, 1, c.ua)
		assert.Equal(t, c.want, spy.events[0].UAClass, c.ua)
	}
}

// The analytics ingest is measurement plumbing, not usage: counting it would make every page
// view a call.
func TestAPIUsage_TheAnalyticsIngestIsNotUsage(t *testing.T) {
	for _, path := range []string{"/v1/analytics/funnel", "/v1/analytics/pageview"} {
		spy := &recordingSpy{}
		trafficUsageRouter(t, spy).ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, http.NoBody))
		assert.Empty(t, spy.events, path)
	}
}
