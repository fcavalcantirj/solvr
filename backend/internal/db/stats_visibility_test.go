package db

import (
	"context"
	"testing"
)

// These tests pin the analytics invariant: every PUBLIC statistic Solvr reports is
// computed live from the database AND permission-filtered. Family-scoped posts
// (BART-151 visibility = 'family') belong to one human's family and must never be
// counted in, or exposed through, the unauthenticated /v1/stats* endpoints.
//
// Research baselines (room/post/contribution totals gathered during planning) are
// deliberately NOT encoded here — public numbers come from the current database,
// never from a snapshot.

// insertVisibilityTestPost inserts a post with an explicit visibility tier and
// upvote count, and removes it when the test ends.
func insertVisibilityTestPost(t *testing.T, pool *Pool, ctx context.Context, postType, title, status, visibility string, upvotes int) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, visibility, upvotes)
		VALUES ($1, $2, $3, ARRAY[]::text[], $4, 'human', $7, $5, $6)
		RETURNING id::text
	`, postType, title, "Visibility fixture for public stats tests.", status, visibility, upvotes, testUser(ctx, t, pool)).Scan(&id)
	if err != nil {
		t.Fatalf("insertVisibilityTestPost(%s, %s): %v", postType, visibility, err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", id); err != nil {
			t.Logf("cleanup of post %s failed: %v", id, err)
		}
	})
	return id
}

// TestGetAllStats_ExcludesFamilyPrivatePosts verifies the /v1/stats aggregate counts
// only public posts: adding one public post moves the counters by exactly one, and
// adding a family-private post moves them not at all.
func TestGetAllStats_ExcludesFamilyPrivatePosts(t *testing.T) {
	pool := setupTestDB(t)
	// Registered first so it runs LAST (t.Cleanup is LIFO): the per-post
	// deletes below must run against an open pool.
	t.Cleanup(pool.Close)

	repo := NewCanonicalStatsRepository(pool)
	ctx := context.Background()

	before, err := repo.GetAllStats(ctx)
	if err != nil {
		t.Fatalf("GetAllStats() baseline error = %v", err)
	}

	insertVisibilityTestPost(t, pool, ctx, "problem", "Public problem counted in public stats", "open", "public", 0)

	afterPublic, err := repo.GetAllStats(ctx)
	if err != nil {
		t.Fatalf("GetAllStats() after public insert error = %v", err)
	}
	if got, want := afterPublic.TotalPosts, before.TotalPosts+1; got != want {
		t.Fatalf("TotalPosts after public post = %d, want %d", got, want)
	}
	if got, want := afterPublic.ActivePosts, before.ActivePosts+1; got != want {
		t.Fatalf("ActivePosts after public post = %d, want %d", got, want)
	}
	if got, want := afterPublic.PostedToday, before.PostedToday+1; got != want {
		t.Fatalf("PostedToday after public post = %d, want %d", got, want)
	}

	insertVisibilityTestPost(t, pool, ctx, "problem", "Family problem must stay out of public stats", "open", "family", 0)

	afterFamily, err := repo.GetAllStats(ctx)
	if err != nil {
		t.Fatalf("GetAllStats() after family insert error = %v", err)
	}
	if got, want := afterFamily.TotalPosts, afterPublic.TotalPosts; got != want {
		t.Errorf("TotalPosts leaked a family-private post: got %d, want %d", got, want)
	}
	if got, want := afterFamily.ActivePosts, afterPublic.ActivePosts; got != want {
		t.Errorf("ActivePosts leaked a family-private post: got %d, want %d", got, want)
	}
	if got, want := afterFamily.PostedToday, afterPublic.PostedToday; got != want {
		t.Errorf("PostedToday leaked a family-private post: got %d, want %d", got, want)
	}
}

// TestGetAllStats_ReflectsCurrentDatabaseState verifies the public totals are computed
// from the live database on every call — no snapshot, no cached research number.
// Soft-deleting the post must take the counter back down.
func TestGetAllStats_ReflectsCurrentDatabaseState(t *testing.T) {
	pool := setupTestDB(t)
	// Registered first so it runs LAST (t.Cleanup is LIFO): the per-post
	// deletes below must run against an open pool.
	t.Cleanup(pool.Close)

	repo := NewCanonicalStatsRepository(pool)
	ctx := context.Background()

	before, err := repo.GetAllStats(ctx)
	if err != nil {
		t.Fatalf("GetAllStats() baseline error = %v", err)
	}

	id := insertVisibilityTestPost(t, pool, ctx, "idea", "Live counter fixture idea", "open", "public", 0)

	after, err := repo.GetAllStats(ctx)
	if err != nil {
		t.Fatalf("GetAllStats() after insert error = %v", err)
	}
	if got, want := after.TotalPosts, before.TotalPosts+1; got != want {
		t.Fatalf("TotalPosts did not follow the database: got %d, want %d", got, want)
	}

	if _, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1", id); err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}

	afterDelete, err := repo.GetAllStats(ctx)
	if err != nil {
		t.Fatalf("GetAllStats() after delete error = %v", err)
	}
	if got, want := afterDelete.TotalPosts, before.TotalPosts; got != want {
		t.Errorf("TotalPosts after soft delete = %d, want %d (stat is stale, not live)", got, want)
	}
}

// TestPublicPostCounters_ExcludeFamilyPrivatePosts covers the individual counter
// getters behind the public stats endpoints.
func TestPublicPostCounters_ExcludeFamilyPrivatePosts(t *testing.T) {
	pool := setupTestDB(t)
	// Registered first so it runs LAST (t.Cleanup is LIFO): the per-post
	// deletes below must run against an open pool.
	t.Cleanup(pool.Close)

	repo := NewCanonicalStatsRepository(pool)
	ctx := context.Background()

	totalBefore, err := repo.GetTotalPostsCount(ctx)
	if err != nil {
		t.Fatalf("GetTotalPostsCount() error = %v", err)
	}
	activeBefore, err := repo.GetActivePostsCount(ctx)
	if err != nil {
		t.Fatalf("GetActivePostsCount() error = %v", err)
	}
	postedBefore, err := repo.GetPostedTodayCount(ctx)
	if err != nil {
		t.Fatalf("GetPostedTodayCount() error = %v", err)
	}
	insertVisibilityTestPost(t, pool, ctx, "post", "Family open post stays private", "open", "family", 0)
	insertVisibilityTestPost(t, pool, ctx, "post", "Family closed post stays private", "closed", "family", 0)

	totalAfter, err := repo.GetTotalPostsCount(ctx)
	if err != nil {
		t.Fatalf("GetTotalPostsCount() error = %v", err)
	}
	if totalAfter != totalBefore {
		t.Errorf("GetTotalPostsCount counted family-private posts: got %d, want %d", totalAfter, totalBefore)
	}

	activeAfter, err := repo.GetActivePostsCount(ctx)
	if err != nil {
		t.Fatalf("GetActivePostsCount() error = %v", err)
	}
	if activeAfter != activeBefore {
		t.Errorf("GetActivePostsCount counted family-private posts: got %d, want %d", activeAfter, activeBefore)
	}

	postedAfter, err := repo.GetPostedTodayCount(ctx)
	if err != nil {
		t.Fatalf("GetPostedTodayCount() error = %v", err)
	}
	if postedAfter != postedBefore {
		t.Errorf("GetPostedTodayCount counted family-private posts: got %d, want %d", postedAfter, postedBefore)
	}
}
