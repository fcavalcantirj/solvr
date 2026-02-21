package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// testPostID is a valid UUID for test use in moderatePostAsync calls.
const testPostID = "00000000-0000-0000-0000-000000000001"

// testRateLimitError implements the RateLimitError interface for testing.
type testRateLimitError struct {
	retryAfter time.Duration
	message    string
}

func (e *testRateLimitError) Error() string {
	return fmt.Sprintf("rate limited: %s", e.message)
}

func (e *testRateLimitError) GetRetryAfter() time.Duration {
	return e.retryAfter
}

// MockContentModerationService implements ContentModerationServiceInterface for testing.
type MockContentModerationService struct {
	mu      sync.Mutex
	results []*ModerationResult // queue of results to return
	errs    []error             // queue of errors to return
	calls   int                 // number of times ModerateContent was called
	inputs  []ModerationInput   // captured inputs
}

func NewMockContentModerationService() *MockContentModerationService {
	return &MockContentModerationService{}
}

func (m *MockContentModerationService) ModerateContent(ctx context.Context, input ModerationInput) (*ModerationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.inputs = append(m.inputs, input)

	idx := m.calls - 1

	// Return error if queued
	if idx < len(m.errs) && m.errs[idx] != nil {
		return nil, m.errs[idx]
	}

	// Return result if queued
	if idx < len(m.results) {
		return m.results[idx], nil
	}

	// Default: approved
	return &ModerationResult{Approved: true, Explanation: "OK"}, nil
}

func (m *MockContentModerationService) SetResult(result *ModerationResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = []*ModerationResult{result}
	m.errs = nil
}

func (m *MockContentModerationService) QueueResults(results []*ModerationResult, errs []error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = results
	m.errs = errs
}

func (m *MockContentModerationService) GetCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// MockFlagCreator implements FlagCreatorInterface for testing.
type MockFlagCreator struct {
	mu    sync.Mutex
	flags []*models.Flag
	err   error
}

func (m *MockFlagCreator) CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	flag.ID = uuid.New()
	flag.CreatedAt = time.Now()
	m.flags = append(m.flags, flag)
	return flag, nil
}

func (m *MockFlagCreator) GetFlags() []*models.Flag {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flags
}

// MockPostStatusUpdater extends MockPostsRepository with UpdateStatus support.
type MockPostStatusUpdater struct {
	mu        sync.Mutex
	statusMap map[string]models.PostStatus // postID -> status
	err       error
}

func NewMockPostStatusUpdater() *MockPostStatusUpdater {
	return &MockPostStatusUpdater{
		statusMap: make(map[string]models.PostStatus),
	}
}

func (m *MockPostStatusUpdater) UpdateStatus(ctx context.Context, postID string, status models.PostStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.statusMap[postID] = status
	return nil
}

func (m *MockPostStatusUpdater) GetStatus(postID string) (models.PostStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.statusMap[postID]
	return s, ok
}

// ============================================================================
// POST /v1/posts - Create Post Sets pending_review Status
// ============================================================================

func TestCreatePost_SetsPendingReview(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := `{
		"type": "question",
		"title": "How do I handle async operations in Go?",
		"description": "I need help understanding how to properly handle async operations in Go with goroutines and channels for concurrent processing."
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the created post has pending_review status
	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}
	if repo.createdPost.Status != models.PostStatusPendingReview {
		t.Errorf("expected status %q, got %q", models.PostStatusPendingReview, repo.createdPost.Status)
	}

	// Also verify the response body includes pending_review status
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}
	if data["status"] != "pending_review" {
		t.Errorf("response status = %v, want 'pending_review'", data["status"])
	}
}

func TestCreatePost_NoModerationService(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)
	// Deliberately NOT setting a content moderation service

	body := `{
		"type": "idea",
		"title": "Exploring new testing approaches",
		"description": "What if we used property-based testing more broadly? Let's discuss the trade-offs and practical applications of this approach."
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-456", "user")

	rr := httptest.NewRecorder()

	// Should not panic even without moderation service
	handler.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Post should still be created as pending_review
	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}
	if repo.createdPost.Status != models.PostStatusPendingReview {
		t.Errorf("expected status %q, got %q", models.PostStatusPendingReview, repo.createdPost.Status)
	}
}

// ============================================================================
// moderatePostAsync Tests
// ============================================================================

func TestModeratePostAsync_Approved(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:    true,
		Explanation: "Content is appropriate",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "human", "user-123")

	// Verify status was updated to open
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}
}

func TestModeratePostAsync_Rejected(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		Explanation:      "Content is not in English",
		RejectionReasons: []string{"non_english"},
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "human", "user-123")

	// Verify status was updated to rejected
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusRejected {
		t.Errorf("expected status %q, got %q", models.PostStatusRejected, status)
	}
}

func TestModeratePostAsync_RetryOnError(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	// First call errors, second call succeeds
	modService.QueueResults(
		[]*ModerationResult{nil, {Approved: true, Explanation: "OK"}},
		[]error{fmt.Errorf("temporary network error"), nil},
	)

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	// Use short retry delays for testing
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "human", "user-123")

	// Should have been called twice (1 error + 1 success)
	if calls := modService.GetCalls(); calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}

	// Verify status was updated to open after retry
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}
}

func TestModeratePostAsync_AllRetriesFail(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	flagCreator := &MockFlagCreator{}
	modService := NewMockContentModerationService()
	// All 3 attempts error
	modService.QueueResults(
		[]*ModerationResult{nil, nil, nil},
		[]error{
			errors.New("error 1"),
			errors.New("error 2"),
			errors.New("error 3"),
		},
	)

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetFlagCreator(flagCreator)
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "human", "user-123")

	// Should have been called 3 times
	if calls := modService.GetCalls(); calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}

	// Status should NOT have been updated (stays pending_review)
	_, ok := statusUpdater.GetStatus(testPostID)
	if ok {
		t.Error("expected UpdateStatus NOT to be called when all retries fail")
	}

	// Flag should have been created
	flags := flagCreator.GetFlags()
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	if flags[0].Reason != "moderation_failed" {
		t.Errorf("expected flag reason 'moderation_failed', got %q", flags[0].Reason)
	}
	if flags[0].TargetType != "post" {
		t.Errorf("expected flag target_type 'post', got %q", flags[0].TargetType)
	}
	if flags[0].ReporterType != "system" {
		t.Errorf("expected flag reporter_type 'system', got %q", flags[0].ReporterType)
	}
}

func TestModeratePostAsync_RateLimitRetry(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	// First call returns rate limit error, second succeeds
	modService.QueueResults(
		[]*ModerationResult{nil, {Approved: true, Explanation: "OK"}},
		[]error{&testRateLimitError{retryAfter: 10 * time.Millisecond, message: "rate limited"}, nil},
	)

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description", []string{"go"}, "human", "user-123")

	// Rate limit retries do NOT count as attempts, so we should have exactly 2 calls
	if calls := modService.GetCalls(); calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}

	// Verify status was updated to open
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}
}

func TestModeratePostAsync_SpawnsGoroutine(t *testing.T) {
	// Verify that Create handler spawns the moderation goroutine
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:    true,
		Explanation: "OK",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)

	body := `{
		"type": "question",
		"title": "How do I handle async operations in Go?",
		"description": "I need help understanding how to properly handle async operations in Go with goroutines and channels for concurrent processing."
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Wait for async moderation to complete
	time.Sleep(200 * time.Millisecond)

	// Verify moderation service was called
	if calls := modService.GetCalls(); calls != 1 {
		t.Errorf("expected 1 moderation call, got %d", calls)
	}

	// Verify status was updated to open
	status, ok := statusUpdater.GetStatus("new-post-id")
	if !ok {
		t.Fatal("expected UpdateStatus to be called for post 'new-post-id'")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}
}
