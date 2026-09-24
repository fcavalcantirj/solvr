package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Canonical Reply model integration tests (BART-585). They run only when
// DATABASE_URL points at a local test database (setupTestDB skips otherwise).

func TestReplyRepository_CreateAndGet(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	created, err := repo.Create(ctx, &models.Reply{
		PostID:     post.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   user.ID,
		Body:       "This is a canonical reply with a ```code block```.",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created reply has no ID")
	}
	if created.LegacyType != nil {
		t.Errorf("native reply carries legacy_type %v, want nil", *created.LegacyType)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Body != created.Body {
		t.Errorf("body = %q, want %q", got.Body, created.Body)
	}
	if got.Author.ID != user.ID {
		t.Errorf("author id = %q, want %q", got.Author.ID, user.ID)
	}
}

func TestReplyRepository_CreateOnMissingPost(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewReplyRepository(pool)
	_, err := repo.Create(context.Background(), &models.Reply{
		PostID:     "00000000-0000-0000-0000-000000000000",
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   "someone",
		Body:       "orphan reply",
	})
	if err != ErrReplyPostNotFound {
		t.Fatalf("err = %v, want ErrReplyPostNotFound", err)
	}
}

func TestReplyRepository_ListOrdersOldestFirstAndExcludesDeleted(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	var ids []string
	for _, body := range []string{"first", "second", "third"} {
		r, err := repo.Create(ctx, &models.Reply{
			PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: body,
		})
		if err != nil {
			t.Fatalf("Create %q failed: %v", body, err)
		}
		ids = append(ids, r.ID)
	}

	list, total, err := repo.ListByPost(ctx, models.ReplyListOptions{PostID: post.ID})
	if err != nil {
		t.Fatalf("ListByPost failed: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("total=%d len=%d, want 3/3", total, len(list))
	}
	if list[0].Body != "first" || list[2].Body != "third" {
		t.Errorf("order = %q..%q, want first..third (oldest first)", list[0].Body, list[2].Body)
	}

	// Soft-delete the middle reply; it disappears from the list and count.
	if err := repo.Delete(ctx, ids[1], models.AuthorTypeHuman, user.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, total, err = repo.ListByPost(ctx, models.ReplyListOptions{PostID: post.ID})
	if err != nil {
		t.Fatalf("ListByPost after delete failed: %v", err)
	}
	if total != 2 {
		t.Errorf("total after delete = %d, want 2", total)
	}
}

func TestReplyRepository_UpdateAuthorOnly(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	created, err := repo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: "original",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// A non-author cannot edit.
	if _, err := repo.Update(ctx, created.ID, models.AuthorTypeAgent, "intruder", "hacked"); err != ErrReplyForbidden {
		t.Fatalf("non-author Update err = %v, want ErrReplyForbidden", err)
	}

	// The author can edit; identity and creation time are preserved.
	updated, err := repo.Update(ctx, created.ID, models.AuthorTypeHuman, user.ID, "edited body")
	if err != nil {
		t.Fatalf("author Update failed: %v", err)
	}
	if updated.Body != "edited body" {
		t.Errorf("body = %q, want edited body", updated.Body)
	}
	if updated.AuthorID != user.ID || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Error("edit reset author identity or creation time")
	}
}

func TestReplyRepository_DeleteAuthorOnly(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	created, err := repo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: "to delete",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repo.Delete(ctx, created.ID, models.AuthorTypeAgent, "intruder"); err != ErrReplyForbidden {
		t.Fatalf("non-author Delete err = %v, want ErrReplyForbidden", err)
	}
	if err := repo.Delete(ctx, created.ID, models.AuthorTypeHuman, user.ID); err != nil {
		t.Fatalf("author Delete failed: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); err != models.ErrReplyNotFound {
		t.Fatalf("GetByID after delete err = %v, want ErrReplyNotFound", err)
	}
}

func TestReplyRepository_Threading(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)
	otherPost := createCommentTestPost(t, pool, user.ID)

	parent, err := repo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: "parent",
	})
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	child, err := repo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID,
		Body: "child", ParentReplyID: &parent.ID,
	})
	if err != nil {
		t.Fatalf("Create child failed: %v", err)
	}
	if child.ParentReplyID == nil || *child.ParentReplyID != parent.ID {
		t.Error("child did not retain parent_reply_id")
	}

	// A parent from a different post is rejected (no cross-post threading).
	_, err = repo.Create(ctx, &models.Reply{
		PostID: otherPost.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID,
		Body: "cross-post child", ParentReplyID: &parent.ID,
	})
	if err != ErrParentReplyInvalid {
		t.Fatalf("cross-post parent err = %v, want ErrParentReplyInvalid", err)
	}
}

func TestReplyRepository_VoteTargetsReplyIdentity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	created, err := repo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: "vote me",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Vote(ctx, created.ID, "human", user.ID, "up"); err != nil {
		t.Fatalf("Vote up failed: %v", err)
	}
	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Upvotes != 1 || got.Score != 1 {
		t.Errorf("after upvote: upvotes=%d score=%d, want 1/1", got.Upvotes, got.Score)
	}

	// The vote row is stored against the canonical reply identity.
	var targetType string
	if err := pool.QueryRow(ctx,
		"SELECT target_type FROM votes WHERE target_id = $1 AND voter_id = $2", created.ID, user.ID,
	).Scan(&targetType); err != nil {
		t.Fatalf("vote row lookup failed: %v", err)
	}
	if targetType != "reply" {
		t.Errorf("vote target_type = %q, want reply", targetType)
	}

	// Switching to down flips the counters.
	if err := repo.Vote(ctx, created.ID, "human", user.ID, "down"); err != nil {
		t.Fatalf("Vote down failed: %v", err)
	}
	got, _ = repo.GetByID(ctx, created.ID)
	if got.Upvotes != 0 || got.Downvotes != 1 || got.Score != -1 {
		t.Errorf("after downvote: up=%d down=%d score=%d, want 0/1/-1", got.Upvotes, got.Downvotes, got.Score)
	}

	dir, err := repo.GetUserVote(ctx, created.ID, "human", user.ID)
	if err != nil {
		t.Fatalf("GetUserVote failed: %v", err)
	}
	if dir == nil || *dir != "down" {
		t.Errorf("GetUserVote = %v, want down", dir)
	}
}

func TestReplyRepository_ReportCanTargetReply(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	replyRepo := NewReplyRepository(pool)
	reportRepo := NewReportsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	created, err := replyRepo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: "report me",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	report, err := reportRepo.Create(ctx, &models.Report{
		TargetType:   models.ReportTargetReply,
		TargetID:     created.ID,
		ReporterType: models.AuthorTypeHuman,
		ReporterID:   user.ID,
		Reason:       models.ReportReasonSpam,
	})
	if err != nil {
		t.Fatalf("report targeting reply failed: %v", err)
	}
	if report.TargetType != models.ReportTargetReply {
		t.Errorf("report target = %q, want reply", report.TargetType)
	}
}

func TestReplyRepository_PreservesProvenance(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	legacyType := string(models.ReplyLegacyApproach)
	legacyID := uuid.NewString()
	prov, _ := json.Marshal(map[string]string{"status": "succeeded", "angle": "cache the tokens"})

	created, err := repo.Create(ctx, &models.Reply{
		PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID,
		Body: "migrated approach body", LegacyType: &legacyType, LegacyID: &legacyID,
		Provenance: prov,
	})
	if err != nil {
		t.Fatalf("Create with provenance failed: %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.LegacyType == nil || *got.LegacyType != legacyType {
		t.Errorf("legacy_type = %v, want %q", got.LegacyType, legacyType)
	}
	if len(got.Provenance) == 0 {
		t.Error("provenance not preserved")
	}
}
