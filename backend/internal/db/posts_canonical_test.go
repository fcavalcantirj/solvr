package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestPostRepository_Canonical_PublicPendingNotEligible verifies a public post created
// pending moderation reads back as draft+pending and is not publicly eligible (BART-583).
func TestPostRepository_Canonical_PublicPendingNotEligible(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewPostRepository(pool)
	user := createCommentTestUser(t, pool)

	created, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Canonical pending post " + time.Now().Format("150405"),
		Description:  "A canonical untyped post created pending moderation for eligibility testing.",
		Tags:         []string{"canonical_pending_test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusPendingReview,
		Visibility:   models.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.PublicationState != models.PublicationDraft || created.ModerationState != models.ModerationPending {
		t.Fatalf("create returned (%q,%q), want (draft,pending)", created.PublicationState, created.ModerationState)
	}

	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if got.PublicationState != models.PublicationDraft || got.ModerationState != models.ModerationPending {
		t.Errorf("read back (%q,%q), want (draft,pending)", got.PublicationState, got.ModerationState)
	}
	if got.Post.PublicEligible() {
		t.Error("a pending post must not be publicly eligible")
	}
}

// TestPostRepository_Canonical_ApprovedEligible verifies an approved public post reads
// back as published+approved and is publicly eligible (BART-583).
func TestPostRepository_Canonical_ApprovedEligible(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewPostRepository(pool)
	user := createCommentTestUser(t, pool)

	created, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Canonical approved post " + time.Now().Format("150405"),
		Description:  "A canonical untyped post that is open/approved for eligibility testing here.",
		Tags:         []string{"canonical_approved_test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusOpen,
		Visibility:   models.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if got.PublicationState != models.PublicationPublished || got.ModerationState != models.ModerationApproved {
		t.Errorf("read back (%q,%q), want (published,approved)", got.PublicationState, got.ModerationState)
	}
	if !got.Post.PublicEligible() {
		t.Error("an open, approved, public post must be publicly eligible")
	}
}

// TestPostRepository_Canonical_UpdateStatusSyncsStates verifies moderation approval via
// UpdateStatus advances the canonical states so a pending post becomes eligible (BART-583).
func TestPostRepository_Canonical_UpdateStatusSyncsStates(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewPostRepository(pool)
	user := createCommentTestUser(t, pool)

	created, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Canonical moderation flow " + time.Now().Format("150405"),
		Description:  "A canonical untyped post that starts pending and gets approved by moderation.",
		Tags:         []string{"canonical_modflow_test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusPendingReview,
		Visibility:   models.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.UpdateStatus(ctx, created.ID, models.PostStatusOpen); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if got.PublicationState != models.PublicationPublished || got.ModerationState != models.ModerationApproved {
		t.Errorf("after approval (%q,%q), want (published,approved)", got.PublicationState, got.ModerationState)
	}
	if !got.Post.PublicEligible() {
		t.Error("post approved via UpdateStatus must become publicly eligible")
	}
}

// TestPostRepository_Canonical_ReplyCount verifies reply_count is the unified sum of
// answers, approaches, and comments in both List and FindByID (BART-583).
func TestPostRepository_Canonical_ReplyCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewPostRepository(pool)
	commentRepo := NewCommentsRepository(pool)

	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	for i := 0; i < 2; i++ {
		if _, err := commentRepo.Create(ctx, &models.Comment{
			TargetType: models.CommentTargetPost,
			TargetID:   post.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   user.ID,
			Content:    "A reply-count test comment",
		}); err != nil {
			t.Fatalf("failed to create comment: %v", err)
		}
	}

	// The replies the contribution cutover makes from these legacy rows (task idx 76): post
	// counts are read from replies.
	cutoverRepliesFor(t, pool, ctx, post.ID)

	got, err := repo.FindByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	wantSum := got.AnswersCount + got.ApproachesCount + got.CommentsCount
	if got.ReplyCount != wantSum {
		t.Errorf("FindByID ReplyCount = %d, want %d (answers+approaches+comments)", got.ReplyCount, wantSum)
	}
	if got.CommentsCount < 2 {
		t.Fatalf("expected at least 2 comments counted, got %d", got.CommentsCount)
	}
	if got.ReplyCount < 2 {
		t.Errorf("ReplyCount = %d, want >= 2", got.ReplyCount)
	}
}
