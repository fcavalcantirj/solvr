package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func createFollowsTestAgent(t *testing.T, repo *AgentRepository, suffix string) *models.Agent {
	t.Helper()
	ctx := context.Background()

	now := time.Now()
	ns := fmt.Sprintf("%04d", now.Nanosecond()/100000)
	agentID := "fa_" + suffix + "_" + now.Format("150405") + ns
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "FA " + suffix + " " + now.Format("150405") + ns,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	return agent
}

func createFollowsTestUser(t *testing.T, repo *UserRepository, suffix string) *models.User {
	t.Helper()
	ctx := context.Background()

	now := time.Now()
	ts := now.Format("150405.000000")
	user := &models.User{
		Username:       "fw" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4],
		DisplayName:    "Follow Test User " + suffix,
		Email:          "follow_" + suffix + "_" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_follow_" + suffix + "_" + ts,
		Role:           "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

func TestFollows_CreateAndFind(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent1 := createFollowsTestAgent(t, agentRepo, "a1")
	agent2 := createFollowsTestAgent(t, agentRepo, "a2")

	// Create follow
	follow, err := followsRepo.Create(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to create follow: %v", err)
	}

	if follow.FollowerType != "agent" {
		t.Errorf("expected follower_type 'agent', got %q", follow.FollowerType)
	}
	if follow.FollowerID != agent1.ID {
		t.Errorf("expected follower_id %q, got %q", agent1.ID, follow.FollowerID)
	}
	if follow.FollowedType != "agent" {
		t.Errorf("expected followed_type 'agent', got %q", follow.FollowedType)
	}
	if follow.FollowedID != agent2.ID {
		t.Errorf("expected followed_id %q, got %q", agent2.ID, follow.FollowedID)
	}

	// Verify IsFollowing returns true
	isFollowing, err := followsRepo.IsFollowing(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to check IsFollowing: %v", err)
	}
	if !isFollowing {
		t.Error("expected IsFollowing to return true after creating follow")
	}
}

func TestFollows_Delete(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent1 := createFollowsTestAgent(t, agentRepo, "del1")
	agent2 := createFollowsTestAgent(t, agentRepo, "del2")

	// Create follow
	_, err := followsRepo.Create(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to create follow: %v", err)
	}

	// Delete follow
	err = followsRepo.Delete(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to delete follow: %v", err)
	}

	// Verify IsFollowing returns false
	isFollowing, err := followsRepo.IsFollowing(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to check IsFollowing: %v", err)
	}
	if isFollowing {
		t.Error("expected IsFollowing to return false after deleting follow")
	}
}

func TestFollows_DuplicateIgnored(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent1 := createFollowsTestAgent(t, agentRepo, "dup1")
	agent2 := createFollowsTestAgent(t, agentRepo, "dup2")

	// Create follow first time
	_, err := followsRepo.Create(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to create follow first time: %v", err)
	}

	// Create same follow second time - should not error (idempotent)
	_, err = followsRepo.Create(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Errorf("expected no error on duplicate follow, got: %v", err)
	}
}

func TestFollows_ListFollowing(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)
	userRepo := NewUserRepository(pool)

	agent1 := createFollowsTestAgent(t, agentRepo, "list1")
	agent2 := createFollowsTestAgent(t, agentRepo, "list2")
	agent3 := createFollowsTestAgent(t, agentRepo, "list3")
	user1 := createFollowsTestUser(t, userRepo, "list1")

	// agent1 follows agent2, agent3, and user1
	_, err := followsRepo.Create(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to create follow 1: %v", err)
	}
	_, err = followsRepo.Create(ctx, "agent", agent1.ID, "agent", agent3.ID)
	if err != nil {
		t.Fatalf("failed to create follow 2: %v", err)
	}
	_, err = followsRepo.Create(ctx, "agent", agent1.ID, "human", user1.ID)
	if err != nil {
		t.Fatalf("failed to create follow 3: %v", err)
	}

	// List following
	follows, err := followsRepo.ListFollowing(ctx, "agent", agent1.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list following: %v", err)
	}

	if len(follows) != 3 {
		t.Errorf("expected 3 follows, got %d", len(follows))
	}
}

func TestFollows_CountFollowers(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)
	userRepo := NewUserRepository(pool)

	// Target agent that will be followed
	target := createFollowsTestAgent(t, agentRepo, "target")

	// Two different followers
	follower1 := createFollowsTestAgent(t, agentRepo, "follower1")
	follower2 := createFollowsTestUser(t, userRepo, "follower2")

	_, err := followsRepo.Create(ctx, "agent", follower1.ID, "agent", target.ID)
	if err != nil {
		t.Fatalf("failed to create follow 1: %v", err)
	}
	_, err = followsRepo.Create(ctx, "human", follower2.ID, "agent", target.ID)
	if err != nil {
		t.Fatalf("failed to create follow 2: %v", err)
	}

	// Count followers
	count, err := followsRepo.CountFollowers(ctx, "agent", target.ID)
	if err != nil {
		t.Fatalf("failed to count followers: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 followers, got %d", count)
	}
}

func TestFollows_ListFollowers(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)

	target := createFollowsTestAgent(t, agentRepo, "ltarget")
	f1 := createFollowsTestAgent(t, agentRepo, "lfol1")
	f2 := createFollowsTestAgent(t, agentRepo, "lfol2")

	_, err := followsRepo.Create(ctx, "agent", f1.ID, "agent", target.ID)
	if err != nil {
		t.Fatalf("failed to create follow 1: %v", err)
	}
	_, err = followsRepo.Create(ctx, "agent", f2.ID, "agent", target.ID)
	if err != nil {
		t.Fatalf("failed to create follow 2: %v", err)
	}

	followers, err := followsRepo.ListFollowers(ctx, "agent", target.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list followers: %v", err)
	}

	if len(followers) != 2 {
		t.Errorf("expected 2 followers, got %d", len(followers))
	}
}

func TestFollows_IsFollowing_NotFollowing(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	followsRepo := NewFollowsRepository(pool)
	agentRepo := NewAgentRepository(pool)

	agent1 := createFollowsTestAgent(t, agentRepo, "nf1")
	agent2 := createFollowsTestAgent(t, agentRepo, "nf2")

	// Check before creating any follows
	isFollowing, err := followsRepo.IsFollowing(ctx, "agent", agent1.ID, "agent", agent2.ID)
	if err != nil {
		t.Fatalf("failed to check IsFollowing: %v", err)
	}
	if isFollowing {
		t.Error("expected IsFollowing to return false when no follow exists")
	}
}
