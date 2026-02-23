package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// mustHash is a helper to create bcrypt hashes for testing.
func mustHashTag(t *testing.T, password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// TestLeaderboardByTag_ValidTag verifies that an agent appears in the leaderboard
// for a tag they have activity in.
func TestLeaderboardByTag_ValidTag(t *testing.T) {
	// Skip if no DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create repositories
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	// Create a test agent
	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_tag_agent_" + suffix,
		DisplayName: "Golang Expert",
		APIKeyHash:  mustHashTag(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create a solved problem with "golang" tag
	post, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "How to use channels in Go",
		Description:  "Need help with Go channels",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Tags:         []string{"golang", "concurrency"},
		Status:       models.PostStatusSolved,
	})
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", post.ID)
	}()

	// Fetch leaderboard for "golang" tag
	opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	entries, total, err := leaderboardRepo.GetLeaderboardByTag(ctx, "golang", opts)
	if err != nil {
		t.Fatalf("GetLeaderboardByTag failed: %v", err)
	}

	// Verify agent appears in results
	if total == 0 {
		t.Fatal("expected at least 1 entry, got 0")
	}

	found := false
	for _, entry := range entries {
		if entry.ID == agent.ID {
			found = true
			// Verify stats
			if entry.KeyStats.ProblemsSolved < 1 {
				t.Errorf("expected problems_solved >= 1, got %d", entry.KeyStats.ProblemsSolved)
			}
			if entry.Reputation < 100 {
				t.Errorf("expected reputation >= 100, got %d", entry.Reputation)
			}
			break
		}
	}

	if !found {
		t.Errorf("agent %s not found in leaderboard results", agent.ID)
	}
}

// TestLeaderboardByTag_NoActivity verifies that querying for a non-existent tag
// returns an empty array.
func TestLeaderboardByTag_NoActivity(t *testing.T) {
	// Skip if no DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	leaderboardRepo := NewLeaderboardRepository(pool)

	// Fetch leaderboard for a non-existent tag
	opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	entries, total, err := leaderboardRepo.GetLeaderboardByTag(ctx, "nonexistenttag12345", opts)
	if err != nil {
		t.Fatalf("GetLeaderboardByTag failed: %v", err)
	}

	// Verify empty results
	if total != 0 {
		t.Errorf("expected total=0 for non-existent tag, got %d", total)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty array for non-existent tag, got %d entries", len(entries))
	}
}

// TestLeaderboardByTag_Pagination verifies that pagination works correctly
// for tag-specific leaderboards.
func TestLeaderboardByTag_Pagination(t *testing.T) {
	// Skip if no DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create repositories
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	// Create 3 test agents with activity in "rust" tag
	tag := "rust"
	agentIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		suffix := time.Now().Format("150405.000000") + fmt.Sprintf("%d", i)
		agent := &models.Agent{
			ID:          "test_rust_agent_" + suffix,
			DisplayName: "Rust Expert " + suffix,
			APIKeyHash:  mustHashTag(t, "test-key-"+suffix),
			Status:      "active",
		}
		if err := agentRepo.Create(ctx, agent); err != nil {
			t.Fatalf("failed to create agent %d: %v", i, err)
		}
		agentIDs[i] = agent.ID
		defer func(id string) {
			_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", id)
		}(agent.ID)

		// Create a solved problem for this agent
		post, err := postRepo.Create(ctx, &models.Post{
			Type:         models.PostTypeProblem,
			Title:        "Rust ownership question",
			Description:  "Need help with Rust ownership",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   agent.ID,
			Tags:         []string{tag},
			Status:       models.PostStatusSolved,
		})
		if err != nil {
			t.Fatalf("failed to create post for agent %d: %v", i, err)
		}
		defer func(id string) {
			_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id)
		}(post.ID)
	}

	// Fetch page 1 (limit=2)
	opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     2,
		Offset:    0,
	}

	page1, total1, err := leaderboardRepo.GetLeaderboardByTag(ctx, tag, opts)
	if err != nil {
		t.Fatalf("GetLeaderboardByTag page 1 failed: %v", err)
	}

	// Fetch page 2 (offset=2)
	opts.Offset = 2
	page2, total2, err := leaderboardRepo.GetLeaderboardByTag(ctx, tag, opts)
	if err != nil {
		t.Fatalf("GetLeaderboardByTag page 2 failed: %v", err)
	}

	// Verify total is consistent
	if total1 != total2 {
		t.Errorf("total count mismatch: page1=%d, page2=%d", total1, total2)
	}

	// Verify we got 2 entries on page 1
	if len(page1) != 2 {
		t.Errorf("expected 2 entries on page 1, got %d", len(page1))
	}

	// Verify ranks are continuous
	if len(page1) >= 2 && page1[1].Rank != page1[0].Rank+1 {
		t.Errorf("ranks not continuous on page 1: %d, %d", page1[0].Rank, page1[1].Rank)
	}

	// Verify no duplicate IDs between pages
	seenIDs := make(map[string]bool)
	for _, entry := range page1 {
		seenIDs[entry.ID] = true
	}
	for _, entry := range page2 {
		if seenIDs[entry.ID] {
			t.Errorf("duplicate ID across pages: %s", entry.ID)
		}
	}
}

// TestLeaderboardByTag_FiltersByTag verifies that tag filtering is isolated
// per tag (activity in one tag doesn't count for another).
func TestLeaderboardByTag_FiltersByTag(t *testing.T) {
	// Skip if no DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create repositories
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	// Create a test agent
	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_multi_agent_" + suffix,
		DisplayName: "Multi-language Expert",
		APIKeyHash:  mustHashTag(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create one problem tagged "rust"
	post1, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Rust borrow checker",
		Description:  "Help with Rust",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Tags:         []string{"rust"},
		Status:       models.PostStatusSolved,
	})
	if err != nil {
		t.Fatalf("failed to create rust post: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", post1.ID)
	}()

	// Create one problem tagged "python"
	post2, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Python decorators",
		Description:  "Help with Python",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Tags:         []string{"python"},
		Status:       models.PostStatusSolved,
	})
	if err != nil {
		t.Fatalf("failed to create python post: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", post2.ID)
	}()

	opts := models.LeaderboardOptions{
		Type:      "all",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	}

	// Fetch rust leaderboard
	rustEntries, _, err := leaderboardRepo.GetLeaderboardByTag(ctx, "rust", opts)
	if err != nil {
		t.Fatalf("GetLeaderboardByTag rust failed: %v", err)
	}

	// Fetch python leaderboard
	pythonEntries, _, err := leaderboardRepo.GetLeaderboardByTag(ctx, "python", opts)
	if err != nil {
		t.Fatalf("GetLeaderboardByTag python failed: %v", err)
	}

	// Find agent in rust results
	var rustStats *models.LeaderboardStats
	for _, entry := range rustEntries {
		if entry.ID == agent.ID {
			rustStats = &entry.KeyStats
			break
		}
	}

	// Find agent in python results
	var pythonStats *models.LeaderboardStats
	for _, entry := range pythonEntries {
		if entry.ID == agent.ID {
			pythonStats = &entry.KeyStats
			break
		}
	}

	// Verify agent appears in both
	if rustStats == nil {
		t.Fatal("agent not found in rust leaderboard")
	}
	if pythonStats == nil {
		t.Fatal("agent not found in python leaderboard")
	}

	// Verify exactly 1 problem solved in each
	if rustStats.ProblemsSolved != 1 {
		t.Errorf("rust: expected 1 problem solved, got %d", rustStats.ProblemsSolved)
	}
	if pythonStats.ProblemsSolved != 1 {
		t.Errorf("python: expected 1 problem solved, got %d", pythonStats.ProblemsSolved)
	}
}
