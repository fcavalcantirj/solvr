package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockStaleApproachUpdater implements StaleApproachUpdater for testing.
type mockStaleApproachUpdater struct {
	abandonedCount int64
	err            error
}

func (m *mockStaleApproachUpdater) AbandonStaleApproaches(ctx context.Context, olderThan time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.abandonedCount, nil
}

// mockStaleApproachWarner implements StaleApproachWarner for testing.
type mockStaleApproachWarner struct {
	warnedCount int64
	err         error
}

func (m *mockStaleApproachWarner) WarnApproachesApproachingAbandonment(ctx context.Context, warningThreshold, abandonThreshold time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.warnedCount, nil
}

// mockDormantPostUpdater implements DormantPostUpdater for testing.
type mockDormantPostUpdater struct {
	dormantCount int64
	err          error
}

func (m *mockDormantPostUpdater) MarkDormantPosts(ctx context.Context, olderThan time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.dormantCount, nil
}

// TestStaleContentJob_AbandonApproaches tests that RunOnce abandons stale approaches.
func TestStaleContentJob_AbandonApproaches(t *testing.T) {
	updater := &mockStaleApproachUpdater{abandonedCount: 3}
	warner := &mockStaleApproachWarner{warnedCount: 0}
	dormant := &mockDormantPostUpdater{dormantCount: 0}

	job := NewStaleContentJob(updater, warner, dormant)
	result := job.RunOnce(context.Background())

	if result.Abandoned != 3 {
		t.Errorf("RunOnce() abandoned = %d, want 3", result.Abandoned)
	}
	if result.Warned != 0 {
		t.Errorf("RunOnce() warned = %d, want 0", result.Warned)
	}
	if result.Dormant != 0 {
		t.Errorf("RunOnce() dormant = %d, want 0", result.Dormant)
	}
}

// TestStaleContentJob_WarnBeforeAbandonment tests that RunOnce sends warning notifications.
func TestStaleContentJob_WarnBeforeAbandonment(t *testing.T) {
	updater := &mockStaleApproachUpdater{abandonedCount: 0}
	warner := &mockStaleApproachWarner{warnedCount: 2}
	dormant := &mockDormantPostUpdater{dormantCount: 0}

	job := NewStaleContentJob(updater, warner, dormant)
	result := job.RunOnce(context.Background())

	if result.Warned != 2 {
		t.Errorf("RunOnce() warned = %d, want 2", result.Warned)
	}
	if result.Abandoned != 0 {
		t.Errorf("RunOnce() abandoned = %d, want 0", result.Abandoned)
	}
}

// TestStaleContentJob_MarkDormant tests that RunOnce marks dormant posts.
func TestStaleContentJob_MarkDormant(t *testing.T) {
	updater := &mockStaleApproachUpdater{abandonedCount: 0}
	warner := &mockStaleApproachWarner{warnedCount: 0}
	dormant := &mockDormantPostUpdater{dormantCount: 2}

	job := NewStaleContentJob(updater, warner, dormant)
	result := job.RunOnce(context.Background())

	if result.Dormant != 2 {
		t.Errorf("RunOnce() dormant = %d, want 2", result.Dormant)
	}
}

// TestStaleContentJob_NoStaleContent tests RunOnce when no stale content exists.
func TestStaleContentJob_NoStaleContent(t *testing.T) {
	updater := &mockStaleApproachUpdater{abandonedCount: 0}
	warner := &mockStaleApproachWarner{warnedCount: 0}
	dormant := &mockDormantPostUpdater{dormantCount: 0}

	job := NewStaleContentJob(updater, warner, dormant)
	result := job.RunOnce(context.Background())

	if result.Abandoned != 0 {
		t.Errorf("RunOnce() abandoned = %d, want 0", result.Abandoned)
	}
	if result.Warned != 0 {
		t.Errorf("RunOnce() warned = %d, want 0", result.Warned)
	}
	if result.Dormant != 0 {
		t.Errorf("RunOnce() dormant = %d, want 0", result.Dormant)
	}
}

// TestStaleContentJob_ErrorsDoNotCrash tests that errors in one step don't prevent others.
func TestStaleContentJob_ErrorsDoNotCrash(t *testing.T) {
	updater := &mockStaleApproachUpdater{err: errors.New("db error")}
	warner := &mockStaleApproachWarner{warnedCount: 2}
	dormant := &mockDormantPostUpdater{dormantCount: 1}

	job := NewStaleContentJob(updater, warner, dormant)
	result := job.RunOnce(context.Background())

	// Abandon failed, but warn and dormant should still run
	if result.Abandoned != 0 {
		t.Errorf("RunOnce() abandoned = %d, want 0 (error case)", result.Abandoned)
	}
	if result.Warned != 2 {
		t.Errorf("RunOnce() warned = %d, want 2", result.Warned)
	}
	if result.Dormant != 1 {
		t.Errorf("RunOnce() dormant = %d, want 1", result.Dormant)
	}
}

// TestStaleContentJob_RunScheduled tests scheduled execution and context cancellation.
func TestStaleContentJob_RunScheduled(t *testing.T) {
	updater := &mockStaleApproachUpdater{abandonedCount: 1}
	warner := &mockStaleApproachWarner{warnedCount: 0}
	dormant := &mockDormantPostUpdater{dormantCount: 0}

	job := NewStaleContentJob(updater, warner, dormant)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		job.RunScheduled(ctx, 10*time.Millisecond)
		close(done)
	}()

	// Wait for at least one execution
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
}

// TestStaleContentJob_DefaultConstants tests the default configuration constants.
func TestStaleContentJob_DefaultConstants(t *testing.T) {
	if DefaultStaleApproachThreshold != 30*24*time.Hour {
		t.Errorf("DefaultStaleApproachThreshold = %v, want 30 days", DefaultStaleApproachThreshold)
	}
	if DefaultDormantPostThreshold != 60*24*time.Hour {
		t.Errorf("DefaultDormantPostThreshold = %v, want 60 days", DefaultDormantPostThreshold)
	}
	if DefaultWarningThreshold != 23*24*time.Hour {
		t.Errorf("DefaultWarningThreshold = %v, want 23 days", DefaultWarningThreshold)
	}
	if DefaultStaleContentInterval != 24*time.Hour {
		t.Errorf("DefaultStaleContentInterval = %v, want 24 hours", DefaultStaleContentInterval)
	}
}
