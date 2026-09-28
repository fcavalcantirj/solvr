package services

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
)

var _ ContentDuplicateFinder = (*db.ContentDuplicateRepository)(nil)

// Task idx 76 step 3 (feature:duplicate-detection), end to end: the moderation service
// with the real finder flags a repeated post and a repeated reply read from the database,
// and leaves the originals alone. Requires DATABASE_URL (a local test database).
func TestModerationService_DuplicateFinderOnTheDatabase(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	author := "agent_moddup_" + time.Now().Format("150405.000000")
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $2, $3, 'active')`, author, author, "hash_"+author); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()

	title := "Moderation duplicate probe " + uuid.NewString()
	desc := "A description long enough to pass every spam rule, posted twice " + uuid.NewString()
	insertPost := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
			VALUES ('post', $1, $2, ARRAY['dup'], 'open', 'agent', $3) RETURNING id`,
			title, desc, author).Scan(&id); err != nil {
			t.Fatalf("seed post: %v", err)
		}
		return id
	}
	firstPost, secondPost := insertPost(), insertPost()

	body := "Same reply twice " + uuid.NewString()
	insertReply := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body)
			VALUES ($1, 'agent', $2, $3) RETURNING id`, firstPost, author, body).Scan(&id); err != nil {
			t.Fatalf("seed reply: %v", err)
		}
		return id
	}
	firstReply := insertReply()
	time.Sleep(5 * time.Millisecond) // created_at orders the original before the repeat
	secondReply := insertReply()

	flags := &MockFlagCreator{}
	svc := NewModerationService(flags, nil, nil, nil)
	svc.SetDuplicateFinder(db.NewContentDuplicateRepository(pool))

	postContent := ModerationContent{Title: title, Description: desc}
	replyContent := ModerationContent{Description: body, PostID: firstPost.String()}
	if err := svc.AutoFlagIfNeeded(ctx, secondPost, "post", postContent); err != nil {
		t.Fatalf("flag repeated post: %v", err)
	}
	if err := svc.AutoFlagIfNeeded(ctx, secondReply, "reply", replyContent); err != nil {
		t.Fatalf("flag repeated reply: %v", err)
	}
	if len(flags.CreatedFlags) != 2 {
		t.Fatalf("want 2 duplicate flags, got %d", len(flags.CreatedFlags))
	}
	if f := flags.CreatedFlags[0]; f.TargetType != "post" || f.TargetID != secondPost ||
		f.Details != "duplicate of post "+firstPost.String() {
		t.Errorf("unexpected post flag %+v", f)
	}
	wantReply := "duplicate of reply " + firstReply.String() + " on post " + firstPost.String()
	if f := flags.CreatedFlags[1]; f.TargetType != "reply" || f.TargetID != secondReply || f.Details != wantReply {
		t.Errorf("unexpected reply flag %+v, want details %q", f, wantReply)
	}
}
