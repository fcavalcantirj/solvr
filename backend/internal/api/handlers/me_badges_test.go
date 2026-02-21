package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockBadgeRepo implements BadgeRepoInterface for testing.
type MockBadgeRepo struct {
	badges map[string][]models.Badge // key: "ownerType:ownerID"
	err    error
}

func NewMockBadgeRepo() *MockBadgeRepo {
	return &MockBadgeRepo{
		badges: make(map[string][]models.Badge),
	}
}

func (m *MockBadgeRepo) ListForOwner(ctx context.Context, ownerType, ownerID string) ([]models.Badge, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := ownerType + ":" + ownerID
	badges, ok := m.badges[key]
	if !ok {
		return []models.Badge{}, nil
	}
	return badges, nil
}

func (m *MockBadgeRepo) addBadge(ownerType, ownerID string, badge models.Badge) {
	key := ownerType + ":" + ownerID
	m.badges[key] = append(m.badges[key], badge)
}

// TestAgentMe_IncludesBadges verifies that GET /me with agent API key includes badges array.
func TestAgentMe_IncludesBadges(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	badgeRepo := NewMockBadgeRepo()
	badgeRepo.addBadge("agent", "test_agent", models.Badge{
		ID:        "badge-1",
		OwnerType: "agent",
		OwnerID:   "test_agent",
		BadgeType: models.BadgeFirstSolve,
		BadgeName: "First Solve",
		AwardedAt: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
	})
	badgeRepo.addBadge("agent", "test_agent", models.Badge{
		ID:        "badge-2",
		OwnerType: "agent",
		OwnerID:   "test_agent",
		BadgeType: models.BadgeHumanBacked,
		BadgeName: "Human-Backed",
		AwardedAt: time.Date(2026, 2, 20, 13, 0, 0, 0, time.UTC),
	})

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetBadgeRepo(badgeRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		Reputation:  100,
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	badges, ok := data["badges"].([]interface{})
	if !ok {
		t.Fatal("response missing 'badges' field or not an array")
	}

	if len(badges) != 2 {
		t.Errorf("expected 2 badges, got %d", len(badges))
	}

	// Verify badge fields
	b0 := badges[0].(map[string]interface{})
	if b0["badge_type"] != models.BadgeFirstSolve {
		t.Errorf("expected badge_type %q, got %q", models.BadgeFirstSolve, b0["badge_type"])
	}
}

// TestUserMe_IncludesBadges verifies that GET /me with JWT includes badges array for humans.
func TestUserMe_IncludesBadges(t *testing.T) {
	repo := NewMockMeUserRepository()
	userID := "user-badges-123"
	repo.users[userID] = &models.User{
		ID:          userID,
		Username:    "badgeuser",
		DisplayName: "Badge User",
		Email:       "badges@example.com",
		Role:        models.UserRoleUser,
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	badgeRepo := NewMockBadgeRepo()
	badgeRepo.addBadge("human", userID, models.Badge{
		ID:        "badge-u1",
		OwnerType: "human",
		OwnerID:   userID,
		BadgeType: models.BadgeFirstSolve,
		BadgeName: "First Solve",
		AwardedAt: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
	})

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetBadgeRepo(badgeRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	claims := &auth.Claims{UserID: userID, Email: "badges@example.com", Role: models.UserRoleUser}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	badges, ok := data["badges"].([]interface{})
	if !ok {
		t.Fatal("response missing 'badges' field or not an array")
	}

	if len(badges) != 1 {
		t.Errorf("expected 1 badge, got %d", len(badges))
	}

	b0 := badges[0].(map[string]interface{})
	if b0["badge_type"] != models.BadgeFirstSolve {
		t.Errorf("expected badge_type %q, got %q", models.BadgeFirstSolve, b0["badge_type"])
	}
}

// TestGetAgentBadges verifies GET /v1/agents/{id}/badges returns correct badges.
func TestGetAgentBadges(t *testing.T) {
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	badgeRepo := NewMockBadgeRepo()
	badgeRepo.addBadge("agent", "agent-with-badges", models.Badge{
		ID:        "badge-a1",
		OwnerType: "agent",
		OwnerID:   "agent-with-badges",
		BadgeType: models.BadgeTenSolves,
		BadgeName: "Ten Solves",
		AwardedAt: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
	})
	badgeRepo.addBadge("agent", "agent-with-badges", models.Badge{
		ID:        "badge-a2",
		OwnerType: "agent",
		OwnerID:   "agent-with-badges",
		BadgeType: models.BadgeHundredUpvotes,
		BadgeName: "Hundred Upvotes",
		AwardedAt: time.Date(2026, 2, 20, 13, 0, 0, 0, time.UTC),
	})

	handler := NewMeHandler(config, nil, nil, nil, nil)
	handler.SetBadgeRepo(badgeRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-with-badges/badges", nil)

	rr := httptest.NewRecorder()
	handler.GetAgentBadges(rr, req, "agent-with-badges")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	badges, ok := data["badges"].([]interface{})
	if !ok {
		t.Fatal("response missing 'badges' field or not an array")
	}

	if len(badges) != 2 {
		t.Errorf("expected 2 badges, got %d", len(badges))
	}
}

// TestGetUserBadges verifies GET /v1/users/{id}/badges returns correct badges.
func TestGetUserBadges(t *testing.T) {
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	badgeRepo := NewMockBadgeRepo()
	badgeRepo.addBadge("human", "user-with-badges", models.Badge{
		ID:        "badge-u1",
		OwnerType: "human",
		OwnerID:   "user-with-badges",
		BadgeType: models.BadgeFirstAnswerAccepted,
		BadgeName: "First Answer Accepted",
		AwardedAt: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
	})

	handler := NewMeHandler(config, nil, nil, nil, nil)
	handler.SetBadgeRepo(badgeRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/user-with-badges/badges", nil)

	rr := httptest.NewRecorder()
	handler.GetUserBadges(rr, req, "user-with-badges")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	badges, ok := data["badges"].([]interface{})
	if !ok {
		t.Fatal("response missing 'badges' field or not an array")
	}

	if len(badges) != 1 {
		t.Errorf("expected 1 badge, got %d", len(badges))
	}

	b0 := badges[0].(map[string]interface{})
	if b0["badge_type"] != models.BadgeFirstAnswerAccepted {
		t.Errorf("expected badge_type %q, got %q", models.BadgeFirstAnswerAccepted, b0["badge_type"])
	}
}

// TestGetBadges_NoBadges verifies empty array (not null) when no badges exist.
func TestGetBadges_NoBadges(t *testing.T) {
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	badgeRepo := NewMockBadgeRepo() // no badges added

	handler := NewMeHandler(config, nil, nil, nil, nil)
	handler.SetBadgeRepo(badgeRepo)

	// Test agent with no badges
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/no-badges-agent/badges", nil)
	rr := httptest.NewRecorder()
	handler.GetAgentBadges(rr, req, "no-badges-agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	badges, ok := data["badges"].([]interface{})
	if !ok {
		t.Fatal("response missing 'badges' field or not an array — should be empty array, not null")
	}

	if len(badges) != 0 {
		t.Errorf("expected 0 badges, got %d", len(badges))
	}

	// Test user with no badges
	req2 := httptest.NewRequest(http.MethodGet, "/v1/users/no-badges-user/badges", nil)
	rr2 := httptest.NewRecorder()
	handler.GetUserBadges(rr2, req2, "no-badges-user")

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var response2 map[string]interface{}
	if err := json.NewDecoder(rr2.Body).Decode(&response2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data2 := response2["data"].(map[string]interface{})
	badges2, ok := data2["badges"].([]interface{})
	if !ok {
		t.Fatal("user response missing 'badges' field or not an array — should be empty array, not null")
	}

	if len(badges2) != 0 {
		t.Errorf("expected 0 badges for user, got %d", len(badges2))
	}
}
