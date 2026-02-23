package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func createBadgesTestAgent(t *testing.T, repo *AgentRepository, suffix string) *models.Agent {
	t.Helper()
	ctx := context.Background()

	agentID := "badge_agent_" + suffix + "_" + time.Now().Format("20060102150405.000000000")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Badge Test Agent " + suffix,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	return agent
}

func createBadgesTestUser(t *testing.T, repo *UserRepository, suffix string) *models.User {
	t.Helper()
	ctx := context.Background()

	now := time.Now()
	ts := now.Format("150405.000000")
	user := &models.User{
		Username:       "bg" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4],
		DisplayName:    "Badge Test User " + suffix,
		Email:          "badge_" + suffix + "_" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_badge_" + suffix + "_" + ts,
		Role:           "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

func TestBadges_AwardAndList(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	badgesRepo := NewBadgeRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent := createBadgesTestAgent(t, agentRepo, "awardlist")

	// Award 2 badges
	badge1 := &models.Badge{
		OwnerType:   "agent",
		OwnerID:     agent.ID,
		BadgeType:   models.BadgeFirstSolve,
		BadgeName:   "First Solve",
		Description: "Solved your first problem",
	}
	err := badgesRepo.Award(ctx, badge1)
	if err != nil {
		t.Fatalf("failed to award badge 1: %v", err)
	}

	badge2 := &models.Badge{
		OwnerType:   "agent",
		OwnerID:     agent.ID,
		BadgeType:   models.BadgeFirstAnswerAccepted,
		BadgeName:   "First Answer Accepted",
		Description: "Had your first answer accepted",
	}
	err = badgesRepo.Award(ctx, badge2)
	if err != nil {
		t.Fatalf("failed to award badge 2: %v", err)
	}

	// List badges for agent
	badges, err := badgesRepo.ListForOwner(ctx, "agent", agent.ID)
	if err != nil {
		t.Fatalf("failed to list badges: %v", err)
	}

	if len(badges) != 2 {
		t.Errorf("expected 2 badges, got %d", len(badges))
	}
}

func TestBadges_AwardToHuman(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	badgesRepo := NewBadgeRepository(pool)
	userRepo := NewUserRepository(pool)

	user := createBadgesTestUser(t, userRepo, "awardh")

	badge := &models.Badge{
		OwnerType:   "human",
		OwnerID:     user.ID,
		BadgeType:   models.BadgeFirstSolve,
		BadgeName:   "First Solve",
		Description: "Solved your first problem",
	}
	err := badgesRepo.Award(ctx, badge)
	if err != nil {
		t.Fatalf("failed to award badge to human: %v", err)
	}

	badges, err := badgesRepo.ListForOwner(ctx, "human", user.ID)
	if err != nil {
		t.Fatalf("failed to list badges: %v", err)
	}

	if len(badges) != 1 {
		t.Errorf("expected 1 badge, got %d", len(badges))
	}

	if badges[0].BadgeType != models.BadgeFirstSolve {
		t.Errorf("expected badge_type %q, got %q", models.BadgeFirstSolve, badges[0].BadgeType)
	}

	if badges[0].OwnerType != "human" {
		t.Errorf("expected owner_type 'human', got %q", badges[0].OwnerType)
	}
}

func TestBadges_IdempotentAward(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	badgesRepo := NewBadgeRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent := createBadgesTestAgent(t, agentRepo, "idempotent")

	badge := &models.Badge{
		OwnerType:   "agent",
		OwnerID:     agent.ID,
		BadgeType:   models.BadgeModelSet,
		BadgeName:   "Model Set",
		Description: "Set your model field",
	}

	// Award first time
	err := badgesRepo.Award(ctx, badge)
	if err != nil {
		t.Fatalf("failed to award badge first time: %v", err)
	}

	// Award second time - should not error (idempotent)
	err = badgesRepo.Award(ctx, badge)
	if err != nil {
		t.Errorf("expected no error on duplicate award, got: %v", err)
	}

	// Verify only 1 exists
	badges, err := badgesRepo.ListForOwner(ctx, "agent", agent.ID)
	if err != nil {
		t.Fatalf("failed to list badges: %v", err)
	}

	if len(badges) != 1 {
		t.Errorf("expected 1 badge after duplicate award, got %d", len(badges))
	}
}

func TestBadges_HasBadge(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	badgesRepo := NewBadgeRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent := createBadgesTestAgent(t, agentRepo, "hasbadge")

	// Check non-existent badge
	has, err := badgesRepo.HasBadge(ctx, "agent", agent.ID, models.BadgeHumanBacked)
	if err != nil {
		t.Fatalf("failed to check HasBadge: %v", err)
	}
	if has {
		t.Error("expected HasBadge to return false for non-existent badge")
	}

	// Award badge
	badge := &models.Badge{
		OwnerType:   "agent",
		OwnerID:     agent.ID,
		BadgeType:   models.BadgeHumanBacked,
		BadgeName:   "Human-Backed",
		Description: "Claimed by a human",
	}
	err = badgesRepo.Award(ctx, badge)
	if err != nil {
		t.Fatalf("failed to award badge: %v", err)
	}

	// Check again
	has, err = badgesRepo.HasBadge(ctx, "agent", agent.ID, models.BadgeHumanBacked)
	if err != nil {
		t.Fatalf("failed to check HasBadge: %v", err)
	}
	if !has {
		t.Error("expected HasBadge to return true after awarding badge")
	}
}
