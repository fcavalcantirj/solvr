package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// replyAtVersion returns a reply owned by the agentReq identity (agent-1) whose
// last-modified time — and therefore its ETag validator — is t.
func replyAtVersion(t time.Time) *models.ReplyWithAuthor {
	return &models.ReplyWithAuthor{
		Reply: models.Reply{
			ID:         "r1",
			Body:       "original body",
			AuthorType: models.AuthorTypeAgent,
			AuthorID:   "agent-1",
			UpdatedAt:  t,
		},
	}
}

// A stale If-Match precondition must reject the edit with 412, must NOT reach
// the repository write, and must echo the current validator so the client can
// refetch and retry. The sentinel updateErr proves the gate short-circuits:
// were repo.Update reached, the handler would surface that error, not 412.
func TestReplies_UpdateStaleIfMatchRejected(t *testing.T) {
	current := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	mock := &MockRepliesRepository{
		getResult: replyAtVersion(current),
		updateErr: errors.New("repo.Update must not run on a stale precondition"),
	}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	req.Header.Set("If-Match", `"1"`) // not the current validator
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412; body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("ETag"), replyETag(current); got != want {
		t.Errorf("ETag = %q, want current %q", got, want)
	}
}

// A matching If-Match lets the edit through and the response echoes the NEW
// validator derived from the persisted version.
func TestReplies_UpdateMatchingIfMatchSucceeds(t *testing.T) {
	current := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	newVer := current.Add(time.Minute)
	mock := &MockRepliesRepository{
		getResult: replyAtVersion(current),
		updated:   &models.Reply{ID: "r1", Body: "edited", UpdatedAt: newVer},
	}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	req.Header.Set("If-Match", replyETag(current))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("ETag"), replyETag(newVer); got != want {
		t.Errorf("ETag = %q, want new %q", got, want)
	}
}

// "If-Match: *" matches any existing reply and proceeds.
func TestReplies_UpdateStarIfMatchSucceeds(t *testing.T) {
	mock := &MockRepliesRepository{getResult: replyAtVersion(time.Now())}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// If-Match is required (spec.json idx 74 step 5, owner decision 8 of
// 2026-09-30): an edit with no precondition is 428 PRECONDITION_REQUIRED, never
// reaches the write, and hands out no ETag to blindly echo back.
func TestReplies_UpdateWithoutIfMatchIs428(t *testing.T) {
	mock := &MockRepliesRepository{getResult: replyAtVersion(time.Now())}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "PRECONDITION_REQUIRED" {
		t.Errorf("error.code = %q, want PRECONDITION_REQUIRED", code)
	}
	if mock.updateCalls != 0 {
		t.Error("an edit without If-Match must not reach the write")
	}
	if got := rec.Header().Get("ETag"); got != "" {
		t.Errorf("a 428 must not hand out the current ETag, got %q", got)
	}
}

// The matching If-Match becomes the write's expected version, so the write
// refuses a reply another writer changed after the check; "*" writes
// unconditionally (no expected version).
func TestReplies_UpdateWritesAtTheVersionItChecked(t *testing.T) {
	current := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		ifMatch string
		want    *time.Time
	}{{replyETag(current), &current}, {"*", nil}} {
		mock := &MockRepliesRepository{getResult: replyAtVersion(current)}
		h := NewRepliesHandler(mock)
		req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
		req.Header.Set("If-Match", tc.ifMatch)
		rec := httptest.NewRecorder()
		h.Update(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("If-Match %s: status = %d, want 200; body=%s", tc.ifMatch, rec.Code, rec.Body.String())
		}
		got := mock.lastExpected
		if mock.updateCalls != 1 || (tc.want == nil) != (got == nil) || (got != nil && !got.Equal(*tc.want)) {
			t.Errorf("If-Match %s: %d writes at %v, want one at %v", tc.ifMatch, mock.updateCalls, got, tc.want)
		}
	}
}

// A precondition that matched at the check but lost the write to a concurrent
// writer is 412 PRECONDITION_FAILED with the winner's ETag.
func TestReplies_UpdateLostToAConcurrentWriterIs412(t *testing.T) {
	current := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	winner := current.Add(time.Second)
	mock := &MockRepliesRepository{
		getResult: replyAtVersion(current),
		updateErr: &models.VersionConflictError{Current: winner},
	}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	req.Header.Set("If-Match", replyETag(current))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "PRECONDITION_FAILED" {
		t.Errorf("error.code = %q, want PRECONDITION_FAILED", code)
	}
	if got, want := rec.Header().Get("ETag"), replyETag(winner); got != want {
		t.Errorf("ETag = %q, want the winner's %q", got, want)
	}
}

// Editing a reply that no longer exists surfaces 404 from the version read,
// not a write against a phantom row.
func TestReplies_UpdateNotFound(t *testing.T) {
	mock := &MockRepliesRepository{getErr: models.ErrReplyNotFound}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/missing", map[string]any{"body": "edited"}, map[string]string{"id": "missing"})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

// GET hands the client the validator it will echo as an If-Match precondition.
func TestReplies_GetSetsETag(t *testing.T) {
	current := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	mock := &MockRepliesRepository{getResult: replyAtVersion(current)}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodGet, "/v1/replies/r1", nil, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("ETag"), replyETag(current); got != want {
		t.Errorf("ETag = %q, want %q", got, want)
	}
}
