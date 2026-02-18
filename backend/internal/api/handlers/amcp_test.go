package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ===== TDD: Registration with AMCP AID =====

func TestRegisterAgent_WithAMCPAid_SetsIdentityAndQuota(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-secret")

	body := `{"name": "amcp_agent", "description": "AMCP enabled agent", "amcp_aid": "EBfdlu8hAs6BUKxIMGHRszqdVpp6CmE4yx6G2p18MJwA"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RegisterAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Agent.HasAMCPIdentity {
		t.Error("expected has_amcp_identity to be true")
	}
	if resp.Agent.AMCPAID != "EBfdlu8hAs6BUKxIMGHRszqdVpp6CmE4yx6G2p18MJwA" {
		t.Errorf("expected amcp_aid to match, got %q", resp.Agent.AMCPAID)
	}
	if resp.Agent.PinningQuotaBytes != AMCPDefaultQuotaBytes {
		t.Errorf("expected pinning_quota_bytes %d, got %d", AMCPDefaultQuotaBytes, resp.Agent.PinningQuotaBytes)
	}
}

func TestRegisterAgent_WithoutAMCPAid_NoIdentityNoQuota(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-secret")

	body := `{"name": "regular_agent", "description": "Regular agent"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RegisterAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Agent.HasAMCPIdentity {
		t.Error("expected has_amcp_identity to be false for non-AMCP agent")
	}
	if resp.Agent.AMCPAID != "" {
		t.Errorf("expected empty amcp_aid, got %q", resp.Agent.AMCPAID)
	}
	if resp.Agent.PinningQuotaBytes != 0 {
		t.Errorf("expected pinning_quota_bytes 0, got %d", resp.Agent.PinningQuotaBytes)
	}
}

func TestRegisterAgent_InvalidAMCPAid_TooLong(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-secret")

	// Generate a 256-char AID (exceeds 255 limit)
	longAID := make([]byte, 256)
	for i := range longAID {
		longAID[i] = 'A'
	}

	body := `{"name": "bad_amcp", "amcp_aid": "` + string(longAID) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for long amcp_aid, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ===== TDD: GET /v1/me with AMCP agent =====

func TestMe_AgentWithAMCPIdentity(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:                "amcp_test_agent",
		DisplayName:       "AMCP Agent",
		Status:            "active",
		Reputation:        100,
		HasAMCPIdentity:   true,
		AMCPAID:           "EBfdlu8hAs6BUKxIMGHRszqdVpp6CmE4yx6G2p18MJwA",
		PinningQuotaBytes: AMCPDefaultQuotaBytes,
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// Verify AMCP fields are present in response
	amcpEnabled, ok := data["amcp_enabled"]
	if !ok {
		t.Fatal("response missing 'amcp_enabled' field")
	}
	if amcpEnabled != true {
		t.Errorf("expected amcp_enabled true, got %v", amcpEnabled)
	}

	quotaBytes, ok := data["pinning_quota_bytes"]
	if !ok {
		t.Fatal("response missing 'pinning_quota_bytes' field")
	}
	if int64(quotaBytes.(float64)) != AMCPDefaultQuotaBytes {
		t.Errorf("expected pinning_quota_bytes %d, got %v", AMCPDefaultQuotaBytes, quotaBytes)
	}
}

func TestMe_AgentWithoutAMCP_ShowsDisabled(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "regular_agent",
		DisplayName: "Regular Agent",
		Status:      "active",
		Reputation:  50,
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// Non-AMCP agent should have amcp_enabled = false
	amcpEnabled, ok := data["amcp_enabled"]
	if !ok {
		t.Fatal("response missing 'amcp_enabled' field")
	}
	if amcpEnabled != false {
		t.Errorf("expected amcp_enabled false, got %v", amcpEnabled)
	}

	// Non-AMCP agent should have pinning_quota_bytes = 0
	quotaBytes, ok := data["pinning_quota_bytes"]
	if !ok {
		t.Fatal("response missing 'pinning_quota_bytes' field")
	}
	if int64(quotaBytes.(float64)) != 0 {
		t.Errorf("expected pinning_quota_bytes 0, got %v", quotaBytes)
	}
}
