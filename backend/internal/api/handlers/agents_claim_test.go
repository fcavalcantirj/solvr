package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockClaimTokenRepository implements ClaimTokenRepositoryInterface for testing.
type MockClaimTokenRepository struct {
	tokens                    map[string]*models.ClaimToken
	createCalled              bool
	createErr                 error
	findByTokenErr            error
	findActiveByAgentErr      error
	markUsedErr               error
	deleteExpiredByAgentCalled bool
	deleteExpiredByAgentErr   error
}

func NewMockClaimTokenRepository() *MockClaimTokenRepository {
	return &MockClaimTokenRepository{
		tokens: make(map[string]*models.ClaimToken),
	}
}

func (m *MockClaimTokenRepository) Create(ctx context.Context, token *models.ClaimToken) error {
	m.createCalled = true
	if m.createErr != nil {
		return m.createErr
	}
	m.tokens[token.Token] = token
	return nil
}

func (m *MockClaimTokenRepository) FindByToken(ctx context.Context, token string) (*models.ClaimToken, error) {
	if m.findByTokenErr != nil {
		return nil, m.findByTokenErr
	}
	t, exists := m.tokens[token]
	if !exists {
		return nil, nil // Match real DB: returns nil, nil when not found
	}
	return t, nil
}

func (m *MockClaimTokenRepository) FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error) {
	if m.findActiveByAgentErr != nil {
		return nil, m.findActiveByAgentErr
	}
	for _, token := range m.tokens {
		if token.AgentID == agentID && token.IsActive() {
			return token, nil
		}
	}
	return nil, nil // Match real DB: returns nil, nil when not found
}

func (m *MockClaimTokenRepository) MarkUsed(ctx context.Context, tokenID, humanID string) error {
	if m.markUsedErr != nil {
		return m.markUsedErr
	}
	for _, token := range m.tokens {
		if token.ID == tokenID {
			now := time.Now()
			token.UsedAt = &now
			token.UsedByHumanID = &humanID
			return nil
		}
	}
	return errors.New("token not found")
}

func (m *MockClaimTokenRepository) DeleteExpiredByAgentID(ctx context.Context, agentID string) (int64, error) {
	m.deleteExpiredByAgentCalled = true
	if m.deleteExpiredByAgentErr != nil {
		return 0, m.deleteExpiredByAgentErr
	}
	var deleted int64
	for key, token := range m.tokens {
		if token.AgentID == agentID && token.IsExpired() && !token.IsUsed() {
			delete(m.tokens, key)
			deleted++
		}
	}
	return deleted, nil
}

// TestGenerateClaim_RequiresAgentAuth tests that API key auth is required.
func TestGenerateClaim_RequiresAgentAuth(t *testing.T) {
	agentRepo := NewMockAgentRepository()
	claimRepo := NewMockClaimTokenRepository()
	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	w := httptest.NewRecorder()

	handler.GenerateClaim(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %v", errObj["code"])
	}
}

// TestGenerateClaim_Success tests successful claim token generation.
func TestGenerateClaim_Success(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.findActiveByAgentErr = errors.New("not found") // No existing active token

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	// Add agent to context (simulating API key auth)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp GenerateClaimResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if resp.ExpiresAt.IsZero() {
		t.Error("expected expires_at to be set")
	}
	if resp.Token == "" {
		t.Error("expected token to be set")
	}
	if resp.Instructions == "" {
		t.Error("expected instructions to be set")
	}

	// Verify token was created in repository
	if !claimRepo.createCalled {
		t.Error("expected Create to be called on claim token repository")
	}

	// Find the created token
	var createdToken *models.ClaimToken
	for _, token := range claimRepo.tokens {
		if token.AgentID == testAgent.ID {
			createdToken = token
			break
		}
	}

	if createdToken == nil {
		t.Fatal("expected token to be created in repository")
	}
	if createdToken.AgentID != testAgent.ID {
		t.Errorf("expected agent_id %s, got %s", testAgent.ID, createdToken.AgentID)
	}
	if createdToken.Token == "" {
		t.Error("expected token to be generated")
	}
	if createdToken.ExpiresAt.Before(time.Now()) {
		t.Error("expected expires_at to be in the future")
	}
	// Verify 1 hour expiry (with some tolerance)
	expectedExpiry := time.Now().Add(1 * time.Hour)
	if createdToken.ExpiresAt.Before(expectedExpiry.Add(-1*time.Minute)) ||
		createdToken.ExpiresAt.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("expected expires_at around 1 hour from now, got %v", createdToken.ExpiresAt)
	}
}

// TestGenerateClaim_ReturnsExistingToken tests that existing active token is returned.
func TestGenerateClaim_ReturnsExistingToken(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	existingToken := &models.ClaimToken{
		ID:        "existing-token-id",
		Token:     "existing_token_value",
		AgentID:   testAgent.ID,
		ExpiresAt: time.Now().Add(12 * time.Hour), // Still valid
		CreatedAt: time.Now().Add(-12 * time.Hour),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens[existingToken.Token] = existingToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp GenerateClaimResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return the existing token
	if resp.Token != existingToken.Token {
		t.Errorf("expected existing token %s, got %s", existingToken.Token, resp.Token)
	}
}

// TestGenerateClaim_TokenFormat tests the generated token format.
func TestGenerateClaim_TokenFormat(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.findActiveByAgentErr = errors.New("not found")

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	// Find the created token
	var createdToken *models.ClaimToken
	for _, token := range claimRepo.tokens {
		createdToken = token
		break
	}

	// Token should be 64 characters (hex-encoded 32 bytes)
	if len(createdToken.Token) != 64 {
		t.Errorf("expected token length 64, got %d", len(createdToken.Token))
	}

	// Token should be URL-safe (alphanumeric only for hex)
	for _, c := range createdToken.Token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("token contains non-hex character: %c", c)
			break
		}
	}
}

// TestGenerateClaim_Instructions tests that response includes instructions.
func TestGenerateClaim_Instructions(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.findActiveByAgentErr = errors.New("not found")

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	var resp GenerateClaimResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Should have instructions for the agent
	if resp.Instructions == "" {
		t.Error("expected instructions to be set")
	}
}

// TestGenerateClaim_CreateError tests handling of repository create error.
func TestGenerateClaim_CreateError(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.findActiveByAgentErr = errors.New("not found")
	claimRepo.createErr = errors.New("database error")

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestGenerateClaim_NoClaimRepo tests error when claim repo is not configured.
func TestGenerateClaim_NoClaimRepo(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	handler := NewAgentsHandler(agentRepo, "test-secret")
	// NOT setting claim token repository
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// ============================================================
// ClaimAgentWithToken handler tests
// ============================================================

// TestClaimAgent_RequiresJWTAuth tests that JWT auth is required to claim.
func TestClaimAgent_RequiresJWTAuth(t *testing.T) {
	agentRepo := NewMockAgentRepository()
	claimRepo := NewMockClaimTokenRepository()
	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	body := bytes.NewBufferString(`{"token":"some_token"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	// No JWT context added
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d: %s", http.StatusUnauthorized, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %v", errObj["code"])
	}
}

// TestClaimAgent_MissingToken tests that token is required in request body.
func TestClaimAgent_MissingToken(t *testing.T) {
	agentRepo := NewMockAgentRepository()
	claimRepo := NewMockClaimTokenRepository()
	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	body := bytes.NewBufferString(`{"token":""}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	req = addJWTClaimsToContext(req, "human-123", "human@test.com", "user")
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "MISSING_TOKEN" {
		t.Errorf("expected error code MISSING_TOKEN, got %v", errObj["code"])
	}
}

// TestClaimAgent_TokenNotFound tests that invalid token returns 404.
// This test exposes Bug 1: nil pointer panic when FindByToken returns (nil, nil).
func TestClaimAgent_TokenNotFound(t *testing.T) {
	agentRepo := NewMockAgentRepository()
	claimRepo := NewMockClaimTokenRepository()
	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	body := bytes.NewBufferString(`{"token":"nonexistent_token_value"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	req = addJWTClaimsToContext(req, "human-123", "human@test.com", "user")
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "TOKEN_NOT_FOUND" {
		t.Errorf("expected error code TOKEN_NOT_FOUND, got %v", errObj["code"])
	}
	// Error message should be clear, not misleading
	if errObj["message"] != "token not found" {
		t.Errorf("expected message 'token not found', got %v", errObj["message"])
	}
}

// TestClaimAgent_ExpiredToken tests that expired token returns 410 Gone.
func TestClaimAgent_ExpiredToken(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	expiredToken := &models.ClaimToken{
		ID:        "expired-token-id",
		Token:     "expired_token_value",
		AgentID:   testAgent.ID,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens[expiredToken.Token] = expiredToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)

	body := bytes.NewBufferString(`{"token":"expired_token_value"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	req = addJWTClaimsToContext(req, "human-123", "human@test.com", "user")
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d: %s", http.StatusGone, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "TOKEN_EXPIRED" {
		t.Errorf("expected error code TOKEN_EXPIRED, got %v", errObj["code"])
	}
}

// TestClaimAgent_UsedToken tests that already-used token returns 409 Conflict.
func TestClaimAgent_UsedToken(t *testing.T) {
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

	body := bytes.NewBufferString(`{"token":"used_token_value"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	req = addJWTClaimsToContext(req, "human-123", "human@test.com", "user")
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "TOKEN_USED" {
		t.Errorf("expected error code TOKEN_USED, got %v", errObj["code"])
	}
}

// TestClaimAgent_AgentAlreadyClaimed tests that claiming an already-claimed agent returns 409.
func TestClaimAgent_AgentAlreadyClaimed(t *testing.T) {
	existingHumanID := "other-human-789"
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		HumanID:     &existingHumanID, // Already claimed
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

	body := bytes.NewBufferString(`{"token":"valid_token_value"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	req = addJWTClaimsToContext(req, "human-123", "human@test.com", "user")
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "ALREADY_CLAIMED" {
		t.Errorf("expected error code ALREADY_CLAIMED, got %v", errObj["code"])
	}
}

// TestClaimAgent_Success tests the full happy path of claiming an agent.
func TestClaimAgent_Success(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		CreatedAt:   time.Now(),
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

	humanID := "human-123"
	body := bytes.NewBufferString(`{"token":"valid_token_value"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim", body)
	req = addJWTClaimsToContext(req, humanID, "human@test.com", "user")
	w := httptest.NewRecorder()

	handler.ClaimAgentWithToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp ClaimAgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Agent.ID != testAgent.ID {
		t.Errorf("expected agent ID %s, got %s", testAgent.ID, resp.Agent.ID)
	}

	// Verify agent was linked to human
	updatedAgent := agentRepo.agents[testAgent.ID]
	if updatedAgent.HumanID == nil || *updatedAgent.HumanID != humanID {
		t.Errorf("expected agent human_id to be %s, got %v", humanID, updatedAgent.HumanID)
	}

	// Verify reputation bonus was granted (+50)
	if updatedAgent.Reputation != ReputationBonusOnClaim {
		t.Errorf("expected reputation %d, got %d", ReputationBonusOnClaim, updatedAgent.Reputation)
	}

	// Verify Human-Backed badge was granted
	if !updatedAgent.HasHumanBackedBadge {
		t.Error("expected HasHumanBackedBadge to be true")
	}

	// Verify token was marked as used
	claimedToken := claimRepo.tokens[validToken.Token]
	if claimedToken.UsedAt == nil {
		t.Error("expected token UsedAt to be set")
	}
	if claimedToken.UsedByHumanID == nil || *claimedToken.UsedByHumanID != humanID {
		t.Errorf("expected token UsedByHumanID to be %s, got %v", humanID, claimedToken.UsedByHumanID)
	}
}

// TestGenerateClaim_RegenerateAfterExpiry tests that an agent can generate a new token
// after the previous one expires (verifies deadlock fix).
func TestGenerateClaim_RegenerateAfterExpiry(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	// Create an expired token in the mock
	expiredToken := &models.ClaimToken{
		ID:        "expired-token-id",
		Token:     "expired_token_value",
		AgentID:   testAgent.ID,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens[expiredToken.Token] = expiredToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	// Verify cleanup was called
	if !claimRepo.deleteExpiredByAgentCalled {
		t.Error("expected DeleteExpiredByAgentID to be called")
	}

	// Verify new token was created (different from expired one)
	var resp GenerateClaimResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Token == expiredToken.Token {
		t.Error("expected a new token, got the expired one")
	}
	if resp.Token == "" {
		t.Error("expected new token to be generated")
	}
}

// TestGenerateClaim_1HourExpiry tests that tokens expire in 1 hour, not 24 hours.
func TestGenerateClaim_1HourExpiry(t *testing.T) {
	testAgent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}

	agentRepo := NewMockAgentRepository()
	agentRepo.agents[testAgent.ID] = testAgent

	claimRepo := NewMockClaimTokenRepository()
	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	ctx := auth.ContextWithAgent(req.Context(), testAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	// Find the created token
	var createdToken *models.ClaimToken
	for _, token := range claimRepo.tokens {
		if token.AgentID == testAgent.ID {
			createdToken = token
			break
		}
	}

	if createdToken == nil {
		t.Fatal("expected token to be created")
	}

	// Verify 1 hour expiry (with 1 minute tolerance)
	expectedExpiry := time.Now().Add(1 * time.Hour)
	if createdToken.ExpiresAt.Before(expectedExpiry.Add(-1*time.Minute)) ||
		createdToken.ExpiresAt.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("expected expires_at around 1 hour from now, got %v (diff: %v)",
			createdToken.ExpiresAt, createdToken.ExpiresAt.Sub(time.Now()))
	}

	// Must NOT be 24 hours
	if createdToken.ExpiresAt.After(time.Now().Add(2 * time.Hour)) {
		t.Errorf("token expiry is too far in the future: %v", createdToken.ExpiresAt)
	}
}
