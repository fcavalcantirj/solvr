// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// ============================================================================
// POST /v1/questions/:id/answers - Create Answer Tests
// ============================================================================

// TestCreateAnswer_Success tests successful answer creation.
func TestCreateAnswer_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "This is a test answer with sufficient content length to be a valid answer.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] == nil {
		t.Error("expected answer id in response")
	}
}

// TestCreateAnswer_NoAuth tests 401 when not authenticated.
func TestCreateAnswer_NoAuth(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "Test answer content",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreateAnswer_QuestionNotFound tests 404 when question doesn't exist.
func TestCreateAnswer_QuestionNotFound(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestion(nil) // Question not found

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "Test answer content",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/nonexistent/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestCreateAnswer_ContentRequired tests validation for empty content.
func TestCreateAnswer_ContentRequired(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// PATCH /v1/answers/:id - Update Answer Tests
// ============================================================================

// TestUpdateAnswer_Success tests successful answer update.
func TestUpdateAnswer_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	newContent := "Updated answer content that is long enough to be valid content for an answer."
	body := map[string]interface{}{
		"content": newContent,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/answer-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user") // Same as answer author
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUpdateAnswer_NoAuth tests 401 when not authenticated.
func TestUpdateAnswer_NoAuth(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "Updated content",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/answer-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUpdateAnswer_NotFound tests 404 when answer doesn't exist.
func TestUpdateAnswer_NotFound(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetAnswer(nil)
	repo.SetAnswersError(ErrAnswerNotFound)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "Updated content",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestUpdateAnswer_Forbidden tests 403 when not the author.
func TestUpdateAnswer_Forbidden(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "Updated content",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/answer-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "different-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// DELETE /v1/answers/:id - Delete Answer Tests
// ============================================================================

// TestDeleteAnswer_Success tests successful answer deletion by author.
func TestDeleteAnswer_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/answers/answer-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user") // Same as answer author
	w := httptest.NewRecorder()

	handler.DeleteAnswer(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if repo.deletedAnswerID != "answer-123" {
		t.Errorf("expected deleted answer ID 'answer-123', got '%s'", repo.deletedAnswerID)
	}
}

// TestDeleteAnswer_ByAdmin tests successful answer deletion by admin.
func TestDeleteAnswer_ByAdmin(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/answers/answer-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "admin-user", "admin") // Admin user
	w := httptest.NewRecorder()

	handler.DeleteAnswer(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// TestDeleteAnswer_NoAuth tests 401 when not authenticated.
func TestDeleteAnswer_NoAuth(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/answers/answer-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.DeleteAnswer(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestDeleteAnswer_Forbidden tests 403 when not the author or admin.
func TestDeleteAnswer_Forbidden(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/answers/answer-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "different-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.DeleteAnswer(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/answers/:id/vote - Vote on Answer Tests
// ============================================================================

// TestVoteOnAnswer_UpvoteSuccess tests successful upvote.
func TestVoteOnAnswer_UpvoteSuccess(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/answers/answer-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "voter-user", "user")
	w := httptest.NewRecorder()

	handler.VoteOnAnswer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestVoteOnAnswer_DownvoteSuccess tests successful downvote.
func TestVoteOnAnswer_DownvoteSuccess(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"direction": "down",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/answers/answer-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "voter-user", "user")
	w := httptest.NewRecorder()

	handler.VoteOnAnswer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestVoteOnAnswer_NoAuth tests 401 when not authenticated.
func TestVoteOnAnswer_NoAuth(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/answers/answer-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.VoteOnAnswer(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestVoteOnAnswer_InvalidDirection tests 400 for invalid vote direction.
func TestVoteOnAnswer_InvalidDirection(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"direction": "invalid",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/answers/answer-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "voter-user", "user")
	w := httptest.NewRecorder()

	handler.VoteOnAnswer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestVoteOnAnswer_AnswerNotFound tests 404 when answer doesn't exist.
func TestVoteOnAnswer_AnswerNotFound(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetAnswer(nil)
	repo.SetAnswersError(ErrAnswerNotFound)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/answers/nonexistent/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "voter-user", "user")
	w := httptest.NewRecorder()

	handler.VoteOnAnswer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/questions/:id/accept/:aid - Accept Answer Tests
// ============================================================================

// TestAcceptAnswer_Success tests successful answer acceptance.
func TestAcceptAnswer_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)
	answer := createTestAnswer("answer-456", "question-123")
	repo.SetAnswer(&answer)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/accept/answer-456", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	rctx.URLParams.Add("aid", "answer-456")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-123", "user") // Same as question author
	w := httptest.NewRecorder()

	handler.AcceptAnswer(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["answer_id"] != "answer-456" {
		t.Errorf("expected answer_id 'answer-456', got %v", resp["answer_id"])
	}
}

// TestAcceptAnswer_NoAuth tests 401 when not authenticated.
func TestAcceptAnswer_NoAuth(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/accept/answer-456", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	rctx.URLParams.Add("aid", "answer-456")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.AcceptAnswer(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestAcceptAnswer_QuestionNotFound tests 404 when question doesn't exist.
func TestAcceptAnswer_QuestionNotFound(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestion(nil) // Question not found

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/nonexistent/accept/answer-456", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	rctx.URLParams.Add("aid", "answer-456")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.AcceptAnswer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestAcceptAnswer_Forbidden tests 403 when not the question owner.
func TestAcceptAnswer_Forbidden(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/accept/answer-456", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	rctx.URLParams.Add("aid", "answer-456")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "different-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.AcceptAnswer(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestAcceptAnswer_AnswerNotFound tests 404 when answer doesn't exist.
func TestAcceptAnswer_AnswerNotFound(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)
	repo.SetAnswer(nil)
	repo.SetAnswersError(ErrAnswerNotFound)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/accept/nonexistent", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	rctx.URLParams.Add("aid", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-123", "user") // Same as question author
	w := httptest.NewRecorder()

	handler.AcceptAnswer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
