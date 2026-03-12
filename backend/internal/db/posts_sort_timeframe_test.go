package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestPostRepository_List_SortByHot tests that sort="hot" (trending) works without SQL errors
// and favors recent posts with high vote scores.
func TestPostRepository_List_SortByHot(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a recent post with high votes
	postRecent, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Recent Hot Post",
		Description:  "Recent post with many votes",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_hot",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_hot_test"},
	})
	if err != nil {
		t.Fatalf("failed to create recent post: %v", err)
	}

	// Create an old post with high votes
	postOld, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Old High Votes Post",
		Description:  "Old post with many votes",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_hot",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_hot_test"},
	})
	if err != nil {
		t.Fatalf("failed to create old post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", postRecent.ID, postOld.ID)
	}()

	// Recent post: 5 upvotes, created now (default)
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 5, downvotes = 0 WHERE id = $1", postRecent.ID)
	if err != nil {
		t.Fatalf("failed to update recent post votes: %v", err)
	}

	// Old post: 10 upvotes, but created 30 days ago
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 10, downvotes = 0, created_at = $2 WHERE id = $1",
		postOld.ID, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to update old post: %v", err)
	}

	// Execute: List with sort="hot"
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Sort:    "hot",
		Tags:    []string{"sort_hot_test"},
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() with sort=hot error = %v", err)
	}

	if len(posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(posts))
	}

	// The recent post should rank higher than the old one despite fewer total votes
	var foundRecent, foundOld int = -1, -1
	for i, post := range posts {
		if post.ID == postRecent.ID {
			foundRecent = i
		} else if post.ID == postOld.ID {
			foundOld = i
		}
	}

	if foundRecent == -1 || foundOld == -1 {
		t.Fatalf("not all test posts found: recent=%d, old=%d", foundRecent, foundOld)
	}

	// Recent post (5 votes, today) should appear before old post (10 votes, 30 days ago)
	if foundRecent > foundOld {
		t.Errorf("recent hot post should rank higher than old post: recent at %d, old at %d", foundRecent, foundOld)
	}
}

// TestPostRepository_List_SortByNew tests that sort="new" (frontend alias for newest) works.
func TestPostRepository_List_SortByNew(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Just verify the query doesn't error out
	_, _, err := repo.List(ctx, models.PostListOptions{
		Sort:    "new",
		PerPage: 5,
	})
	if err != nil {
		t.Fatalf("List() with sort=new error = %v", err)
	}
}

// TestPostRepository_List_TimeframeToday tests that timeframe="today" filters to last 24h.
func TestPostRepository_List_TimeframeToday(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post now (should be included)
	postToday, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Today Post",
		Description:  "Created today",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_timeframe",
		Status:       models.PostStatusOpen,
		Tags:         []string{"timeframe_test"},
	})
	if err != nil {
		t.Fatalf("failed to create today post: %v", err)
	}

	// Create a post 3 days ago (should be excluded)
	postOld, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Old Post",
		Description:  "Created 3 days ago",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_timeframe",
		Status:       models.PostStatusOpen,
		Tags:         []string{"timeframe_test"},
	})
	if err != nil {
		t.Fatalf("failed to create old post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", postToday.ID, postOld.ID)
	}()

	// Backdate the old post
	_, err = pool.Exec(ctx, "UPDATE posts SET created_at = $2 WHERE id = $1",
		postOld.ID, time.Now().Add(-3*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to backdate old post: %v", err)
	}

	// Execute: List with timeframe="today"
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Timeframe: "today",
		Tags:      []string{"timeframe_test"},
		PerPage:   50,
	})
	if err != nil {
		t.Fatalf("List() with timeframe=today error = %v", err)
	}

	// The today post should be included
	var foundToday, foundOld bool
	for _, post := range posts {
		if post.ID == postToday.ID {
			foundToday = true
		}
		if post.ID == postOld.ID {
			foundOld = true
		}
	}

	if !foundToday {
		t.Error("expected today's post to be included in timeframe=today results")
	}
	if foundOld {
		t.Error("expected old post (3 days ago) to be excluded from timeframe=today results")
	}
}

// TestPostRepository_List_TimeframeWeek tests that timeframe="week" filters to last 7 days.
func TestPostRepository_List_TimeframeWeek(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a recent post (3 days ago — included)
	postRecent, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Recent Week Post",
		Description:  "Created 3 days ago",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_timeframe_w",
		Status:       models.PostStatusOpen,
		Tags:         []string{"timeframe_week_test"},
	})
	if err != nil {
		t.Fatalf("failed to create recent post: %v", err)
	}

	// Create an old post (14 days ago — excluded)
	postOld, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Old Week Post",
		Description:  "Created 14 days ago",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_timeframe_w",
		Status:       models.PostStatusOpen,
		Tags:         []string{"timeframe_week_test"},
	})
	if err != nil {
		t.Fatalf("failed to create old post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", postRecent.ID, postOld.ID)
	}()

	// Backdate posts
	_, err = pool.Exec(ctx, "UPDATE posts SET created_at = $2 WHERE id = $1",
		postRecent.ID, time.Now().Add(-3*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to backdate recent post: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE posts SET created_at = $2 WHERE id = $1",
		postOld.ID, time.Now().Add(-14*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to backdate old post: %v", err)
	}

	// Execute
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Timeframe: "week",
		Tags:      []string{"timeframe_week_test"},
		PerPage:   50,
	})
	if err != nil {
		t.Fatalf("List() with timeframe=week error = %v", err)
	}

	var foundRecent, foundOld bool
	for _, post := range posts {
		if post.ID == postRecent.ID {
			foundRecent = true
		}
		if post.ID == postOld.ID {
			foundOld = true
		}
	}

	if !foundRecent {
		t.Error("expected recent post (3 days) to be included in timeframe=week results")
	}
	if foundOld {
		t.Error("expected old post (14 days) to be excluded from timeframe=week results")
	}
}

// TestPostRepository_List_TimeframeMonth tests that timeframe="month" filters to last 30 days.
func TestPostRepository_List_TimeframeMonth(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a recent post (15 days ago — included)
	postRecent, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Recent Month Post",
		Description:  "Created 15 days ago",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_timeframe_m",
		Status:       models.PostStatusOpen,
		Tags:         []string{"timeframe_month_test"},
	})
	if err != nil {
		t.Fatalf("failed to create recent post: %v", err)
	}

	// Create an old post (45 days ago — excluded)
	postOld, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Old Month Post",
		Description:  "Created 45 days ago",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_timeframe_m",
		Status:       models.PostStatusOpen,
		Tags:         []string{"timeframe_month_test"},
	})
	if err != nil {
		t.Fatalf("failed to create old post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", postRecent.ID, postOld.ID)
	}()

	// Backdate posts
	_, err = pool.Exec(ctx, "UPDATE posts SET created_at = $2 WHERE id = $1",
		postRecent.ID, time.Now().Add(-15*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to backdate recent post: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE posts SET created_at = $2 WHERE id = $1",
		postOld.ID, time.Now().Add(-45*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to backdate old post: %v", err)
	}

	// Execute
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Timeframe: "month",
		Tags:      []string{"timeframe_month_test"},
		PerPage:   50,
	})
	if err != nil {
		t.Fatalf("List() with timeframe=month error = %v", err)
	}

	var foundRecent, foundOld bool
	for _, post := range posts {
		if post.ID == postRecent.ID {
			foundRecent = true
		}
		if post.ID == postOld.ID {
			foundOld = true
		}
	}

	if !foundRecent {
		t.Error("expected recent post (15 days) to be included in timeframe=month results")
	}
	if foundOld {
		t.Error("expected old post (45 days) to be excluded from timeframe=month results")
	}
}
