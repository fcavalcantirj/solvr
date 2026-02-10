package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

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
