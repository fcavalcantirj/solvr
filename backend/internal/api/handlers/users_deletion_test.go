package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// MockMeUserRepositoryWithDelete extends MockMeUserRepository to support soft delete testing.
// This mock implements the behavior that will be added in Tasks 11-12:
// - Delete() sets DeletedAt timestamp (soft delete)
// - FindByID() filters out soft-deleted users (deleted_at IS NOT NULL)
type MockMeUserRepositoryWithDelete struct {
	users       map[string]*models.User
	stats       map[string]*models.UserStats
	findError   error
	deleteError error
}

func NewMockMeUserRepositoryWithDelete() *MockMeUserRepositoryWithDelete {
	return &MockMeUserRepositoryWithDelete{
		users: make(map[string]*models.User),
		stats: make(map[string]*models.UserStats),
	}
}

// Delete implements soft delete by setting DeletedAt timestamp.
// This mimics the behavior that will be implemented in Task 12.
func (m *MockMeUserRepositoryWithDelete) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}

	user, ok := m.users[id]
	if !ok {
		return db.ErrNotFound
	}

	// Check if already soft-deleted (mimics: WHERE deleted_at IS NULL)
	if user.DeletedAt != nil {
		return db.ErrNotFound
	}

	// Soft delete: set DeletedAt timestamp
	now := time.Now()
	user.DeletedAt = &now

	return nil
}

// FindByID filters out soft-deleted users (WHERE deleted_at IS NULL).
// This mimics the behavior that will be implemented in Task 12.
func (m *MockMeUserRepositoryWithDelete) FindByID(ctx context.Context, id string) (*models.User, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	user, ok := m.users[id]
	if !ok {
		return nil, db.ErrNotFound
	}

	// Filter soft-deleted users
	// Note: User model doesn't have DeletedAt field yet - Task 11 will add it
	if user.DeletedAt != nil {
		return nil, db.ErrNotFound
	}

	return user, nil
}

// GetUserStats returns stats for a user.
func (m *MockMeUserRepositoryWithDelete) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	stats, ok := m.stats[userID]
	if !ok {
		return &models.UserStats{}, nil
	}
	return stats, nil
}

// MockPool implements PoolInterface for testing agent unclaiming.
type MockPool struct {
	agents map[string]*models.Agent
}

func NewMockPool() *MockPool {
	return &MockPool{
		agents: make(map[string]*models.Agent),
	}
}

func (p *MockPool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	// Simulate: UPDATE agents SET human_id = NULL WHERE human_id = $1
	if len(arguments) > 0 {
		userID, ok := arguments[0].(string)
		if ok {
			for _, agent := range p.agents {
				if agent.HumanID != nil && *agent.HumanID == userID {
					agent.HumanID = nil
				}
			}
		}
	}
	return pgconn.CommandTag{}, nil
}

// MockMeUserRepositoryWithCascade extends soft delete mock to track cascade effects.
// This tests that:
// - Posts remain visible after user deletion (no cascade delete)
// - Agents get unclaimed (human_id set to NULL)
type MockMeUserRepositoryWithCascade struct {
	*MockMeUserRepositoryWithDelete
	posts  map[string]*models.Post
	agents map[string]*models.Agent
}

func NewMockMeUserRepositoryWithCascade() *MockMeUserRepositoryWithCascade {
	return &MockMeUserRepositoryWithCascade{
		MockMeUserRepositoryWithDelete: NewMockMeUserRepositoryWithDelete(),
		posts:                          make(map[string]*models.Post),
		agents:                         make(map[string]*models.Agent),
	}
}

// Delete implements soft delete with cascade effects:
// - User gets soft-deleted (deleted_at set)
// - Agents get unclaimed (human_id = NULL)
// - Posts remain unchanged (no cascade)
func (m *MockMeUserRepositoryWithCascade) Delete(ctx context.Context, id string) error {
	// First, do the soft delete on user
	if err := m.MockMeUserRepositoryWithDelete.Delete(ctx, id); err != nil {
		return err
	}

	// Unclaim all agents owned by this user
	// This mimics: UPDATE agents SET human_id = NULL WHERE human_id = $1
	for _, agent := range m.agents {
		if agent.HumanID != nil && *agent.HumanID == id {
			agent.HumanID = nil
		}
	}

	// Posts remain unchanged (no cascade delete)
	// This is intentional per SPEC.md: posts persist after user deletion

	return nil
}

// TestDeleteMe_Success verifies authenticated user can delete their own account (soft delete).
//
// TDD RED PHASE: This test will FAIL because:
// - MeHandler.DeleteMe() method doesn't exist yet (Task 12)
// - User model doesn't have DeletedAt field yet (Task 11)
//
// Expected behavior after Tasks 11-12:
// - HTTP 200 OK
// - Response: {"data": {"message": "Account deleted successfully"}}
// - User's DeletedAt field is set (not nil)
// - Subsequent FindByID() returns ErrNotFound (soft-deleted users filtered)
func TestDeleteMe_Success(t *testing.T) {
	// Setup: create mock repository with soft delete support
	repo := NewMockMeUserRepositoryWithDelete()
	userID := "user-123"
	repo.users[userID] = &models.User{
		ID:          userID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Role:        models.UserRoleUser,
		CreatedAt:   testTime,
		UpdatedAt:   testTime,
		// DeletedAt: nil (not deleted yet)
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	// Create request: DELETE /v1/me with JWT authentication
	req := httptest.NewRequest(http.MethodDelete, "/v1/me", nil)

	// Add JWT claims to context (authenticated user)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
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

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	if data["message"] != "Account deleted successfully" {
		t.Errorf("expected message 'Account deleted successfully', got %q", data["message"])
	}

	// Assert: user is soft-deleted (DeletedAt is set)
	user := repo.users[userID]
	if user.DeletedAt == nil {
		t.Error("expected user.DeletedAt to be set, got nil")
	}

	// Assert: subsequent FindByID returns ErrNotFound (soft-deleted users filtered)
	foundUser, err := repo.FindByID(context.Background(), userID)
	if err != db.ErrNotFound {
		t.Errorf("expected ErrNotFound for soft-deleted user, got err=%v, user=%v", err, foundUser)
	}
}

// TestDeleteMe_Unauthorized verifies requests without authentication are rejected.
//
// TDD RED PHASE: This test will FAIL because DeleteMe() method doesn't exist yet.
//
// Expected behavior after Task 12:
// - HTTP 401 Unauthorized
// - Response: {"error": {"code": "UNAUTHORIZED", "message": "authentication required"}}
func TestDeleteMe_Unauthorized(t *testing.T) {
	// Setup: create handler with mock repository
	repo := NewMockMeUserRepositoryWithDelete()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	// Create request: DELETE /v1/me WITHOUT authentication
	req := httptest.NewRequest(http.MethodDelete, "/v1/me", nil)
	// No claims in context - unauthenticated request

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

	if errObj["message"] != "authentication required" {
		t.Errorf("expected message 'authentication required', got %q", errObj["message"])
	}
}

// TestDeleteMe_AgentCannotDeleteHumans verifies agents with API keys cannot delete user accounts.
//
// TDD RED PHASE: This test will FAIL because DeleteMe() method doesn't exist yet.
//
// Expected behavior after Task 12:
// - HTTP 403 Forbidden
// - Response: {"error": {"code": "FORBIDDEN", "message": "agents cannot delete user accounts"}}
//
// Rationale: Agents should never be able to delete human user accounts, even if they have
// API key authentication. This is a security measure to prevent abuse.
func TestDeleteMe_AgentCannotDeleteHumans(t *testing.T) {
	// Setup: create handler with mock repository
	repo := NewMockMeUserRepositoryWithDelete()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	// Create request: DELETE /v1/me with agent authentication (API key)
	req := httptest.NewRequest(http.MethodDelete, "/v1/me", nil)

	// Add agent to context (simulating API key middleware)
	agent := &models.Agent{
		ID:          "test-agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
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

	if errObj["message"] != "agents cannot delete user accounts" {
		t.Errorf("expected message 'agents cannot delete user accounts', got %q", errObj["message"])
	}
}

// TestDeleteMe_AlreadyDeleted verifies cannot delete an already-deleted account.
//
// TDD RED PHASE: This test will FAIL because:
// - DeleteMe() method doesn't exist yet
// - User model doesn't have DeletedAt field yet
//
// Expected behavior after Tasks 11-12:
// - HTTP 404 Not Found
// - Response: {"error": {"code": "NOT_FOUND", "message": "user not found"}}
//
// Rationale: FindByID filters WHERE deleted_at IS NULL, so soft-deleted users
// appear as "not found" when attempting operations on them.
func TestDeleteMe_AlreadyDeleted(t *testing.T) {
	// Setup: create user that's already soft-deleted
	repo := NewMockMeUserRepositoryWithDelete()
	userID := "deleted-user"
	deletedTime := testTime.Add(-24 * time.Hour)
	repo.users[userID] = &models.User{
		ID:          userID,
		Username:    "deleteduser",
		DisplayName: "Deleted User",
		Email:       "deleted@example.com",
		Role:        models.UserRoleUser,
		CreatedAt:   testTime.Add(-72 * time.Hour),
		UpdatedAt:   testTime.Add(-24 * time.Hour),
		DeletedAt:   &deletedTime, // Already deleted
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	// Create request: DELETE /v1/me with JWT for deleted user
	req := httptest.NewRequest(http.MethodDelete, "/v1/me", nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "deleted@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 404 Not Found
	// This happens because FindByID filters soft-deleted users (WHERE deleted_at IS NULL)
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

	if errObj["message"] != "user not found" {
		t.Errorf("expected message 'user not found', got %q", errObj["message"])
	}
}

// TestDeleteMe_CascadeChecks verifies correct cascade behavior on user deletion.
//
// TDD RED PHASE: This test will FAIL because:
// - DeleteMe() method doesn't exist yet
// - User model doesn't have DeletedAt field yet
// - AgentRepository.UnclaimByHumanID() doesn't exist yet (Task 12 will add it)
//
// Expected behavior after Task 12:
// - User is soft-deleted (deleted_at set)
// - All posts by user remain visible (no cascade delete)
// - All agents owned by user get unclaimed (human_id = NULL)
//
// Per SPEC.md:
// - Posts persist after user deletion (content preservation)
// - Agents become available for claiming by other users
func TestDeleteMe_CascadeChecks(t *testing.T) {
	// Setup: create user with posts and agents
	repo := NewMockMeUserRepositoryWithCascade()
	userID := "user-with-content"

	// Create user
	repo.users[userID] = &models.User{
		ID:          userID,
		Username:    "contentcreator",
		DisplayName: "Content Creator",
		Email:       "creator@example.com",
		Role:        models.UserRoleUser,
		CreatedAt:   testTime,
		UpdatedAt:   testTime,
	}

	// Create 3 posts by this user
	repo.posts["post-1"] = &models.Post{
		ID:           "post-1",
		Title:        "First Post",
		Type:         "problem",
		PostedByType: "human",
		PostedByID:   userID,
	}
	repo.posts["post-2"] = &models.Post{
		ID:           "post-2",
		Title:        "Second Post",
		Type:         "question",
		PostedByType: "human",
		PostedByID:   userID,
	}
	repo.posts["post-3"] = &models.Post{
		ID:           "post-3",
		Title:        "Third Post",
		Type:         "idea",
		PostedByType: "human",
		PostedByID:   userID,
	}

	// Create 2 agents owned by this user
	humanID1 := userID
	humanID2 := userID
	repo.agents["agent-1"] = &models.Agent{
		ID:          "agent-1",
		DisplayName: "First Agent",
		HumanID:     &humanID1,
		Status:      "active",
	}
	repo.agents["agent-2"] = &models.Agent{
		ID:          "agent-2",
		DisplayName: "Second Agent",
		HumanID:     &humanID2,
		Status:      "active",
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	// Create request: DELETE /v1/me with JWT
	req := httptest.NewRequest(http.MethodDelete, "/v1/me", nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "creator@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.DeleteMe(rr, req) // This will FAIL - method doesn't exist yet

	// Assert: HTTP 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Assert: user is deleted (deleted_at set)
	user := repo.users[userID]
	if user.DeletedAt == nil {
		t.Error("expected user.DeletedAt to be set, got nil")
	}

	// Assert: all 3 posts still exist (no cascade delete)
	if len(repo.posts) != 3 {
		t.Errorf("expected 3 posts to remain, got %d", len(repo.posts))
	}

	for postID, post := range repo.posts {
		if post.PostedByID != userID {
			t.Errorf("post %s has wrong author: expected %q, got %q", postID, userID, post.PostedByID)
		}
	}

	// Assert: both agents have been unclaimed (human_id = NULL)
	agent1 := repo.agents["agent-1"]
	if agent1.HumanID != nil {
		t.Errorf("expected agent-1.HumanID to be nil (unclaimed), got %q", *agent1.HumanID)
	}

	agent2 := repo.agents["agent-2"]
	if agent2.HumanID != nil {
		t.Errorf("expected agent-2.HumanID to be nil (unclaimed), got %q", *agent2.HumanID)
	}
}
