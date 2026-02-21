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

// mockDiffNotificationsRepo counts new notifications since a given time.
type mockDiffNotificationsRepo struct {
	countByAgent map[string]int
	err          error
}

func newMockDiffNotificationsRepo() *mockDiffNotificationsRepo {
	return &mockDiffNotificationsRepo{countByAgent: make(map[string]int)}
}

func (m *mockDiffNotificationsRepo) CountNewSince(ctx context.Context, agentID string, since time.Time) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.countByAgent[agentID], nil
}

// mockDiffReputationRepo returns reputation changes since a given time.
type mockDiffReputationRepo struct {
	result *models.ReputationChangesResult
	err    error
}

func (m *mockDiffReputationRepo) GetReputationChangesSince(ctx context.Context, agentID string, since time.Time) (*models.ReputationChangesResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockDiffOpportunitiesRepo counts new opportunities since a given time.
type mockDiffOpportunitiesRepo struct {
	count int
	err   error
}

func (m *mockDiffOpportunitiesRepo) CountNewOpportunitiesSince(ctx context.Context, agentID string, specialties []string, since time.Time) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

// mockDiffBadgesRepo returns badges earned since a given time.
type mockDiffBadgesRepo struct {
	badges []models.Badge
	err    error
}

func (m *mockDiffBadgesRepo) ListAwardedSince(ctx context.Context, ownerType, ownerID string, since time.Time) ([]models.Badge, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.badges == nil {
		return []models.Badge{}, nil
	}
	return m.badges, nil
}

// mockDiffAgentUpdater updates last_seen_at.
type mockDiffAgentUpdater struct {
	lastSeenCalled bool
	err            error
}

func (m *mockDiffAgentUpdater) UpdateLastSeen(ctx context.Context, id string) error {
	m.lastSeenCalled = true
	return m.err
}

// mockDiffTrendingRepo counts new trending posts since a given time.
type mockDiffTrendingRepo struct {
	count int
	err   error
}

func (m *mockDiffTrendingRepo) CountTrendingSince(ctx context.Context, since time.Time) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

func newMeDiffHandler() (*MeDiffHandler, *mockDiffNotificationsRepo, *mockDiffReputationRepo, *mockDiffOpportunitiesRepo, *mockDiffBadgesRepo, *mockDiffAgentUpdater, *mockDiffTrendingRepo) {
	notifRepo := newMockDiffNotificationsRepo()
	repRepo := &mockDiffReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}}
	oppsRepo := &mockDiffOpportunitiesRepo{}
	badgesRepo := &mockDiffBadgesRepo{}
	agentUpdater := &mockDiffAgentUpdater{}
	trendingRepo := &mockDiffTrendingRepo{}

	handler := &MeDiffHandler{
		notificationsRepo: notifRepo,
		reputationRepo:    repRepo,
		opportunitiesRepo: oppsRepo,
		badgesRepo:        badgesRepo,
		agentUpdater:      agentUpdater,
		trendingRepo:      trendingRepo,
	}

	return handler, notifRepo, repRepo, oppsRepo, badgesRepo, agentUpdater, trendingRepo
}

// TestMeDiff_WithRecentSince verifies delta response with correct counts.
func TestMeDiff_WithRecentSince(t *testing.T) {
	handler, notifRepo, repRepo, oppsRepo, badgesRepo, agentUpdater, trendingRepo := newMeDiffHandler()

	// Set up mock data
	notifRepo.countByAgent["test_agent"] = 3
	repRepo.result = &models.ReputationChangesResult{
		SinceLastCheck: "+15",
		Breakdown: []models.ReputationEvent{
			{Reason: "answer_accepted", PostID: "p1", PostTitle: "Test", Delta: 50},
		},
	}
	oppsRepo.count = 2
	badgesRepo.badges = []models.Badge{
		{ID: "b1", BadgeType: models.BadgeFirstSolve, BadgeName: "First Solve", AwardedAt: time.Now()},
	}
	trendingRepo.count = 5

	// Request with recent since param (10 minutes ago)
	since := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/me/diff?since="+since, nil)
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		Specialties: []string{"golang"},
		CreatedAt:   time.Now().Add(-72 * time.Hour),
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetDiff(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// Check new_notifications count
	newNotif := int(data["new_notifications"].(float64))
	if newNotif != 3 {
		t.Errorf("expected new_notifications=3, got %d", newNotif)
	}

	// Check reputation_delta
	repDelta := data["reputation_delta"].(string)
	if repDelta != "+15" {
		t.Errorf("expected reputation_delta='+15', got %q", repDelta)
	}

	// Check new_opportunities
	newOpps := int(data["new_opportunities"].(float64))
	if newOpps != 2 {
		t.Errorf("expected new_opportunities=2, got %d", newOpps)
	}

	// Check badges_earned
	badges := data["badges_earned"].([]interface{})
	if len(badges) != 1 {
		t.Errorf("expected 1 badge earned, got %d", len(badges))
	}

	// Check new_trending_count
	trendCount := int(data["new_trending_count"].(float64))
	if trendCount != 5 {
		t.Errorf("expected new_trending_count=5, got %d", trendCount)
	}

	// Check since is echoed back
	if data["since"] == nil || data["since"].(string) == "" {
		t.Error("expected 'since' field to be set")
	}

	// Check next_full_briefing is set
	if data["next_full_briefing"] == nil || data["next_full_briefing"].(string) == "" {
		t.Error("expected 'next_full_briefing' field to be set")
	}

	// Verify last_seen_at was updated
	if !agentUpdater.lastSeenCalled {
		t.Error("expected UpdateLastSeen to be called")
	}
}

// TestMeDiff_MissingSince verifies 302 redirect to /v1/me when since param is missing.
func TestMeDiff_MissingSince(t *testing.T) {
	handler, _, _, _, _, _, _ := newMeDiffHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/me/diff", nil)
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetDiff(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d: %s", rr.Code, rr.Body.String())
	}

	location := rr.Header().Get("Location")
	if location != "/v1/me" {
		t.Errorf("expected Location header '/v1/me', got %q", location)
	}
}

// TestMeDiff_OldSince verifies 302 redirect to /v1/me when since is older than 24h.
func TestMeDiff_OldSince(t *testing.T) {
	handler, _, _, _, _, _, _ := newMeDiffHandler()

	// Since 25 hours ago — older than 24h threshold
	since := time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/me/diff?since="+since, nil)
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetDiff(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d: %s", rr.Code, rr.Body.String())
	}

	location := rr.Header().Get("Location")
	if location != "/v1/me" {
		t.Errorf("expected Location header '/v1/me', got %q", location)
	}
}

// TestMeDiff_NoChanges verifies zeroed response when nothing changed since timestamp.
func TestMeDiff_NoChanges(t *testing.T) {
	handler, _, _, _, _, _, _ := newMeDiffHandler()

	// All mock repos return zero/empty by default
	since := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/me/diff?since="+since, nil)
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		CreatedAt:   time.Now().Add(-72 * time.Hour),
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetDiff(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})

	// All counts should be zero
	if int(data["new_notifications"].(float64)) != 0 {
		t.Errorf("expected new_notifications=0, got %v", data["new_notifications"])
	}
	if data["reputation_delta"].(string) != "+0" {
		t.Errorf("expected reputation_delta='+0', got %q", data["reputation_delta"])
	}
	if int(data["new_opportunities"].(float64)) != 0 {
		t.Errorf("expected new_opportunities=0, got %v", data["new_opportunities"])
	}
	if int(data["new_trending_count"].(float64)) != 0 {
		t.Errorf("expected new_trending_count=0, got %v", data["new_trending_count"])
	}

	// badges_earned should be empty array (not null)
	badges := data["badges_earned"].([]interface{})
	if len(badges) != 0 {
		t.Errorf("expected 0 badges earned, got %d", len(badges))
	}
}
