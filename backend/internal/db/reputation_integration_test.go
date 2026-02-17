package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestReputation_CrossEndpointConsistency verifies ALL endpoints return SAME reputation
// This is the critical integration test ensuring the centralized reputation system works.
func TestReputation_CrossEndpointConsistency(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Setup: Create test user with activity
	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	suffix := time.Now().Format("150405.000")
	user := &models.User{
		Username:       "consistency_test_" + suffix,
		DisplayName:    "Consistency Test User",
		Email:          "consistency_" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_consistency_" + suffix,
		Role:           models.UserRoleUser,
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create user error = %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_type = 'post' AND target_id IN (SELECT id FROM posts WHERE posted_by_id = $1)", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// Create some activity: 1 solved problem (100 + 25 = 125 points)
	_, err = postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test Solved Problem",
		Description:  "Test",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   created.ID,
		Status:       models.PostStatusSolved,
	})
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	// 1. Get reputation from GetUserStats
	stats, err := userRepo.GetUserStats(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserStats error = %v", err)
	}
	reputationFromStats := stats.Reputation

	// 2. Get reputation from List
	users, _, err := userRepo.List(ctx, models.PublicUserListOptions{Limit: 100})
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	var reputationFromList int
	for _, u := range users {
		if u.ID == created.ID {
			reputationFromList = u.Reputation
			break
		}
	}

	// 3. Get reputation from Leaderboard
	entries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "users",
		Timeframe: "all_time",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard error = %v", err)
	}
	var reputationFromLeaderboard int
	for _, e := range entries {
		if e.ID == created.ID {
			reputationFromLeaderboard = e.Reputation
			break
		}
	}

	// ASSERT: All three must match
	t.Logf("Reputation from GetUserStats: %d", reputationFromStats)
	t.Logf("Reputation from List: %d", reputationFromList)
	t.Logf("Reputation from Leaderboard: %d", reputationFromLeaderboard)

	if reputationFromStats != reputationFromList {
		t.Errorf("❌ GetUserStats (%d) != List (%d)", reputationFromStats, reputationFromList)
	}
	if reputationFromStats != reputationFromLeaderboard {
		t.Errorf("❌ GetUserStats (%d) != Leaderboard (%d)", reputationFromStats, reputationFromLeaderboard)
	}

	if reputationFromStats == reputationFromList && reputationFromStats == reputationFromLeaderboard {
		t.Logf("✅ All endpoints consistent: %d points", reputationFromStats)
	}
}
