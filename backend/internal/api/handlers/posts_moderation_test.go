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

// MockCommentCreator implements CommentCreatorInterface for testing.
type MockCommentCreator struct {
	mu       sync.Mutex
	comments []*models.Comment
	err      error
}

func (m *MockCommentCreator) Create(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	comment.ID = fmt.Sprintf("comment-%d", len(m.comments)+1)
	m.comments = append(m.comments, comment)
	return comment, nil
}

func (m *MockCommentCreator) GetComments() []*models.Comment {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.comments
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
	mu           sync.Mutex
	statusMap    map[string]models.PostStatus // postID -> status
	languageMap  map[string]string            // postID -> original_language (set via UpdateOriginalLanguage)
	err          error
}

func NewMockPostStatusUpdater() *MockPostStatusUpdater {
	return &MockPostStatusUpdater{
		statusMap:   make(map[string]models.PostStatus),
		languageMap: make(map[string]string),
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

func (m *MockPostStatusUpdater) UpdateOriginalLanguage(ctx context.Context, postID, language string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.statusMap[postID] = models.PostStatusDraft
	m.languageMap[postID] = language
	return nil
}

func (m *MockPostStatusUpdater) GetStatus(postID string) (models.PostStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.statusMap[postID]
	return s, ok
}

func (m *MockPostStatusUpdater) GetLanguage(postID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.languageMap[postID]
	return l, ok
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
	commentCreator := &MockCommentCreator{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:    true,
		Explanation: "Content is appropriate",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "question", "human", "user-123")

	// Verify status was updated to open
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}

	// Verify system comment was created
	comments := commentCreator.GetComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0]
	if c.AuthorType != models.AuthorTypeSystem {
		t.Errorf("expected author_type %q, got %q", models.AuthorTypeSystem, c.AuthorType)
	}
	if c.AuthorID != "solvr-moderator" {
		t.Errorf("expected author_id 'solvr-moderator', got %q", c.AuthorID)
	}
	if c.TargetType != models.CommentTargetPost {
		t.Errorf("expected target_type %q, got %q", models.CommentTargetPost, c.TargetType)
	}
	expectedText := "Post approved by Solvr moderation. Your post is now visible in the feed."
	if c.Content != expectedText {
		t.Errorf("expected content %q, got %q", expectedText, c.Content)
	}
}

func TestModeratePostAsync_Rejected(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		Explanation:      "Content is not in English",
		RejectionReasons: []string{"non_english"},
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "question", "human", "user-123")

	// Verify status was updated to rejected
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusRejected {
		t.Errorf("expected status %q, got %q", models.PostStatusRejected, status)
	}

	// Verify system comment was created with rejection reason
	comments := commentCreator.GetComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0]
	if c.AuthorType != models.AuthorTypeSystem {
		t.Errorf("expected author_type %q, got %q", models.AuthorTypeSystem, c.AuthorType)
	}
	if c.AuthorID != "solvr-moderator" {
		t.Errorf("expected author_id 'solvr-moderator', got %q", c.AuthorID)
	}
	if !strings.Contains(c.Content, "Post rejected by Solvr moderation.") {
		t.Errorf("expected content to contain rejection header, got %q", c.Content)
	}
	if !strings.Contains(c.Content, "Content is not in English") {
		t.Errorf("expected content to contain explanation, got %q", c.Content)
	}
	if !strings.Contains(c.Content, "You can edit your post and resubmit for review.") {
		t.Errorf("expected content to contain resubmit instructions, got %q", c.Content)
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

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "question", "human", "user-123")

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

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description content", []string{"go"}, "question", "human", "user-123")

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

	handler.moderatePostAsync(testPostID, "Test Title Here", "Test description", []string{"go"}, "question", "human", "user-123")

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

// MockNotificationServiceForModeration implements NotificationServiceInterface for testing moderation notifications.
type MockNotificationServiceForModeration struct {
	mu            sync.Mutex
	notifications []moderationNotifCall
	err           error
}

type moderationNotifCall struct {
	PostID      string
	PostTitle   string
	PostType    string
	AuthorType  string
	AuthorID    string
	Approved    bool
	Explanation string
}

func (m *MockNotificationServiceForModeration) NotifyOnModerationResult(ctx context.Context, postID, postTitle, postType, authorType, authorID string, approved bool, explanation string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.notifications = append(m.notifications, moderationNotifCall{
		PostID:      postID,
		PostTitle:   postTitle,
		PostType:    postType,
		AuthorType:  authorType,
		AuthorID:    authorID,
		Approved:    approved,
		Explanation: explanation,
	})
	return nil
}

func (m *MockNotificationServiceForModeration) GetNotifications() []moderationNotifCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notifications
}

// TestModeratePostAsync_NotifiesOnApproval verifies that moderatePostAsync sends an approval notification.
func TestModeratePostAsync_NotifiesOnApproval(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	notifService := &MockNotificationServiceForModeration{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:    true,
		Explanation: "Content is appropriate",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)
	handler.SetNotificationService(notifService)

	handler.moderatePostAsync(testPostID, "Test Title", "Test description", []string{"go"}, "question", "human", "user-123")

	// Verify notification was sent
	notifs := notifService.GetNotifications()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	n := notifs[0]
	if !n.Approved {
		t.Error("expected approved=true")
	}
	if n.PostID != testPostID {
		t.Errorf("expected postID %q, got %q", testPostID, n.PostID)
	}
	if n.PostTitle != "Test Title" {
		t.Errorf("expected postTitle 'Test Title', got %q", n.PostTitle)
	}
	if n.PostType != "question" {
		t.Errorf("expected postType 'question', got %q", n.PostType)
	}
	if n.AuthorType != "human" {
		t.Errorf("expected authorType 'human', got %q", n.AuthorType)
	}
	if n.AuthorID != "user-123" {
		t.Errorf("expected authorID 'user-123', got %q", n.AuthorID)
	}
}

// TestModeratePostAsync_NotifiesOnRejection verifies that moderatePostAsync sends a rejection notification.
func TestModeratePostAsync_NotifiesOnRejection(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	notifService := &MockNotificationServiceForModeration{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:    false,
		Explanation: "Content is not in English",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)
	handler.SetNotificationService(notifService)

	handler.moderatePostAsync(testPostID, "Test Title", "Test description", []string{"go"}, "problem", "agent", "claude_bot")

	// Verify notification was sent
	notifs := notifService.GetNotifications()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Approved {
		t.Error("expected approved=false")
	}
	if n.PostType != "problem" {
		t.Errorf("expected postType 'problem', got %q", n.PostType)
	}
	if n.Explanation != "Content is not in English" {
		t.Errorf("expected explanation 'Content is not in English', got %q", n.Explanation)
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

// ============================================================================
// isLanguageOnlyRejection Tests
// ============================================================================

func TestIsLanguageOnlyRejection_TrueForSingleLanguageReason(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
	}
	if !isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=true for single LANGUAGE reason")
	}
}

func TestIsLanguageOnlyRejection_CaseInsensitive(t *testing.T) {
	cases := []string{"LANGUAGE", "language", "Language"}
	for _, reason := range cases {
		result := &ModerationResult{
			Approved:         false,
			LanguageDetected: "Spanish",
			RejectionReasons: []string{reason},
		}
		if !isLanguageOnlyRejection(result) {
			t.Errorf("expected isLanguageOnlyRejection=true for reason %q", reason)
		}
	}
}

func TestIsLanguageOnlyRejection_FalseWhenApproved(t *testing.T) {
	result := &ModerationResult{
		Approved:         true,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when approved")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenNoLanguage(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "",
		RejectionReasons: []string{"LANGUAGE"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when no language detected")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenEnglish(t *testing.T) {
	for _, lang := range []string{"en", "EN", "english", "English", "ENGLISH"} {
		result := &ModerationResult{
			Approved:         false,
			LanguageDetected: lang,
			RejectionReasons: []string{"LANGUAGE"},
		}
		if isLanguageOnlyRejection(result) {
			t.Errorf("expected isLanguageOnlyRejection=false for English language %q", lang)
		}
	}
}

func TestIsLanguageOnlyRejection_FalseWhenMultipleReasons(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE", "SPAM"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when multiple rejection reasons")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenNonLanguageReason(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"SPAM"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false for non-LANGUAGE reason")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenNoReasons(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when no rejection reasons")
	}
}

// ============================================================================
// Language-only rejection → draft + translation comment
// ============================================================================

func TestModeratePostAsync_LanguageOnlyRejection_SetsDraft(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
		Explanation:      "Content is in Portuguese, not English",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)

	handler.moderatePostAsync(testPostID, "Título de teste", "Descrição de teste", []string{"go"}, "question", "human", "user-123")

	// Status should be draft (not rejected)
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateOriginalLanguage to set status")
	}
	if status != models.PostStatusDraft {
		t.Errorf("expected status %q, got %q", models.PostStatusDraft, status)
	}

	// Language should be recorded
	lang, langOK := statusUpdater.GetLanguage(testPostID)
	if !langOK {
		t.Fatal("expected original language to be recorded")
	}
	if lang != "Portuguese" {
		t.Errorf("expected language 'Portuguese', got %q", lang)
	}

	// Comment should mention translation
	comments := commentCreator.GetComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0]
	if !strings.Contains(c.Content, "Portuguese") {
		t.Errorf("expected comment to mention 'Portuguese', got %q", c.Content)
	}
	if !strings.Contains(c.Content, "translat") {
		t.Errorf("expected comment to mention translation, got %q", c.Content)
	}
}

func TestModeratePostAsync_MultipleReasonsNotDraft(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE", "SPAM"},
		Explanation:      "Content is in Portuguese and is spam",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)

	handler.moderatePostAsync(testPostID, "Título de teste", "Descrição de teste", []string{"go"}, "question", "human", "user-123")

	// Multiple reasons → regular rejection, not draft
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected status to be set")
	}
	if status != models.PostStatusRejected {
		t.Errorf("expected status %q for multi-reason rejection, got %q", models.PostStatusRejected, status)
	}
}
