package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestAgentLifecycle_Integration tests the full agent lifecycle:
// Create agent → Get profile → Update → Regenerate key → Revoke key
// This verifies all operations work together correctly.
func TestAgentLifecycle_Integration(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")
	userID := "integration-test-user"

	// Step 1: Create agent
	t.Run("Step1_CreateAgent", func(t *testing.T) {
		reqBody := CreateAgentRequest{
			ID:          "lifecycle_test_agent",
			DisplayName: "Lifecycle Test Agent",
			Bio:         "An agent for integration testing",
			Specialties: []string{"testing", "integration"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.CreateAgent(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("Step 1 failed: expected status 201, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp CreateAgentResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 1 failed: failed to decode response: %v", err)
		}

		// Verify created agent
		if resp.Data.Agent.ID != "lifecycle_test_agent" {
			t.Errorf("Step 1: expected ID 'lifecycle_test_agent', got '%s'", resp.Data.Agent.ID)
		}
		if resp.Data.Agent.DisplayName != "Lifecycle Test Agent" {
			t.Errorf("Step 1: expected display name 'Lifecycle Test Agent', got '%s'", resp.Data.Agent.DisplayName)
		}
		if resp.Data.Agent.Bio != "An agent for integration testing" {
			t.Errorf("Step 1: expected correct bio, got '%s'", resp.Data.Agent.Bio)
		}
		if len(resp.Data.Agent.Specialties) != 2 {
			t.Errorf("Step 1: expected 2 specialties, got %d", len(resp.Data.Agent.Specialties))
		}
		if resp.Data.APIKey == "" {
			t.Error("Step 1: expected API key in response")
		}
		if len(resp.Data.APIKey) < 10 || resp.Data.APIKey[:6] != "solvr_" {
			t.Errorf("Step 1: expected API key with solvr_ prefix, got '%s'", resp.Data.APIKey)
		}
		if resp.Data.Agent.HumanID == nil || *resp.Data.Agent.HumanID != userID {
			t.Error("Step 1: expected human_id to be set correctly")
		}

		t.Logf("Step 1 passed: Agent created with ID=%s", resp.Data.Agent.ID)
	})

	// Step 2: Get agent profile
	t.Run("Step2_GetProfile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/agents/lifecycle_test_agent", nil)
		rr := httptest.NewRecorder()

		handler.GetAgent(rr, req, "lifecycle_test_agent")

		if rr.Code != http.StatusOK {
			t.Fatalf("Step 2 failed: expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp GetAgentResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 2 failed: failed to decode response: %v", err)
		}

		// Verify agent profile
		if resp.Data.Agent.ID != "lifecycle_test_agent" {
			t.Errorf("Step 2: expected ID 'lifecycle_test_agent', got '%s'", resp.Data.Agent.ID)
		}
		if resp.Data.Agent.DisplayName != "Lifecycle Test Agent" {
			t.Errorf("Step 2: expected display name 'Lifecycle Test Agent', got '%s'", resp.Data.Agent.DisplayName)
		}
		// Stats should be included
		if resp.Data.Stats.Reputation < 0 {
			t.Error("Step 2: expected valid reputation in stats")
		}

		t.Logf("Step 2 passed: Retrieved agent profile with reputation=%d", resp.Data.Stats.Reputation)
	})

	// Step 3: Update agent
	t.Run("Step3_UpdateAgent", func(t *testing.T) {
		newDisplayName := "Updated Lifecycle Agent"
		newBio := "Updated bio for the lifecycle test"
		reqBody := UpdateAgentRequest{
			DisplayName: &newDisplayName,
			Bio:         &newBio,
			Specialties: []string{"testing", "integration", "lifecycle"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/v1/agents/lifecycle_test_agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.UpdateAgent(rr, req, "lifecycle_test_agent")

		if rr.Code != http.StatusOK {
			t.Fatalf("Step 3 failed: expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp GetAgentResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 3 failed: failed to decode response: %v", err)
		}

		// Verify updated fields
		if resp.Data.Agent.DisplayName != "Updated Lifecycle Agent" {
			t.Errorf("Step 3: expected display name 'Updated Lifecycle Agent', got '%s'", resp.Data.Agent.DisplayName)
		}
		if resp.Data.Agent.Bio != "Updated bio for the lifecycle test" {
			t.Errorf("Step 3: expected updated bio, got '%s'", resp.Data.Agent.Bio)
		}
		if len(resp.Data.Agent.Specialties) != 3 {
			t.Errorf("Step 3: expected 3 specialties, got %d", len(resp.Data.Agent.Specialties))
		}

		t.Logf("Step 3 passed: Agent updated with new display name and bio")
	})

	// Step 4: Regenerate API key
	t.Run("Step4_RegenerateAPIKey", func(t *testing.T) {
		// Get the old API key hash before regeneration
		oldAgent := repo.agents["lifecycle_test_agent"]
		oldHash := oldAgent.APIKeyHash

		req := httptest.NewRequest(http.MethodPost, "/v1/agents/lifecycle_test_agent/api-key", nil)
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.RegenerateAPIKey(rr, req, "lifecycle_test_agent")

		if rr.Code != http.StatusOK {
			t.Fatalf("Step 4 failed: expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 4 failed: failed to decode response: %v", err)
		}

		// Verify new API key is returned
		data := resp["data"].(map[string]interface{})
		newAPIKey := data["api_key"].(string)
		if newAPIKey == "" {
			t.Error("Step 4: expected new API key in response")
		}
		if len(newAPIKey) < 10 || newAPIKey[:6] != "solvr_" {
			t.Errorf("Step 4: expected new API key with solvr_ prefix, got '%s'", newAPIKey)
		}

		// Verify the hash was updated (old key should no longer work)
		newAgent := repo.agents["lifecycle_test_agent"]
		if newAgent.APIKeyHash == oldHash {
			t.Error("Step 4: expected API key hash to be different after regeneration")
		}
		if newAgent.APIKeyHash == "" {
			t.Error("Step 4: expected new API key hash to be set")
		}

		t.Logf("Step 4 passed: API key regenerated, old hash replaced")
	})

	// Step 5: Revoke API key
	t.Run("Step5_RevokeAPIKey", func(t *testing.T) {
		// Verify agent has API key before revocation
		beforeAgent := repo.agents["lifecycle_test_agent"]
		if beforeAgent.APIKeyHash == "" {
			t.Fatal("Step 5 precondition failed: agent should have API key hash before revocation")
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1/agents/lifecycle_test_agent/api-key", nil)
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.RevokeAPIKey(rr, req, "lifecycle_test_agent")

		if rr.Code != http.StatusNoContent {
			t.Fatalf("Step 5 failed: expected status 204, got %d: %s", rr.Code, rr.Body.String())
		}

		// Verify API key hash is cleared
		afterAgent := repo.agents["lifecycle_test_agent"]
		if afterAgent.APIKeyHash != "" {
			t.Error("Step 5: expected API key hash to be empty after revocation")
		}

		t.Logf("Step 5 passed: API key revoked, agent can no longer authenticate")
	})

	// Final verification: Get profile should still work (public endpoint)
	t.Run("Step6_VerifyFinalState", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/agents/lifecycle_test_agent", nil)
		rr := httptest.NewRecorder()

		handler.GetAgent(rr, req, "lifecycle_test_agent")

		if rr.Code != http.StatusOK {
			t.Fatalf("Final verification failed: expected status 200, got %d", rr.Code)
		}

		var resp GetAgentResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Final verification failed: failed to decode response: %v", err)
		}

		// Verify final state reflects all updates
		if resp.Data.Agent.DisplayName != "Updated Lifecycle Agent" {
			t.Errorf("Final: expected display name 'Updated Lifecycle Agent', got '%s'", resp.Data.Agent.DisplayName)
		}
		if resp.Data.Agent.Bio != "Updated bio for the lifecycle test" {
			t.Errorf("Final: expected updated bio, got '%s'", resp.Data.Agent.Bio)
		}

		t.Logf("Final verification passed: Agent in correct final state")
	})
}

// TestAgentLifecycle_OwnershipEnforcement tests that non-owners cannot modify agents.
func TestAgentLifecycle_OwnershipEnforcement(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")
	ownerID := "owner-user-id"
	nonOwnerID := "non-owner-user-id"

	// Create agent as owner
	humanID := ownerID
	repo.agents["owned_agent"] = &models.Agent{
		ID:          "owned_agent",
		DisplayName: "Owned Agent",
		HumanID:     &humanID,
		APIKeyHash:  "some_hash",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test: Non-owner cannot update
	t.Run("NonOwnerCannotUpdate", func(t *testing.T) {
		newName := "Hacked Name"
		reqBody := UpdateAgentRequest{
			DisplayName: &newName,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/v1/agents/owned_agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, nonOwnerID, "nonowner@example.com", "user")

		rr := httptest.NewRecorder()
		handler.UpdateAgent(rr, req, "owned_agent")

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})

	// Test: Non-owner cannot regenerate API key
	t.Run("NonOwnerCannotRegenerateKey", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/owned_agent/api-key", nil)
		req = addJWTClaimsToContext(req, nonOwnerID, "nonowner@example.com", "user")

		rr := httptest.NewRecorder()
		handler.RegenerateAPIKey(rr, req, "owned_agent")

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})

	// Test: Non-owner cannot revoke API key
	t.Run("NonOwnerCannotRevokeKey", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/agents/owned_agent/api-key", nil)
		req = addJWTClaimsToContext(req, nonOwnerID, "nonowner@example.com", "user")

		rr := httptest.NewRecorder()
		handler.RevokeAPIKey(rr, req, "owned_agent")

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})

	// Test: Owner can still perform operations
	t.Run("OwnerCanUpdate", func(t *testing.T) {
		newName := "Legitimately Updated"
		reqBody := UpdateAgentRequest{
			DisplayName: &newName,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/v1/agents/owned_agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, ownerID, "owner@example.com", "user")

		rr := httptest.NewRecorder()
		handler.UpdateAgent(rr, req, "owned_agent")

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// TestAgentLifecycle_APIKeyFlow verifies the complete API key flow.
func TestAgentLifecycle_APIKeyFlow(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")
	userID := "apikey-test-user"

	// Create agent and capture initial API key
	var initialAPIKey string
	t.Run("CreateAndCaptureInitialKey", func(t *testing.T) {
		reqBody := CreateAgentRequest{
			ID:          "apikey_flow_agent",
			DisplayName: "API Key Flow Agent",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.CreateAgent(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rr.Code)
		}

		var resp CreateAgentResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		initialAPIKey = resp.Data.APIKey

		if initialAPIKey == "" {
			t.Fatal("expected initial API key")
		}
	})

	// Regenerate and verify old key would no longer work
	var newAPIKey string
	t.Run("RegenerateAndVerifyOldKeyInvalid", func(t *testing.T) {
		// Store hash before regen for comparison
		oldHash := repo.agents["apikey_flow_agent"].APIKeyHash

		req := httptest.NewRequest(http.MethodPost, "/v1/agents/apikey_flow_agent/api-key", nil)
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.RegenerateAPIKey(rr, req, "apikey_flow_agent")

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&resp)
		data := resp["data"].(map[string]interface{})
		newAPIKey = data["api_key"].(string)

		// New key should be different from initial
		if newAPIKey == initialAPIKey {
			t.Error("expected new API key to be different from initial")
		}

		// Hash should be different
		newHash := repo.agents["apikey_flow_agent"].APIKeyHash
		if newHash == oldHash {
			t.Error("expected hash to be updated")
		}

		// Verify old key would not authenticate (hash mismatch)
		// The hash of the initial key should not match the new hash
		oldKeyHash, _ := auth.HashAPIKey(initialAPIKey)
		if oldKeyHash == newHash {
			t.Error("old key hash should not match new stored hash")
		}
	})

	// Revoke and verify agent cannot authenticate
	t.Run("RevokeAndVerifyNoAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/agents/apikey_flow_agent/api-key", nil)
		req = addJWTClaimsToContext(req, userID, "test@example.com", "user")

		rr := httptest.NewRecorder()
		handler.RevokeAPIKey(rr, req, "apikey_flow_agent")

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", rr.Code)
		}

		// Verify hash is cleared
		agent := repo.agents["apikey_flow_agent"]
		if agent.APIKeyHash != "" {
			t.Error("expected API key hash to be empty after revocation")
		}
	})
}

// TestAgentLifecycle_ConcurrentOperations tests that operations don't interfere.
func TestAgentLifecycle_ConcurrentOperations(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	user1ID := "user1"
	user2ID := "user2"

	// Create agents for different users
	createAgent := func(userID, agentID, displayName string) {
		reqBody := CreateAgentRequest{
			ID:          agentID,
			DisplayName: displayName,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, userID, userID+"@example.com", "user")
		rr := httptest.NewRecorder()
		handler.CreateAgent(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("failed to create agent %s: status %d", agentID, rr.Code)
		}
	}

	createAgent(user1ID, "user1_agent", "User 1 Agent")
	createAgent(user2ID, "user2_agent", "User 2 Agent")

	// User 1 should be able to update their agent but not user 2's
	t.Run("User1CanOnlyUpdateOwnAgent", func(t *testing.T) {
		newName := "User 1 Updated"
		reqBody := UpdateAgentRequest{DisplayName: &newName}
		body, _ := json.Marshal(reqBody)

		// User 1 updates own agent - should succeed
		req := httptest.NewRequest(http.MethodPatch, "/v1/agents/user1_agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, user1ID, "user1@example.com", "user")
		rr := httptest.NewRecorder()
		handler.UpdateAgent(rr, req, "user1_agent")
		if rr.Code != http.StatusOK {
			t.Errorf("user1 updating own agent: expected 200, got %d", rr.Code)
		}

		// User 1 tries to update user 2's agent - should fail
		body, _ = json.Marshal(reqBody)
		req = httptest.NewRequest(http.MethodPatch, "/v1/agents/user2_agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, user1ID, "user1@example.com", "user")
		rr = httptest.NewRecorder()
		handler.UpdateAgent(rr, req, "user2_agent")
		if rr.Code != http.StatusForbidden {
			t.Errorf("user1 updating user2 agent: expected 403, got %d", rr.Code)
		}
	})

	// Both users can read each other's agents (public profiles)
	t.Run("BothUsersCanReadProfiles", func(t *testing.T) {
		// User 1 reads user 2's profile
		req := httptest.NewRequest(http.MethodGet, "/v1/agents/user2_agent", nil)
		rr := httptest.NewRecorder()
		handler.GetAgent(rr, req, "user2_agent")
		if rr.Code != http.StatusOK {
			t.Errorf("reading user2 agent: expected 200, got %d", rr.Code)
		}

		// User 2 reads user 1's profile
		req = httptest.NewRequest(http.MethodGet, "/v1/agents/user1_agent", nil)
		rr = httptest.NewRecorder()
		handler.GetAgent(rr, req, "user1_agent")
		if rr.Code != http.StatusOK {
			t.Errorf("reading user1 agent: expected 200, got %d", rr.Code)
		}
	})
}

// MockAgentRepositoryWithActivity extends MockAgentRepository with activity support.
type MockAgentRepositoryWithActivity struct {
	*MockAgentRepository
	activities map[string][]models.ActivityItem
}

func NewMockAgentRepositoryWithActivity() *MockAgentRepositoryWithActivity {
	return &MockAgentRepositoryWithActivity{
		MockAgentRepository: NewMockAgentRepository(),
		activities:          make(map[string][]models.ActivityItem),
	}
}

func (m *MockAgentRepositoryWithActivity) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	if _, exists := m.agents[agentID]; !exists {
		return nil, 0, ErrAgentNotFound
	}
	activities, ok := m.activities[agentID]
	if !ok {
		return []models.ActivityItem{}, 0, nil
	}
	// Simple pagination
	start := (page - 1) * perPage
	if start >= len(activities) {
		return []models.ActivityItem{}, len(activities), nil
	}
	end := start + perPage
	if end > len(activities) {
		end = len(activities)
	}
	return activities[start:end], len(activities), nil
}
