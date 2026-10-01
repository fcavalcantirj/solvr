package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// PATCH /v1/posts/:id — optimistic concurrency (If-Match) tests
// Pins idx 73 step 5: "Require version or If-Match on conflicting edits and
// reject stale updates; ensure retries cannot overwrite a newer directive,
// change ownership, or duplicate a publication."
// These are handler unit tests against the mock repo, so they run without a
// database (no SKIP).
// ============================================================================

// ownerPatchRequest builds a PATCH /v1/posts/post-123 request from the post
// owner (user-123), optionally carrying an If-Match precondition.
func ownerPatchRequest(body map[string]interface{}, ifMatch string) *http.Request {
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return addAuthContext(req, "user-123", "user")
}

// TestUpdatePost_StaleIfMatchRejected verifies a stale If-Match precondition
// yields 412 PRECONDITION_FAILED and does not persist the edit.
func TestUpdatePost_StaleIfMatchRejected(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	req := ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, `"0"`)
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected status 412, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "PRECONDITION_FAILED" {
		t.Errorf("expected code PRECONDITION_FAILED, got %v", errObj["code"])
	}
	if repo.updatedPost != nil {
		t.Error("a stale update must not be persisted")
	}
	// The current validator is echoed so the client can refetch and retry.
	if got := w.Header().Get("ETag"); got != postETag(post.UpdatedAt) {
		t.Errorf("expected ETag %q, got %q", postETag(post.UpdatedAt), got)
	}
}

// TestUpdatePost_MatchingIfMatchSucceeds verifies an If-Match equal to the
// current version passes the precondition and the edit is applied.
func TestUpdatePost_MatchingIfMatchSucceeds(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	req := ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, postETag(post.UpdatedAt))
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.updatedPost == nil {
		t.Error("a matching precondition must persist the edit")
	}
}

// TestUpdatePost_WildcardIfMatchSucceeds verifies "If-Match: *" matches any
// existing post.
func TestUpdatePost_WildcardIfMatchSucceeds(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	req := ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, "*")
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdatePost_MissingIfMatchIs428 verifies If-Match is required (spec.json
// idx 74 step 5, owner decision 8 of 2026-09-30): an edit without it is refused
// with 428 PRECONDITION_REQUIRED, persists nothing, and carries no ETag, so a
// client has to read the post before it can overwrite it.
func TestUpdatePost_MissingIfMatchIs428(t *testing.T) {
	repo := &conditionalPostsRepo{MockPostsRepository: NewMockPostsRepository()}
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	req := ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, "")
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected status 428, got %d: %s", w.Code, w.Body.String())
	}
	if code := errorCode(t, w); code != "PRECONDITION_REQUIRED" {
		t.Errorf("expected code PRECONDITION_REQUIRED, got %q", code)
	}
	if repo.updatedPost != nil || repo.conditionalCalls != 0 {
		t.Error("an edit without If-Match must not be persisted")
	}
	if got := w.Header().Get("ETag"); got != "" {
		t.Errorf("a 428 must not hand out the current ETag, got %q", got)
	}
}

// TestUpdatePost_NonOwnerWithoutIfMatchStillForbidden verifies ownership is
// checked before the precondition is required: a non-owner gets 403, not 428.
func TestUpdatePost_NonOwnerWithoutIfMatchStillForbidden(t *testing.T) {
	repo := &conditionalPostsRepo{MockPostsRepository: NewMockPostsRepository()}
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	jsonBody, _ := json.Marshal(map[string]interface{}{"title": "Updated Title That Is Long Enough"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "other-user", "user")
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdatePost_WritesAtTheVersionItChecked verifies the matching If-Match is
// handed to the repository as the expected version, so the write itself
// refuses a row another writer changed after the check.
func TestUpdatePost_WritesAtTheVersionItChecked(t *testing.T) {
	repo := &conditionalPostsRepo{MockPostsRepository: NewMockPostsRepository()}
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	w := httptest.NewRecorder()
	handler.Update(w, ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, postETag(post.UpdatedAt)))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.conditionalCalls != 1 || repo.lastExpected == nil || !repo.lastExpected.Equal(post.UpdatedAt) {
		t.Errorf("expected one conditional write at %v, got %d calls at %v", post.UpdatedAt, repo.conditionalCalls, repo.lastExpected)
	}
}

// TestUpdatePost_WildcardWritesUnconditionally verifies "If-Match: *" is the
// explicit unconditional edit: the repository gets no expected version.
func TestUpdatePost_WildcardWritesUnconditionally(t *testing.T) {
	repo := &conditionalPostsRepo{MockPostsRepository: NewMockPostsRepository()}
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	w := httptest.NewRecorder()
	handler.Update(w, ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, "*"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.conditionalCalls != 1 || repo.lastExpected != nil {
		t.Errorf("expected one unconditional write, got %d calls at %v", repo.conditionalCalls, repo.lastExpected)
	}
}

// TestUpdatePost_EditLostToAConcurrentWriterIs412 verifies an edit whose
// precondition matched but whose write found the row already changed (another
// writer won the race) is 412 PRECONDITION_FAILED with the winner's ETag.
func TestUpdatePost_EditLostToAConcurrentWriterIs412(t *testing.T) {
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	winner := post.UpdatedAt.Add(time.Second)
	repo := &conditionalPostsRepo{MockPostsRepository: NewMockPostsRepository(), conflictAt: &winner}
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	w := httptest.NewRecorder()
	handler.Update(w, ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, postETag(post.UpdatedAt)))

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected status 412, got %d: %s", w.Code, w.Body.String())
	}
	if code := errorCode(t, w); code != "PRECONDITION_FAILED" {
		t.Errorf("expected code PRECONDITION_FAILED, got %q", code)
	}
	if got := w.Header().Get("ETag"); got != postETag(winner) {
		t.Errorf("expected the winner's ETag %q, got %q", postETag(winner), got)
	}
}

// TestUpdatePost_NonOwnerStaleIfMatchStillForbidden verifies ownership is
// checked before the precondition, so a non-owner never learns the version via
// a 412 (they still receive 403).
func TestUpdatePost_NonOwnerStaleIfMatchStillForbidden(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	jsonBody, _ := json.Marshal(map[string]interface{}{"title": "Updated Title That Is Long Enough"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"0"`)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "other-user", "user")
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetPost_SetsETag verifies GET returns the version validator so a client
// can send it back as If-Match on a later edit.
func TestGetPost_SetsETag(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("ETag"); got != postETag(post.UpdatedAt) {
		t.Errorf("expected ETag %q, got %q", postETag(post.UpdatedAt), got)
	}
}

// TestIfMatchIsStale pins the precondition parsing: optional header, weak-tag
// value comparison, wildcard match, and comma-separated candidate lists.
func TestIfMatchIsStale(t *testing.T) {
	const current = `"12345"`
	cases := []struct {
		name      string
		header    string
		wantStale bool
	}{
		{"no header is never stale", "", false},
		{"exact match", `"12345"`, false},
		{"wildcard matches", "*", false},
		{"different tag is stale", `"999"`, true},
		{"weak prefix compares by value", `W/"12345"`, false},
		{"one of several candidates matches", `"999", "12345"`, false},
		{"none of several candidates matches", `"1", "2"`, true},
		{"whitespace around value", `  "12345"  `, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", nil)
			if tc.header != "" {
				req.Header.Set("If-Match", tc.header)
			}
			if got := ifMatchIsStale(req, current); got != tc.wantStale {
				t.Errorf("ifMatchIsStale(%q) = %v, want %v", tc.header, got, tc.wantStale)
			}
		})
	}
}

// conditionalPostsRepo wraps the posts mock to record the expected version the
// handler hands to the conditional write, and to simulate losing the write to
// a concurrent writer (conflictAt is that writer's version).
type conditionalPostsRepo struct {
	*MockPostsRepository
	conditionalCalls int
	lastExpected     *time.Time
	conflictAt       *time.Time
}

func (m *conditionalPostsRepo) UpdateIfUnmodified(ctx context.Context, post *models.Post, expected *time.Time) (*models.Post, error) {
	m.conditionalCalls++
	m.lastExpected = expected
	if m.conflictAt != nil {
		return nil, &models.VersionConflictError{Current: *m.conflictAt}
	}
	return m.MockPostsRepository.UpdateIfUnmodified(ctx, post, expected)
}

// errorCode decodes error.code from an error envelope response.
func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error envelope: %v (%s)", err, w.Body.String())
	}
	return resp.Error.Code
}
