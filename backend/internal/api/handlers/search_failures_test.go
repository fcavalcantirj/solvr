package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/requestid"
)

// failureLog keeps the failures the handler records, for the tests below.
type failureLog struct {
	got  chan struct{}
	rows []models.SearchFailure
	mu   sync.Mutex
}

func newFailureLog() *failureLog { return &failureLog{got: make(chan struct{}, 16)} }

func (f *failureLog) RecordFailure(_ context.Context, sf *models.SearchFailure) error {
	f.mu.Lock()
	f.rows = append(f.rows, *sf)
	f.mu.Unlock()
	f.got <- struct{}{}
	return nil
}

// wait returns the first n recorded failures, or fails the test after 2 s.
func (f *failureLog) wait(t *testing.T, n int) []models.SearchFailure {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-f.got:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d failures recorded", i, n)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.SearchFailure(nil), f.rows...)
}

func searchFailureCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Error.Code
}

// SPEC.md 22.7: a parameter that is not UTF-8 text, or that holds a NUL, is the client's error
// (400), never a 500 from the database refusing it. The search is not run.
func TestSearch_RejectsTextThatIsNotUTF8OrHoldsNUL(t *testing.T) {
	for _, target := range []string{
		"/v1/search?q=a%FFb",
		"/v1/search?q=a%00b",
		"/v1/search?q=ok&tags=%FF",
		"/v1/search?q=ok&author=x%00y",
	} {
		repo := NewMockSearchRepository()
		h := NewSearchHandler(repo)
		w := httptest.NewRecorder()
		h.Search(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody))
		assert.Equal(t, http.StatusBadRequest, w.Code, target)
		assert.Equal(t, "VALIDATION_ERROR", searchFailureCode(t, w), target)
		assert.Contains(t, w.Body.String(), "must be UTF-8 text without NUL characters", target)
		assert.Empty(t, repo.searchQuery, "the search did not run: %s", target)
	}
}

// SPEC.md 5.4 and 22.8: a client that closed the request before the search finished gets 499
// CLIENT_CLOSED_REQUEST (recorded, never seen by a client), not a 500: the server did not fail.
// The local C1 spike (2026-10-17) reproduced production's 5xx signature this way.
func TestSearch_AClientThatLeftIs499NotA500(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetError(context.Canceled)
	h := NewSearchHandler(repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	h.Search(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/search?q=scraping", http.NoBody).WithContext(ctx))
	assert.Equal(t, 499, w.Code)
	assert.Equal(t, "CLIENT_CLOSED_REQUEST", searchFailureCode(t, w))
}

// SPEC.md 22.8: a search the server failed is a 500 logged once, with the request id, the
// error code and the Postgres SQLSTATE.
func TestSearch_AServerFailureIsLoggedWithItsRequestID(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(old)

	repo := NewMockSearchRepository()
	repo.SetError(&pgconn.PgError{Code: "42601", Message: "syntax error in tsquery"})
	h := NewSearchHandler(repo)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/search?q=x", http.NoBody)
	req = req.WithContext(requestid.With(req.Context(), "req-search-1"))
	w := httptest.NewRecorder()
	h.Search(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_ERROR", searchFailureCode(t, w))

	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line), buf.String())
	assert.Equal(t, "search failed", line["msg"])
	assert.Equal(t, "req-search-1", line["request_id"])
	assert.Equal(t, "INTERNAL_ERROR", line["error_code"])
	assert.Equal(t, "42601", line["sqlstate"])
}

// SPEC.md 22.8: every search answered 400, 499 or 5xx after the q check is recorded, with its
// input, outcome and request id. A search that succeeds records no failure.
func TestSearch_EveryFailureIsRecorded(t *testing.T) {
	failures := newFailureLog()
	run := func(target string, repoErr error, cancelled bool) {
		repo := NewMockSearchRepository()
		if repoErr != nil {
			repo.SetError(repoErr)
		}
		h := NewSearchHandler(repo)
		h.SetFailureRecorder(failures)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
		ctx := requestid.With(req.Context(), "req-"+target)
		if cancelled {
			c, cancel := context.WithCancel(ctx)
			cancel()
			ctx = c
		}
		h.Search(httptest.NewRecorder(), req.WithContext(ctx))
	}
	run("/v1/search?q=a%FFb", nil, false)
	run("/v1/search?q=gone", context.Canceled, true)
	run("/v1/search?q=broken", &pgconn.PgError{Code: "57014", Message: "canceling statement"}, false)
	run("/v1/search?q=dated&from_date=yesterday", nil, false)
	run("/v1/search?q=fine", nil, false)

	rows := failures.wait(t, 4)
	byQuery := map[string]models.SearchFailure{}
	for _, r := range rows {
		byQuery[r.Query] = r
	}
	require.Len(t, byQuery, 4, "the successful search recorded no failure")
	assert.Equal(t, 400, byQuery["a\xffb"].Status)
	assert.Equal(t, "VALIDATION_ERROR", byQuery["a\xffb"].ErrorCode)
	assert.Equal(t, 499, byQuery["gone"].Status)
	assert.Equal(t, "CLIENT_CLOSED_REQUEST", byQuery["gone"].ErrorCode)
	assert.Equal(t, 500, byQuery["broken"].Status)
	assert.Equal(t, "INTERNAL_ERROR", byQuery["broken"].ErrorCode)
	assert.Equal(t, "57014", byQuery["broken"].SQLState)
	assert.Equal(t, "req-/v1/search?q=broken", byQuery["broken"].RequestID)
	assert.Equal(t, 400, byQuery["dated"].Status)
	assert.Equal(t, "anonymous", byQuery["broken"].SearcherType)
	assert.False(t, byQuery["broken"].OccurredAt.IsZero())
}
