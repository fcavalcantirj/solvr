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
	want := []string{"problem", "question", "idea", "post"}
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

	before := knowledgeTotalsFor(t, repo, "question")

	open := insertVisibilityTestPost(t, pool, ctx, "question", "Knowledge open question", "open", "public", 0)
	solved := insertVisibilityTestPost(t, pool, ctx, "question", "Knowledge solved question", "solved", "public", 0)
	// None of these is visible to an anonymous list, so none may count.
	insertVisibilityTestPost(t, pool, ctx, "question", "Knowledge family question", "open", "family", 0)
	insertVisibilityTestPost(t, pool, ctx, "question", "Knowledge draft question", "draft", "public", 0)
	insertVisibilityTestPost(t, pool, ctx, "question", "Knowledge pending question", "pending_review", "public", 0)
	deleted := insertVisibilityTestPost(t, pool, ctx, "question", "Knowledge deleted question", "open", "public", 0)
	if _, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1", deleted); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	insertKnowledgeReply(t, pool, open, false)
	insertKnowledgeReply(t, pool, open, false)
	insertKnowledgeReply(t, pool, open, true) // deleted reply: not counted
	accepted := insertKnowledgeReply(t, pool, solved, false)
	insertKnowledgeReply(t, pool, deleted, false) // reply on a deleted post: not counted
	if _, err := pool.Exec(ctx, "UPDATE posts SET accepted_answer_id = $2 WHERE id = $1", solved, accepted); err != nil {
		t.Fatalf("accept reply: %v", err)
	}

	after := knowledgeTotalsFor(t, repo, "question")

	if got, want := after.Total, before.Total+2; got != want {
		t.Errorf("Total = %d, want %d (only the two visible questions)", got, want)
	}
	if got, want := after.ByStatus["open"], before.ByStatus["open"]+1; got != want {
		t.Errorf("ByStatus[open] = %d, want %d", got, want)
	}
	if got, want := after.ByStatus["solved"], before.ByStatus["solved"]+1; got != want {
		t.Errorf("ByStatus[solved] = %d, want %d", got, want)
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
	if got, want := after.WithAcceptedReply, before.WithAcceptedReply+1; got != want {
		t.Errorf("WithAcceptedReply = %d, want %d", got, want)
	}
	sum := 0
	for _, n := range after.ByStatus {
		sum += n
	}
	if sum != after.Total {
		t.Errorf("sum(ByStatus) = %d, Total = %d: the status split must add up", sum, after.Total)
	}
}
