package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestMigrations_CrystallizationFields tests that migration 000040 adds
// crystallization_cid and crystallized_at columns to the posts table.
func TestMigrations_CrystallizationFields(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify crystallization_cid column exists on posts table
	columns := []string{"crystallization_cid", "crystallized_at"}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'posts' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in posts table: %v", col, err)
		}
	}

	// Verify crystallization_cid column is TEXT (nullable)
	var dataType string
	err = pool.QueryRow(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'posts' AND column_name = 'crystallization_cid'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("Could not get data type for crystallization_cid: %v", err)
	}
	if dataType != "text" {
		t.Errorf("crystallization_cid data type = %q, want 'text'", dataType)
	}

	// Verify crystallized_at column is TIMESTAMPTZ
	err = pool.QueryRow(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'posts' AND column_name = 'crystallized_at'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("Could not get data type for crystallized_at: %v", err)
	}
	if dataType != "timestamp with time zone" {
		t.Errorf("crystallized_at data type = %q, want 'timestamp with time zone'", dataType)
	}

	// Verify both columns are nullable (no NOT NULL constraint)
	for _, col := range columns {
		var isNullable string
		err = pool.QueryRow(ctx, `
			SELECT is_nullable
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'posts' AND column_name = $1
		`, col).Scan(&isNullable)
		if err != nil {
			t.Fatalf("Could not check nullable for %s: %v", col, err)
		}
		if isNullable != "YES" {
			t.Errorf("Column %s is_nullable = %q, want 'YES'", col, isNullable)
		}
	}

	// Verify partial index exists on crystallization_cid
	var idxName string
	err = pool.QueryRow(ctx, `
		SELECT indexname
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = 'posts' AND indexname = 'idx_posts_crystallization_cid'
	`).Scan(&idxName)
	if err != nil {
		t.Error("Index idx_posts_crystallization_cid does not exist on posts table")
	}
}

// TestCrystallization_PostModelFields tests that the Post model correctly
// handles crystallization_cid and crystallized_at fields through the repository.
func TestCrystallization_PostModelFields(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a test user first (needed for posted_by_id)
	var userID string
	err = pool.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id)
		VALUES ('crystal_test_user', 'Crystal Test User', 'crystal_test@example.com', 'github', 'crystal_github_id')
		RETURNING id::text
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1::uuid", userID)
	}()

	postRepo := db.NewPostRepository(pool)

	// Test 1: Create a problem post — crystallization fields should be nil
	post := &models.Post{
		Type:            models.PostTypeProblem,
		Title:           "Test Crystallization Problem",
		Description:     "A problem to test crystallization fields.",
		Tags:            []string{"test", "crystallization"},
		PostedByType:    models.AuthorTypeHuman,
		PostedByID:      userID,
		Status:          models.PostStatusOpen,
		SuccessCriteria: []string{"CID is set after crystallization"},
		Weight:          intPtr(3),
	}

	created, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify crystallization fields are nil on new post
	if created.CrystallizationCID != nil {
		t.Errorf("New post crystallization_cid = %v, want nil", *created.CrystallizationCID)
	}
	if created.CrystallizedAt != nil {
		t.Errorf("New post crystallized_at = %v, want nil", *created.CrystallizedAt)
	}

	// Test 2: FindByID should also return nil crystallization fields
	found, err := postRepo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.CrystallizationCID != nil {
		t.Errorf("FindByID crystallization_cid = %v, want nil", *found.CrystallizationCID)
	}
	if found.CrystallizedAt != nil {
		t.Errorf("FindByID crystallized_at = %v, want nil", *found.CrystallizedAt)
	}

	// Test 3: Set crystallization fields via direct SQL (simulating crystallization service)
	testCID := "QmTestCrystallizationCID1234567890abcdef"
	_, err = pool.Exec(ctx, `
		UPDATE posts SET crystallization_cid = $1, crystallized_at = NOW()
		WHERE id = $2
	`, testCID, created.ID)
	if err != nil {
		t.Fatalf("Failed to set crystallization fields: %v", err)
	}

	// Test 4: FindByID should now return the crystallization fields
	crystallized, err := postRepo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() after crystallization error = %v", err)
	}
	if crystallized.CrystallizationCID == nil || *crystallized.CrystallizationCID != testCID {
		t.Errorf("Crystallized post CID = %v, want %q", crystallized.CrystallizationCID, testCID)
	}
	if crystallized.CrystallizedAt == nil {
		t.Error("Crystallized post crystallized_at should not be nil")
	}

	// Test 5: List should include crystallization fields
	posts, _, err := postRepo.List(ctx, models.PostListOptions{
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   userID,
		Page:       1,
		PerPage:    10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("List() returned no posts")
	}

	foundInList := false
	for _, p := range posts {
		if p.ID == created.ID {
			foundInList = true
			if p.CrystallizationCID == nil || *p.CrystallizationCID != testCID {
				t.Errorf("List result CID = %v, want %q", p.CrystallizationCID, testCID)
			}
			if p.CrystallizedAt == nil {
				t.Error("List result crystallized_at should not be nil")
			}
		}
	}
	if !foundInList {
		t.Error("Crystallized post not found in List() results")
	}

	// Test 6: Update should preserve crystallization fields (they're not in the UPDATE SET clause)
	created.Status = models.PostStatusSolved
	updated, err := postRepo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.CrystallizationCID == nil || *updated.CrystallizationCID != testCID {
		t.Errorf("Updated post CID = %v, want %q", updated.CrystallizationCID, testCID)
	}
}

// TestCrystallization_SetCrystallizationCID tests the SetCrystallizationCID repository method.
func TestCrystallization_SetCrystallizationCID(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a test user
	var userID string
	err = pool.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id)
		VALUES ('crystal_set_user', 'Crystal Set User', 'crystal_set@example.com', 'github', 'crystal_set_github')
		RETURNING id::text
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1::uuid", userID)
	}()

	postRepo := db.NewPostRepository(pool)

	// Create a problem post
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Problem for CID Setting",
		Description:  "Testing the SetCrystallizationCID method.",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   userID,
		Status:       models.PostStatusSolved,
	}

	created, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test: Set crystallization CID
	testCID := "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"
	err = postRepo.SetCrystallizationCID(ctx, created.ID, testCID)
	if err != nil {
		t.Fatalf("SetCrystallizationCID() error = %v", err)
	}

	// Verify via FindByID
	found, err := postRepo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.CrystallizationCID == nil || *found.CrystallizationCID != testCID {
		t.Errorf("Post CID = %v, want %q", found.CrystallizationCID, testCID)
	}
	if found.CrystallizedAt == nil {
		t.Error("Post crystallized_at should be set")
	}

	// Test: SetCrystallizationCID on non-existent post returns error
	err = postRepo.SetCrystallizationCID(ctx, "00000000-0000-0000-0000-000000000000", testCID)
	if err == nil {
		t.Error("SetCrystallizationCID() on non-existent post should return error")
	}
}

// intPtr returns a pointer to an int value.
func intPtr(v int) *int {
	return &v
}
