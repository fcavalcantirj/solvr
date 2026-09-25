package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// fakeIdempotencyStore is a test double of the durable store (production uses
// db.IdempotencyRepository; its SQL is covered by the db integration test).
type fakeIdempotencyStore struct {
	mu        sync.Mutex
	records   map[models.IdempotencyScope]*models.IdempotencyRecord
	reserveEr error
	completed int
	released  int
}

func newFakeIdempotencyStore() *fakeIdempotencyStore {
	return &fakeIdempotencyStore{records: map[models.IdempotencyScope]*models.IdempotencyRecord{}}
}

func (s *fakeIdempotencyStore) Reserve(_ context.Context, scope models.IdempotencyScope, hash string) (*models.IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reserveEr != nil {
		return nil, false, s.reserveEr
	}
	if rec, ok := s.records[scope]; ok {
		cp := *rec
		return &cp, false, nil
	}
	s.records[scope] = &models.IdempotencyRecord{RequestHash: hash, Status: models.IdempotencyPending}
	return nil, true, nil
}

func (s *fakeIdempotencyStore) Complete(_ context.Context, scope models.IdempotencyScope, status int, contentType string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.records[scope]
	rec.Status = models.IdempotencyCompleted
	rec.ResponseStatus = status
	rec.ResponseContentType = contentType
	rec.ResponseBody = append([]byte(nil), body...)
	s.completed++
	return nil
}

func (s *fakeIdempotencyStore) Release(_ context.Context, scope models.IdempotencyScope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, scope)
	s.released++
	return nil
}

// countingCreate is a create handler that records how many times it ran.
type countingCreate struct {
	calls  int
	status int
	bodies []string
}

func (c *countingCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.calls++
	b, _ := io.ReadAll(r.Body)
	c.bodies = append(c.bodies, string(b))
	status := c.status
	if status == 0 {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"data":{"id":"created-` + string(rune('0'+c.calls)) + `"}}`))
}

func idemRequest(body, key, agentID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
	if key != "" {
		r.Header.Set(IdempotencyKeyHeader, key)
	}
	if agentID != "" {
		r = r.WithContext(auth.ContextWithAgent(r.Context(), &models.Agent{ID: agentID}))
	}
	return r
}

func serveIdem(store IdempotencyStore, h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Idempotency(store, "post.create")(h).ServeHTTP(rec, r)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	return body.Error.Code
}

func TestIdempotency_NoHeaderPassesThrough(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	serveIdem(store, h, idemRequest(`{"a":1}`, "", "agent-1"))
	serveIdem(store, h, idemRequest(`{"a":1}`, "", "agent-1"))
	if h.calls != 2 {
		t.Fatalf("handler ran %d times, want 2 (no key = no dedupe)", h.calls)
	}
	if len(store.records) != 0 {
		t.Fatalf("store has %d records, want 0", len(store.records))
	}
}

func TestIdempotency_RetryReplaysStoredResultWithoutRerunning(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	first := serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	second := serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))

	if h.calls != 1 {
		t.Fatalf("handler ran %d times, want 1", h.calls)
	}
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("statuses = %d, %d; want 201, 201", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("replay body %q != original %q", second.Body.String(), first.Body.String())
	}
	if second.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("replay Content-Type = %q", second.Header().Get("Content-Type"))
	}
	if second.Header().Get(IdempotentReplayedHeader) != "true" {
		t.Fatalf("replay missing %s header", IdempotentReplayedHeader)
	}
	if first.Header().Get(IdempotentReplayedHeader) != "" {
		t.Fatalf("original response must not be marked replayed")
	}
	// The handler still received the full body on the first (live) run.
	if h.bodies[0] != `{"a":1}` {
		t.Fatalf("handler saw body %q", h.bodies[0])
	}
}

func TestIdempotency_ReuseWithDifferentPayloadRejected(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	rec := serveIdem(store, h, idemRequest(`{"a":2}`, "k-1", "agent-1"))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if got := errCode(t, rec); got != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("code = %q, want IDEMPOTENCY_KEY_REUSED", got)
	}
	if h.calls != 1 {
		t.Fatalf("handler ran %d times, want 1", h.calls)
	}
}

func TestIdempotency_SameKeyDifferentPathIsDifferentPayload(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	r1 := httptest.NewRequest(http.MethodPost, "/v1/posts/p1/replies", strings.NewReader(`{"a":1}`))
	r2 := httptest.NewRequest(http.MethodPost, "/v1/posts/p2/replies", strings.NewReader(`{"a":1}`))
	for _, r := range []*http.Request{r1, r2} {
		r.Header.Set(IdempotencyKeyHeader, "k-1")
	}
	r1 = r1.WithContext(auth.ContextWithAgent(r1.Context(), &models.Agent{ID: "agent-1"}))
	r2 = r2.WithContext(auth.ContextWithAgent(r2.Context(), &models.Agent{ID: "agent-1"}))
	serveIdem(store, h, r1)
	rec := serveIdem(store, h, r2)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (same key reused on another target)", rec.Code)
	}
}

func TestIdempotency_KeysAreScopedToActor(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	human := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(`{"a":1}`))
	human.Header.Set(IdempotencyKeyHeader, "k-1")
	human = human.WithContext(auth.ContextWithClaims(human.Context(), &auth.Claims{UserID: "user-1"}))
	serveIdem(store, h, human)
	serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-2"))

	if h.calls != 3 {
		t.Fatalf("handler ran %d times, want 3 (same key, three actors)", h.calls)
	}
}

func TestIdempotency_KeysAreScopedToOperation(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	rec := httptest.NewRecorder()
	Idempotency(store, "room.create")(h).ServeHTTP(rec, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	if h.calls != 2 {
		t.Fatalf("handler ran %d times, want 2 (same key, two operations)", h.calls)
	}
}

func TestIdempotency_FailedCreateReleasesKey(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{status: http.StatusInternalServerError}
	serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	if store.released != 1 || store.completed != 0 {
		t.Fatalf("released=%d completed=%d, want 1/0", store.released, store.completed)
	}
	h.status = http.StatusCreated
	rec := serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	if rec.Code != http.StatusCreated || h.calls != 2 {
		t.Fatalf("retry after failure: status=%d calls=%d, want 201/2", rec.Code, h.calls)
	}
}

func TestIdempotency_InFlightKeyConflicts(t *testing.T) {
	store := newFakeIdempotencyStore()
	scope := models.IdempotencyScope{ActorType: "agent", ActorID: "agent-1", Operation: "post.create", Key: "k-1"}
	store.records[scope] = &models.IdempotencyRecord{RequestHash: requestHash(idemRequest(`{"a":1}`, "", ""), []byte(`{"a":1}`)), Status: models.IdempotencyPending}
	h := &countingCreate{}
	rec := serveIdem(store, h, idemRequest(`{"a":1}`, "k-1", "agent-1"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if got := errCode(t, rec); got != "IDEMPOTENCY_REQUEST_IN_PROGRESS" {
		t.Fatalf("code = %q", got)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("in-progress conflict must carry Retry-After")
	}
	if h.calls != 0 {
		t.Fatalf("handler ran %d times, want 0", h.calls)
	}
}

func TestIdempotency_InvalidKeyRejected(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	rec := serveIdem(store, h, idemRequest(`{}`, strings.Repeat("x", 256), "agent-1"))
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "VALIDATION_ERROR" {
		t.Fatalf("status=%d body=%q, want 400 VALIDATION_ERROR", rec.Code, rec.Body.String())
	}
	if h.calls != 0 {
		t.Fatalf("handler ran on an invalid key")
	}
}

func TestIdempotency_StoreFailureFailsClosed(t *testing.T) {
	store := newFakeIdempotencyStore()
	store.reserveEr = errors.New("db down")
	h := &countingCreate{}
	rec := serveIdem(store, h, idemRequest(`{}`, "k-1", "agent-1"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if h.calls != 0 {
		t.Fatalf("handler ran without an idempotency reservation")
	}
}

func TestIdempotency_AnonymousPassesThroughToHandlerAuth(t *testing.T) {
	store := newFakeIdempotencyStore()
	h := &countingCreate{}
	serveIdem(store, h, idemRequest(`{}`, "k-1", ""))
	if h.calls != 1 || len(store.records) != 0 {
		t.Fatalf("anonymous: calls=%d records=%d, want 1/0", h.calls, len(store.records))
	}
}

func TestIdempotency_NilStoreIsPassThrough(t *testing.T) {
	h := &countingCreate{}
	rec := httptest.NewRecorder()
	Idempotency(nil, "post.create")(h).ServeHTTP(rec, idemRequest(`{}`, "k-1", "agent-1"))
	Idempotency(nil, "post.create")(h).ServeHTTP(rec, idemRequest(`{}`, "k-1", "agent-1"))
	if h.calls != 2 {
		t.Fatalf("calls=%d, want 2", h.calls)
	}
}
