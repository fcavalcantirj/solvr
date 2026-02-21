package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// Tests for PATCH /v1/agents/me/identity — KERI identity management
// ============================================================================

// TestUpdateIdentity_SetAMCPFields tests that agent can PATCH amcp_aid and keri_public_key.
// Per prd-v5: Agent PATCHes amcp_aid and keri_public_key, both fields updated on agent.
func TestUpdateIdentity_SetAMCPFields(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Create agent via mock
	apiKey := "solvr_testapikey123"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["agent_identity_test"] = &models.Agent{
		ID:          "agent_identity_test",
		DisplayName: "Identity Agent",
		APIKeyHash:  apiKeyHash,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateIdentityRequest{
		AMCPAID:       strPtr("ELI7pg979AdhmvrjDeam2eAO2sRnVerenQApFL0Zef1U"),
		KERIPublicKey: strPtr("DG1q5cYDaQvEgYwK0FJC_O5YfHxr_4HP0gzgrSS8BQMC"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["agent_identity_test"])

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify AMCP AID is set
	if resp.Data.Agent.AMCPAID != "ELI7pg979AdhmvrjDeam2eAO2sRnVerenQApFL0Zef1U" {
		t.Errorf("expected amcp_aid set, got '%s'", resp.Data.Agent.AMCPAID)
	}

	// Verify KERI public key is set
	if resp.Data.Agent.KERIPublicKey != "DG1q5cYDaQvEgYwK0FJC_O5YfHxr_4HP0gzgrSS8BQMC" {
		t.Errorf("expected keri_public_key set, got '%s'", resp.Data.Agent.KERIPublicKey)
	}

	// Verify HasAMCPIdentity is true
	if !resp.Data.Agent.HasAMCPIdentity {
		t.Error("expected has_amcp_identity to be true")
	}

	// Verify pinning quota was set (1 GB for AMCP agents)
	if resp.Data.Agent.PinningQuotaBytes != AMCPDefaultQuotaBytes {
		t.Errorf("expected pinning_quota_bytes %d, got %d", AMCPDefaultQuotaBytes, resp.Data.Agent.PinningQuotaBytes)
	}
}

// TestUpdateIdentity_DuplicateAID tests that two agents cannot share the same amcp_aid.
// Per prd-v5: 409 conflict when duplicate amcp_aid is detected.
func TestUpdateIdentity_DuplicateAID(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	sharedAID := "ELI7pg979AdhmvrjDeam2eAO2sRnVerenQApFL0Zef1U"

	// First agent already has this AMCP AID
	apiKey1 := "solvr_testapikey_one"
	apiKeyHash1, _ := auth.HashAPIKey(apiKey1)
	repo.agents["agent_one"] = &models.Agent{
		ID:              "agent_one",
		DisplayName:     "Agent One",
		APIKeyHash:      apiKeyHash1,
		HasAMCPIdentity: true,
		AMCPAID:         sharedAID,
		Status:          "active",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Second agent tries to claim the same AID
	apiKey2 := "solvr_testapikey_two"
	apiKeyHash2, _ := auth.HashAPIKey(apiKey2)
	repo.agents["agent_two"] = &models.Agent{
		ID:          "agent_two",
		DisplayName: "Agent Two",
		APIKeyHash:  apiKeyHash2,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateIdentityRequest{
		AMCPAID: strPtr(sharedAID),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["agent_two"])

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409 for duplicate AID, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify error code
	var errResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&errResp)
	errorObj := errResp["error"].(map[string]interface{})
	if errorObj["code"] != "DUPLICATE_AID" {
		t.Errorf("expected error code DUPLICATE_AID, got %s", errorObj["code"])
	}
}

// TestUpdateIdentity_HumanCannot tests that a human JWT cannot call PATCH /v1/agents/me/identity.
// Per prd-v5: 403 for human callers — agent-only endpoint.
func TestUpdateIdentity_HumanCannot(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := UpdateIdentityRequest{
		AMCPAID: strPtr("ELI7pg979AdhmvrjDeam2eAO2sRnVerenQApFL0Zef1U"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for human caller, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateIdentity_PartialUpdate tests that sending only keri_public_key leaves amcp_aid unchanged.
// Per prd-v5: Partial update — only provided fields are updated.
func TestUpdateIdentity_PartialUpdate(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	existingAID := "ELI7pg979AdhmvrjDeam2eAO2sRnVerenQApFL0Zef1U"

	apiKey := "solvr_testapikey_partial"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["agent_partial"] = &models.Agent{
		ID:              "agent_partial",
		DisplayName:     "Partial Agent",
		APIKeyHash:      apiKeyHash,
		HasAMCPIdentity: true,
		AMCPAID:         existingAID,
		Status:          "active",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Only send keri_public_key, no amcp_aid
	reqBody := UpdateIdentityRequest{
		KERIPublicKey: strPtr("DG1q5cYDaQvEgYwK0FJC_O5YfHxr_4HP0gzgrSS8BQMC"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["agent_partial"])

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify amcp_aid is unchanged
	if resp.Data.Agent.AMCPAID != existingAID {
		t.Errorf("expected amcp_aid '%s' unchanged, got '%s'", existingAID, resp.Data.Agent.AMCPAID)
	}

	// Verify keri_public_key is set
	if resp.Data.Agent.KERIPublicKey != "DG1q5cYDaQvEgYwK0FJC_O5YfHxr_4HP0gzgrSS8BQMC" {
		t.Errorf("expected keri_public_key set, got '%s'", resp.Data.Agent.KERIPublicKey)
	}
}

// TestUpdateIdentity_NoAuth tests that unauthenticated request returns 401.
func TestUpdateIdentity_NoAuth(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := UpdateIdentityRequest{
		AMCPAID: strPtr("ELI7pg979AdhmvrjDeam2eAO2sRnVerenQApFL0Zef1U"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No auth context

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateIdentity_InvalidJSON tests that invalid JSON returns 400.
func TestUpdateIdentity_InvalidJSON(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	apiKey := "solvr_testapikey_invalid"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["agent_invalid_json"] = &models.Agent{
		ID:         "agent_invalid_json",
		APIKeyHash: apiKeyHash,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["agent_invalid_json"])

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", rr.Code)
	}
}

// TestUpdateIdentity_AMCPAIDTooLong tests AMCP AID validation (max 255 chars).
func TestUpdateIdentity_AMCPAIDTooLong(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	apiKey := "solvr_testapikey_long"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["agent_long_aid"] = &models.Agent{
		ID:         "agent_long_aid",
		APIKeyHash: apiKeyHash,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	longAID := strings.Repeat("A", 256)
	reqBody := UpdateIdentityRequest{
		AMCPAID: &longAID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["agent_long_aid"])

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for AID too long, got %d", rr.Code)
	}
}

// TestUpdateIdentity_KERIPublicKeyTooLong tests KERI public key validation (max 512 chars).
func TestUpdateIdentity_KERIPublicKeyTooLong(t *testing.T) {
	repo := NewMockAgentRepositoryWithIdentity()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	apiKey := "solvr_testapikey_longkey"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["agent_long_key"] = &models.Agent{
		ID:         "agent_long_key",
		APIKeyHash: apiKeyHash,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	longKey := strings.Repeat("D", 513)
	reqBody := UpdateIdentityRequest{
		KERIPublicKey: &longKey,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/me/identity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["agent_long_key"])

	rr := httptest.NewRecorder()
	handler.UpdateIdentity(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for KERI key too long, got %d", rr.Code)
	}
}

// ============================================================================
// Mock repository with identity support
// ============================================================================

// MockAgentRepositoryWithIdentity wraps MockAgentRepositoryWithNameLookup for identity tests.
// Inherits UpdateIdentity from MockAgentRepository (via MockAgentRepositoryWithNameLookup).
type MockAgentRepositoryWithIdentity struct {
	*MockAgentRepositoryWithNameLookup
}

func NewMockAgentRepositoryWithIdentity() *MockAgentRepositoryWithIdentity {
	return &MockAgentRepositoryWithIdentity{
		MockAgentRepositoryWithNameLookup: NewMockAgentRepositoryWithNameLookup(),
	}
}
