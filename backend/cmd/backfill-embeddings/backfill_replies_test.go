package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Task idx 76 step 3 (feature:embedding-workers): the backfill embeds canonical replies
// (replies.embedding), the contribution model that replaces answers and approaches.

func (m *mockDB) GetRepliesWithoutEmbedding(ctx context.Context, limit, offset int) ([]replyRow, error) {
	end := offset + limit
	if end > len(m.replies) {
		end = len(m.replies)
	}
	if offset >= len(m.replies) {
		return nil, nil
	}
	return m.replies[offset:end], nil
}

func (m *mockDB) CountRepliesWithoutEmbedding(ctx context.Context) (int, error) {
	return len(m.replies), nil
}

func (m *mockDB) UpdateReplyEmbedding(ctx context.Context, id string, embedding []float32) error {
	m.updateReplyCalls++
	if m.updateReplyErr == nil {
		for i, r := range m.replies {
			if r.ID == id {
				m.replies = append(m.replies[:i], m.replies[i+1:]...)
				break
			}
		}
	}
	return m.updateReplyErr
}

func TestBackfillWorker_RepliesOnly(t *testing.T) {
	mdb := &mockDB{
		posts:   []postRow{{ID: "post-1", Title: "T", Description: "D"}},
		replies: []replyRow{{ID: "rep-1", Body: "first reply body"}, {ID: "rep-2", Body: "second reply body"}},
	}
	embSvc := &mockEmbeddingService{}
	worker := &backfillWorker{db: mdb, embeddingService: embSvc, batchSize: 10, contentTypes: []string{"replies"}}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.repliesFound != 2 || result.repliesEmbedded != 2 || mdb.updateReplyCalls != 2 {
		t.Fatalf("replies found=%d embedded=%d updates=%d, want 2/2/2", result.repliesFound, result.repliesEmbedded, mdb.updateReplyCalls)
	}
	if embSvc.lastInput != "second reply body" {
		t.Errorf("embedding input = %q, want the reply body", embSvc.lastInput)
	}
	if mdb.updateCalls != 0 || len(mdb.posts) != 1 {
		t.Error("a replies-only run must not touch posts")
	}
	if result.embedded != 2 || result.totalFound != 2 {
		t.Errorf("totals embedded=%d found=%d, want 2/2", result.embedded, result.totalFound)
	}
}

func TestBackfillWorker_ReplyErrorsAreCountedAndSkipped(t *testing.T) {
	mdb := &mockDB{replies: []replyRow{{ID: "rep-1", Body: "a"}, {ID: "rep-2", Body: "b"}, {ID: "rep-3", Body: "c"}}}
	calls := 0
	embSvc := &mockEmbeddingService{generateFunc: func(ctx context.Context, text string) ([]float32, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("embedding API error")
		}
		return make([]float32, 1024), nil
	}}
	worker := &backfillWorker{db: mdb, embeddingService: embSvc, batchSize: 10, contentTypes: []string{"replies"}}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.repliesEmbedded != 2 || result.repliesErrors != 1 || result.errors != 1 {
		t.Fatalf("replies embedded=%d errors=%d total errors=%d, want 2/1/1", result.repliesEmbedded, result.repliesErrors, result.errors)
	}
}

func TestBackfillWorker_DryRunCountsReplies(t *testing.T) {
	mdb := &mockDB{replies: []replyRow{{ID: "rep-1", Body: "a"}}}
	embSvc := &mockEmbeddingService{}
	worker := &backfillWorker{db: mdb, embeddingService: embSvc, batchSize: 10, dryRun: true, contentTypes: parseContentTypes("all")}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.repliesFound != 1 || embSvc.callCount != 0 || mdb.updateReplyCalls != 0 {
		t.Fatalf("dry run found=%d calls=%d updates=%d, want 1/0/0", result.repliesFound, embSvc.callCount, mdb.updateReplyCalls)
	}
}

func TestParseContentTypes_Replies(t *testing.T) {
	types := parseContentTypes("replies,posts")
	if len(types) != 2 || types[0] != "replies" || types[1] != "posts" {
		t.Fatalf("parseContentTypes(replies,posts) = %v, want [replies posts]", types)
	}
}

// Integration test for pgBackfillDB's replies queries against a local test database.
// Skipped when DATABASE_URL is not set.
//
//	DATABASE_URL="postgres://solvr:solvr_dev@localhost:5435/solvr_test" go test ./cmd/backfill-embeddings/ -count=1 -v
func TestPGBackfillDB_EmbedsLiveHumanAndAgentRepliesOnly(t *testing.T) {
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

	agentID := fmt.Sprintf("backfill_agent_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name) VALUES ($1, 'Backfill Replies Probe')`, agentID); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	var postID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Backfill replies probe', 'A post whose replies the backfill embeds',
		        'agent', $1, 'open')
		RETURNING id`, agentID).Scan(&postID); err != nil {
		t.Fatalf("seed post: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", postID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM agents WHERE id = $1", agentID)
	})

	seed := func(authorType, authorID, body string, deleted bool) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body, deleted_at, updated_at)
			VALUES ($1, $2, $3, $4, CASE WHEN $5 THEN NOW() END, NOW() - INTERVAL '1 day')
			RETURNING id`, postID, authorType, authorID, body, deleted).Scan(&id); err != nil {
			t.Fatalf("seed reply: %v", err)
		}
		return id
	}
	live := seed("agent", agentID, "a live agent reply", false)
	system := seed("system", "solvr-moderator", "Post approved by Solvr moderation.", false)
	gone := seed("agent", agentID, "a deleted agent reply", true)

	pg := &pgBackfillDB{pool: pool}
	pending := func() map[string]string {
		t.Helper()
		rows, err := pg.GetRepliesWithoutEmbedding(ctx, 1000000, 0)
		if err != nil {
			t.Fatalf("GetRepliesWithoutEmbedding: %v", err)
		}
		got := map[string]string{}
		for _, r := range rows {
			got[r.ID] = r.Body
		}
		return got
	}

	before := pending()
	if before[live] != "a live agent reply" {
		t.Fatalf("the live agent reply is not pending embedding (got %q)", before[live])
	}
	if _, ok := before[system]; ok {
		t.Error("a system moderation reply must not be embedded")
	}
	if _, ok := before[gone]; ok {
		t.Error("a deleted reply must not be embedded")
	}
	var want int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM replies
		WHERE deleted_at IS NULL AND embedding IS NULL AND author_type <> 'system'`).Scan(&want); err != nil {
		t.Fatalf("count: %v", err)
	}
	count, err := pg.CountRepliesWithoutEmbedding(ctx)
	if err != nil || count != want || count != len(before) {
		t.Fatalf("CountRepliesWithoutEmbedding = %d, %v; want %d (= %d listed)", count, err, want, len(before))
	}

	var updatedBefore time.Time
	if err := pool.QueryRow(ctx, "SELECT updated_at FROM replies WHERE id = $1", live).Scan(&updatedBefore); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	vec := make([]float32, 1024)
	vec[3] = 1
	if err := pg.UpdateReplyEmbedding(ctx, live, vec); err != nil {
		t.Fatalf("UpdateReplyEmbedding: %v", err)
	}
	if err := pg.UpdateReplyEmbedding(ctx, gone, vec); err != nil {
		t.Fatalf("UpdateReplyEmbedding(deleted): %v", err)
	}

	var dims int
	var updatedAfter time.Time
	if err := pool.QueryRow(ctx, "SELECT vector_dims(embedding), updated_at FROM replies WHERE id = $1", live).Scan(&dims, &updatedAfter); err != nil {
		t.Fatalf("read embedding: %v", err)
	}
	if dims != 1024 {
		t.Errorf("stored embedding has %d dims, want 1024", dims)
	}
	// updated_at is the reply's ETag validator (If-Match); a backfill must not move it.
	if !updatedAfter.Equal(updatedBefore) {
		t.Errorf("backfill moved updated_at from %v to %v", updatedBefore, updatedAfter)
	}
	var goneEmbedded bool
	if err := pool.QueryRow(ctx, "SELECT embedding IS NOT NULL FROM replies WHERE id = $1", gone).Scan(&goneEmbedded); err != nil {
		t.Fatalf("read deleted reply: %v", err)
	}
	if goneEmbedded {
		t.Error("the backfill wrote an embedding onto a deleted reply")
	}
	if _, ok := pending()[live]; ok {
		t.Error("an embedded reply is still pending")
	}
}
