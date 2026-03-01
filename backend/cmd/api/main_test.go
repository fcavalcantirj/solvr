package main

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// --- Mocks ---

type mockModerationService struct {
	mu     sync.Mutex
	result *services.ModerationResult
	err    error
}

func (m *mockModerationService) ModerateContent(_ context.Context, _ services.ModerationInput) (*services.ModerationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.result, m.err
}

type mockPostStatusUpdater struct {
	mu       sync.Mutex
	postID   string
	status   models.PostStatus
	err      error
	called   bool
}

func (m *mockPostStatusUpdater) UpdateStatus(_ context.Context, postID string, status models.PostStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.postID = postID
	m.status = status
	return m.err
}

type mockCommentCreator struct {
	mu       sync.Mutex
	comments []*models.Comment
	err      error
}

func (m *mockCommentCreator) Create(_ context.Context, comment *models.Comment) (*models.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	comment.ID = fmt.Sprintf("comment-%d", len(m.comments)+1)
	m.comments = append(m.comments, comment)
	return comment, nil
}

func (m *mockCommentCreator) getComments() []*models.Comment {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.comments
}

type mockNotifService struct {
	mu       sync.Mutex
	calls    []notifCall
	err      error
}

type notifCall struct {
	postID      string
	postTitle   string
	postType    string
	authorType  string
	authorID    string
	approved    bool
	explanation string
}

func (m *mockNotifService) NotifyOnModerationResult(_ context.Context, postID, postTitle, postType, authorType, authorID string, approved bool, explanation string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, notifCall{
		postID:      postID,
		postTitle:   postTitle,
		postType:    postType,
		authorType:  authorType,
		authorID:    authorID,
		approved:    approved,
		explanation: explanation,
	})
	return m.err
}

func (m *mockNotifService) getCalls() []notifCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// --- Tests ---

func TestTriggerModeration_RejectionCreatesSystemComment(t *testing.T) {
	modSvc := &mockModerationService{
		result: &services.ModerationResult{
			Approved:         false,
			LanguageDetected: "English",
			RejectionReasons: []string{"spam"},
			Explanation:      "Content appears to be promotional spam",
		},
	}
	statusUpdater := &mockPostStatusUpdater{}
	commentCreator := &mockCommentCreator{}

	trigger := &translationModTrigger{
		modSvc:         modSvc,
		postRepo:       statusUpdater,
		commentCreator: commentCreator,
	}

	ctx := context.Background()
	trigger.triggerModeration(ctx, "post-123", "Test Title", "Test Description", []string{"go"}, "idea", "agent", "agent-1")

	// Status should be rejected
	if !statusUpdater.called {
		t.Fatal("expected status update to be called")
	}
	if statusUpdater.status != models.PostStatusRejected {
		t.Errorf("expected status %q, got %q", models.PostStatusRejected, statusUpdater.status)
	}

	// System comment should be created
	comments := commentCreator.getComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 system comment, got %d", len(comments))
	}
	c := comments[0]
	if c.TargetType != models.CommentTargetPost {
		t.Errorf("expected target type %q, got %q", models.CommentTargetPost, c.TargetType)
	}
	if c.TargetID != "post-123" {
		t.Errorf("expected target ID %q, got %q", "post-123", c.TargetID)
	}
	if c.AuthorType != models.AuthorTypeSystem {
		t.Errorf("expected author type %q, got %q", models.AuthorTypeSystem, c.AuthorType)
	}
	if c.AuthorID != "solvr-moderator" {
		t.Errorf("expected author ID %q, got %q", "solvr-moderator", c.AuthorID)
	}
	if c.Content == "" {
		t.Error("expected non-empty comment content with rejection reason")
	}
}

func TestTriggerModeration_ApprovalCreatesSystemComment(t *testing.T) {
	modSvc := &mockModerationService{
		result: &services.ModerationResult{
			Approved:         true,
			LanguageDetected: "English",
			Explanation:      "Content is appropriate",
		},
	}
	statusUpdater := &mockPostStatusUpdater{}
	commentCreator := &mockCommentCreator{}

	trigger := &translationModTrigger{
		modSvc:         modSvc,
		postRepo:       statusUpdater,
		commentCreator: commentCreator,
	}

	ctx := context.Background()
	trigger.triggerModeration(ctx, "post-456", "Good Post", "Helpful content", []string{"python"}, "idea", "human", "user-1")

	// Status should be open
	if statusUpdater.status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, statusUpdater.status)
	}

	// System comment should be created
	comments := commentCreator.getComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 system comment, got %d", len(comments))
	}
	c := comments[0]
	if c.AuthorID != "solvr-moderator" {
		t.Errorf("expected author ID %q, got %q", "solvr-moderator", c.AuthorID)
	}
}

func TestTriggerModeration_RejectionSendsNotification(t *testing.T) {
	modSvc := &mockModerationService{
		result: &services.ModerationResult{
			Approved:         false,
			LanguageDetected: "English",
			RejectionReasons: []string{"quality"},
			Explanation:      "Low quality content",
		},
	}
	statusUpdater := &mockPostStatusUpdater{}
	notif := &mockNotifService{}

	trigger := &translationModTrigger{
		modSvc:       modSvc,
		postRepo:     statusUpdater,
		notifService: notif,
	}

	ctx := context.Background()
	trigger.triggerModeration(ctx, "post-789", "Bad Post", "gibberish", nil, "idea", "agent", "agent-2")

	calls := notif.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 notification call, got %d", len(calls))
	}
	nc := calls[0]
	if nc.postID != "post-789" {
		t.Errorf("expected postID %q, got %q", "post-789", nc.postID)
	}
	if nc.approved {
		t.Error("expected approved=false in notification")
	}
	if nc.postType != "idea" {
		t.Errorf("expected postType %q, got %q", "idea", nc.postType)
	}
	if nc.authorType != "agent" {
		t.Errorf("expected authorType %q, got %q", "agent", nc.authorType)
	}
}

func TestTriggerModeration_ApprovalSendsNotification(t *testing.T) {
	modSvc := &mockModerationService{
		result: &services.ModerationResult{
			Approved:         true,
			LanguageDetected: "English",
			Explanation:      "Looks good",
		},
	}
	statusUpdater := &mockPostStatusUpdater{}
	notif := &mockNotifService{}

	trigger := &translationModTrigger{
		modSvc:       modSvc,
		postRepo:     statusUpdater,
		notifService: notif,
	}

	ctx := context.Background()
	trigger.triggerModeration(ctx, "post-abc", "Nice Post", "Great content", nil, "idea", "human", "user-5")

	calls := notif.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 notification call, got %d", len(calls))
	}
	if !calls[0].approved {
		t.Error("expected approved=true in notification")
	}
}

func TestTriggerModeration_NilCommentCreatorDoesNotPanic(t *testing.T) {
	modSvc := &mockModerationService{
		result: &services.ModerationResult{
			Approved:    true,
			Explanation: "OK",
		},
	}
	statusUpdater := &mockPostStatusUpdater{}

	trigger := &translationModTrigger{
		modSvc:   modSvc,
		postRepo: statusUpdater,
		// commentCreator and notifService intentionally nil
	}

	ctx := context.Background()
	// Should not panic
	trigger.triggerModeration(ctx, "post-nil", "Title", "Desc", nil, "idea", "agent", "a1")

	if !statusUpdater.called {
		t.Error("expected status update to be called even without comment creator")
	}
}
