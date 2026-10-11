package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// setupDBBackedSearchRepo creates a real search repository for integration testing.
// Returns nil if DATABASE_URL not set (test will be skipped).
func setupDBBackedSearchRepo(t *testing.T) (*db.SearchRepository, *db.Pool) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping DB-backed integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Clean up test data before tests
	_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id LIKE 'search-test-%'")

	t.Cleanup(func() {
		pool.Close()
	})

	return db.NewSearchRepository(pool), pool
}

// searchTestUserID is the human account these posts name (000117: a post names an existing
// author). It is the shared-database fixture user the db package's search fixtures post as too.
const searchTestUserID = "7e57c0de-0000-4000-8000-000000000001"

func insertSearchTestPost(t *testing.T, pool *db.Pool, ctx context.Context,
	postType, title, desc string, tags []string, status string) {
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, 'fixture_test_user', 'Test User', 'fixture-test-user@example.test', 'email', 'fixture-test-user', 'TSTUSER1')
		ON CONFLICT DO NOTHING`, searchTestUserID)
	if err != nil {
		t.Fatalf("failed to create the fixture test user: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (type, title, description, tags, status,
			posted_by_type, posted_by_id, created_at)
		VALUES ($1, $2, $3, $4, $5, 'human', $6, NOW())
	`, postType, title, desc, tags, status, searchTestUserID)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
}

// TestSearchIntegration_ExactTitleMatch tests that exact title matches rank correctly.
// This is a database-backed integration test verifying real PostgreSQL ts_rank behavior.
func TestSearchIntegration_ExactTitleMatch(t *testing.T) {
	repo, pool := setupDBBackedSearchRepo(t)
	ctx := context.Background()

	// Create 4 posts with similar titles
	titles := []string{
		"Race Conditions in Go",
		"How to Handle Race Conditions",
		"Understanding Race Conditions",
		"Thread Safety and Race Conditions",
	}

	for _, title := range titles {
		insertSearchTestPost(t, pool, ctx,
			"post", title, "Test description", []string{"go"}, "open")
	}

	handler := NewSearchHandler(repo)

	// Test: Exact title search ranks that post #1
	req := httptest.NewRequest(http.MethodGet,
		"/v1/search?q=Race+Conditions+in+Go&sort=relevance", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	data := resp["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected at least 1 result for exact title match")
	}

	firstResult := data[0].(map[string]any)
	if firstResult["title"] != "Race Conditions in Go" {
		t.Errorf("exact title should rank first, got: %s", firstResult["title"])
	}
}

// TestSearchIntegration_MultiWordQuery tests that multi-word queries use OR logic.
// Searches find posts with ANY of the search terms, not requiring ALL terms.
func TestSearchIntegration_MultiWordQuery(t *testing.T) {
	repo, pool := setupDBBackedSearchRepo(t)
	ctx := context.Background()

	// Create posts with only ONE of the search terms
	insertSearchTestPost(t, pool, ctx, "post",
		"Race detection in programs", "About race", []string{}, "open")

	insertSearchTestPost(t, pool, ctx, "post",
		"Conditional logic patterns", "About conditions", []string{}, "open")

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=race+condition", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	meta := resp["meta"].(map[string]any)
	total := int(meta["total"].(float64))

	// With OR logic: should find both posts (one has "race", one has "condition")
	if total < 2 {
		t.Errorf("OR logic should find posts with ANY term, got %d results", total)
	}
}

// TestSearchIntegration_PartialWordMatch tests that prefix matching works.
// Searches like "rac" should find "race", "cond" should find "condition".
func TestSearchIntegration_PartialWordMatch(t *testing.T) {
	repo, pool := setupDBBackedSearchRepo(t)
	ctx := context.Background()

	insertSearchTestPost(t, pool, ctx, "post",
		"Race conditions in Go", "Description", []string{}, "open")

	handler := NewSearchHandler(repo)

	// Test: "rac" should find "race"
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=rac", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	meta := resp["meta"].(map[string]any)
	total := int(meta["total"].(float64))

	if total == 0 {
		t.Error("prefix match 'rac' should find posts with 'race'")
	}
}
