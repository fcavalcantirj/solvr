package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/requestid"
)

// usageEvents keeps what APIUsage records, for the request-id wiring test.
type usageEvents struct {
	mu     sync.Mutex
	events []db.APIRequestEvent
}

func (u *usageEvents) Record(e db.APIRequestEvent) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.events = append(u.events, e)
}

// SPEC.md 17.3: the id the client sees in X-Request-ID is the id in the request's context, in
// its X-Request-ID header (which the Logging and APIUsage middlewares read), in the db layer's
// log lines and in the recorded api_request_events row. Before this, the id lived only in the
// response header, and 0 of 188,629 production rows carried one.
func TestRequestID_OneIDReachesTheContextTheLogsAndTheUsageRow(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(old)

	rec := &usageEvents{}
	var seenCtx, seenHeader string
	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(apimiddleware.APIUsage(rec))
	r.Get("/v1/search", func(w http.ResponseWriter, req *http.Request) {
		seenCtx = requestid.From(req.Context())
		seenHeader = req.Header.Get("X-Request-ID")
		db.LogQueryError(req.Context(), "Search.Probe", "posts", errors.New("probe"))
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name, client string
		keep         bool
	}{
		{"no client id", "", false},
		{"safe client id", "req-abc.1_2", true},
		{"unsafe client id", "bad id\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf.Reset()
			rec.events = nil
			req := httptest.NewRequest(http.MethodGet, "/v1/search?q=x", nil)
			if c.client != "" {
				req.Header.Set("X-Request-ID", c.client)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			id := w.Header().Get("X-Request-ID")
			require.NotEmpty(t, id)
			if c.keep {
				assert.Equal(t, c.client, id)
			} else {
				assert.NotEqual(t, c.client, id)
			}
			assert.Equal(t, id, seenCtx, "the handler's context carries the id")
			assert.Equal(t, id, seenHeader, "the request header carries the id")

			var line map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
			assert.Equal(t, id, line["request_id"], "the db layer's log line names the id")

			require.Len(t, rec.events, 1)
			assert.Equal(t, id, rec.events[0].RequestID, "the usage row carries the id")
		})
	}
}
