// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockCooldownStore implements CooldownStore for testing.
type MockCooldownStore struct {
	mu        sync.Mutex
	lastPosts map[string]time.Time
}

// NewMockCooldownStore creates a new mock store.
func NewMockCooldownStore() *MockCooldownStore {
	return &MockCooldownStore{
		lastPosts: make(map[string]time.Time),
	}
}

// GetLastPostTime returns the last post time for a user/content type.
func (m *MockCooldownStore) GetLastPostTime(ctx context.Context, key string) (*time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.lastPosts[key]; ok {
		return &t, nil
	}
	return nil, nil
}

// SetLastPostTime sets the last post time for a user/content type.
func (m *MockCooldownStore) SetLastPostTime(ctx context.Context, key string, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPosts[key] = t
	return nil
}

// --- CooldownConfig Tests ---

func TestDefaultCooldownConfig(t *testing.T) {
	config := DefaultCooldownConfig()

	if config.Problem != 10*time.Minute {
		t.Errorf("Expected Problem cooldown 10min, got %v", config.Problem)
	}
	if config.Question != 5*time.Minute {
		t.Errorf("Expected Question cooldown 5min, got %v", config.Question)
	}
	if config.Idea != 5*time.Minute {
		t.Errorf("Expected Idea cooldown 5min, got %v", config.Idea)
	}
	if config.Answer != 2*time.Minute {
		t.Errorf("Expected Answer cooldown 2min, got %v", config.Answer)
	}
	if config.Comment != 30*time.Second {
		t.Errorf("Expected Comment cooldown 30sec, got %v", config.Comment)
	}
}

// --- CooldownService Tests ---

func TestCooldownService_CheckCooldown_NoPreviousPost(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	// First post, no cooldown
	result, err := service.CheckCooldown(ctx, "user123", ContentTypeProblem)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for first post (no cooldown)")
	}
}

func TestCooldownService_CheckCooldown_CooldownActive(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "user456"

	// Record a recent post
	err := service.RecordPost(ctx, userID, ContentTypeProblem, time.Now())
	if err != nil {
		t.Fatalf("Unexpected error recording post: %v", err)
	}

	// Check cooldown - should be active
	result, err := service.CheckCooldown(ctx, userID, ContentTypeProblem)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected cooldown to be active")
	}

	if result.Remaining <= 0 {
		t.Error("Expected positive remaining time")
	}

	if result.Remaining > 10*time.Minute {
		t.Errorf("Remaining time should be <= 10 min, got %v", result.Remaining)
	}
}

func TestCooldownService_CheckCooldown_CooldownExpired(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "user789"

	// Record an old post (11 minutes ago, more than 10 min cooldown)
	err := service.RecordPost(ctx, userID, ContentTypeProblem, time.Now().Add(-11*time.Minute))
	if err != nil {
		t.Fatalf("Unexpected error recording post: %v", err)
	}

	// Check cooldown - should be expired
	result, err := service.CheckCooldown(ctx, userID, ContentTypeProblem)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for expired cooldown")
	}
}

func TestCooldownService_Problem10MinCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "problem_user"

	// Record post 5 minutes ago (still in 10 min cooldown)
	err := service.RecordPost(ctx, userID, ContentTypeProblem, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result, _ := service.CheckCooldown(ctx, userID, ContentTypeProblem)
	if result == nil {
		t.Fatal("Expected cooldown to be active")
	}

	// Should have ~5 minutes remaining
	if result.Remaining < 4*time.Minute || result.Remaining > 6*time.Minute {
		t.Errorf("Expected ~5 min remaining, got %v", result.Remaining)
	}
}

func TestCooldownService_Question5MinCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "question_user"

	// Record post 3 minutes ago (still in 5 min cooldown)
	err := service.RecordPost(ctx, userID, ContentTypeQuestion, time.Now().Add(-3*time.Minute))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result, _ := service.CheckCooldown(ctx, userID, ContentTypeQuestion)
	if result == nil {
		t.Fatal("Expected cooldown to be active")
	}

	// Should have ~2 minutes remaining
	if result.Remaining < 1*time.Minute || result.Remaining > 3*time.Minute {
		t.Errorf("Expected ~2 min remaining, got %v", result.Remaining)
	}
}

func TestCooldownService_Idea5MinCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "idea_user"

	// Record post 6 minutes ago (past 5 min cooldown)
	err := service.RecordPost(ctx, userID, ContentTypeIdea, time.Now().Add(-6*time.Minute))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result, _ := service.CheckCooldown(ctx, userID, ContentTypeIdea)
	if result != nil {
		t.Error("Expected cooldown to be expired (6 min > 5 min)")
	}
}

func TestCooldownService_Answer2MinCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "answer_user"

	// Record post 1 minute ago (still in 2 min cooldown)
	err := service.RecordPost(ctx, userID, ContentTypeAnswer, time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result, _ := service.CheckCooldown(ctx, userID, ContentTypeAnswer)
	if result == nil {
		t.Fatal("Expected cooldown to be active")
	}

	// Should have ~1 minute remaining
	if result.Remaining < 30*time.Second || result.Remaining > 90*time.Second {
		t.Errorf("Expected ~1 min remaining, got %v", result.Remaining)
	}
}

func TestCooldownService_Comment30SecCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "comment_user"

	// Record post 10 seconds ago (still in 30 sec cooldown)
	err := service.RecordPost(ctx, userID, ContentTypeComment, time.Now().Add(-10*time.Second))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result, _ := service.CheckCooldown(ctx, userID, ContentTypeComment)
	if result == nil {
		t.Fatal("Expected cooldown to be active")
	}

	// Should have ~20 seconds remaining
	if result.Remaining < 15*time.Second || result.Remaining > 25*time.Second {
		t.Errorf("Expected ~20 sec remaining, got %v", result.Remaining)
	}
}

func TestCooldownService_DifferentContentTypesIndependent(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "multi_user"

	// Post a problem
	err := service.RecordPost(ctx, userID, ContentTypeProblem, time.Now())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Problem should be on cooldown
	problemResult, _ := service.CheckCooldown(ctx, userID, ContentTypeProblem)
	if problemResult == nil {
		t.Error("Expected problem cooldown to be active")
	}

	// Question should NOT be on cooldown (different content type)
	questionResult, _ := service.CheckCooldown(ctx, userID, ContentTypeQuestion)
	if questionResult != nil {
		t.Error("Expected question cooldown to NOT be active (different content type)")
	}
}

func TestCooldownService_DifferentUsersIndependent(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userA := "userA"
	userB := "userB"

	// User A posts a problem
	err := service.RecordPost(ctx, userA, ContentTypeProblem, time.Now())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// User A should be on cooldown
	resultA, _ := service.CheckCooldown(ctx, userA, ContentTypeProblem)
	if resultA == nil {
		t.Error("Expected user A cooldown to be active")
	}

	// User B should NOT be on cooldown
	resultB, _ := service.CheckCooldown(ctx, userB, ContentTypeProblem)
	if resultB != nil {
		t.Error("Expected user B cooldown to NOT be active")
	}
}

func TestCooldownService_RecordPost(t *testing.T) {
	ctx := context.Background()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "record_user"
	now := time.Now()

	err := service.RecordPost(ctx, userID, ContentTypeProblem, now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify it was stored
	key := service.buildKey(userID, ContentTypeProblem)
	lastTime, _ := store.GetLastPostTime(ctx, key)
	if lastTime == nil {
		t.Fatal("Expected last post time to be stored")
	}

	// Should be the same time (or very close)
	if !lastTime.Equal(now) {
		t.Errorf("Expected stored time %v, got %v", now, *lastTime)
	}
}

func TestCooldownService_GetCooldownDuration(t *testing.T) {
	config := DefaultCooldownConfig()
	store := NewMockCooldownStore()
	service := NewCooldownService(store, config)

	tests := []struct {
		contentType ContentType
		expected    time.Duration
	}{
		{ContentTypeProblem, 10 * time.Minute},
		{ContentTypeQuestion, 5 * time.Minute},
		{ContentTypeIdea, 5 * time.Minute},
		{ContentTypeAnswer, 2 * time.Minute},
		{ContentTypeComment, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(string(tt.contentType), func(t *testing.T) {
			duration := service.GetCooldownDuration(tt.contentType)
			if duration != tt.expected {
				t.Errorf("Expected %v for %s, got %v", tt.expected, tt.contentType, duration)
			}
		})
	}
}

func TestCooldownService_UnknownContentType(t *testing.T) {
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	// Unknown content type should return 0 duration
	duration := service.GetCooldownDuration(ContentType("unknown"))
	if duration != 0 {
		t.Errorf("Expected 0 duration for unknown content type, got %v", duration)
	}
}

// --- CooldownResult Tests ---

func TestCooldownResult_RetryAfterSeconds(t *testing.T) {
	result := &CooldownResult{
		ContentType: ContentTypeProblem,
		Remaining:   5*time.Minute + 30*time.Second,
	}

	seconds := result.RetryAfterSeconds()
	if seconds != 330 { // 5.5 minutes = 330 seconds
		t.Errorf("Expected 330 seconds, got %d", seconds)
	}
}

func TestCooldownResult_RetryAfterSeconds_Ceiling(t *testing.T) {
	result := &CooldownResult{
		ContentType: ContentTypeComment,
		Remaining:   25*time.Second + 100*time.Millisecond,
	}

	// Should round up to next second
	seconds := result.RetryAfterSeconds()
	if seconds != 26 {
		t.Errorf("Expected 26 seconds (ceiling), got %d", seconds)
	}
}

// --- Key Generation Tests ---

func TestCooldownService_BuildKey(t *testing.T) {
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	key := service.buildKey("user123", ContentTypeProblem)
	expected := "cooldown:user123:problem"

	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}
}

func TestCooldownService_BuildKey_DifferentTypes(t *testing.T) {
	store := NewMockCooldownStore()
	service := NewCooldownService(store, DefaultCooldownConfig())

	userID := "sameuser"

	problemKey := service.buildKey(userID, ContentTypeProblem)
	questionKey := service.buildKey(userID, ContentTypeQuestion)

	if problemKey == questionKey {
		t.Error("Different content types should produce different keys")
	}
}
