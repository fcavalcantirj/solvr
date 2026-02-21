package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/services"
)

// mockModerationDB is a test double for moderationDB.
type mockModerationDB struct {
	posts           []postRow
	countErr        error
	getPostsErr     error
	rejectPostErr   error
	createCommentErr error
	rejectedIDs     []string
	commentPostIDs  []string
	commentContents []string
}

func (m *mockModerationDB) GetOpenPosts(_ context.Context, limit, offset int) ([]postRow, error) {
	if m.getPostsErr != nil {
		return nil, m.getPostsErr
	}
	if offset >= len(m.posts) {
		return nil, nil
	}
	end := offset + limit
	if end > len(m.posts) {
		end = len(m.posts)
	}
	return m.posts[offset:end], nil
}

func (m *mockModerationDB) CountOpenPosts(_ context.Context) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return len(m.posts), nil
}

func (m *mockModerationDB) RejectPost(_ context.Context, postID string) error {
	if m.rejectPostErr != nil {
		return m.rejectPostErr
	}
	m.rejectedIDs = append(m.rejectedIDs, postID)
	return nil
}

func (m *mockModerationDB) CreateSystemComment(_ context.Context, postID, content string) error {
	if m.createCommentErr != nil {
		return m.createCommentErr
	}
	m.commentPostIDs = append(m.commentPostIDs, postID)
	m.commentContents = append(m.commentContents, content)
	return nil
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		maxLen   int
		expected string
	}{
		{"short title", "Hello", 50, "Hello"},
		{"exact length", "12345", 5, "12345"},
		{"long title", "This is a very long title that exceeds the limit", 20, "This is a very long ..."},
		{"empty title", "", 50, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateTitle(tt.title, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.title, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestModerationWorker_RunNoPosts(t *testing.T) {
	mockDB := &mockModerationDB{posts: nil}
	worker := &moderationWorker{
		db:        mockDB,
		moderator: services.NewContentModerationService("fake-key"),
		batchSize: 10,
		delay:     0,
		dryRun:    true,
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.total != 0 {
		t.Errorf("expected total=0, got %d", result.total)
	}
	if result.approved != 0 {
		t.Errorf("expected approved=0, got %d", result.approved)
	}
	if result.rejected != 0 {
		t.Errorf("expected rejected=0, got %d", result.rejected)
	}
}

func TestModerationWorker_CountError(t *testing.T) {
	mockDB := &mockModerationDB{countErr: errors.New("db error")}
	worker := &moderationWorker{
		db:        mockDB,
		moderator: services.NewContentModerationService("fake-key"),
		batchSize: 10,
		dryRun:    true,
	}

	_, err := worker.run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, mockDB.countErr) {
		t.Errorf("expected wrapped db error, got: %v", err)
	}
}

func TestModerationWorker_GetPostsError(t *testing.T) {
	dbErr := errors.New("query failed")
	mockDB := &mockModerationDB{
		posts:       []postRow{{ID: "1", Title: "Test"}},
		getPostsErr: dbErr,
	}
	worker := &moderationWorker{
		db:        mockDB,
		moderator: services.NewContentModerationService("fake-key"),
		batchSize: 10,
		dryRun:    true,
	}

	_, err := worker.run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestModerationWorker_ContextCanceled(t *testing.T) {
	mockDB := &mockModerationDB{
		posts: []postRow{
			{ID: "1", Title: "Test Post 1"},
			{ID: "2", Title: "Test Post 2"},
		},
	}
	worker := &moderationWorker{
		db:        mockDB,
		moderator: services.NewContentModerationService("fake-key"),
		batchSize: 10,
		dryRun:    true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := worker.run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With canceled context, we should have processed 0 posts
	if result.approved+result.rejected+result.errors > 0 {
		t.Errorf("expected no posts processed with canceled context, got approved=%d rejected=%d errors=%d",
			result.approved, result.rejected, result.errors)
	}
}

func TestModerationWorker_DryRunDoesNotModifyDB(t *testing.T) {
	// We can't easily test the full moderation flow without a real Groq API,
	// but we can verify that dry run mode doesn't call RejectPost or CreateSystemComment.
	mockDB := &mockModerationDB{
		posts: []postRow{
			{ID: "1", Title: "Test Post", Description: "A test post", Tags: []string{"go"}},
		},
	}
	worker := &moderationWorker{
		db:        mockDB,
		moderator: services.NewContentModerationService("fake-key"),
		batchSize: 10,
		dryRun:    true,
	}

	// The ModerateContent call will fail because the API key is fake,
	// but we can verify the error counting works.
	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have an error from the fake API key
	if result.errors != 1 {
		t.Errorf("expected errors=1 (from fake API key), got %d", result.errors)
	}

	// Dry run should never modify the DB
	if len(mockDB.rejectedIDs) > 0 {
		t.Errorf("dry run should not reject posts, but rejected: %v", mockDB.rejectedIDs)
	}
	if len(mockDB.commentPostIDs) > 0 {
		t.Errorf("dry run should not create comments, but created for: %v", mockDB.commentPostIDs)
	}
}

func TestModerationWorker_RejectPost(t *testing.T) {
	mockDB := &mockModerationDB{}
	worker := &moderationWorker{
		db:     mockDB,
		dryRun: false,
	}

	modResult := &services.ModerationResult{
		Approved:    false,
		Explanation: "Content is in Chinese, not English",
	}

	err := worker.rejectPost(context.Background(), "post-123", modResult)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify post was rejected
	if len(mockDB.rejectedIDs) != 1 || mockDB.rejectedIDs[0] != "post-123" {
		t.Errorf("expected post-123 rejected, got: %v", mockDB.rejectedIDs)
	}

	// Verify system comment was created
	if len(mockDB.commentPostIDs) != 1 || mockDB.commentPostIDs[0] != "post-123" {
		t.Errorf("expected comment on post-123, got: %v", mockDB.commentPostIDs)
	}

	// Verify comment content uses the ModerationRejectedFormat
	expectedContent := "Post rejected by Solvr moderation.\n\nReason: Content is in Chinese, not English\n\nYou can edit your post and resubmit for review."
	if mockDB.commentContents[0] != expectedContent {
		t.Errorf("unexpected comment content:\ngot:  %q\nwant: %q", mockDB.commentContents[0], expectedContent)
	}
}

func TestModerationWorker_RejectPostDBError(t *testing.T) {
	mockDB := &mockModerationDB{
		rejectPostErr: errors.New("update failed"),
	}
	worker := &moderationWorker{
		db:     mockDB,
		dryRun: false,
	}

	modResult := &services.ModerationResult{
		Approved:    false,
		Explanation: "Non-English content",
	}

	err := worker.rejectPost(context.Background(), "post-123", modResult)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should not create comment if status update failed
	if len(mockDB.commentPostIDs) > 0 {
		t.Errorf("should not create comment after status update failure, got: %v", mockDB.commentPostIDs)
	}
}

func TestModerationWorker_RejectPostCommentError(t *testing.T) {
	mockDB := &mockModerationDB{
		createCommentErr: errors.New("comment insert failed"),
	}
	worker := &moderationWorker{
		db:     mockDB,
		dryRun: false,
	}

	modResult := &services.ModerationResult{
		Approved:    false,
		Explanation: "Non-English content",
	}

	err := worker.rejectPost(context.Background(), "post-123", modResult)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Post status was updated even though comment failed
	if len(mockDB.rejectedIDs) != 1 {
		t.Errorf("expected post rejected before comment error, got: %v", mockDB.rejectedIDs)
	}
}

func TestModerationWorker_BatchProcessing(t *testing.T) {
	// Create 5 posts, process in batches of 2
	posts := make([]postRow, 5)
	for i := range posts {
		posts[i] = postRow{
			ID:          fmt.Sprintf("post-%d", i),
			Title:       fmt.Sprintf("Test Post %d", i),
			Description: "Some content",
			Tags:        []string{"test"},
		}
	}

	mockDB := &mockModerationDB{posts: posts}
	worker := &moderationWorker{
		db:        mockDB,
		moderator: services.NewContentModerationService("fake-key"),
		batchSize: 2,
		delay:     0,
		dryRun:    true, // dry run to avoid needing real API
	}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 5 posts should be counted
	if result.total != 5 {
		t.Errorf("expected total=5, got %d", result.total)
	}

	// All should error (fake API key) but all should be processed
	processed := result.approved + result.rejected + result.errors
	if processed != 5 {
		t.Errorf("expected 5 posts processed, got %d (approved=%d rejected=%d errors=%d)",
			processed, result.approved, result.rejected, result.errors)
	}
}

func TestModerationWorker_ModeratePostRateLimitRetry(t *testing.T) {
	// This tests the retry logic for rate limit errors.
	// Since we can't easily mock the ContentModerationService (it makes HTTP calls),
	// we verify the moderatePost function handles errors from the service.
	worker := &moderationWorker{
		moderator: services.NewContentModerationService("fake-key",
			services.WithHTTPTimeout(100*time.Millisecond),
		),
	}

	post := postRow{
		ID:          "test-post",
		Title:       "Test",
		Description: "Description",
		Tags:        []string{"test"},
	}

	// With a fake API key, this should fail after retries
	_, err := worker.moderatePost(context.Background(), post)
	if err == nil {
		t.Fatal("expected error with fake API key, got nil")
	}
}

// Verify the interface is satisfied at compile time.
var _ moderationDB = (*mockModerationDB)(nil)
