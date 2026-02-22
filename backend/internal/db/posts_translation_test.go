// Package db provides database access for Solvr.
package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// insertTestPostWithOriginalLanguage inserts a draft post with original_language set for translation tests.
func insertTestPostWithOriginalLanguage(t *testing.T, pool *Pool, ctx context.Context, title, desc, originalLanguage string, attempts int) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, original_language, translation_attempts)
		VALUES ('problem', $1, $2, '{}', 'draft', 'human', 'test-user', $3, $4)
		RETURNING id
	`, title, desc, originalLanguage, attempts).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestPostWithOriginalLanguage: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id)
	})
	return id
}

// TestApplyTranslation_PreservesOriginalTitle verifies that calling ApplyTranslation
// twice does NOT overwrite original_title with the translated title.
// Without the COALESCE fix, the second call would store the first English translation
// as original_title, permanently losing the true Chinese original.
func TestApplyTranslation_PreservesOriginalTitle(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	chineseTitle := "如何在Go中使用goroutines"
	chineseDesc := "我想了解Go语言中的goroutines工作原理"

	postID := insertTestPostWithOriginalLanguage(t, pool, ctx, chineseTitle, chineseDesc, "Chinese", 0)

	// First translation
	err := repo.ApplyTranslation(ctx, postID, "How to use goroutines in Go", "I want to understand how goroutines work in Go")
	if err != nil {
		t.Fatalf("first ApplyTranslation failed: %v", err)
	}

	var originalTitle, title string
	err = pool.QueryRow(ctx, "SELECT original_title, title FROM posts WHERE id = $1", postID).Scan(&originalTitle, &title)
	if err != nil {
		t.Fatalf("failed to query post after first translation: %v", err)
	}

	if originalTitle != chineseTitle {
		t.Errorf("after first translation: original_title = %q, want %q", originalTitle, chineseTitle)
	}
	if title != "How to use goroutines in Go" {
		t.Errorf("after first translation: title = %q, want 'How to use goroutines in Go'", title)
	}

	// Second translation (simulating a retry/re-entry into the translation queue)
	err = repo.ApplyTranslation(ctx, postID, "How to use goroutines in Go (revised)", "Updated goroutines description")
	if err != nil {
		t.Fatalf("second ApplyTranslation failed: %v", err)
	}

	err = pool.QueryRow(ctx, "SELECT original_title, title FROM posts WHERE id = $1", postID).Scan(&originalTitle, &title)
	if err != nil {
		t.Fatalf("failed to query post after second translation: %v", err)
	}

	// COALESCE fix ensures original_title is NOT overwritten with the first English translation
	if originalTitle != chineseTitle {
		t.Errorf("after second translation: original_title = %q, want %q (COALESCE should preserve Chinese original)", originalTitle, chineseTitle)
	}
	if title != "How to use goroutines in Go (revised)" {
		t.Errorf("after second translation: title = %q, want 'How to use goroutines in Go (revised)'", title)
	}
}

// TestListPostsNeedingTranslation_ReturnsOnlyEligible verifies the query
// correctly filters by status='draft' AND original_language IS NOT NULL AND attempts < 3.
func TestListPostsNeedingTranslation_ReturnsOnlyEligible(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Post 1: draft + original_language set + attempts=0 → should appear
	eligibleID := insertTestPostWithOriginalLanguage(t, pool, ctx, "如何使用goroutines", "goroutines的问题", "Chinese", 0)

	// Post 2: open + original_language set → should NOT appear (wrong status)
	var id2 string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, original_language, translation_attempts)
		VALUES ('problem', 'Open Chinese post', 'desc', '{}', 'open', 'human', 'test-user', 'Chinese', 0)
		RETURNING id
	`).Scan(&id2)
	if err != nil {
		t.Fatalf("failed to insert open post: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id2) })

	// Post 3: draft + original_language NULL → should NOT appear (English post, no language detected)
	var id3 string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, translation_attempts)
		VALUES ('problem', 'Draft English post no language', 'desc', '{}', 'draft', 'human', 'test-user', 0)
		RETURNING id
	`).Scan(&id3)
	if err != nil {
		t.Fatalf("failed to insert draft English post: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id3) })

	// Post 4: draft + original_language set + attempts=3 → should NOT appear (maxed out)
	maxedID := insertTestPostWithOriginalLanguage(t, pool, ctx, "已达最大翻译次数", "最大次数描述", "Chinese", 3)

	posts, err := repo.ListPostsNeedingTranslation(ctx, 100)
	if err != nil {
		t.Fatalf("ListPostsNeedingTranslation failed: %v", err)
	}

	// Filter to only our test posts (other draft posts may exist in the DB)
	testIDs := map[string]bool{eligibleID: true, id2: true, id3: true, maxedID: true}
	var found []*models.Post
	for _, p := range posts {
		if testIDs[p.ID] {
			found = append(found, p)
		}
	}

	if len(found) != 1 {
		ids := make([]string, len(found))
		for i, p := range found {
			ids[i] = p.ID
		}
		t.Errorf("expected exactly 1 eligible post among test posts, got %d: %v", len(found), ids)
	}
	if len(found) > 0 && found[0].ID != eligibleID {
		t.Errorf("expected eligible post ID %q, got %q", eligibleID, found[0].ID)
	}
}
