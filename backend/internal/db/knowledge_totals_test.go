package db

import (
	"context"
	"testing"
)

// Task idx 72 step 3: the homepage knowledge aggregates (per post type) are read ONCE, by
// GetKnowledgeTotals, with the same visibility rule an anonymous GET /v1/posts applies:
// public, not deleted, not pending_review / rejected / draft. Reply figures come from the
// canonical replies table, so a native canonical reply counts.

func knowledgeTotalsFor(t *testing.T, repo *StatsRepository, postType string) KnowledgeTypeTotals {
	t.Helper()
	all, err := repo.GetKnowledgeTotals(context.Background())
	if err != nil {
		t.Fatalf("GetKnowledgeTotals() error = %v", err)
	}
	for _, k := range all {
		if k.Type == postType {
			return k
		}
	}
	t.Fatalf("GetKnowledgeTotals() has no %q entry: %+v", postType, all)
	return KnowledgeTypeTotals{}
}

func insertKnowledgeReply(t *testing.T, pool *Pool, postID string, deleted bool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO replies (post_id, author_type, author_id, body, deleted_at)
		VALUES ($1, 'human', $3, 'Knowledge totals reply fixture.',
		        CASE WHEN $2 THEN NOW() ELSE NULL END)
		RETURNING id::text
	`, postID, deleted, authorHuman(context.Background(), t, pool, "knowledge-user")).Scan(&id)
	if err != nil {
		t.Fatalf("insertKnowledgeReply(%s): %v", postID, err)
	}
	return id
}

// One entry, post: the per-type entries were retired with the legacy post types (idx 68).
func TestGetKnowledgeTotals_ListsEveryKnowledgeTypeInOrder(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)

	all, err := NewStatsRepository(pool).GetKnowledgeTotals(context.Background())
	if err != nil {
		t.Fatalf("GetKnowledgeTotals() error = %v", err)
	}
	var types []string
	for _, k := range all {
		types = append(types, k.Type)
		if k.ByStatus == nil {
			t.Errorf("%s: by_status must be an empty map, never nil", k.Type)
		}
	}
	want := []string{"post"}
	if len(types) != len(want) {
		t.Fatalf("types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("types = %v, want %v", types, want)
		}
	}
}

func TestGetKnowledgeTotals_CountsWhatAnAnonymousListShows(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	repo := NewStatsRepository(pool)
	ctx := context.Background()

	before := knowledgeTotalsFor(t, repo, "post")

	open := insertVisibilityTestPost(t, pool, ctx, "post", "Knowledge open post", "open", "public", 0)
	closed := insertVisibilityTestPost(t, pool, ctx, "post", "Knowledge closed post", "closed", "public", 0)
	// None of these is visible to an anonymous list, so none may count.
	insertVisibilityTestPost(t, pool, ctx, "post", "Knowledge family post", "open", "family", 0)
	insertVisibilityTestPost(t, pool, ctx, "post", "Knowledge draft post", "draft", "public", 0)
	insertVisibilityTestPost(t, pool, ctx, "post", "Knowledge pending post", "pending_review", "public", 0)
	deleted := insertVisibilityTestPost(t, pool, ctx, "post", "Knowledge deleted post", "open", "public", 0)
	if _, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1", deleted); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	insertKnowledgeReply(t, pool, open, false)
	insertKnowledgeReply(t, pool, open, false)
	insertKnowledgeReply(t, pool, open, true) // deleted reply: not counted
	insertKnowledgeReply(t, pool, closed, false)
	insertKnowledgeReply(t, pool, deleted, false) // reply on a deleted post: not counted

	after := knowledgeTotalsFor(t, repo, "post")

	if got, want := after.Total, before.Total+2; got != want {
		t.Errorf("Total = %d, want %d (only the two visible posts)", got, want)
	}
	if got, want := after.ByStatus["open"], before.ByStatus["open"]+1; got != want {
		t.Errorf("ByStatus[open] = %d, want %d", got, want)
	}
	if got, want := after.ByStatus["closed"], before.ByStatus["closed"]+1; got != want {
		t.Errorf("ByStatus[closed] = %d, want %d", got, want)
	}
	for _, hidden := range []string{"draft", "pending_review"} {
		if after.ByStatus[hidden] != before.ByStatus[hidden] {
			t.Errorf("ByStatus[%s] moved: %d -> %d", hidden, before.ByStatus[hidden], after.ByStatus[hidden])
		}
	}
	if got, want := after.WithReplies, before.WithReplies+2; got != want {
		t.Errorf("WithReplies = %d, want %d", got, want)
	}
	if got, want := after.Replies, before.Replies+3; got != want {
		t.Errorf("Replies = %d, want %d (deleted replies and replies on deleted posts excluded)", got, want)
	}
	sum := 0
	for _, n := range after.ByStatus {
		sum += n
	}
	if sum != after.Total {
		t.Errorf("sum(ByStatus) = %d, Total = %d: the status split must add up", sum, after.Total)
	}
}
