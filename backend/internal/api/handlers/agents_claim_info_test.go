package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// addChiURLParam adds a chi URL parameter to the request context.
func addChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestGetClaimInfo_ValidToken tests successful claim info retrieval.
func TestGetClaimInfo_ValidToken(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Bio:         "I am a test agent",
		Status:      "active",
		Reputation:  42,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now(),
	}

	validToken := &models.ClaimToken{
		ID:        "valid-token-id",
		Token:     "valid_token_value",
		AgentID:   testAgent.ID,
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now().Add(-30 * time.Minute),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/claim/valid_token_value", nil)
	req = addChiURLParam(req, "token", "valid_token_value")
	w := httptest.NewRecorder()

	handler.GetClaimInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp ClaimInfoResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.TokenValid {
		t.Error("expected token_valid to be true")
	}
	if resp.Agent == nil {
		t.Fatal("expected agent to be present")
	}
	if resp.Agent.ID != testAgent.ID {
		t.Errorf("expected agent ID %s, got %s", testAgent.ID, resp.Agent.ID)
	}
	if resp.Agent.DisplayName != testAgent.DisplayName {
		t.Errorf("expected display_name %s, got %s", testAgent.DisplayName, resp.Agent.DisplayName)
	}
	if resp.ExpiresAt == "" {
		t.Error("expected expires_at to be set")
	}
	if resp.Error != "" {
		t.Errorf("expected no error, got %s", resp.Error)
	}
}

// TestGetClaimInfo_TokenNotFound tests that invalid token returns token_valid=false.
func TestGetClaimInfo_TokenNotFound(t *testing.T) {
	agentRepo := NewMockAgentRepository()
	claimRepo := NewMockClaimTokenRepository()

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/claim/nonexistent", nil)
	req = addChiURLParam(req, "token", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetClaimInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp ClaimInfoResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TokenValid {
		t.Error("expected token_valid to be false")
	}
	if resp.Error == "" {
		t.Error("expected error message for invalid token")
	}
}

// TestGetClaimInfo_ExpiredToken tests that expired token returns token_valid=false.
func TestGetClaimInfo_ExpiredToken(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	expiredToken := &models.ClaimToken{
		ID:        "expired-token-id",
		Token:     "expired_token_value",
		AgentID:   testAgent.ID,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens[expiredToken.Token] = expiredToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/claim/expired_token_value", nil)
	req = addChiURLParam(req, "token", "expired_token_value")
	w := httptest.NewRecorder()

	handler.GetClaimInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp ClaimInfoResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TokenValid {
		t.Error("expected token_valid to be false for expired token")
	}
	if resp.Error == "" {
		t.Error("expected error message for expired token")
	}
}

// TestGetClaimInfo_UsedToken tests that used token returns token_valid=false.
func TestGetClaimInfo_UsedToken(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	usedAt := time.Now().Add(-30 * time.Minute)
	usedBy := "other-human-456"
	usedToken := &models.ClaimToken{
		ID:            "used-token-id",
		Token:         "used_token_value",
		AgentID:       testAgent.ID,
		ExpiresAt:     time.Now().Add(30 * time.Minute),
		UsedAt:        &usedAt,
		UsedByHumanID: &usedBy,
		CreatedAt:     time.Now().Add(-1 * time.Hour),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens[usedToken.Token] = usedToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/claim/used_token_value", nil)
	req = addChiURLParam(req, "token", "used_token_value")
	w := httptest.NewRecorder()

	handler.GetClaimInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp ClaimInfoResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TokenValid {
		t.Error("expected token_valid to be false for used token")
	}
	if resp.Error == "" {
		t.Error("expected error message for used token")
	}
}

// TestGetClaimInfo_NoClaimRepo tests error when claim repo is not configured.
func TestGetClaimInfo_NoClaimRepo(t *testing.T) {
	agentRepo := NewMockAgentRepository()
	handler := NewAgentsHandler(agentRepo, "test-secret")
	// NOT setting claim token repository

	req := httptest.NewRequest(http.MethodGet, "/v1/claim/some_token", nil)
	req = addChiURLParam(req, "token", "some_token")
	w := httptest.NewRecorder()

	handler.GetClaimInfo(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
