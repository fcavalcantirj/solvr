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
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockClaimTokenRepository implements ClaimTokenRepositoryInterface for testing.
type MockClaimTokenRepository struct {
	tokens              map[string]*models.ClaimToken
	createCalled        bool
	createErr           error
	findByTokenErr      error
	findActiveByAgentErr error
	markUsedErr         error
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
		return nil, errors.New("token not found")
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
	return nil, errors.New("no active token found")
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
	// Verify 24 hour expiry (with some tolerance)
	expectedExpiry := time.Now().Add(24 * time.Hour)
	if createdToken.ExpiresAt.Before(expectedExpiry.Add(-1*time.Minute)) ||
		createdToken.ExpiresAt.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("expected expires_at around 24 hours from now, got %v", createdToken.ExpiresAt)
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
