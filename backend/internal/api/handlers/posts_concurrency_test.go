package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

// TestUpdatePost_NoIfMatchStillSucceeds verifies the precondition is optional:
// a client that omits If-Match keeps the prior edit behavior (back-compat).
func TestUpdatePost_NoIfMatchStillSucceeds(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)
	handler := NewPostsHandler(repo)

	req := ownerPatchRequest(map[string]interface{}{"title": "Updated Title That Is Long Enough"}, "")
	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.updatedPost == nil {
		t.Error("an edit without If-Match must persist")
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
