package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestGetTrendingPosts_ExcludesDraft verifies that draft posts are excluded from trending results.
func TestGetTrendingPosts_ExcludesDraft(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	statsRepo := NewStatsRepository(pool)
	ctx := context.Background()

	// Insert a draft post (should NOT appear in trending)
	draftID := insertTestPost(t, pool, ctx, "problem", "Draft trending problem about Go performance",
		"This is a draft and should not appear in trending.", []string{"go"}, "draft")

	// Insert an open post (should appear in trending)
	openID := insertTestPost(t, pool, ctx, "problem", "Open trending problem about Go performance",
		"This is open and should appear in trending.", []string{"go"}, "open")

	posts, err := statsRepo.GetTrendingPosts(ctx, 10)
	if err != nil {
		t.Fatalf("GetTrendingPosts() error = %v", err)
	}

	for _, p := range posts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == draftID {
			t.Errorf("expected draft post %s to be excluded from trending, but it appeared", draftID)
		}
	}

	foundOpen := false
	for _, p := range posts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == openID {
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Errorf("expected open post %s to appear in trending, but it did not", openID)
	}
}

// TestGetTrendingPosts_ExcludesPendingReview verifies that pending_review posts are excluded from trending results.
func TestGetTrendingPosts_ExcludesPendingReview(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	statsRepo := NewStatsRepository(pool)
	ctx := context.Background()

	// Insert a pending_review post (should NOT appear in trending)
	pendingID := insertTestPost(t, pool, ctx, "question", "Pending review trending question about databases",
		"This is pending review and should not appear in trending.", []string{"databases"}, "pending_review")

	// Insert an open post (should appear in trending)
	openID := insertTestPost(t, pool, ctx, "question", "Open trending question about databases",
		"This is open and should appear in trending.", []string{"databases"}, "open")

	posts, err := statsRepo.GetTrendingPosts(ctx, 10)
	if err != nil {
		t.Fatalf("GetTrendingPosts() error = %v", err)
	}

	for _, p := range posts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == pendingID {
			t.Errorf("expected pending_review post %s to be excluded from trending, but it appeared", pendingID)
		}
	}

	foundOpen := false
	for _, p := range posts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == openID {
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Errorf("expected open post %s to appear in trending, but it did not", openID)
	}
}

// TestGetTrendingPosts_ExcludesRejected verifies that rejected posts are excluded from trending results.
func TestGetTrendingPosts_ExcludesRejected(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	statsRepo := NewStatsRepository(pool)
	ctx := context.Background()

	// Insert a rejected post (should NOT appear in trending)
	rejectedID := insertTestPost(t, pool, ctx, "idea", "Rejected trending idea about AI agents",
		"This idea was rejected and should not appear in trending.", []string{"ai"}, "rejected")

	// Insert an open post (should appear in trending)
	openID := insertTestPost(t, pool, ctx, "idea", "Open trending idea about AI agents",
		"This idea is open and should appear in trending.", []string{"ai"}, "open")

	posts, err := statsRepo.GetTrendingPosts(ctx, 10)
	if err != nil {
		t.Fatalf("GetTrendingPosts() error = %v", err)
	}

	for _, p := range posts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == rejectedID {
			t.Errorf("expected rejected post %s to be excluded from trending, but it appeared", rejectedID)
		}
	}

	foundOpen := false
	for _, p := range posts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == openID {
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Errorf("expected open post %s to appear in trending, but it did not", openID)
	}
}

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.

func TestStatsRepository_GetTopSparklers(t *testing.T) {
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

	userRepo := NewUserRepository(pool)
	statsRepo := NewStatsRepository(pool)

	// Create a test user
	suffix := time.Now().Format("150405.000")
	user := &models.User{
		Username:       "sp_" + suffix,
		DisplayName:    "Sparkler Test",
		Email:          "sparkler_" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_sparkler_" + suffix,
		Role:           models.UserRoleUser,
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create(user) error = %v", err)
	}

	// Insert a test idea post
	var postID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Test idea description', '{}', 'human', $1, 'active')
		RETURNING id
	`, created.ID).Scan(&postID)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}

	// Clean up after test
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// GetTopSparklers — this triggers the query with a.name (the bug)
	sparklers, err := statsRepo.GetTopSparklers(ctx, 5)
	if err != nil {
		t.Fatalf("GetTopSparklers() error = %v", err)
	}

	if len(sparklers) == 0 {
		t.Error("GetTopSparklers() returned empty results, want at least 1")
	}
}
