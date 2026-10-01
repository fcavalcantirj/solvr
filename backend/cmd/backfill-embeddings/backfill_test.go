package main

import (
	"context"
	"errors"
	"testing"
)

// mockEmbeddingService implements services.EmbeddingService for testing.
type mockEmbeddingService struct {
	generateFunc func(ctx context.Context, text string) ([]float32, error)
	callCount    int
	lastInput    string
}

func (m *mockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	m.lastInput = text
	if m.generateFunc != nil {
		return m.generateFunc(ctx, text)
	}
	return make([]float32, 1024), nil
}

func (m *mockEmbeddingService) GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return m.GenerateEmbedding(ctx, text)
}

// mockDB implements the database operations needed by the backfill worker.
type mockDB struct {
	posts            []postRow
	updateCalls      int
	updateErr        error
	replies          []replyRow
	updateReplyCalls int
	updateReplyErr   error
	// changedPosts / changedReplies are rows whose text changed after the read: the write
	// lands nowhere and the row stays pending.
	changedPosts   map[string]bool
	changedReplies map[string]bool
}

func (m *mockDB) GetPostsWithoutEmbedding(ctx context.Context, limit, offset int) ([]postRow, error) {
	end := offset + limit
	if end > len(m.posts) {
		end = len(m.posts)
	}
	if offset >= len(m.posts) {
		return nil, nil
	}
	return m.posts[offset:end], nil
}

func (m *mockDB) CountPostsWithoutEmbedding(ctx context.Context) (int, error) {
	return len(m.posts), nil
}

func (m *mockDB) UpdatePostEmbedding(ctx context.Context, post postRow, embedding []float32) (bool, error) {
	m.updateCalls++
	if m.updateErr != nil {
		return false, m.updateErr
	}
	if m.changedPosts[post.ID] {
		return false, nil
	}
	for i, p := range m.posts {
		if p.ID == post.ID {
			m.posts = append(m.posts[:i], m.posts[i+1:]...)
			break
		}
	}
	return true, nil
}

func TestBackfillWorker_DryRun(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Test Post 1", Description: "Description 1"},
			{ID: "post-2", Title: "Test Post 2", Description: "Description 2"},
			{ID: "post-3", Title: "Test Post 3", Description: "Description 3"},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         2,
		dryRun:            true,
		delayBetweenItems: 0,
		contentTypes:      []string{"posts"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dry run should not generate any embeddings
	if embSvc.callCount != 0 {
		t.Errorf("expected 0 embedding calls in dry run, got %d", embSvc.callCount)
	}

	// Dry run should not update database
	if db.updateCalls != 0 {
		t.Errorf("expected 0 update calls in dry run, got %d", db.updateCalls)
	}

	// Should report correct total count
	if result.totalFound != 3 {
		t.Errorf("expected totalFound=3, got %d", result.totalFound)
	}

	if result.embedded != 0 {
		t.Errorf("expected embedded=0 in dry run, got %d", result.embedded)
	}
}

func TestBackfillWorker_ProcessesBatches(t *testing.T) {
	posts := make([]postRow, 5)
	for i := range posts {
		posts[i] = postRow{
			ID:          "post-" + string(rune('a'+i)),
			Title:       "Title",
			Description: "Desc",
		}
	}

	db := &mockDB{posts: posts}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         2,
		dryRun:            false,
		delayBetweenItems: 0, // no delay in tests
		contentTypes:      []string{"posts"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.embedded != 5 {
		t.Errorf("expected embedded=5, got %d", result.embedded)
	}

	if result.errors != 0 {
		t.Errorf("expected errors=0, got %d", result.errors)
	}

	if embSvc.callCount != 5 {
		t.Errorf("expected 5 embedding calls, got %d", embSvc.callCount)
	}

	if db.updateCalls != 5 {
		t.Errorf("expected 5 update calls, got %d", db.updateCalls)
	}
}

func TestBackfillWorker_ContinuesOnEmbeddingError(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Title 1", Description: "Desc 1"},
			{ID: "post-2", Title: "Title 2", Description: "Desc 2"},
			{ID: "post-3", Title: "Title 3", Description: "Desc 3"},
		},
	}

	callNum := 0
	embSvc := &mockEmbeddingService{
		generateFunc: func(ctx context.Context, text string) ([]float32, error) {
			callNum++
			if callNum == 2 {
				return nil, errors.New("embedding API error")
			}
			return make([]float32, 1024), nil
		},
	}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         10,
		dryRun:            false,
		delayBetweenItems: 0,
		contentTypes:      []string{"posts"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should process 2 successfully and 1 error
	if result.embedded != 2 {
		t.Errorf("expected embedded=2, got %d", result.embedded)
	}

	if result.errors != 1 {
		t.Errorf("expected errors=1, got %d", result.errors)
	}

	// Should still have called embedding for all 3
	if embSvc.callCount != 3 {
		t.Errorf("expected 3 embedding calls, got %d", embSvc.callCount)
	}

	// Should only update DB for the 2 successes
	if db.updateCalls != 2 {
		t.Errorf("expected 2 update calls, got %d", db.updateCalls)
	}
}

func TestBackfillWorker_ContinuesOnDBError(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Title 1", Description: "Desc 1"},
			{ID: "post-2", Title: "Title 2", Description: "Desc 2"},
		},
		updateErr: errors.New("db connection error"),
	}

	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         10,
		dryRun:            false,
		delayBetweenItems: 0,
		contentTypes:      []string{"posts"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.embedded != 0 {
		t.Errorf("expected embedded=0 (all DB updates failed), got %d", result.embedded)
	}

	if result.errors != 2 {
		t.Errorf("expected errors=2, got %d", result.errors)
	}
}

func TestBackfillWorker_EmptyDatabase(t *testing.T) {
	db := &mockDB{posts: nil}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         100,
		dryRun:            false,
		delayBetweenItems: 0,
		contentTypes:      []string{"posts"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.totalFound != 0 {
		t.Errorf("expected totalFound=0, got %d", result.totalFound)
	}

	if result.embedded != 0 {
		t.Errorf("expected embedded=0, got %d", result.embedded)
	}
}

func TestBackfillWorker_ContextCanceled(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Title 1", Description: "Desc 1"},
			{ID: "post-2", Title: "Title 2", Description: "Desc 2"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	embSvc := &mockEmbeddingService{
		generateFunc: func(ctx context.Context, text string) ([]float32, error) {
			cancel() // Cancel context on first call
			return make([]float32, 1024), nil
		},
	}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         10,
		dryRun:            false,
		delayBetweenItems: 0,
		contentTypes:      []string{"posts"},
	}

	result, err := worker.run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should stop early when context is canceled
	// At least 1 embedded (the one where we canceled), but not all
	if result.embedded > 1 {
		t.Errorf("expected at most 1 embedded after cancel, got %d", result.embedded)
	}
}

// --- Tests across content types (posts and replies; answers and approaches are replies) ---

func TestBackfillWorker_AllContentTypes(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Post Title", Description: "Post Desc"},
		},
		replies: []replyRow{
			{ID: "rep-1", Body: "Reply body"},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         10,
		dryRun:            false,
		delayBetweenItems: 0,
		contentTypes:      mustParseContentTypes(t, "all"),
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.postsEmbedded != 1 {
		t.Errorf("expected postsEmbedded=1, got %d", result.postsEmbedded)
	}
	if result.repliesEmbedded != 1 {
		t.Errorf("expected repliesEmbedded=1, got %d", result.repliesEmbedded)
	}

	// Total embedded should be sum of all
	totalEmbedded := result.postsEmbedded + result.repliesEmbedded
	if result.embedded != totalEmbedded {
		t.Errorf("expected embedded=%d (sum), got %d", totalEmbedded, result.embedded)
	}

	if embSvc.callCount != 2 {
		t.Errorf("expected 2 embedding calls, got %d", embSvc.callCount)
	}
}

func TestBackfillWorker_DryRunAllTypes(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Title", Description: "Desc"},
			{ID: "post-2", Title: "Title", Description: "Desc"},
		},
		replies: []replyRow{
			{ID: "rep-1", Body: "Content"},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:                db,
		embeddingService:  embSvc,
		batchSize:         10,
		dryRun:            true,
		delayBetweenItems: 0,
		contentTypes:      mustParseContentTypes(t, "all"),
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dry run should not embed anything
	if embSvc.callCount != 0 {
		t.Errorf("expected 0 embedding calls in dry run, got %d", embSvc.callCount)
	}

	// Should report counts
	if result.postsFound != 2 {
		t.Errorf("expected postsFound=2, got %d", result.postsFound)
	}
	if result.repliesFound != 1 {
		t.Errorf("expected repliesFound=1, got %d", result.repliesFound)
	}
	if result.totalFound != 3 {
		t.Errorf("expected totalFound=3, got %d", result.totalFound)
	}
}

// mustParseContentTypes parses a -content-types value the test expects to be valid.
func mustParseContentTypes(t *testing.T, s string) []string {
	t.Helper()
	types, err := parseContentTypes(s)
	if err != nil {
		t.Fatalf("parseContentTypes(%q): %v", s, err)
	}
	return types
}

func TestParseContentTypes_All(t *testing.T) {
	types := mustParseContentTypes(t, "all")
	if len(types) != 2 {
		t.Fatalf("expected 2 types for 'all', got %d", len(types))
	}
	expected := map[string]bool{"posts": true, "replies": true}
	for _, ct := range types {
		if !expected[ct] {
			t.Errorf("unexpected content type %q", ct)
		}
	}
}

func TestParseContentTypes_Single(t *testing.T) {
	types := mustParseContentTypes(t, "posts")
	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}
	if types[0] != "posts" {
		t.Errorf("expected 'posts', got %q", types[0])
	}
}

func TestParseContentTypes_Multiple(t *testing.T) {
	types := mustParseContentTypes(t, "posts,replies")
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	has := map[string]bool{}
	for _, ct := range types {
		has[ct] = true
	}
	if !has["posts"] || !has["replies"] {
		t.Errorf("expected posts and replies, got %v", types)
	}
}

func TestParseContentTypes_EmptyDefaultsToAll(t *testing.T) {
	types := mustParseContentTypes(t, "")
	if len(types) != 2 {
		t.Fatalf("expected 2 types for empty string, got %d", len(types))
	}
}
