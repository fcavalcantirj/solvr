package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// POST /v1/posts - Create Post Tests
// ============================================================================

// TestCreatePost_Success tests successful post creation.
func TestCreatePost_Success(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Test Problem Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":        []string{"go", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] == nil {
		t.Error("expected post id in response")
	}

	if repo.createdPost.PostedByType != "human" {
		t.Errorf("expected posted_by_type 'human', got '%s'", repo.createdPost.PostedByType)
	}

	if repo.createdPost.PostedByID != "user-123" {
		t.Errorf("expected posted_by_id 'user-123', got '%s'", repo.createdPost.PostedByID)
	}
}

// TestCreatePost_NoAuth tests 401 when not authenticated.
func TestCreatePost_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Test Problem Title",
		"description": "Test description",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No auth context
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreatePost_InvalidType tests 400 for invalid type.
func TestCreatePost_InvalidType(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "invalid",
		"title":       "Test Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "INVALID_TYPE" {
		t.Errorf("expected error code INVALID_TYPE, got %v", errObj["code"])
	}
}

// TestCreatePost_TitleTooShort tests 400 for title < 10 chars.
func TestCreatePost_TitleTooShort(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Short", // Less than 10 chars
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
	}
}

// TestCreatePost_TitleTooLong tests 400 for title > 200 chars.
func TestCreatePost_TitleTooLong(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	body := map[string]interface{}{
		"type":        "problem",
		"title":       string(longTitle),
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreatePost_DescriptionTooShort tests 400 for description < 50 chars.
func TestCreatePost_DescriptionTooShort(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Valid Title That Is Long Enough",
		"description": "Too short", // Less than 50 chars
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreatePost_MissingTitle tests 400 for missing title.
func TestCreatePost_MissingTitle(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreatePost_InvalidJSON tests 400 for malformed JSON.
func TestCreatePost_InvalidJSON(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
