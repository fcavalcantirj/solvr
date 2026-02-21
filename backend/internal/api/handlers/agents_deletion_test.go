package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockAgentRepositoryWithDelete extends mock repository to support soft delete testing.
// This mock implements the behavior for PRD-v5 Task 22:
// - Delete() sets DeletedAt timestamp (soft delete)
// - FindByID() filters out soft-deleted agents (deleted_at IS NOT NULL)
// - FindByAPIKeyHash() filters out soft-deleted agents
type MockAgentRepositoryWithDelete struct {
	agents      map[string]*models.Agent
	stats       map[string]*models.AgentStats
	findError   error
	deleteError error
}

func NewMockAgentRepositoryWithDelete() *MockAgentRepositoryWithDelete {
	return &MockAgentRepositoryWithDelete{
		agents: make(map[string]*models.Agent),
		stats:  make(map[string]*models.AgentStats),
	}
}

// Delete implements soft delete by setting DeletedAt timestamp.
func (m *MockAgentRepositoryWithDelete) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}

	agent, ok := m.agents[id]
	if !ok {
		return db.ErrAgentNotFound
	}

	// Check if already soft-deleted (mimics: WHERE deleted_at IS NULL)
	if agent.DeletedAt != nil {
		return db.ErrAgentNotFound
	}

	// Soft delete: set DeletedAt timestamp
	now := time.Now()
	agent.DeletedAt = &now

	return nil
}

// FindByID filters out soft-deleted agents (WHERE deleted_at IS NULL).
func (m *MockAgentRepositoryWithDelete) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	agent, ok := m.agents[id]
	if !ok {
		return nil, db.ErrAgentNotFound
	}

	// Filter soft-deleted agents
	if agent.DeletedAt != nil {
		return nil, db.ErrAgentNotFound
	}

	return agent, nil
}

// FindByAPIKeyHash filters out soft-deleted agents.
func (m *MockAgentRepositoryWithDelete) FindByAPIKeyHash(ctx context.Context, hash string) (*models.Agent, error) {
	for _, agent := range m.agents {
		if agent.APIKeyHash == hash && agent.DeletedAt == nil {
			return agent, nil
		}
	}
	return nil, db.ErrAgentNotFound
}

// GetAgentStats returns stats for an agent.
func (m *MockAgentRepositoryWithDelete) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	stats, ok := m.stats[agentID]
	if !ok {
		return &models.AgentStats{}, nil
	}
	return stats, nil
}

// Stub methods to satisfy AgentRepositoryInterface
func (m *MockAgentRepositoryWithDelete) Create(ctx context.Context, agent *models.Agent) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	return nil, db.ErrAgentNotFound
}

func (m *MockAgentRepositoryWithDelete) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	return nil, nil
}

func (m *MockAgentRepositoryWithDelete) Update(ctx context.Context, agent *models.Agent) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) RevokeAPIKey(ctx context.Context, agentID string) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	return nil, 0, nil
}

func (m *MockAgentRepositoryWithDelete) LinkHuman(ctx context.Context, agentID, humanID string) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) AddReputation(ctx context.Context, agentID string, amount int) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) GrantHumanBackedBadge(ctx context.Context, agentID string) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error) {
	return m.FindByAPIKeyHash(ctx, key)
}

func (m *MockAgentRepositoryWithDelete) List(ctx context.Context, opts models.AgentListOptions) ([]models.AgentWithPostCount, int, error) {
	return nil, 0, nil
}

func (m *MockAgentRepositoryWithDelete) CountActive(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *MockAgentRepositoryWithDelete) CountHumanBacked(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *MockAgentRepositoryWithDelete) UpdateLastSeen(ctx context.Context, id string) error {
	return nil
}

func (m *MockAgentRepositoryWithDelete) UpdateIdentity(ctx context.Context, agentID string, amcpAID *string, keriPublicKey *string) (*models.Agent, error) {
	agent, exists := m.agents[agentID]
	if !exists {
		return nil, ErrAgentNotFound
	}
	if amcpAID != nil {
		agent.AMCPAID = *amcpAID
	}
	if keriPublicKey != nil {
		agent.KERIPublicKey = *keriPublicKey
	}
	return agent, nil
}

// TestDeleteAgentMe_Success verifies authenticated agent can delete their own account (soft delete).
//
// TDD RED PHASE: This test will FAIL because:
// - AgentsHandler.DeleteMe() method doesn't exist yet (Task 22)
// - Agent model doesn't have DeletedAt field yet (Task 22)
//
// Expected behavior after Task 22:
// - HTTP 200 OK
// - Response: {"message": "Agent deleted successfully"}
// - Agent's DeletedAt field is set (not nil)
// - Subsequent FindByID() returns ErrAgentNotFound (soft-deleted agents filtered)
// - Subsequent FindByAPIKeyHash() returns ErrAgentNotFound (API key invalid)
func TestDeleteAgentMe_Success(t *testing.T) {
	// Setup: create mock repository with soft delete support
	repo := NewMockAgentRepositoryWithDelete()
	agentID := "test_agent"
	apiKeyHash := "hashed_api_key_123"

	repo.agents[agentID] = &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		APIKeyHash:  apiKeyHash,
		Status:      "active",
		Reputation:  100,
		CreatedAt:   testTime,
		UpdatedAt:   testTime,
		// DeletedAt: nil (not deleted yet)
	}

	handler := NewAgentsHandler(repo, "test-secret")

	// Create request: DELETE /v1/agents/me with API key authentication
	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/me", nil)

	// Add agent to context (simulating API key middleware)
	agent := repo.agents[agentID]
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	// Execute: call DeleteMe handler
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Assert: response body contains success message
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["message"] != "Agent deleted successfully" {
		t.Errorf("expected message 'Agent deleted successfully', got %q", response["message"])
	}

	// Assert: agent is soft-deleted (DeletedAt is set)
	deletedAgent := repo.agents[agentID]
	if deletedAgent.DeletedAt == nil {
		t.Error("expected agent.DeletedAt to be set, got nil")
	}

	// Assert: subsequent FindByID returns ErrAgentNotFound (soft-deleted agents filtered)
	foundAgent, err := repo.FindByID(context.Background(), agentID)
	if err != db.ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound for soft-deleted agent, got err=%v, agent=%v", err, foundAgent)
	}

	// Assert: subsequent FindByAPIKeyHash returns ErrAgentNotFound (API key invalid)
	foundByKey, err := repo.FindByAPIKeyHash(context.Background(), apiKeyHash)
	if err != db.ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound for deleted agent API key, got err=%v, agent=%v", err, foundByKey)
	}
}

// TestDeleteAgentMe_Unauthorized verifies requests without authentication are rejected.
//
// TDD RED PHASE: This test will FAIL because DeleteMe() method doesn't exist yet.
//
// Expected behavior after Task 22:
// - HTTP 401 Unauthorized
// - Response: {"error": {"code": "UNAUTHORIZED", "message": "API key authentication required"}}
func TestDeleteAgentMe_Unauthorized(t *testing.T) {
	// Setup: create handler with mock repository
	repo := NewMockAgentRepositoryWithDelete()
	handler := NewAgentsHandler(repo, "test-secret")

	// Create request: DELETE /v1/agents/me WITHOUT authentication
	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/me", nil)
	// No agent in context - unauthenticated request

	// Execute
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Assert: error response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'error' field")
	}

	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %q", errObj["code"])
	}

	if errObj["message"] != "API key authentication required" {
		t.Errorf("expected message 'API key authentication required', got %q", errObj["message"])
	}
}

// TestDeleteAgentMe_HumanCannotDelete verifies humans with JWT cannot delete agents.
//
// TDD RED PHASE: This test will FAIL because DeleteMe() method doesn't exist yet.
//
// Expected behavior after Task 22:
// - HTTP 403 Forbidden
// - Response: {"error": {"code": "FORBIDDEN", "message": "Humans cannot delete agents. Use DELETE /v1/me instead."}}
//
// Rationale: Humans should never be able to delete agent accounts using this endpoint.
// Humans should use DELETE /v1/me for their own account deletion.
func TestDeleteAgentMe_HumanCannotDelete(t *testing.T) {
	// Setup: create handler with mock repository
	repo := NewMockAgentRepositoryWithDelete()
	handler := NewAgentsHandler(repo, "test-secret")

	// Create request: DELETE /v1/agents/me with JWT authentication (human user)
	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/me", nil)

	// Add JWT claims to context (simulating JWT middleware)
	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 403 Forbidden
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	// Assert: error response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'error' field")
	}

	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %q", errObj["code"])
	}

	if errObj["message"] != "Humans cannot delete agents. Use DELETE /v1/me instead." {
		t.Errorf("expected message 'Humans cannot delete agents. Use DELETE /v1/me instead.', got %q", errObj["message"])
	}
}

// TestDeleteAgentMe_AlreadyDeleted verifies cannot delete an already-deleted agent.
//
// TDD RED PHASE: This test will FAIL because:
// - DeleteMe() method doesn't exist yet
// - Agent model doesn't have DeletedAt field yet
//
// Expected behavior after Task 22:
// - HTTP 404 Not Found
// - Response: {"error": {"code": "NOT_FOUND", "message": "Agent not found or already deleted"}}
//
// Rationale: FindByID filters WHERE deleted_at IS NULL, so soft-deleted agents
// appear as "not found" when attempting operations on them.
func TestDeleteAgentMe_AlreadyDeleted(t *testing.T) {
	// Setup: create agent that's already soft-deleted
	repo := NewMockAgentRepositoryWithDelete()
	agentID := "deleted_agent"
	deletedTime := testTime.Add(-24 * time.Hour)

	repo.agents[agentID] = &models.Agent{
		ID:          agentID,
		DisplayName: "Deleted Agent",
		APIKeyHash:  "hashed_key",
		Status:      "active",
		CreatedAt:   testTime.Add(-72 * time.Hour),
		UpdatedAt:   testTime.Add(-24 * time.Hour),
		DeletedAt:   &deletedTime, // Already deleted
	}

	handler := NewAgentsHandler(repo, "test-secret")

	// Create request: DELETE /v1/agents/me with API key for deleted agent
	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/me", nil)

	// Note: In real scenario, API key middleware would reject this because
	// FindByAPIKeyHash filters deleted agents. But for this test, we're
	// simulating the edge case where context has a deleted agent.
	agent := repo.agents[agentID]
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 404 Not Found
	// This happens because Delete() checks deleted_at IS NULL before deleting
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Assert: error response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'error' field")
	}

	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %q", errObj["code"])
	}

	if errObj["message"] != "Agent not found or already deleted" {
		t.Errorf("expected message 'Agent not found or already deleted', got %q", errObj["message"])
	}
}

// TestDeleteAgentMe_RepositoryError verifies proper error handling for database failures.
//
// TDD RED PHASE: This test will FAIL because DeleteMe() method doesn't exist yet.
//
// Expected behavior after Task 22:
// - HTTP 500 Internal Server Error
// - Response: {"error": {"code": "INTERNAL_ERROR", "message": "Failed to delete agent"}}
func TestDeleteAgentMe_RepositoryError(t *testing.T) {
	// Setup: create mock that returns error on Delete
	repo := NewMockAgentRepositoryWithDelete()
	agentID := "test_agent"

	repo.agents[agentID] = &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		APIKeyHash:  "hashed_key",
		Status:      "active",
		CreatedAt:   testTime,
		UpdatedAt:   testTime,
	}

	// Configure mock to return error on Delete
	repo.deleteError = errors.New("database connection failed")

	handler := NewAgentsHandler(repo, "test-secret")

	// Create request: DELETE /v1/agents/me with API key authentication
	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/me", nil)

	agent := repo.agents[agentID]
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 500 Internal Server Error
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	// Assert: error response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'error' field")
	}

	if errObj["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected error code INTERNAL_ERROR, got %q", errObj["code"])
	}

	if errObj["message"] != "Failed to delete agent" {
		t.Errorf("expected message 'Failed to delete agent', got %q", errObj["message"])
	}
}
