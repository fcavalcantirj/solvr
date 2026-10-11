package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// === Vote Tests ===

func TestPostRepository_Vote_Upvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Upvote Test",
		Description:  "Testing upvote functionality",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_1", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Verify post upvotes increased
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 1 {
		t.Errorf("expected 1 upvote, got %d", updatedPost.Upvotes)
	}

	if updatedPost.Downvotes != 0 {
		t.Errorf("expected 0 downvotes, got %d", updatedPost.Downvotes)
	}

	if updatedPost.VoteScore != 1 {
		t.Errorf("expected vote score 1, got %d", updatedPost.VoteScore)
	}
}

func TestPostRepository_Vote_Downvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Downvote Test",
		Description:  "Testing downvote functionality",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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
	err = repo.Vote(ctx, createdPost.ID, "human", "voter_human_1", "down")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Verify post downvotes increased
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 0 {
		t.Errorf("expected 0 upvotes, got %d", updatedPost.Upvotes)
	}

	if updatedPost.Downvotes != 1 {
		t.Errorf("expected 1 downvote, got %d", updatedPost.Downvotes)
	}

	if updatedPost.VoteScore != -1 {
		t.Errorf("expected vote score -1, got %d", updatedPost.VoteScore)
	}
}

func TestPostRepository_Vote_ChangeVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Change Vote Test",
		Description:  "Testing vote change functionality",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// First upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_change", "up")
	if err != nil {
		t.Fatalf("Vote() upvote error = %v", err)
	}

	// Verify upvote
	post1, _ := repo.FindByID(ctx, createdPost.ID)
	if post1.Upvotes != 1 || post1.Downvotes != 0 {
		t.Errorf("after upvote: expected upvotes=1, downvotes=0, got upvotes=%d, downvotes=%d", post1.Upvotes, post1.Downvotes)
	}

	// Change to downvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_change", "down")
	if err != nil {
		t.Fatalf("Vote() downvote error = %v", err)
	}

	// Verify vote changed: upvotes should decrease, downvotes should increase
	post2, _ := repo.FindByID(ctx, createdPost.ID)
	if post2.Upvotes != 0 || post2.Downvotes != 1 {
		t.Errorf("after change to downvote: expected upvotes=0, downvotes=1, got upvotes=%d, downvotes=%d", post2.Upvotes, post2.Downvotes)
	}

	// Change back to upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_change", "up")
	if err != nil {
		t.Fatalf("Vote() change back error = %v", err)
	}

	// Verify vote changed back
	post3, _ := repo.FindByID(ctx, createdPost.ID)
	if post3.Upvotes != 1 || post3.Downvotes != 0 {
		t.Errorf("after change back to upvote: expected upvotes=1, downvotes=0, got upvotes=%d, downvotes=%d", post3.Upvotes, post3.Downvotes)
	}
}

func TestPostRepository_Vote_MultipleVoters(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Multiple Voters Test",
		Description:  "Testing multiple voters",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Multiple voters upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter1", "up")
	if err != nil {
		t.Fatalf("Vote() voter1 error = %v", err)
	}

	err = repo.Vote(ctx, createdPost.ID, "human", "voter2", "up")
	if err != nil {
		t.Fatalf("Vote() voter2 error = %v", err)
	}

	err = repo.Vote(ctx, createdPost.ID, "agent", "voter3", "down")
	if err != nil {
		t.Fatalf("Vote() voter3 error = %v", err)
	}

	// Verify counts
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 2 {
		t.Errorf("expected 2 upvotes, got %d", updatedPost.Upvotes)
	}

	if updatedPost.Downvotes != 1 {
		t.Errorf("expected 1 downvote, got %d", updatedPost.Downvotes)
	}

	if updatedPost.VoteScore != 1 {
		t.Errorf("expected vote score 1, got %d", updatedPost.VoteScore)
	}
}

func TestPostRepository_Vote_SameVoteTwice(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Same Vote Test",
		Description:  "Testing same vote twice",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "same_voter", "up")
	if err != nil {
		t.Fatalf("Vote() first error = %v", err)
	}

	// Upvote again (same direction) - should not change counts
	err = repo.Vote(ctx, createdPost.ID, "agent", "same_voter", "up")
	if err != nil {
		t.Fatalf("Vote() second error = %v", err)
	}

	// Verify counts unchanged (still 1 upvote)
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 1 {
		t.Errorf("expected 1 upvote (unchanged), got %d", updatedPost.Upvotes)
	}
}

func TestPostRepository_Vote_PostNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to vote on non-existent post
	err := repo.Vote(ctx, "non_existent_post_id", "agent", "voter", "up")
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostRepository_Vote_InvalidDirection(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Invalid Direction Test",
		Description:  "Testing invalid vote direction",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Try invalid direction
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}

	if err != ErrInvalidVoteDirection {
		t.Errorf("expected ErrInvalidVoteDirection, got %v", err)
	}
}

func TestPostRepository_Vote_InvalidVoterType(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Invalid Voter Type Test",
		Description:  "Testing invalid voter type",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Try invalid voter type
	err = repo.Vote(ctx, createdPost.ID, "invalid", "voter", "up")
	if err == nil {
		t.Fatal("expected error for invalid voter type")
	}

	if err != ErrInvalidVoterType {
		t.Errorf("expected ErrInvalidVoterType, got %v", err)
	}
}

// TestPostRepository_Vote_SetsConfirmedTrue verifies that votes are inserted
// with confirmed = true so they count toward reputation calculations.
func TestPostRepository_Vote_SetsConfirmedTrue(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_vote")

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post for Confirmed Vote Test",
		Description:  "Testing that votes are auto-confirmed",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Cast an upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_confirmed", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Query the votes table directly to verify confirmed = true
	var confirmed bool
	err = pool.QueryRow(ctx,
		"SELECT confirmed FROM votes WHERE target_type = 'post' AND target_id = $1 AND voter_id = $2",
		createdPost.ID, "voter_agent_confirmed",
	).Scan(&confirmed)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}

	if !confirmed {
		t.Error("expected vote to be confirmed = true, got false")
	}
}
