package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockMeUserRepository implements MeUserRepositoryInterface for testing
type MockMeUserRepository struct {
	users      map[string]*models.User
	stats      map[string]*models.UserStats
	findError  error
	statsError error
}

func NewMockMeUserRepository() *MockMeUserRepository {
	return &MockMeUserRepository{
		users: make(map[string]*models.User),
		stats: make(map[string]*models.UserStats),
	}
}

func (m *MockMeUserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	user, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (m *MockMeUserRepository) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	if m.statsError != nil {
		return nil, m.statsError
	}
	stats, ok := m.stats[userID]
	if !ok {
		// Return default stats if not set
		return &models.UserStats{}, nil
	}
	return stats, nil
}

func TestMe_Success(t *testing.T) {
	// Setup: create user in mock repository
	repo := NewMockMeUserRepository()
	userID := "user-123"
	repo.users[userID] = &models.User{
		ID:          userID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		AvatarURL:   "https://example.com/avatar.png",
		Role:        models.UserRoleUser,
	}
	repo.stats[userID] = &models.UserStats{
		PostsCreated:    10,
		AnswersGiven:    25,
		AnswersAccepted: 5,
		UpvotesReceived: 100,
		Reputation:      500,
	}

	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)

	// Add claims to context (simulating JWT middleware)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Assert: response contains user data and stats
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	// Check user fields
	if data["id"] != userID {
		t.Errorf("expected id %q, got %q", userID, data["id"])
	}
	if data["username"] != "testuser" {
		t.Errorf("expected username 'testuser', got %q", data["username"])
	}
	if data["display_name"] != "Test User" {
		t.Errorf("expected display_name 'Test User', got %q", data["display_name"])
	}
	if data["email"] != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", data["email"])
	}

	// Check stats
	stats, ok := data["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'stats' field")
	}
	if int(stats["posts_created"].(float64)) != 10 {
		t.Errorf("expected posts_created 10, got %v", stats["posts_created"])
	}
	if int(stats["reputation"].(float64)) != 500 {
		t.Errorf("expected reputation 500, got %v", stats["reputation"])
	}
}

func TestMe_NoAuth(t *testing.T) {
	// Setup: create handler without setting claims in context
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request WITHOUT claims in context
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 401 Unauthorized
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
}

func TestMe_UserNotFound(t *testing.T) {
	// Setup: empty repository (user doesn't exist)
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request with claims for non-existent user
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	claims := &auth.Claims{
		UserID: "non-existent-user",
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 404 Not Found
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
}

func TestMe_IncludesAllStats(t *testing.T) {
	// Setup: create user with specific stats
	repo := NewMockMeUserRepository()
	userID := "user-with-stats"
	repo.users[userID] = &models.User{
		ID:       userID,
		Username: "statsuser",
		Email:    "stats@example.com",
		Role:     models.UserRoleUser,
	}
	repo.stats[userID] = &models.UserStats{
		PostsCreated:    15,
		AnswersGiven:    30,
		AnswersAccepted: 8,
		UpvotesReceived: 200,
		Reputation:      750,
	}

	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "stats@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	stats := data["stats"].(map[string]interface{})

	// Check all stats fields per SPEC.md Part 2.8
	expectedStats := map[string]float64{
		"posts_created":    15,
		"answers_given":    30,
		"answers_accepted": 8,
		"upvotes_received": 200,
		"reputation":       750,
	}

	for field, expected := range expectedStats {
		actual, ok := stats[field].(float64)
		if !ok {
			t.Errorf("stats missing field %q", field)
			continue
		}
		if actual != expected {
			t.Errorf("expected %s = %v, got %v", field, expected, actual)
		}
	}
}

func TestMe_AdminUser(t *testing.T) {
	// Setup: create admin user
	repo := NewMockMeUserRepository()
	userID := "admin-user"
	repo.users[userID] = &models.User{
		ID:          userID,
		Username:    "adminuser",
		DisplayName: "Admin User",
		Email:       "admin@example.com",
		Role:        models.UserRoleAdmin,
	}
	repo.stats[userID] = &models.UserStats{
		Reputation: 10000,
	}

	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request with admin claims
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "admin@example.com",
		Role:   models.UserRoleAdmin,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// Check admin role is returned
	if data["role"] != models.UserRoleAdmin {
		t.Errorf("expected role %q, got %q", models.UserRoleAdmin, data["role"])
	}
}

// MockMeAgentStats implements MeAgentStatsInterface for testing.
type MockMeAgentStats struct {
	stats map[string]*models.AgentStats
}

func NewMockMeAgentStats() *MockMeAgentStats {
	return &MockMeAgentStats{stats: make(map[string]*models.AgentStats)}
}

func (m *MockMeAgentStats) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	stats, ok := m.stats[agentID]
	if !ok {
		return &models.AgentStats{}, nil
	}
	return stats, nil
}

// TestMe_Agent_ReturnsComputedReputation verifies agent /me uses computed reputation, not raw bonus.
func TestMe_Agent_ReturnsComputedReputation(t *testing.T) {
	repo := NewMockMeUserRepository()
	agentStats := NewMockMeAgentStats()
	agentStats.stats["agent-with-activity"] = &models.AgentStats{
		ProblemsSolved: 5,
		Reputation:     750, // Computed: much higher than raw bonus of 50
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, repo, agentStats)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "agent-with-activity",
		DisplayName: "Active Agent",
		Status:      "active",
		Reputation:  50, // Raw bonus only
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})

	// Should be computed reputation (750), NOT raw bonus (50)
	if int(data["reputation"].(float64)) != 750 {
		t.Errorf("expected computed reputation 750, got %v", data["reputation"])
	}
}

// Tests for API key authentication (agents) - per FIX-005

func TestMe_AgentWithAPIKey(t *testing.T) {
	// Setup: create handler
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request with agent in context (simulating API key middleware)
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)

	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Bio:         "A test AI agent",
		Specialties: []string{"golang", "testing"},
		Status:      "active",
		Reputation:  100,
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Assert: response contains agent data
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	// Check agent fields
	if data["id"] != "test_agent" {
		t.Errorf("expected id 'test_agent', got %q", data["id"])
	}
	if data["display_name"] != "Test Agent" {
		t.Errorf("expected display_name 'Test Agent', got %q", data["display_name"])
	}
	if data["type"] != "agent" {
		t.Errorf("expected type 'agent', got %q", data["type"])
	}
	if data["bio"] != "A test AI agent" {
		t.Errorf("expected bio 'A test AI agent', got %q", data["bio"])
	}
	if int(data["reputation"].(float64)) != 100 {
		t.Errorf("expected reputation 100, got %v", data["reputation"])
	}

	// Check specialties
	specialties, ok := data["specialties"].([]interface{})
	if !ok {
		t.Fatal("response missing 'specialties' field")
	}
	if len(specialties) != 2 {
		t.Errorf("expected 2 specialties, got %d", len(specialties))
	}
}

func TestMe_AgentWithHumanBackedBadge(t *testing.T) {
	// Setup: create handler
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	// Create request with claimed agent
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)

	humanID := "human-123"
	agent := &models.Agent{
		ID:                  "claimed_agent",
		DisplayName:         "Claimed Agent",
		HumanID:             &humanID,
		Status:              "active",
		Reputation:          150,
		HasHumanBackedBadge: true,
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// Check human_backed_badge is true
	if data["has_human_backed_badge"] != true {
		t.Errorf("expected has_human_backed_badge true, got %v", data["has_human_backed_badge"])
	}

	// Check human_id is present
	if data["human_id"] != humanID {
		t.Errorf("expected human_id %q, got %v", humanID, data["human_id"])
	}
}

func TestMe_PrefersAgentOverClaims(t *testing.T) {
	// Setup: both agent and claims in context - agent should take precedence
	repo := NewMockMeUserRepository()
	userID := "user-123"
	repo.users[userID] = &models.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     models.UserRoleUser,
	}

	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	handler := NewMeHandler(config, repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)

	// Add both agent and claims to context
	agent := &models.Agent{
		ID:          "priority_agent",
		DisplayName: "Priority Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx = auth.ContextWithClaims(ctx, claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response - should return agent data, not user data
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// Should be agent, not user
	if data["id"] != "priority_agent" {
		t.Errorf("expected agent id 'priority_agent', got %q", data["id"])
	}
	if data["type"] != "agent" {
		t.Errorf("expected type 'agent', got %q", data["type"])
	}
}
