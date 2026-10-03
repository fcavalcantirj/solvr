package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// Integration test for pgModerationDB against a local test database. Skipped when
// DATABASE_URL is not set.
//
//	DATABASE_URL="postgres://solvr:solvr_dev@localhost:5435/solvr_test" go test ./cmd/moderate-existing/ -count=1 -v
func TestPGModerationDB_CreateSystemCommentRecordsAReply(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	agentID := fmt.Sprintf("modexist_agent_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name) VALUES ($1, 'Moderate Existing Probe')`, agentID); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	var postID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Moderate existing verdict probe', 'A post the moderate-existing tool rejects',
		        'agent', $1, 'open')
		RETURNING id`, agentID).Scan(&postID); err != nil {
		t.Fatalf("seed post: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", postID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM agents WHERE id = $1", agentID)
	})

	verdict := fmt.Sprintf(services.ModerationRejectedFormat, "not in English")
	if err := (&pgModerationDB{pool: pool}).CreateSystemComment(ctx, postID, verdict); err != nil {
		t.Fatalf("CreateSystemComment: %v", err)
	}

	var authorType, authorID, body string
	err = pool.QueryRow(ctx, `
		SELECT author_type, author_id, body FROM replies
		WHERE post_id = $1 AND parent_reply_id IS NULL AND deleted_at IS NULL`, postID,
	).Scan(&authorType, &authorID, &body)
	if err != nil {
		t.Fatalf("the rejection verdict is not a reply on the post: %v", err)
	}
	if authorType != "system" || authorID != services.ModerationAuthorID || body != verdict {
		t.Errorf("reply = %s/%s %q, want system/%s %q", authorType, authorID, body, services.ModerationAuthorID, verdict)
	}

	// No legacy comment can be written: the legacy archive migration (idx 68) moved the
	// comments table out of public.
	var commentsLive bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.comments') IS NOT NULL").Scan(&commentsLive); err != nil {
		t.Fatalf("look up the comments table: %v", err)
	}
	if commentsLive {
		t.Error("the legacy comments table is still live storage")
	}
}
