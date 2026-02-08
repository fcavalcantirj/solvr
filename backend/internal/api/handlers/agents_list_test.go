package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestListAgents_Success tests successful listing of agents.
// Per API-001: GET /v1/agents returns paginated agent list.
func TestListAgents_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Set up test agents
	repo.agents["agent1"] = &models.Agent{
		ID:          "agent1",
		DisplayName: "Agent One",
		Bio:         "First test agent",
		Status:      "active",
		Reputation:       100,
		CreatedAt:   time.Now().Add(-1 * time.Hour),
		UpdatedAt:   time.Now(),
	}
	repo.agents["agent2"] = &models.Agent{
		ID:          "agent2",
		DisplayName: "Agent Two",
		Bio:         "Second test agent",
		Status:      "active",
		Reputation:       50,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp AgentsListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 agents, got %d", len(resp.Data))
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
		t.Errorf("expected has_more false, got true")
	}
}

// TestListAgents_Pagination tests pagination parameters.
func TestListAgents_Pagination(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Set up 5 test agents
	for i := 0; i < 5; i++ {
		id := "agent" + string(rune('a'+i))
		repo.agents[id] = &models.Agent{
			ID:          id,
			DisplayName: "Agent " + string(rune('A'+i)),
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}

	// Test with per_page=2
	req := httptest.NewRequest(http.MethodGet, "/v1/agents?per_page=2", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp AgentsListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 agents, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Meta.Total)
	}
	if resp.Meta.PerPage != 2 {
		t.Errorf("expected per_page 2, got %d", resp.Meta.PerPage)
	}
	if resp.Meta.HasMore != true {
		t.Errorf("expected has_more true for page 1 of 5 items with per_page=2")
	}
}

// TestListAgents_InvalidSort tests invalid sort parameter.
func TestListAgents_InvalidSort(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/agents?sort=invalid", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestListAgents_InvalidStatus tests invalid status parameter.
func TestListAgents_InvalidStatus(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/agents?status=invalid", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestListAgents_Empty tests empty result set.
func TestListAgents_Empty(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp AgentsListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 0 {
		t.Errorf("expected 0 agents, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Meta.Total)
	}
	if resp.Meta.HasMore != false {
		t.Errorf("expected has_more false for empty result")
	}
}

// TestListAgents_LimitAndOffset tests limit/offset style pagination.
func TestListAgents_LimitAndOffset(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Set up 5 test agents
	for i := 0; i < 5; i++ {
		id := "agent" + string(rune('a'+i))
		repo.agents[id] = &models.Agent{
			ID:          id,
			DisplayName: "Agent " + string(rune('A'+i)),
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}

	// Test with limit=2&offset=2
	req := httptest.NewRequest(http.MethodGet, "/v1/agents?limit=2&offset=2", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp AgentsListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.PerPage != 2 {
		t.Errorf("expected per_page 2, got %d", resp.Meta.PerPage)
	}
	if resp.Meta.Page != 2 {
		t.Errorf("expected page 2 (from offset 2), got %d", resp.Meta.Page)
	}
}

// TestListAgents_ValidSorts tests valid sort parameters are accepted.
func TestListAgents_ValidSorts(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	validSorts := []string{"newest", "oldest", "reputation", "posts", ""}
	for _, sort := range validSorts {
		url := "/v1/agents"
		if sort != "" {
			url = "/v1/agents?sort=" + sort
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rr := httptest.NewRecorder()
		handler.ListAgents(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("sort=%q: expected status 200, got %d: %s", sort, rr.Code, rr.Body.String())
		}
	}
}

// TestListAgents_ValidStatuses tests valid status parameters are accepted.
func TestListAgents_ValidStatuses(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	validStatuses := []string{"active", "pending", "all", ""}
	for _, status := range validStatuses {
		url := "/v1/agents"
		if status != "" {
			url = "/v1/agents?status=" + status
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rr := httptest.NewRecorder()
		handler.ListAgents(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status=%q: expected status 200, got %d: %s", status, rr.Code, rr.Body.String())
		}
	}
}
