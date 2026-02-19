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
	posts              []postRow
	answers            []answerRow
	approaches         []approachRow
	updateCalls        int
	updateErr          error
	updateAnswerCalls  int
	updateAnswerErr    error
	updateApprCalls    int
	updateApprErr      error
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

func (m *mockDB) UpdatePostEmbedding(ctx context.Context, id string, embedding []float32) error {
	m.updateCalls++
	return m.updateErr
}

func (m *mockDB) GetAnswersWithoutEmbedding(ctx context.Context, limit, offset int) ([]answerRow, error) {
	end := offset + limit
	if end > len(m.answers) {
		end = len(m.answers)
	}
	if offset >= len(m.answers) {
		return nil, nil
	}
	return m.answers[offset:end], nil
}

func (m *mockDB) CountAnswersWithoutEmbedding(ctx context.Context) (int, error) {
	return len(m.answers), nil
}

func (m *mockDB) UpdateAnswerEmbedding(ctx context.Context, id string, embedding []float32) error {
	m.updateAnswerCalls++
	return m.updateAnswerErr
}

func (m *mockDB) GetApproachesWithoutEmbedding(ctx context.Context, limit, offset int) ([]approachRow, error) {
	end := offset + limit
	if end > len(m.approaches) {
		end = len(m.approaches)
	}
	if offset >= len(m.approaches) {
		return nil, nil
	}
	return m.approaches[offset:end], nil
}

func (m *mockDB) CountApproachesWithoutEmbedding(ctx context.Context) (int, error) {
	return len(m.approaches), nil
}

func (m *mockDB) UpdateApproachEmbedding(ctx context.Context, id string, embedding []float32) error {
	m.updateApprCalls++
	return m.updateApprErr
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        2,
		dryRun:           true,
		rateLimit:        50,
		contentTypes:     []string{"posts"},
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        2,
		dryRun:           false,
		rateLimit:        1000, // high limit to avoid delays
		contentTypes:     []string{"posts"},
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"posts"},
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"posts"},
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        100,
		dryRun:           false,
		rateLimit:        50,
		contentTypes:     []string{"posts"},
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"posts"},
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

// --- New tests for answers and approaches backfill ---

func TestBackfillWorker_AnswersOnly(t *testing.T) {
	db := &mockDB{
		answers: []answerRow{
			{ID: "ans-1", Content: "Answer content 1"},
			{ID: "ans-2", Content: "Answer content 2"},
			{ID: "ans-3", Content: "Answer content 3"},
		},
		posts: []postRow{
			{ID: "post-1", Title: "Should not be touched", Description: "Desc"},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"answers"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should embed 3 answers
	if result.answersEmbedded != 3 {
		t.Errorf("expected answersEmbedded=3, got %d", result.answersEmbedded)
	}

	// Should NOT touch posts
	if result.postsEmbedded != 0 {
		t.Errorf("expected postsEmbedded=0, got %d", result.postsEmbedded)
	}

	// Posts DB should not be updated
	if db.updateCalls != 0 {
		t.Errorf("expected 0 post update calls, got %d", db.updateCalls)
	}

	// Answers DB should be updated
	if db.updateAnswerCalls != 3 {
		t.Errorf("expected 3 answer update calls, got %d", db.updateAnswerCalls)
	}
}

func TestBackfillWorker_ApproachesOnly(t *testing.T) {
	db := &mockDB{
		approaches: []approachRow{
			{ID: "apr-1", Angle: "Angle 1", Method: "Method 1", Outcome: "Outcome 1", Solution: "Solution 1"},
			{ID: "apr-2", Angle: "Angle 2", Method: "Method 2", Outcome: "", Solution: ""},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"approaches"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.approachesEmbedded != 2 {
		t.Errorf("expected approachesEmbedded=2, got %d", result.approachesEmbedded)
	}

	if db.updateApprCalls != 2 {
		t.Errorf("expected 2 approach update calls, got %d", db.updateApprCalls)
	}

	// Should NOT touch posts or answers
	if db.updateCalls != 0 {
		t.Errorf("expected 0 post update calls, got %d", db.updateCalls)
	}
	if db.updateAnswerCalls != 0 {
		t.Errorf("expected 0 answer update calls, got %d", db.updateAnswerCalls)
	}
}

func TestBackfillWorker_ApproachTextCombination(t *testing.T) {
	db := &mockDB{
		approaches: []approachRow{
			{ID: "apr-1", Angle: "My angle", Method: "My method", Outcome: "My outcome", Solution: "My solution"},
		},
	}

	var capturedText string
	embSvc := &mockEmbeddingService{
		generateFunc: func(ctx context.Context, text string) ([]float32, error) {
			capturedText = text
			return make([]float32, 1024), nil
		},
	}

	worker := &backfillWorker{
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"approaches"},
	}

	_, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "My angle My method My outcome My solution"
	if capturedText != expected {
		t.Errorf("expected text=%q, got %q", expected, capturedText)
	}
}

func TestBackfillWorker_ApproachTextOmitsEmptyFields(t *testing.T) {
	db := &mockDB{
		approaches: []approachRow{
			{ID: "apr-1", Angle: "My angle", Method: "My method", Outcome: "", Solution: ""},
		},
	}

	var capturedText string
	embSvc := &mockEmbeddingService{
		generateFunc: func(ctx context.Context, text string) ([]float32, error) {
			capturedText = text
			return make([]float32, 1024), nil
		},
	}

	worker := &backfillWorker{
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"approaches"},
	}

	_, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "My angle My method"
	if capturedText != expected {
		t.Errorf("expected text=%q, got %q", expected, capturedText)
	}
}

func TestBackfillWorker_AllContentTypes(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Post Title", Description: "Post Desc"},
		},
		answers: []answerRow{
			{ID: "ans-1", Content: "Answer content"},
		},
		approaches: []approachRow{
			{ID: "apr-1", Angle: "Angle", Method: "Method", Outcome: "", Solution: ""},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"posts", "answers", "approaches"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.postsEmbedded != 1 {
		t.Errorf("expected postsEmbedded=1, got %d", result.postsEmbedded)
	}
	if result.answersEmbedded != 1 {
		t.Errorf("expected answersEmbedded=1, got %d", result.answersEmbedded)
	}
	if result.approachesEmbedded != 1 {
		t.Errorf("expected approachesEmbedded=1, got %d", result.approachesEmbedded)
	}

	// Total embedded should be sum of all
	totalEmbedded := result.postsEmbedded + result.answersEmbedded + result.approachesEmbedded
	if result.embedded != totalEmbedded {
		t.Errorf("expected embedded=%d (sum), got %d", totalEmbedded, result.embedded)
	}

	if embSvc.callCount != 3 {
		t.Errorf("expected 3 embedding calls, got %d", embSvc.callCount)
	}
}

func TestBackfillWorker_DryRunAllTypes(t *testing.T) {
	db := &mockDB{
		posts: []postRow{
			{ID: "post-1", Title: "Title", Description: "Desc"},
			{ID: "post-2", Title: "Title", Description: "Desc"},
		},
		answers: []answerRow{
			{ID: "ans-1", Content: "Content"},
		},
		approaches: []approachRow{
			{ID: "apr-1", Angle: "Angle", Method: "Method", Outcome: "", Solution: ""},
		},
	}
	embSvc := &mockEmbeddingService{}

	worker := &backfillWorker{
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           true,
		rateLimit:        50,
		contentTypes:     []string{"posts", "answers", "approaches"},
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
	if result.answersFound != 1 {
		t.Errorf("expected answersFound=1, got %d", result.answersFound)
	}
	if result.approachesFound != 1 {
		t.Errorf("expected approachesFound=1, got %d", result.approachesFound)
	}
}

func TestBackfillWorker_AnswerEmbeddingError(t *testing.T) {
	db := &mockDB{
		answers: []answerRow{
			{ID: "ans-1", Content: "Answer 1"},
			{ID: "ans-2", Content: "Answer 2"},
			{ID: "ans-3", Content: "Answer 3"},
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
		db:               db,
		embeddingService: embSvc,
		batchSize:        10,
		dryRun:           false,
		rateLimit:        1000,
		contentTypes:     []string{"answers"},
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.answersEmbedded != 2 {
		t.Errorf("expected answersEmbedded=2, got %d", result.answersEmbedded)
	}

	if result.answersErrors != 1 {
		t.Errorf("expected answersErrors=1, got %d", result.answersErrors)
	}
}

func TestParseContentTypes_All(t *testing.T) {
	types := parseContentTypes("all")
	if len(types) != 3 {
		t.Fatalf("expected 3 types for 'all', got %d", len(types))
	}
	expected := map[string]bool{"posts": true, "answers": true, "approaches": true}
	for _, ct := range types {
		if !expected[ct] {
			t.Errorf("unexpected content type %q", ct)
		}
	}
}

func TestParseContentTypes_Single(t *testing.T) {
	types := parseContentTypes("answers")
	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}
	if types[0] != "answers" {
		t.Errorf("expected 'answers', got %q", types[0])
	}
}

func TestParseContentTypes_Multiple(t *testing.T) {
	types := parseContentTypes("posts,approaches")
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	has := map[string]bool{}
	for _, ct := range types {
		has[ct] = true
	}
	if !has["posts"] || !has["approaches"] {
		t.Errorf("expected posts and approaches, got %v", types)
	}
}

func TestParseContentTypes_EmptyDefaultsToAll(t *testing.T) {
	types := parseContentTypes("")
	if len(types) != 3 {
		t.Fatalf("expected 3 types for empty string, got %d", len(types))
	}
}
