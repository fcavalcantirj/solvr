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

// If-Match is opt-in: an edit with no precondition keeps the prior behavior.
func TestReplies_UpdateWithoutIfMatchStillSucceeds(t *testing.T) {
	mock := &MockRepliesRepository{getResult: replyAtVersion(time.Now())}
	h := NewRepliesHandler(mock)

	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
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
