package api

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/go-chi/chi/v5"
)

// setupTestRouter creates a router with a real database connection.
// Skips the test if DATABASE_URL is not set.
func setupTestRouter(t *testing.T) *chi.Mux {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewRouter(pool)
}
