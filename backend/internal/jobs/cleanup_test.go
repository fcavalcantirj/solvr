package jobs

import (
	"context"
	"testing"
	"time"
)

// MockClaimTokenRepository implements the interface needed for testing.
type MockClaimTokenRepository struct {
	DeleteExpiredTokensCalled bool
	DeleteExpiredTokensResult int64
	DeleteExpiredTokensError  error
}

func (m *MockClaimTokenRepository) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	m.DeleteExpiredTokensCalled = true
	return m.DeleteExpiredTokensResult, m.DeleteExpiredTokensError
}

// TestCleanupJob_DeleteExpiredTokens tests that the cleanup job
// calls DeleteExpiredTokens on the repository.
func TestCleanupJob_DeleteExpiredTokens(t *testing.T) {
	mockRepo := &MockClaimTokenRepository{
		DeleteExpiredTokensResult: 5,
	}

	job := NewCleanupJob(mockRepo)
	result, err := job.CleanupExpiredTokens(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mockRepo.DeleteExpiredTokensCalled {
		t.Error("expected DeleteExpiredTokens to be called")
	}

	if result != 5 {
		t.Errorf("expected 5 deleted tokens, got %d", result)
	}
}

// TestCleanupJob_DeleteExpiredTokensError tests error handling.
func TestCleanupJob_DeleteExpiredTokensError(t *testing.T) {
	mockRepo := &MockClaimTokenRepository{
		DeleteExpiredTokensError: context.DeadlineExceeded,
	}

	job := NewCleanupJob(mockRepo)
	_, err := job.CleanupExpiredTokens(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got %v", err)
	}
}

// TestCleanupJob_RunScheduled tests that the scheduled job runs
// and can be stopped properly.
func TestCleanupJob_RunScheduled(t *testing.T) {
	mockRepo := &MockClaimTokenRepository{
		DeleteExpiredTokensResult: 0,
	}

	job := NewCleanupJob(mockRepo)

	// Start with a short interval for testing
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		job.RunScheduled(ctx, 10*time.Millisecond)
		close(done)
	}()

	// Wait a bit for at least one execution
	time.Sleep(50 * time.Millisecond)

	// Stop the job
	cancel()

	// Wait for job to finish
	select {
	case <-done:
		// Success - job stopped
	case <-time.After(1 * time.Second):
		t.Fatal("job did not stop within timeout")
	}

	// Should have been called at least once
	if !mockRepo.DeleteExpiredTokensCalled {
		t.Error("expected job to have run at least once")
	}
}

// TestCleanupJob_DefaultInterval tests the default interval constant.
func TestCleanupJob_DefaultInterval(t *testing.T) {
	// Per requirement: "Run every hour"
	expected := 1 * time.Hour
	if DefaultCleanupInterval != expected {
		t.Errorf("expected default interval %v, got %v", expected, DefaultCleanupInterval)
	}
}
