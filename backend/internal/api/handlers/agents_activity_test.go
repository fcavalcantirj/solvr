package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockActivityAgentRepository extends MockAgentRepository with GetActivity.
type MockActivityAgentRepository struct {
	*MockAgentRepository
	activities    []models.ActivityItem
	activitiesErr error
	activityTotal int
}

func NewMockActivityAgentRepository() *MockActivityAgentRepository {
	return &MockActivityAgentRepository{
		MockAgentRepository: NewMockAgentRepository(),
		activities:          []models.ActivityItem{},
		activityTotal:       0,
	}
}

func (m *MockActivityAgentRepository) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	if m.activitiesErr != nil {
		return nil, 0, m.activitiesErr
	}
	// Check if agent exists first
	if _, exists := m.agents[agentID]; !exists {
		return nil, 0, ErrAgentNotFound
	}
	return m.activities, m.activityTotal, nil
}

func TestGetActivity_Success(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Set up agent
	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Set up activity data
	now := time.Now()
	repo.activities = []models.ActivityItem{
		{
			ID:        "post-1",
			Type:      "post",
			Action:    "created",
			Title:     "How to handle async errors?",
			PostType:  "question",
			Status:    "open",
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:          "answer-1",
			Type:        "answer",
			Action:      "answered",
			Title:       "Use try-catch blocks",
			Status:      "accepted",
			CreatedAt:   now.Add(-30 * time.Minute),
			TargetID:    "post-2",
			TargetTitle: "Error handling in Go",
		},
	}
	repo.activityTotal = 2

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/activity", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ActivityResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 activities, got %d", len(resp.Data))
	}

	if resp.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Meta.Total)
	}
	if resp.Meta.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Meta.Page)
	}
	if resp.Meta.PerPage != 20 {
		t.Errorf("expected per_page 20, got %d", resp.Meta.PerPage)
	}
	if resp.Meta.HasMore != false {
		t.Error("expected has_more false")
	}
}

func TestGetActivity_NotFound(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/nonexistent/activity", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var errResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&errResp)
	errorObj := errResp["error"].(map[string]interface{})
	if errorObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %s", errorObj["code"])
	}
}

func TestGetActivity_Pagination(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:      "test_agent",
		HumanID: &humanID,
	}

	// Create 25 activities, simulating total of 50
	activities := make([]models.ActivityItem, 25)
	for i := 0; i < 25; i++ {
		activities[i] = models.ActivityItem{
			ID:        "post-" + string(rune('a'+i)),
			Type:      "post",
			Action:    "created",
			Title:     "Post " + string(rune('a'+i)),
			PostType:  "question",
			Status:    "open",
			CreatedAt: time.Now(),
		}
	}
	repo.activities = activities
	repo.activityTotal = 50

	// Test page=2, per_page=25
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/activity?page=2&per_page=25", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ActivityResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.Page != 2 {
		t.Errorf("expected page 2, got %d", resp.Meta.Page)
	}
	if resp.Meta.PerPage != 25 {
		t.Errorf("expected per_page 25, got %d", resp.Meta.PerPage)
	}
	if resp.Meta.Total != 50 {
		t.Errorf("expected total 50, got %d", resp.Meta.Total)
	}
	if resp.Meta.HasMore != false {
		t.Error("expected has_more false for last page")
	}
}

func TestGetActivity_PerPageMax(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:      "test_agent",
		HumanID: &humanID,
	}
	repo.activityTotal = 100

	// Test per_page > 50 should be capped at 50
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/activity?per_page=100", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp ActivityResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Meta.PerPage != 50 {
		t.Errorf("expected per_page capped at 50, got %d", resp.Meta.PerPage)
	}
}

func TestGetActivity_InvalidPageParam(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:      "test_agent",
		HumanID: &humanID,
	}
	repo.activityTotal = 10

	// Test invalid page (should default to 1)
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/activity?page=-1", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp ActivityResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Meta.Page != 1 {
		t.Errorf("expected page defaulted to 1, got %d", resp.Meta.Page)
	}
}

func TestGetActivity_EmptyResult(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:      "test_agent",
		HumanID: &humanID,
	}
	// Empty activities
	repo.activities = []models.ActivityItem{}
	repo.activityTotal = 0

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/activity", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp ActivityResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Data) != 0 {
		t.Errorf("expected 0 activities, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Meta.Total)
	}
	if resp.Meta.HasMore != false {
		t.Error("expected has_more false")
	}
}

func TestGetActivity_HasMoreTrue(t *testing.T) {
	repo := NewMockActivityAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:      "test_agent",
		HumanID: &humanID,
	}

	// Return 20 items but total is 30
	activities := make([]models.ActivityItem, 20)
	for i := 0; i < 20; i++ {
		activities[i] = models.ActivityItem{
			ID:        "post-" + string(rune('a'+i)),
			Type:      "post",
			Action:    "created",
			CreatedAt: time.Now(),
		}
	}
	repo.activities = activities
	repo.activityTotal = 30

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/activity?page=1&per_page=20", nil)
	rr := httptest.NewRecorder()

	handler.GetActivity(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp ActivityResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Meta.HasMore != true {
		t.Error("expected has_more true when there are more pages")
	}
}
