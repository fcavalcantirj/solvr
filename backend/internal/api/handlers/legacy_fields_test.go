package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// idx 68: the legacy post types, the problem-only fields and the legacy statuses are refused
// with 400 LEGACY_FIELD_RETIRED naming the field, never silently dropped or accepted.

func decodeLegacyFieldError(t *testing.T, w *httptest.ResponseRecorder) (code, field string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Field        string `json:"field"`
				Instructions string `json:"instructions"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	if body.Error.Details.Instructions == "" {
		t.Errorf("a retired field must say what to send instead: %s", w.Body.String())
	}
	return body.Error.Code, body.Error.Details.Field
}

func createPostRequest(t *testing.T, body map[string]any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return addAuthContext(req, "user-123", "user")
}

func legacyFieldsPostBody() map[string]any {
	return map[string]any{
		"title":       "A post about worker pools draining slowly",
		"description": "Workers drain slowly under load; this describes what we saw and what we tried so far.",
	}
}

func TestCreatePost_LegacyTypeIsLegacyFieldRetired(t *testing.T) {
	for _, legacyType := range []string{"problem", "question", "idea"} {
		repo := NewMockPostsRepository()
		body := legacyFieldsPostBody()
		body["type"] = legacyType
		w := httptest.NewRecorder()
		NewPostsHandler(repo).Create(w, createPostRequest(t, body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("type %q: status %d, want 400: %s", legacyType, w.Code, w.Body.String())
		}
		if code, field := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || field != "type" {
			t.Errorf("type %q: code %q field %q, want %s/type", legacyType, code, field, ErrCodeLegacyFieldRetired)
		}
		if repo.createdPost != nil {
			t.Errorf("type %q: a refused post must not be stored", legacyType)
		}
	}
}

func TestCreatePost_PostTypeAndOmittedTypeAreAccepted(t *testing.T) {
	for _, typ := range []any{nil, "post"} {
		repo := NewMockPostsRepository()
		body := legacyFieldsPostBody()
		if typ != nil {
			body["type"] = typ
		}
		w := httptest.NewRecorder()
		NewPostsHandler(repo).Create(w, createPostRequest(t, body))

		if w.Code != http.StatusCreated {
			t.Fatalf("type %v: status %d, want 201: %s", typ, w.Code, w.Body.String())
		}
		if repo.createdPost == nil || repo.createdPost.Type != models.PostTypePost {
			t.Errorf("type %v: the stored post must be type post, got %+v", typ, repo.createdPost)
		}
	}
}

func TestCreatePost_ProblemOnlyFieldsAreLegacyFieldRetired(t *testing.T) {
	for field, value := range map[string]any{
		"success_criteria": []string{"it works"}, "weight": 3,
		"accepted_answer_id": "11111111-1111-1111-1111-111111111111", "evolved_into": []string{},
	} {
		repo := NewMockPostsRepository()
		body := legacyFieldsPostBody()
		body[field] = value
		w := httptest.NewRecorder()
		NewPostsHandler(repo).Create(w, createPostRequest(t, body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400: %s", field, w.Code, w.Body.String())
		}
		if code, got := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || got != field {
			t.Errorf("%s: code %q field %q", field, code, got)
		}
		if repo.createdPost != nil {
			t.Errorf("%s: a refused post must not be stored", field)
		}
	}
}

func patchPost(t *testing.T, repo *MockPostsRepository, post models.PostWithAuthor, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/"+post.ID, bytes.NewReader(raw))
	req.Header.Set("If-Match", postETag(post.UpdatedAt))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", post.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()
	NewPostsHandler(repo).Update(w, req)
	return w
}

func TestUpdatePost_RetiredStatusIsLegacyFieldRetired(t *testing.T) {
	for _, status := range models.RetiredPostStatuses {
		repo := NewMockPostsRepository()
		post := createTestPostWithStatus("post-123", "Open Post Title Here", models.PostTypePost, models.PostStatusOpen)
		repo.SetPost(&post)

		w := patchPost(t, repo, post, map[string]any{"status": string(status)})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %q: %d, want 400: %s", status, w.Code, w.Body.String())
		}
		if code, field := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || field != "status" {
			t.Errorf("status %q: code %q field %q", status, code, field)
		}
		if repo.updatedPost != nil {
			t.Errorf("status %q: a refused edit must not be stored", status)
		}
	}
}

func TestUpdatePost_ProblemOnlyFieldIsLegacyFieldRetired(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-123", "Open Post Title Here", models.PostTypePost, models.PostStatusOpen)
	repo.SetPost(&post)

	w := patchPost(t, repo, post, map[string]any{"title": "A new title for the post", "weight": 2})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
	}
	if code, field := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || field != "weight" {
		t.Errorf("code %q field %q", code, field)
	}
	if repo.updatedPost != nil {
		t.Error("a refused edit must not be stored")
	}
}

func TestListPosts_LegacyFiltersAreLegacyFieldRetired(t *testing.T) {
	for query, field := range map[string]string{
		"type=problem": "type", "type=question": "type", "type=idea": "type",
		"status=solved": "status", "status=in_progress": "status",
	} {
		repo := NewMockPostsRepository()
		w := httptest.NewRecorder()
		NewPostsHandler(repo).List(w, httptest.NewRequest(http.MethodGet, "/v1/posts?"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400: %s", query, w.Code, w.Body.String())
		}
		if code, got := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || got != field {
			t.Errorf("%s: code %q field %q", query, code, got)
		}
	}
	for _, query := range []string{"type=post", "type=all", "status=open"} {
		repo := NewMockPostsRepository()
		w := httptest.NewRecorder()
		NewPostsHandler(repo).List(w, httptest.NewRequest(http.MethodGet, "/v1/posts?"+query, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200: %s", query, w.Code, w.Body.String())
		}
		if repo.listOpts.Type != "" {
			t.Errorf("%s: every post is type post, so no type filter applies; got %q", query, repo.listOpts.Type)
		}
	}
}

func TestSearch_LegacyFiltersAreLegacyFieldRetired(t *testing.T) {
	for query, field := range map[string]string{"type=problem": "type", "status=solved": "status"} {
		w := httptest.NewRecorder()
		NewSearchHandler(NewMockSearchRepository()).Search(w, httptest.NewRequest(http.MethodGet, "/v1/search?q=workers&"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400: %s", query, w.Code, w.Body.String())
		}
		if code, got := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || got != field {
			t.Errorf("%s: code %q field %q", query, code, got)
		}
	}
}

func TestCreatePost_UnknownTypeIsInvalidType(t *testing.T) {
	repo := NewMockPostsRepository()
	body := legacyFieldsPostBody()
	body["type"] = "bogus"
	w := httptest.NewRecorder()
	NewPostsHandler(repo).Create(w, createPostRequest(t, body))
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte(`"INVALID_TYPE"`)) {
		t.Fatalf("an unknown type is INVALID_TYPE, not a retired field: %d %s", w.Code, w.Body.String())
	}
	if repo.createdPost != nil {
		t.Error("a refused post must not be stored")
	}
}
