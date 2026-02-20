package services

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock dependencies ---

type mockBadgeRepo struct {
	awarded    map[string]bool
	awardCalls []models.Badge
}

func newMockBadgeRepo() *mockBadgeRepo {
	return &mockBadgeRepo{
		awarded: make(map[string]bool),
	}
}

func (m *mockBadgeRepo) Award(ctx context.Context, badge *models.Badge) error {
	key := badge.OwnerType + ":" + badge.OwnerID + ":" + badge.BadgeType
	m.awarded[key] = true
	m.awardCalls = append(m.awardCalls, *badge)
	return nil
}

func (m *mockBadgeRepo) HasBadge(ctx context.Context, ownerType, ownerID, badgeType string) (bool, error) {
	key := ownerType + ":" + ownerID + ":" + badgeType
	return m.awarded[key], nil
}

// preAward simulates a badge already existing (for idempotency tests).
func (m *mockBadgeRepo) preAward(ownerType, ownerID, badgeType string) {
	key := ownerType + ":" + ownerID + ":" + badgeType
	m.awarded[key] = true
}

type mockAgentStatsProvider struct {
	stats *models.AgentStats
	err   error
}

func (m *mockAgentStatsProvider) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	return m.stats, m.err
}

type mockUserStatsProvider struct {
	stats *models.UserStats
	err   error
}

func (m *mockUserStatsProvider) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	return m.stats, m.err
}

// --- Tests ---

func TestBadgeService_FirstSolve(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{
		stats: &models.AgentStats{ProblemsSolved: 1},
	}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "agent", "test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !badgeRepo.awarded["agent:test-agent:first_solve"] {
		t.Error("expected first_solve badge to be awarded")
	}

	// Verify the badge details
	var found bool
	for _, b := range badgeRepo.awardCalls {
		if b.BadgeType == models.BadgeFirstSolve {
			found = true
			if b.OwnerType != "agent" {
				t.Errorf("expected owner_type 'agent', got %q", b.OwnerType)
			}
			if b.OwnerID != "test-agent" {
				t.Errorf("expected owner_id 'test-agent', got %q", b.OwnerID)
			}
			if b.BadgeName == "" {
				t.Error("expected badge_name to be set")
			}
		}
	}
	if !found {
		t.Error("first_solve badge not found in award calls")
	}
}

func TestBadgeService_HumanFirstSolve(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{}
	userStats := &mockUserStatsProvider{
		stats: &models.UserStats{ProblemsSolved: 1},
	}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "human", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !badgeRepo.awarded["human:user-123:first_solve"] {
		t.Error("expected first_solve badge to be awarded for human")
	}

	// Verify owner type is human
	for _, b := range badgeRepo.awardCalls {
		if b.BadgeType == models.BadgeFirstSolve {
			if b.OwnerType != "human" {
				t.Errorf("expected owner_type 'human', got %q", b.OwnerType)
			}
		}
	}
}

func TestBadgeService_NoMilestoneMet(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{
		stats: &models.AgentStats{
			ProblemsSolved:  0,
			UpvotesReceived: 0,
			AnswersAccepted: 0,
		},
	}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "agent", "new-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(badgeRepo.awardCalls) != 0 {
		t.Errorf("expected 0 badges awarded, got %d", len(badgeRepo.awardCalls))
	}
}

func TestBadgeService_IdempotentCheck(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	// Pre-award first_solve badge
	badgeRepo.preAward("agent", "test-agent", models.BadgeFirstSolve)

	agentStats := &mockAgentStatsProvider{
		stats: &models.AgentStats{ProblemsSolved: 1},
	}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "agent", "test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Award should NOT have been called for first_solve since it already exists
	for _, b := range badgeRepo.awardCalls {
		if b.BadgeType == models.BadgeFirstSolve {
			t.Error("Award should NOT have been called for first_solve (already exists)")
		}
	}
}

func TestBadgeService_MultipleBadges(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{
		stats: &models.AgentStats{
			ProblemsSolved:  1,
			AnswersAccepted: 1,
		},
	}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "agent", "multi-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !badgeRepo.awarded["agent:multi-agent:first_solve"] {
		t.Error("expected first_solve badge to be awarded")
	}
	if !badgeRepo.awarded["agent:multi-agent:first_answer_accepted"] {
		t.Error("expected first_answer_accepted badge to be awarded")
	}

	if len(badgeRepo.awardCalls) != 2 {
		t.Errorf("expected 2 badges awarded, got %d", len(badgeRepo.awardCalls))
	}
}

func TestBadgeService_TenSolves(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{
		stats: &models.AgentStats{ProblemsSolved: 10},
	}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "agent", "prolific-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get both first_solve AND ten_solves
	if !badgeRepo.awarded["agent:prolific-agent:first_solve"] {
		t.Error("expected first_solve badge")
	}
	if !badgeRepo.awarded["agent:prolific-agent:ten_solves"] {
		t.Error("expected ten_solves badge")
	}
}

func TestBadgeService_HundredUpvotes(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{
		stats: &models.AgentStats{UpvotesReceived: 100},
	}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "agent", "popular-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !badgeRepo.awarded["agent:popular-agent:hundred_upvotes"] {
		t.Error("expected hundred_upvotes badge")
	}
}

func TestBadgeService_InvalidOwnerType(t *testing.T) {
	badgeRepo := newMockBadgeRepo()
	agentStats := &mockAgentStatsProvider{}
	userStats := &mockUserStatsProvider{}

	svc := NewBadgeService(badgeRepo, agentStats, userStats)

	err := svc.CheckAndAwardBadges(context.Background(), "unknown", "some-id")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}
