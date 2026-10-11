package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// === GetUserVote Tests ===

func TestPostRepository_GetUserVote_NoVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_getuservote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for GetUserVote Test - No Vote",
		Description:  "Testing GetUserVote when user hasn't voted",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Get vote when user hasn't voted
	vote, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user123")
	if err != nil {
		t.Fatalf("GetUserVote() error = %v", err)
	}

	if vote != nil {
		t.Errorf("expected nil vote, got %v", *vote)
	}
}

func TestPostRepository_GetUserVote_Upvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_getuservote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for GetUserVote Test - Upvote",
		Description:  "Testing GetUserVote after upvoting",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_id = $1", createdPost.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Vote upvote
	err = repo.Vote(ctx, createdPost.ID, "human", "user456", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Get user's vote
	vote, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user456")
	if err != nil {
		t.Fatalf("GetUserVote() error = %v", err)
	}

	if vote == nil {
		t.Fatal("expected vote to be 'up', got nil")
	}

	if *vote != "up" {
		t.Errorf("expected vote 'up', got %s", *vote)
	}
}

func TestPostRepository_GetUserVote_Downvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_getuservote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for GetUserVote Test - Downvote",
		Description:  "Testing GetUserVote after downvoting",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_id = $1", createdPost.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Vote downvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "agent789", "down")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Get user's vote
	vote, err := repo.GetUserVote(ctx, createdPost.ID, "agent", "agent789")
	if err != nil {
		t.Fatalf("GetUserVote() error = %v", err)
	}

	if vote == nil {
		t.Fatal("expected vote to be 'down', got nil")
	}

	if *vote != "down" {
		t.Errorf("expected vote 'down', got %s", *vote)
	}
}

func TestPostRepository_GetUserVote_ChangeVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for GetUserVote Test - Change Vote",
		Description:  "Testing GetUserVote after changing vote",
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   authorHuman(ctx, t, pool, "test_human_getuservote"),
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_id = $1", createdPost.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Initial upvote
	err = repo.Vote(ctx, createdPost.ID, "human", "changer_user", "up")
	if err != nil {
		t.Fatalf("Vote() upvote error = %v", err)
	}

	// Verify upvote
	vote1, _ := repo.GetUserVote(ctx, createdPost.ID, "human", "changer_user")
	if vote1 == nil || *vote1 != "up" {
		t.Errorf("expected 'up', got %v", vote1)
	}

	// Change to downvote
	err = repo.Vote(ctx, createdPost.ID, "human", "changer_user", "down")
	if err != nil {
		t.Fatalf("Vote() downvote error = %v", err)
	}

	// Verify vote changed
	vote2, _ := repo.GetUserVote(ctx, createdPost.ID, "human", "changer_user")
	if vote2 == nil || *vote2 != "down" {
		t.Errorf("expected 'down', got %v", vote2)
	}
}

func TestPostRepository_GetUserVote_DifferentUsers(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_getuservote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for GetUserVote Test - Different Users",
		Description:  "Testing GetUserVote with multiple users",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_id = $1", createdPost.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// User1 upvotes
	err = repo.Vote(ctx, createdPost.ID, "human", "user_a", "up")
	if err != nil {
		t.Fatalf("Vote() user_a error = %v", err)
	}

	// User2 downvotes
	err = repo.Vote(ctx, createdPost.ID, "agent", "user_b", "down")
	if err != nil {
		t.Fatalf("Vote() user_b error = %v", err)
	}

	// User3 doesn't vote

	// Get user_a's vote
	voteA, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user_a")
	if err != nil {
		t.Fatalf("GetUserVote() user_a error = %v", err)
	}
	if voteA == nil || *voteA != "up" {
		t.Errorf("user_a: expected 'up', got %v", voteA)
	}

	// Get user_b's vote
	voteB, err := repo.GetUserVote(ctx, createdPost.ID, "agent", "user_b")
	if err != nil {
		t.Fatalf("GetUserVote() user_b error = %v", err)
	}
	if voteB == nil || *voteB != "down" {
		t.Errorf("user_b: expected 'down', got %v", voteB)
	}

	// Get user_c's vote (hasn't voted)
	voteC, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user_c")
	if err != nil {
		t.Fatalf("GetUserVote() user_c error = %v", err)
	}
	if voteC != nil {
		t.Errorf("user_c: expected nil, got %v", *voteC)
	}
}

func TestPostRepository_GetUserVote_PostNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to get vote for non-existent post
	_, err := repo.GetUserVote(ctx, "non_existent_post_id", "human", "user999")
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostRepository_GetUserVote_InvalidUUID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try with invalid UUID
	_, err := repo.GetUserVote(ctx, "not-a-uuid", "human", "user999")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}
