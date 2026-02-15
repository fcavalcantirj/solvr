package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// Mock repositories for contributions tests
// ============================================================================

type MockContribAnswersRepository struct {
	answers []models.AnswerWithContext
	total   int
}

func (m *MockContribAnswersRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.AnswerWithContext, int, error) {
	return m.answers, m.total, nil
}

type MockContribApproachesRepository struct {
	approaches []models.ApproachWithContext
	total      int
}

func (m *MockContribApproachesRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ApproachWithContext, int, error) {
	return m.approaches, m.total, nil
}

type MockContribResponsesRepository struct {
	responses []models.ResponseWithContext
	total     int
}

func (m *MockContribResponsesRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ResponseWithContext, int, error) {
	return m.responses, m.total, nil
}

// ============================================================================
// Helper to build a handler with all contribution repos
// ============================================================================

func newContribHandler(
	answersRepo *MockContribAnswersRepository,
	approachesRepo *MockContribApproachesRepository,
	responsesRepo *MockContribResponsesRepository,
) *UsersHandler {
	userRepo := NewMockUsersUserRepository()
	// Add a user for lookups
	userRepo.users["a1b2c3d4-e5f6-7890-abcd-ef1234567890"] = &models.User{
		ID:          "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Username:    "testuser",
		DisplayName: "Test User",
	}

	handler := NewUsersHandler(userRepo, nil)
	handler.SetContributionRepositories(answersRepo, approachesRepo, responsesRepo)
	return handler
}

// ============================================================================
// Tests for GET /v1/users/{id}/contributions
// ============================================================================

// TestGetUserContributions_AllTypes tests that contributions returns answers + approaches + responses mixed.
func TestGetUserContributions_AllTypes(t *testing.T) {
	now := time.Now()

	answersRepo := &MockContribAnswersRepository{
		answers: []models.AnswerWithContext{
			{
				AnswerWithAuthor: models.AnswerWithAuthor{
					Answer: models.Answer{
						ID:         "ans-1",
						QuestionID: "q-1",
						AuthorType: models.AuthorTypeHuman,
						AuthorID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						Content:    "This is my answer to the question",
						CreatedAt:  now.Add(-1 * time.Hour),
					},
				},
				QuestionTitle: "How to fix X?",
			},
		},
		total: 1,
	}

	approachesRepo := &MockContribApproachesRepository{
		approaches: []models.ApproachWithContext{
			{
				ApproachWithAuthor: models.ApproachWithAuthor{
					Approach: models.Approach{
						ID:         "app-1",
						ProblemID:  "p-1",
						AuthorType: models.AuthorTypeHuman,
						AuthorID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						Angle:      "Try using method Y",
						Status:     models.ApproachStatusWorking,
						CreatedAt:  now.Add(-2 * time.Hour),
					},
				},
				ProblemTitle: "Cannot deploy to production",
			},
		},
		total: 1,
	}

	responsesRepo := &MockContribResponsesRepository{
		responses: []models.ResponseWithContext{
			{
				ResponseWithAuthor: models.ResponseWithAuthor{
					Response: models.Response{
						ID:           "resp-1",
						IdeaID:       "i-1",
						AuthorType:   models.AuthorTypeHuman,
						AuthorID:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						Content:      "Great idea, I would also add...",
						ResponseType: models.ResponseTypeBuild,
						CreatedAt:    now,
					},
				},
				IdeaTitle: "AI-powered code review",
			},
		},
		total: 1,
	}

	handler := newContribHandler(answersRepo, approachesRepo, responsesRepo)

	userID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+userID+"/contributions", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			Page    int  `json:"page"`
			PerPage int  `json:"per_page"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 3 contributions total (1 answer + 1 approach + 1 response)
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 contributions, got %d", len(resp.Data))
	}

	// Should be sorted by created_at DESC (newest first)
	// resp-1 (now) > ans-1 (now-1h) > app-1 (now-2h)
	if resp.Data[0].Type != models.ContributionTypeResponse {
		t.Errorf("expected first item type 'response', got '%s'", resp.Data[0].Type)
	}
	if resp.Data[1].Type != models.ContributionTypeAnswer {
		t.Errorf("expected second item type 'answer', got '%s'", resp.Data[1].Type)
	}
	if resp.Data[2].Type != models.ContributionTypeApproach {
		t.Errorf("expected third item type 'approach', got '%s'", resp.Data[2].Type)
	}

	// Verify meta
	if resp.Meta.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Meta.Total)
	}
	if resp.Meta.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Meta.Page)
	}

	// Verify contribution fields
	item := resp.Data[0]
	if item.ID != "resp-1" {
		t.Errorf("expected ID 'resp-1', got '%s'", item.ID)
	}
	if item.ParentID != "i-1" {
		t.Errorf("expected parent_id 'i-1', got '%s'", item.ParentID)
	}
	if item.ParentTitle != "AI-powered code review" {
		t.Errorf("expected parent_title 'AI-powered code review', got '%s'", item.ParentTitle)
	}
	if item.ParentType != "idea" {
		t.Errorf("expected parent_type 'idea', got '%s'", item.ParentType)
	}
}

// TestGetUserContributions_FilterAnswers tests ?type=answers filter.
func TestGetUserContributions_FilterAnswers(t *testing.T) {
	now := time.Now()

	answersRepo := &MockContribAnswersRepository{
		answers: []models.AnswerWithContext{
			{
				AnswerWithAuthor: models.AnswerWithAuthor{
					Answer: models.Answer{
						ID:         "ans-1",
						QuestionID: "q-1",
						AuthorType: models.AuthorTypeHuman,
						AuthorID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						Content:    "My answer",
						CreatedAt:  now,
					},
				},
				QuestionTitle: "Test question",
			},
		},
		total: 1,
	}
	approachesRepo := &MockContribApproachesRepository{}
	responsesRepo := &MockContribResponsesRepository{}

	handler := newContribHandler(answersRepo, approachesRepo, responsesRepo)

	userID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+userID+"/contributions?type=answers", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Data))
	}

	if resp.Data[0].Type != models.ContributionTypeAnswer {
		t.Errorf("expected type 'answer', got '%s'", resp.Data[0].Type)
	}

	if resp.Meta.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Meta.Total)
	}
}

// TestGetUserContributions_FilterApproaches tests ?type=approaches filter.
func TestGetUserContributions_FilterApproaches(t *testing.T) {
	now := time.Now()

	answersRepo := &MockContribAnswersRepository{}
	approachesRepo := &MockContribApproachesRepository{
		approaches: []models.ApproachWithContext{
			{
				ApproachWithAuthor: models.ApproachWithAuthor{
					Approach: models.Approach{
						ID:         "app-1",
						ProblemID:  "p-1",
						AuthorType: models.AuthorTypeHuman,
						AuthorID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						Angle:      "My approach",
						Status:     models.ApproachStatusSucceeded,
						CreatedAt:  now,
					},
				},
				ProblemTitle: "Test problem",
			},
		},
		total: 1,
	}
	responsesRepo := &MockContribResponsesRepository{}

	handler := newContribHandler(answersRepo, approachesRepo, responsesRepo)

	userID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+userID+"/contributions?type=approaches", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 approach, got %d", len(resp.Data))
	}

	if resp.Data[0].Type != models.ContributionTypeApproach {
		t.Errorf("expected type 'approach', got '%s'", resp.Data[0].Type)
	}

	if resp.Data[0].Status != "succeeded" {
		t.Errorf("expected status 'succeeded', got '%s'", resp.Data[0].Status)
	}
}

// TestGetUserContributions_FilterResponses tests ?type=responses filter.
func TestGetUserContributions_FilterResponses(t *testing.T) {
	now := time.Now()

	answersRepo := &MockContribAnswersRepository{}
	approachesRepo := &MockContribApproachesRepository{}
	responsesRepo := &MockContribResponsesRepository{
		responses: []models.ResponseWithContext{
			{
				ResponseWithAuthor: models.ResponseWithAuthor{
					Response: models.Response{
						ID:           "resp-1",
						IdeaID:       "i-1",
						AuthorType:   models.AuthorTypeHuman,
						AuthorID:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						Content:      "My response to idea",
						ResponseType: models.ResponseTypeCritique,
						CreatedAt:    now,
					},
				},
				IdeaTitle: "Test idea",
			},
		},
		total: 1,
	}

	handler := newContribHandler(answersRepo, approachesRepo, responsesRepo)

	userID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+userID+"/contributions?type=responses", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resp.Data))
	}

	if resp.Data[0].Type != models.ContributionTypeResponse {
		t.Errorf("expected type 'response', got '%s'", resp.Data[0].Type)
	}
}

// TestGetUserContributions_Pagination tests page/per_page params.
func TestGetUserContributions_Pagination(t *testing.T) {
	now := time.Now()

	// Create 3 answers to test pagination
	answers := make([]models.AnswerWithContext, 3)
	for i := 0; i < 3; i++ {
		answers[i] = models.AnswerWithContext{
			AnswerWithAuthor: models.AnswerWithAuthor{
				Answer: models.Answer{
					ID:         "ans-" + string(rune('1'+i)),
					QuestionID: "q-1",
					AuthorType: models.AuthorTypeHuman,
					AuthorID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
					Content:    "Answer content",
					CreatedAt:  now.Add(-time.Duration(i) * time.Hour),
				},
			},
			QuestionTitle: "Test question",
		}
	}

	answersRepo := &MockContribAnswersRepository{answers: answers, total: 3}
	approachesRepo := &MockContribApproachesRepository{}
	responsesRepo := &MockContribResponsesRepository{}

	handler := newContribHandler(answersRepo, approachesRepo, responsesRepo)

	userID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+userID+"/contributions?page=1&per_page=2", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			Page    int  `json:"page"`
			PerPage int  `json:"per_page"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// per_page=2, so should get at most 2 items
	if len(resp.Data) > 2 {
		t.Errorf("expected at most 2 items with per_page=2, got %d", len(resp.Data))
	}

	if resp.Meta.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Meta.Page)
	}

	if resp.Meta.PerPage != 2 {
		t.Errorf("expected per_page 2, got %d", resp.Meta.PerPage)
	}

	if resp.Meta.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Meta.Total)
	}

	if !resp.Meta.HasMore {
		t.Error("expected has_more to be true since total (3) > per_page (2)")
	}
}

// TestGetUserContributions_InvalidUserID tests non-UUID returns 400.
func TestGetUserContributions_InvalidUserID(t *testing.T) {
	handler := newContribHandler(
		&MockContribAnswersRepository{},
		&MockContribApproachesRepository{},
		&MockContribResponsesRepository{},
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/not-a-uuid/contributions", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetUserContributions_UserNotFound tests that unknown user returns 404.
func TestGetUserContributions_UserNotFound(t *testing.T) {
	handler := newContribHandler(
		&MockContribAnswersRepository{},
		&MockContribApproachesRepository{},
		&MockContribResponsesRepository{},
	)

	unknownID := "b2c3d4e5-f6a7-8901-bcde-f12345678901"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+unknownID+"/contributions", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", unknownID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetUserContributions_Empty tests empty contributions returns empty array.
func TestGetUserContributions_Empty(t *testing.T) {
	handler := newContribHandler(
		&MockContribAnswersRepository{},
		&MockContribApproachesRepository{},
		&MockContribResponsesRepository{},
	)

	userID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+userID+"/contributions", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data == nil {
		t.Error("expected empty array [], got null")
	}

	if len(resp.Data) != 0 {
		t.Errorf("expected 0 contributions, got %d", len(resp.Data))
	}

	if resp.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Meta.Total)
	}
}

// ============================================================================
// Tests for GET /v1/me/contributions (updated to use same logic)
// ============================================================================

// TestGetMyContributions_ReturnsContributions tests that /me/contributions
// returns real contribution data (not just posts).
func TestGetMyContributions_ReturnsContributions(t *testing.T) {
	now := time.Now()

	answersRepo := &MockContribAnswersRepository{
		answers: []models.AnswerWithContext{
			{
				AnswerWithAuthor: models.AnswerWithAuthor{
					Answer: models.Answer{
						ID:         "ans-1",
						QuestionID: "q-1",
						AuthorType: models.AuthorTypeHuman,
						AuthorID:   "user-123",
						Content:    "My answer",
						CreatedAt:  now,
					},
				},
				QuestionTitle: "Test question",
			},
		},
		total: 1,
	}
	approachesRepo := &MockContribApproachesRepository{}
	responsesRepo := &MockContribResponsesRepository{}

	userRepo := NewMockUsersUserRepository()
	handler := NewUsersHandler(userRepo, nil)
	handler.SetContributionRepositories(answersRepo, approachesRepo, responsesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/contributions", nil)
	rr := httptest.NewRecorder()

	// Set JWT claims in context to simulate authenticated human
	claims := &auth.Claims{UserID: "user-123", Role: "user"}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.GetMyContributions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.ContributionItem `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 contribution, got %d", len(resp.Data))
	}

	if resp.Data[0].Type != models.ContributionTypeAnswer {
		t.Errorf("expected type 'answer', got '%s'", resp.Data[0].Type)
	}
}

// TestGetMyContributions_Unauthenticated tests that /me/contributions requires auth.
func TestGetMyContributions_Unauthenticated(t *testing.T) {
	handler := NewUsersHandler(nil, nil)
	handler.SetContributionRepositories(
		&MockContribAnswersRepository{},
		&MockContribApproachesRepository{},
		&MockContribResponsesRepository{},
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/contributions", nil)
	rr := httptest.NewRecorder()

	handler.GetMyContributions(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rr.Code, rr.Body.String())
	}
}
