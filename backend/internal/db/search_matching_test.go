package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestSearchRepository_Search_ExactTitleMatch tests that exact title matches are found.
// Per plan: Verify "race condition" finds posts with those words in the title.
func TestSearchRepository_Search_ExactTitleMatch(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Create posts with specific titles
	titles := []string{
		"Race Conditions in Go",
		"How to Handle Race Conditions",
		"Understanding Race Conditions",
		"Thread Safety and Race Conditions",
	}

	for _, title := range titles {
		insertTestPost(t, pool, ctx, "post", title,
			"This is a test description", []string{"go"}, "open")
	}

	// Test 1: Search for "race condition" (without s) - should find all 4 posts
	results, total, _, _, err := repo.Search(ctx, "race condition", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total < 4 {
		t.Errorf("expected at least 4 results for 'race condition', got %d", total)
		t.Logf("Found posts:")
		for _, r := range results {
			t.Logf("  - %s: %s", r.ID, r.Title)
		}
	}

	// Test 2: Search for "race conditions" (with s) - should find all 4 posts
	_, total2, _, _, err := repo.Search(ctx, "race conditions", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total2 < 4 {
		t.Errorf("expected at least 4 results for 'race conditions', got %d", total2)
	}

	// Test 3: Search for exact title "Race Conditions in Go" - should return that post as #1
	results3, _, _, _, err := repo.Search(ctx, "Race Conditions in Go", models.SearchOptions{
		Sort:    "relevance",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results3) == 0 {
		t.Fatal("expected at least 1 result for exact title match")
	}

	// The first result should be the exact match (highest relevance)
	if results3[0].Title != "Race Conditions in Go" {
		t.Logf("Warning: exact title match not first. Got: %s", results3[0].Title)
		t.Logf("All results:")
		for i, r := range results3 {
			t.Logf("  %d. [score=%.2f] %s", i+1, r.Score, r.Title)
		}
	}
}

// TestSearchRepository_Search_MultiWordQuery tests AND vs OR logic for multi-word queries.
// Per plan: Verify if "race condition" uses AND (strict) or OR (relaxed).
func TestSearchRepository_Search_MultiWordQuery(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Create posts with only one of the search terms
	insertTestPost(t, pool, ctx, "post",
		"Race detection in concurrent programs",
		"This post is only about race detection", []string{}, "open")

	insertTestPost(t, pool, ctx, "post",
		"Understanding conditional logic",
		"This post is only about conditions", []string{}, "open")

	postMW3ID := insertTestPost(t, pool, ctx, "post",
		"Race condition debugging",
		"This post has both race and condition", []string{}, "open")

	// Search for "race condition"
	results, total, _, _, err := repo.Search(ctx, "race condition", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Search for 'race condition' returned %d results:", total)
	for _, r := range results {
		t.Logf("  - %s: %s", r.ID, r.Title)
	}

	// If using AND logic: only postMW3 should match (has both words)
	// If using OR logic: all 3 posts should match (has either word)
	// Current implementation uses OR (|), so we expect 3 results

	if total == 1 {
		t.Logf("CURRENT: Using AND logic - only posts with both 'race' AND 'condition' are returned")
		if results[0].ID != postMW3ID {
			t.Errorf("expected post with both words to be first, got %s", results[0].ID)
		}
	} else if total >= 3 {
		t.Logf("Using OR logic - posts with 'race' OR 'condition' are returned")
	} else {
		t.Errorf("unexpected result count: got %d, expected 1 (AND logic) or 3+ (OR logic)", total)
	}
}

// TestSearch_IdeasFoundAndVoteScorePresent is a regression test for two bugs:
// 1. Case-insensitive search: "solvr" (lowercase) must find ideas with "Solvr" (capitalized) in title
// 2. vote_score field: search results must return correct vote_score value (not zero/missing)
// This validates the fix in backend/internal/db/search.go (SQL alias votes→vote_score)
// and backend/internal/models/search.go (JSON tag votes→vote_score).
func TestSearch_IdeasFoundAndVoteScorePresent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert idea with "Solvr" capitalized in title (simulates the production bug scenario)
	postSolvrID := insertTestPostWithVotes(t, pool, ctx, "post",
		"Pattern: Solvr API Key Not in Environment",
		"When SOLVR_API_KEY is not set, the agent fails silently with no helpful message.",
		[]string{"solvr", "environment"}, "open", 5, 0) // upvotes=5, downvotes=0 → vote_score=5

	// Search for "solvr" (lowercase) with the type filter (every post is type post, idx 68)
	// PostgreSQL tsvector normalizes case so "Solvr" and "solvr" are equivalent
	results, total, _, _, err := repo.Search(ctx, "solvr", models.SearchOptions{
		Type:    "post",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total == 0 {
		t.Fatal("case-insensitive search for 'solvr' returned 0 results — tsvector normalization broken or idea not indexed")
	}

	// Verify the specific idea is in results with correct vote_score
	found := false
	for _, r := range results {
		if r.ID == postSolvrID {
			found = true
			if r.VoteScore != 5 {
				t.Errorf("vote_score field missing or wrong: expected 5 (upvotes-downvotes), got %d — check SQL alias in search.go", r.VoteScore)
			}
			if r.Type != "post" {
				t.Errorf("expected type=post, got %s", r.Type)
			}
			break
		}
	}

	if !found {
		t.Error("idea 'Pattern: Solvr API Key Not in Environment' not found when searching 'solvr' with type=idea filter")
		t.Log("Results returned:")
		for _, r := range results {
			t.Logf("  - %s: %s (type=%s, vote_score=%d)", r.ID, r.Title, r.Type, r.VoteScore)
		}
	}
}

// TestSearchRepository_Search_PartialWordMatch tests prefix matching.
// Per plan: Verify "rac" finds "race" and "cond" finds "condition".
func TestSearchRepository_Search_PartialWordMatch(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "post",
		"Race conditions in Go",
		"Description", []string{}, "open")

	// Test 1: "rac" should find "race"
	_, total1, _, _, err := repo.Search(ctx, "rac", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total1 == 0 {
		t.Error("expected 'rac' to find posts with 'race' (prefix matching)")
	} else {
		t.Logf("Prefix match 'rac' found %d results", total1)
	}

	// Test 2: "cond" should find "condition"
	_, total2, _, _, err := repo.Search(ctx, "cond", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total2 == 0 {
		t.Error("expected 'cond' to find posts with 'conditions' (prefix matching)")
	} else {
		t.Logf("Prefix match 'cond' found %d results", total2)
	}
}
