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
	handler := NewMeHandler(config, repo)

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
	handler := NewMeHandler(config, repo)

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
	handler := NewMeHandler(config, repo)

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
	handler := NewMeHandler(config, repo)

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
	handler := NewMeHandler(config, repo)

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
