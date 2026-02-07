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

// MockClaimTokenRepoForConfirm provides mocks for claim confirmation tests.
type MockClaimTokenRepoForConfirm struct {
	tokens      map[string]*models.ClaimToken
	markUsedErr error
}

func NewMockClaimTokenRepoForConfirm() *MockClaimTokenRepoForConfirm {
	return &MockClaimTokenRepoForConfirm{
		tokens: make(map[string]*models.ClaimToken),
	}
}

func (m *MockClaimTokenRepoForConfirm) Create(ctx context.Context, token *models.ClaimToken) error {
	m.tokens[token.Token] = token
	return nil
}

func (m *MockClaimTokenRepoForConfirm) FindByToken(ctx context.Context, token string) (*models.ClaimToken, error) {
	if t, ok := m.tokens[token]; ok {
		return t, nil
	}
	return nil, errors.New("token not found")
}

func (m *MockClaimTokenRepoForConfirm) FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error) {
	for _, t := range m.tokens {
		if t.AgentID == agentID && t.IsActive() {
			return t, nil
		}
	}
	return nil, errors.New("no active token")
}

func (m *MockClaimTokenRepoForConfirm) MarkUsed(ctx context.Context, tokenID, humanID string) error {
	if m.markUsedErr != nil {
		return m.markUsedErr
	}
	for _, t := range m.tokens {
		if t.ID == tokenID {
			now := time.Now()
			t.UsedAt = &now
			t.UsedByHumanID = &humanID
			return nil
		}
	}
	return errors.New("token not found")
}

// MockAgentRepoForClaim provides mock for agent operations during claim.
type MockAgentRepoForClaim struct {
	agents          map[string]*models.Agent
	createErr       error
	updateErr       error
	linkHumanErr    error
	addKarmaErr     error
	linkedHumanID   *string
	karmaAdded      int
	badgeGranted    bool
}

func NewMockAgentRepoForClaim() *MockAgentRepoForClaim {
	return &MockAgentRepoForClaim{
		agents: make(map[string]*models.Agent),
	}
}

func (m *MockAgentRepoForClaim) Create(ctx context.Context, agent *models.Agent) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepoForClaim) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	if a, ok := m.agents[id]; ok {
		return a, nil
	}
	return nil, ErrAgentNotFound
}

func (m *MockAgentRepoForClaim) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	for _, a := range m.agents {
		if a.DisplayName == name {
			return a, nil
		}
	}
	return nil, ErrAgentNotFound
}

func (m *MockAgentRepoForClaim) Update(ctx context.Context, agent *models.Agent) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepoForClaim) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	return &models.AgentStats{}, nil
}

func (m *MockAgentRepoForClaim) UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error {
	return nil
}

func (m *MockAgentRepoForClaim) RevokeAPIKey(ctx context.Context, agentID string) error {
	return nil
}

func (m *MockAgentRepoForClaim) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	return []models.ActivityItem{}, 0, nil
}

func (m *MockAgentRepoForClaim) LinkHuman(ctx context.Context, agentID, humanID string) error {
	if m.linkHumanErr != nil {
		return m.linkHumanErr
	}
	if a, ok := m.agents[agentID]; ok {
		m.linkedHumanID = &humanID
		a.HumanID = &humanID
		now := time.Now()
		a.HumanClaimedAt = &now
	}
	return nil
}

func (m *MockAgentRepoForClaim) AddKarma(ctx context.Context, agentID string, amount int) error {
	if m.addKarmaErr != nil {
		return m.addKarmaErr
	}
	m.karmaAdded = amount
	if a, ok := m.agents[agentID]; ok {
		a.Karma += amount
	}
	return nil
}

func (m *MockAgentRepoForClaim) GrantHumanBackedBadge(ctx context.Context, agentID string) error {
	m.badgeGranted = true
	if a, ok := m.agents[agentID]; ok {
		a.HasHumanBackedBadge = true
	}
	return nil
}

// GetAgentByAPIKeyHash finds an agent by comparing the API key against stored hashes.
// FIX-002: Required for API key authentication middleware.
func (m *MockAgentRepoForClaim) GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error) {
	// No API key lookup needed for claim confirm tests - return nil, nil
	return nil, nil
}

// FindByHumanID finds all agents owned by a human user.
// Per prd-v4: GET /v1/users/{id}/agents endpoint.
func (m *MockAgentRepoForClaim) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	var agents []*models.Agent
	for _, agent := range m.agents {
		if agent.HumanID != nil && *agent.HumanID == humanID {
			agents = append(agents, agent)
		}
	}
	if agents == nil {
		agents = []*models.Agent{}
	}
	return agents, nil
}

// List returns a paginated list of agents (API-001 requirement).
func (m *MockAgentRepoForClaim) List(ctx context.Context, opts models.AgentListOptions) ([]models.AgentWithPostCount, int, error) {
	return []models.AgentWithPostCount{}, 0, nil
}

// Test: ConfirmClaim requires human authentication
func TestConfirmClaim_RequiresHumanAuth(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	req := httptest.NewRequest(http.MethodPost, "/v1/claim/sometoken", nil)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "sometoken")

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		if errObj["code"] != "UNAUTHORIZED" {
			t.Errorf("expected UNAUTHORIZED error code, got %v", errObj["code"])
		}
	}
}

// Test: ConfirmClaim returns 404 for invalid token
func TestConfirmClaim_InvalidToken_Returns404(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	// Add JWT claims to context (human authenticated)
	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/invalid-token", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "invalid-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		if errObj["code"] != "TOKEN_NOT_FOUND" {
			t.Errorf("expected TOKEN_NOT_FOUND error code, got %v", errObj["code"])
		}
	}
}

// Test: ConfirmClaim returns 410 for expired token
func TestConfirmClaim_ExpiredToken_Returns410(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create expired token
	expiredToken := &models.ClaimToken{
		ID:        "token-id-1",
		Token:     "expired-token-abc",
		AgentID:   "agent-1",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		CreatedAt: time.Now().Add(-25 * time.Hour),
	}
	tokenRepo.tokens[expiredToken.Token] = expiredToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/expired-token-abc", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "expired-token-abc")

	resp := w.Result()
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 Gone, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		if errObj["code"] != "TOKEN_EXPIRED" {
			t.Errorf("expected TOKEN_EXPIRED error code, got %v", errObj["code"])
		}
	}
}

// Test: ConfirmClaim returns 409 for already used token
func TestConfirmClaim_AlreadyUsedToken_Returns409(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create already used token
	usedAt := time.Now().Add(-1 * time.Hour)
	usedByHumanID := "other-human-456"
	usedToken := &models.ClaimToken{
		ID:            "token-id-2",
		Token:         "used-token-xyz",
		AgentID:       "agent-2",
		ExpiresAt:     time.Now().Add(23 * time.Hour),
		UsedAt:        &usedAt,
		UsedByHumanID: &usedByHumanID,
		CreatedAt:     time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[usedToken.Token] = usedToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/used-token-xyz", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "used-token-xyz")

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		if errObj["code"] != "TOKEN_ALREADY_USED" {
			t.Errorf("expected TOKEN_ALREADY_USED error code, got %v", errObj["code"])
		}
	}
}

// Test: ConfirmClaim returns 409 if agent already claimed by another human
func TestConfirmClaim_AgentAlreadyClaimed_Returns409(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create agent that's already claimed
	otherHumanID := "other-human-999"
	claimedAt := time.Now().Add(-7 * 24 * time.Hour)
	agent := &models.Agent{
		ID:                  "agent-claimed",
		DisplayName:         "Already Claimed Bot",
		HumanID:             &otherHumanID,
		HumanClaimedAt:      &claimedAt,
		HasHumanBackedBadge: true,
		Status:              "active",
		CreatedAt:           time.Now().Add(-30 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token for the already-claimed agent
	validToken := &models.ClaimToken{
		ID:        "token-id-3",
		Token:     "valid-but-agent-claimed",
		AgentID:   "agent-claimed",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/valid-but-agent-claimed", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "valid-but-agent-claimed")

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		if errObj["code"] != "AGENT_ALREADY_CLAIMED" {
			t.Errorf("expected AGENT_ALREADY_CLAIMED error code, got %v", errObj["code"])
		}
	}
}

// Test: ConfirmClaim success - links agent to human
func TestConfirmClaim_Success_LinksAgentToHuman(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent
	agent := &models.Agent{
		ID:          "agent-unclaimed",
		DisplayName: "Unclaimed Bot",
		Bio:         "A helpful AI assistant",
		Status:      "active",
		Karma:       0,
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-4",
		Token:     "valid-claim-token",
		AgentID:   "agent-unclaimed",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/valid-claim-token", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "valid-claim-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	// Verify response structure
	if body["success"] != true {
		t.Errorf("expected success: true, got %v", body["success"])
	}

	// Check agent is linked
	if agentRepo.linkedHumanID == nil || *agentRepo.linkedHumanID != "user-123" {
		t.Errorf("expected agent to be linked to user-123, got %v", agentRepo.linkedHumanID)
	}
}

// Test: ConfirmClaim success - grants +50 karma bonus
func TestConfirmClaim_Success_GrantsKarmaBonus(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent
	agent := &models.Agent{
		ID:          "agent-for-karma",
		DisplayName: "Karma Test Bot",
		Status:      "active",
		Karma:       100, // Starting karma
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-5",
		Token:     "karma-token",
		AgentID:   "agent-for-karma",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-456",
		Email:  "karma@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/karma-token", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "karma-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify +50 karma was added
	if agentRepo.karmaAdded != 50 {
		t.Errorf("expected 50 karma added, got %d", agentRepo.karmaAdded)
	}

	// Verify final karma
	updatedAgent := agentRepo.agents["agent-for-karma"]
	if updatedAgent.Karma != 150 {
		t.Errorf("expected final karma 150, got %d", updatedAgent.Karma)
	}
}

// Test: ConfirmClaim success - grants Human-Backed badge
func TestConfirmClaim_Success_GrantsBadge(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent without badge
	agent := &models.Agent{
		ID:                  "agent-for-badge",
		DisplayName:         "Badge Test Bot",
		Status:              "active",
		HasHumanBackedBadge: false,
		CreatedAt:           time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-6",
		Token:     "badge-token",
		AgentID:   "agent-for-badge",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-789",
		Email:  "badge@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/badge-token", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "badge-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify badge was granted
	if !agentRepo.badgeGranted {
		t.Error("expected Human-Backed badge to be granted")
	}

	// Verify agent has badge
	updatedAgent := agentRepo.agents["agent-for-badge"]
	if !updatedAgent.HasHumanBackedBadge {
		t.Error("expected agent to have Human-Backed badge")
	}
}

// Test: ConfirmClaim success - marks token as used
func TestConfirmClaim_Success_MarksTokenUsed(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent
	agent := &models.Agent{
		ID:          "agent-token-mark",
		DisplayName: "Token Mark Bot",
		Status:      "active",
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-7",
		Token:     "mark-used-token",
		AgentID:   "agent-token-mark",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-mark",
		Email:  "mark@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/mark-used-token", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "mark-used-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify token is marked as used
	updatedToken := tokenRepo.tokens["mark-used-token"]
	if !updatedToken.IsUsed() {
		t.Error("expected token to be marked as used")
	}
	if updatedToken.UsedByHumanID == nil || *updatedToken.UsedByHumanID != "user-mark" {
		t.Errorf("expected token used_by_human_id to be user-mark, got %v", updatedToken.UsedByHumanID)
	}
}

// Test: ConfirmClaim success - returns agent profile in response
func TestConfirmClaim_Success_ReturnsAgentProfile(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent
	agent := &models.Agent{
		ID:          "agent-profile-test",
		DisplayName: "Profile Test Bot",
		Bio:         "An agent for testing profile response",
		Status:      "active",
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-8",
		Token:     "profile-token",
		AgentID:   "agent-profile-test",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-profile",
		Email:  "profile@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/profile-token", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "profile-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	// Verify agent is in response
	agentData, ok := body["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agent in response")
	}

	if agentData["id"] != "agent-profile-test" {
		t.Errorf("expected agent id agent-profile-test, got %v", agentData["id"])
	}
	if agentData["display_name"] != "Profile Test Bot" {
		t.Errorf("expected display_name Profile Test Bot, got %v", agentData["display_name"])
	}

	// Verify redirect_url is provided
	if body["redirect_url"] == nil {
		t.Error("expected redirect_url in response")
	}
}

// Test: ConfirmClaim handles claim token repo not configured
func TestConfirmClaim_NoClaimRepo_Returns500(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()

	handler := NewAgentsHandler(agentRepo, "test-secret")
	// Don't set claim token repo

	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "human@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/sometoken", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "sometoken")

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// Test: ConfirmClaim returns agent info for confirmation (GET method)
func TestGetClaimInfo_ReturnsAgentInfo(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent
	agent := &models.Agent{
		ID:          "agent-info-test",
		DisplayName: "Info Test Bot",
		Bio:         "An agent for testing claim info",
		Status:      "active",
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-9",
		Token:     "info-token",
		AgentID:   "agent-info-test",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	// GET request - no auth needed to view claim info
	req := httptest.NewRequest(http.MethodGet, "/v1/claim/info-token", nil)
	w := httptest.NewRecorder()

	handler.GetClaimInfo(w, req, "info-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	// Verify agent info is returned
	agentData, ok := body["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agent in response")
	}

	if agentData["id"] != "agent-info-test" {
		t.Errorf("expected agent id agent-info-test, got %v", agentData["id"])
	}
	if agentData["display_name"] != "Info Test Bot" {
		t.Errorf("expected display_name Info Test Bot, got %v", agentData["display_name"])
	}

	// Verify token validity info
	if body["token_valid"] != true {
		t.Errorf("expected token_valid: true, got %v", body["token_valid"])
	}
}

// Verify empty body doesn't break ConfirmClaim
func TestConfirmClaim_EmptyBody_StillWorks(t *testing.T) {
	agentRepo := NewMockAgentRepoForClaim()
	tokenRepo := NewMockClaimTokenRepoForConfirm()

	// Create unclaimed agent
	agent := &models.Agent{
		ID:          "agent-empty-body",
		DisplayName: "Empty Body Bot",
		Status:      "active",
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
	}
	agentRepo.agents[agent.ID] = agent

	// Create valid token
	validToken := &models.ClaimToken{
		ID:        "token-id-10",
		Token:     "empty-body-token",
		AgentID:   "agent-empty-body",
		ExpiresAt: time.Now().Add(23 * time.Hour),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	tokenRepo.tokens[validToken.Token] = validToken

	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(tokenRepo)

	claims := &auth.Claims{
		UserID: "user-empty",
		Email:  "empty@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	// Empty body
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/empty-body-token", bytes.NewReader([]byte{}))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ConfirmClaim(w, req, "empty-body-token")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
