package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// TestLeaderboardIntegration_AllTypes tests fetching mixed agents and users.
func TestLeaderboardIntegration_AllTypes(t *testing.T) {
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

	repo := NewLeaderboardRepository(pool)
	agentRepo := NewAgentRepository(pool)
	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test agent with some activity
	agent := &models.Agent{
		ID:          "test_leaderboard_agent_" + time.Now().Format("150405.000"),
		DisplayName: "Test Leaderboard Agent",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create test user
	suffix := time.Now().Format("150405.000")
	user := &models.User{
		Username:       "leaderboard_" + suffix,
		Email:          "leaderboard_" + suffix + "@example.com",
		DisplayName:    "Test Leaderboard User",
		AvatarURL:      "https://example.com/user-avatar.jpg",
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "leaderboard-provider-" + suffix,
		Role:           models.UserRoleUser,
	}
	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// Create a solved problem for agent
	problem, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Leaderboard Test Problem",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Status:       models.PostStatusSolved,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problem.ID)
	}()

	// Fetch leaderboard
	opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	entries, total, err := repo.GetLeaderboard(ctx, opts)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if total == 0 {
		t.Error("expected non-zero total count")
	}

	if len(entries) == 0 {
		t.Error("expected non-zero entries")
	}

	// NEW: Verify agent reputation is calculated correctly from solved post
	// Agent created solved post, should have 100 reputation
	var agentEntry *models.LeaderboardEntry
	for i := range entries {
		if entries[i].ID == agent.ID && entries[i].Type == "agent" {
			agentEntry = &entries[i]
			break
		}
	}

	if agentEntry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	if agentEntry.Reputation != 125 {
		t.Errorf("expected reputation 125 for solved post (100 solved + 25 contributed), got %d", agentEntry.Reputation)
	}

	t.Logf("✓ Agent reputation correctly calculated: %d", agentEntry.Reputation)

	// Verify entries have ranks
	for i, entry := range entries {
		if entry.Rank != i+1+opts.Offset {
			t.Errorf("entry %d: expected rank=%d, got %d", i, i+1+opts.Offset, entry.Rank)
		}

		if entry.ID == "" {
			t.Errorf("entry %d: ID is empty", i)
		}

		if entry.Type != "agent" && entry.Type != "user" {
			t.Errorf("entry %d: invalid type %s", i, entry.Type)
		}
	}

	// Verify our test agent appears in the leaderboard
	foundAgent := false
	for _, entry := range entries {
		if entry.ID == agent.ID {
			foundAgent = true
			if entry.Type != "agent" {
				t.Errorf("agent entry has wrong type: %s", entry.Type)
			}
			if entry.DisplayName != agent.DisplayName {
				t.Errorf("agent entry has wrong display_name: got %s, want %s", entry.DisplayName, agent.DisplayName)
			}
			if entry.Reputation != agent.Reputation {
				t.Logf("agent reputation: got %d, agent.reputation=%d", entry.Reputation, agent.Reputation)
			}
		}
	}

	if !foundAgent {
		t.Error("test agent not found in leaderboard")
	}
}

// TestLeaderboardIntegration_AgentsOnly tests filtering to agents only.
func TestLeaderboardIntegration_AgentsOnly(t *testing.T) {
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

	repo := NewLeaderboardRepository(pool)

	opts := models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	entries, total, err := repo.GetLeaderboard(ctx, opts)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// All entries should be agents
	for i, entry := range entries {
		if entry.Type != "agent" {
			t.Errorf("entry %d: expected type=agent, got %s", i, entry.Type)
		}
	}

	t.Logf("Found %d agents out of %d total", len(entries), total)
}

// TestLeaderboardIntegration_UsersOnly tests filtering to users only.
func TestLeaderboardIntegration_UsersOnly(t *testing.T) {
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

	repo := NewLeaderboardRepository(pool)

	opts := models.LeaderboardOptions{
		Type:      "users",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	entries, total, err := repo.GetLeaderboard(ctx, opts)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// All entries should be users
	for i, entry := range entries {
		if entry.Type != "user" {
			t.Errorf("entry %d: expected type=user, got %s", i, entry.Type)
		}
	}

	t.Logf("Found %d users out of %d total", len(entries), total)
}

// TestLeaderboardIntegration_Pagination tests limit and offset.
func TestLeaderboardIntegration_Pagination(t *testing.T) {
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

	repo := NewLeaderboardRepository(pool)

	// Fetch page 1
	page1Opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     5,
		Offset:    0,
	}

	page1, total, err := repo.GetLeaderboard(ctx, page1Opts)
	if err != nil {
		t.Fatalf("GetLeaderboard page 1 failed: %v", err)
	}

	if total == 0 {
		t.Skip("No entries in leaderboard, skipping pagination test")
	}

	// Fetch page 2
	page2Opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     5,
		Offset:    5,
	}

	page2, _, err := repo.GetLeaderboard(ctx, page2Opts)
	if err != nil {
		t.Fatalf("GetLeaderboard page 2 failed: %v", err)
	}

	// Verify ranks are continuous
	if len(page1) > 0 && len(page2) > 0 {
		lastRankPage1 := page1[len(page1)-1].Rank
		firstRankPage2 := page2[0].Rank

		if firstRankPage2 != lastRankPage1+1 {
			t.Errorf("pagination ranks not continuous: page1 last=%d, page2 first=%d",
				lastRankPage1, firstRankPage2)
		}
	}

	// Verify no duplicate IDs across pages
	idsSeen := make(map[string]bool)
	for _, entry := range page1 {
		idsSeen[entry.ID] = true
	}
	for _, entry := range page2 {
		if idsSeen[entry.ID] {
			t.Errorf("duplicate ID across pages: %s", entry.ID)
		}
	}

	t.Logf("Page 1: %d entries, Page 2: %d entries, Total: %d", len(page1), len(page2), total)
}

// TestLeaderboardIntegration_RankingOrder verifies reputation sorting.
func TestLeaderboardIntegration_RankingOrder(t *testing.T) {
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

	repo := NewLeaderboardRepository(pool)

	opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	entries, _, err := repo.GetLeaderboard(ctx, opts)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if len(entries) < 2 {
		t.Skip("Not enough entries to test ranking order")
	}

	// Verify descending reputation order
	for i := 0; i < len(entries)-1; i++ {
		if entries[i].Reputation < entries[i+1].Reputation {
			t.Errorf("reputation not in descending order: entry %d (%d) < entry %d (%d)",
				i, entries[i].Reputation, i+1, entries[i+1].Reputation)
		}
	}

	// Verify rank numbers are sequential
	for i, entry := range entries {
		expectedRank := i + 1
		if entry.Rank != expectedRank {
			t.Errorf("entry %d: expected rank=%d, got %d", i, expectedRank, entry.Rank)
		}
	}

	t.Logf("Verified %d entries in correct ranking order", len(entries))
}

// mustHash is a test helper that hashes a string or fails the test.
func mustHash(t *testing.T, s string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	return string(hash)
}
