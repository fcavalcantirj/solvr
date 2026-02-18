package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockCandidateLister implements CrystallizationCandidateLister for testing.
type mockCandidateLister struct {
	candidateIDs []string
	err          error
}

func (m *mockCandidateLister) ListCrystallizationCandidates(ctx context.Context, stabilityPeriod time.Duration, limit int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.candidateIDs, nil
}

// mockCrystallizer implements ProblemCrystallizer for testing.
type mockCrystallizer struct {
	crystallizedIDs []string
	errMap          map[string]error // per-ID errors
	defaultErr      error
}

func (m *mockCrystallizer) CrystallizeProblem(ctx context.Context, problemID string) (string, error) {
	if m.errMap != nil {
		if err, ok := m.errMap[problemID]; ok {
			return "", err
		}
	}
	if m.defaultErr != nil {
		return "", m.defaultErr
	}
	m.crystallizedIDs = append(m.crystallizedIDs, problemID)
	return "bafytest_" + problemID, nil
}

// TestCrystallizationJob_RunOnce_CrystallizesCandidates tests that RunOnce
// lists candidates and crystallizes them.
func TestCrystallizationJob_RunOnce_CrystallizesCandidates(t *testing.T) {
	lister := &mockCandidateLister{
		candidateIDs: []string{"problem-1", "problem-2", "problem-3"},
	}
	crystallizer := &mockCrystallizer{}

	job := NewCrystallizationJob(lister, crystallizer, DefaultCrystallizationStabilityPeriod)
	crystallized, failed := job.RunOnce(context.Background())

	if crystallized != 3 {
		t.Errorf("RunOnce() crystallized = %d, want 3", crystallized)
	}
	if failed != 0 {
		t.Errorf("RunOnce() failed = %d, want 0", failed)
	}
	if len(crystallizer.crystallizedIDs) != 3 {
		t.Errorf("Expected 3 IDs crystallized, got %d", len(crystallizer.crystallizedIDs))
	}
}

// TestCrystallizationJob_RunOnce_NoCandidates tests RunOnce with no candidates.
func TestCrystallizationJob_RunOnce_NoCandidates(t *testing.T) {
	lister := &mockCandidateLister{
		candidateIDs: []string{},
	}
	crystallizer := &mockCrystallizer{}

	job := NewCrystallizationJob(lister, crystallizer, DefaultCrystallizationStabilityPeriod)
	crystallized, failed := job.RunOnce(context.Background())

	if crystallized != 0 {
		t.Errorf("RunOnce() crystallized = %d, want 0", crystallized)
	}
	if failed != 0 {
		t.Errorf("RunOnce() failed = %d, want 0", failed)
	}
}

// TestCrystallizationJob_RunOnce_ListError tests RunOnce when listing fails.
func TestCrystallizationJob_RunOnce_ListError(t *testing.T) {
	lister := &mockCandidateLister{
		err: errors.New("database unavailable"),
	}
	crystallizer := &mockCrystallizer{}

	job := NewCrystallizationJob(lister, crystallizer, DefaultCrystallizationStabilityPeriod)
	crystallized, failed := job.RunOnce(context.Background())

	// When listing fails, we crystallize 0 and report 0 failures (listing error logged)
	if crystallized != 0 {
		t.Errorf("RunOnce() crystallized = %d, want 0", crystallized)
	}
	if failed != 0 {
		t.Errorf("RunOnce() failed = %d, want 0", failed)
	}
}

// TestCrystallizationJob_RunOnce_PartialFailure tests RunOnce when some crystallizations fail.
func TestCrystallizationJob_RunOnce_PartialFailure(t *testing.T) {
	lister := &mockCandidateLister{
		candidateIDs: []string{"problem-1", "problem-2", "problem-3"},
	}
	crystallizer := &mockCrystallizer{
		errMap: map[string]error{
			"problem-2": errors.New("IPFS unreachable"),
		},
	}

	job := NewCrystallizationJob(lister, crystallizer, DefaultCrystallizationStabilityPeriod)
	crystallized, failed := job.RunOnce(context.Background())

	if crystallized != 2 {
		t.Errorf("RunOnce() crystallized = %d, want 2", crystallized)
	}
	if failed != 1 {
		t.Errorf("RunOnce() failed = %d, want 1", failed)
	}
}

// TestCrystallizationJob_RunScheduled tests that the scheduled job runs
// and can be stopped properly via context cancellation.
func TestCrystallizationJob_RunScheduled(t *testing.T) {
	lister := &mockCandidateLister{
		candidateIDs: []string{"problem-1"},
	}
	crystallizer := &mockCrystallizer{}

	job := NewCrystallizationJob(lister, crystallizer, DefaultCrystallizationStabilityPeriod)

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

	// Should have crystallized at least once
	if len(crystallizer.crystallizedIDs) == 0 {
		t.Error("expected job to have crystallized at least one problem")
	}
}

// TestCrystallizationJob_DefaultInterval tests the default interval constant.
func TestCrystallizationJob_DefaultInterval(t *testing.T) {
	expected := 24 * time.Hour
	if DefaultCrystallizationInterval != expected {
		t.Errorf("DefaultCrystallizationInterval = %v, want %v", DefaultCrystallizationInterval, expected)
	}
}

// TestCrystallizationJob_DefaultStabilityPeriod tests the default stability period.
func TestCrystallizationJob_DefaultStabilityPeriod(t *testing.T) {
	expected := 7 * 24 * time.Hour
	if DefaultCrystallizationStabilityPeriod != expected {
		t.Errorf("DefaultCrystallizationStabilityPeriod = %v, want %v", DefaultCrystallizationStabilityPeriod, expected)
	}
}

// TestCrystallizationJob_DefaultCandidateLimit tests the candidate limit constant.
func TestCrystallizationJob_DefaultCandidateLimit(t *testing.T) {
	if DefaultCrystallizationCandidateLimit < 10 {
		t.Errorf("DefaultCrystallizationCandidateLimit = %d, should be at least 10", DefaultCrystallizationCandidateLimit)
	}
}

// TestCrystallizationJob_AllFail tests RunOnce when all crystallizations fail.
func TestCrystallizationJob_AllFail(t *testing.T) {
	lister := &mockCandidateLister{
		candidateIDs: []string{"problem-1", "problem-2"},
	}
	crystallizer := &mockCrystallizer{
		defaultErr: errors.New("all fail"),
	}

	job := NewCrystallizationJob(lister, crystallizer, DefaultCrystallizationStabilityPeriod)
	crystallized, failed := job.RunOnce(context.Background())

	if crystallized != 0 {
		t.Errorf("RunOnce() crystallized = %d, want 0", crystallized)
	}
	if failed != 2 {
		t.Errorf("RunOnce() failed = %d, want 2", failed)
	}
}
