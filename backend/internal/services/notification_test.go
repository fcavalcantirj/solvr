// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MockNotificationRepository is a mock implementation of NotificationRepository.
type MockNotificationRepository struct {
	createFunc             func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error)
	findByUserIDFunc       func(ctx context.Context, userID string, limit int) ([]NotificationRecord, error)
	findByAgentIDFunc      func(ctx context.Context, agentID string, limit int) ([]NotificationRecord, error)
	getUserUpvoteCountFunc func(ctx context.Context, targetType, targetID string) (int, error)
}

func (m *MockNotificationRepository) Create(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, n)
	}
	return nil, errors.New("not implemented")
}

func (m *MockNotificationRepository) FindByUserID(ctx context.Context, userID string, limit int) ([]NotificationRecord, error) {
	if m.findByUserIDFunc != nil {
		return m.findByUserIDFunc(ctx, userID, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *MockNotificationRepository) FindByAgentID(ctx context.Context, agentID string, limit int) ([]NotificationRecord, error) {
	if m.findByAgentIDFunc != nil {
		return m.findByAgentIDFunc(ctx, agentID, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *MockNotificationRepository) GetUpvoteCount(ctx context.Context, targetType, targetID string) (int, error) {
	if m.getUserUpvoteCountFunc != nil {
		return m.getUserUpvoteCountFunc(ctx, targetType, targetID)
	}
	return 0, errors.New("not implemented")
}

// MockUserLookup is a mock implementation of UserLookup.
type MockUserLookup struct {
	findByIDFunc func(ctx context.Context, id string) (*UserInfo, error)
}

func (m *MockUserLookup) FindByID(ctx context.Context, id string) (*UserInfo, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

// MockAgentLookup is a mock implementation of AgentLookup.
type MockAgentLookup struct {
	findByIDFunc func(ctx context.Context, id string) (*AgentInfo, error)
}

func (m *MockAgentLookup) FindByID(ctx context.Context, id string) (*AgentInfo, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

// MockPostLookup is a mock implementation of PostLookup.
type MockPostLookup struct {
	findByIDFunc func(ctx context.Context, id string) (*PostInfo, error)
}

func (m *MockPostLookup) FindByID(ctx context.Context, id string) (*PostInfo, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

// MockAnswerLookup is a mock implementation of AnswerLookup.
type MockAnswerLookup struct {
	findByIDFunc func(ctx context.Context, id string) (*AnswerInfo, error)
}

func (m *MockAnswerLookup) FindByID(ctx context.Context, id string) (*AnswerInfo, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

// MockApproachLookup is a mock implementation of ApproachLookup.
type MockApproachLookup struct {
	findByIDFunc func(ctx context.Context, id string) (*ApproachInfo, error)
}

func (m *MockApproachLookup) FindByID(ctx context.Context, id string) (*ApproachInfo, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

// TestCreateNotification_ForUser tests CreateNotification for a user recipient.
func TestCreateNotification_ForUser(t *testing.T) {
	userID := uuid.New().String()
	notifID := uuid.New().String()
	now := time.Now()

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			if n.UserID == nil || *n.UserID != userID {
				t.Errorf("expected userID %s, got %v", userID, n.UserID)
			}
			if n.AgentID != nil {
				t.Errorf("expected nil agentID, got %v", n.AgentID)
			}
			if n.Type != NotificationTypeAnswerCreated {
				t.Errorf("expected type %s, got %s", NotificationTypeAnswerCreated, n.Type)
			}
			if n.Title != "New answer" {
				t.Errorf("expected title 'New answer', got %s", n.Title)
			}
			return &NotificationRecord{
				ID:        notifID,
				UserID:    &userID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: now,
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	result, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		UserID: &userID,
		Type:   NotificationTypeAnswerCreated,
		Title:  "New answer",
		Body:   "Someone answered your question",
		Link:   "/questions/123",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ID != notifID {
		t.Errorf("expected ID %s, got %s", notifID, result.ID)
	}
}

// TestCreateNotification_ForAgent tests CreateNotification for an agent recipient.
func TestCreateNotification_ForAgent(t *testing.T) {
	agentID := "test_agent"
	notifID := uuid.New().String()
	now := time.Now()

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			if n.AgentID == nil || *n.AgentID != agentID {
				t.Errorf("expected agentID %s, got %v", agentID, n.AgentID)
			}
			if n.UserID != nil {
				t.Errorf("expected nil userID, got %v", n.UserID)
			}
			return &NotificationRecord{
				ID:        notifID,
				AgentID:   &agentID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: now,
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	result, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		AgentID: &agentID,
		Type:    NotificationTypeCommentCreated,
		Title:   "New comment",
		Body:    "Someone commented on your approach",
		Link:    "/approaches/456",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.AgentID == nil || *result.AgentID != agentID {
		t.Errorf("expected agentID %s, got %v", agentID, result.AgentID)
	}
}

// TestCreateNotification_ValidationError tests CreateNotification with missing recipient.
func TestCreateNotification_ValidationError(t *testing.T) {
	repo := &MockNotificationRepository{}
	svc := NewNotificationService(repo, nil, nil, nil, nil)

	_, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		// Missing both UserID and AgentID
		Type:  NotificationTypeAnswerCreated,
		Title: "New answer",
	})

	if err == nil {
		t.Fatal("expected error for missing recipient")
	}
	if !errors.Is(err, ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient, got %v", err)
	}
}

// TestCreateNotification_BothRecipients tests CreateNotification with both user and agent.
func TestCreateNotification_BothRecipients(t *testing.T) {
	repo := &MockNotificationRepository{}
	svc := NewNotificationService(repo, nil, nil, nil, nil)

	userID := uuid.New().String()
	agentID := "test_agent"

	_, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		UserID:  &userID,
		AgentID: &agentID, // Both set - should error
		Type:    NotificationTypeAnswerCreated,
		Title:   "New answer",
	})

	if err == nil {
		t.Fatal("expected error for both recipients")
	}
	if !errors.Is(err, ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient, got %v", err)
	}
}

// TestCreateNotification_MissingType tests CreateNotification with missing type.
func TestCreateNotification_MissingType(t *testing.T) {
	repo := &MockNotificationRepository{}
	svc := NewNotificationService(repo, nil, nil, nil, nil)

	userID := uuid.New().String()

	_, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		UserID: &userID,
		Title:  "Test",
		// Missing Type
	})

	if err == nil {
		t.Fatal("expected error for missing type")
	}
	if !errors.Is(err, ErrInvalidNotificationType) {
		t.Errorf("expected ErrInvalidNotificationType, got %v", err)
	}
}

// TestCreateNotification_MissingTitle tests CreateNotification with missing title.
func TestCreateNotification_MissingTitle(t *testing.T) {
	repo := &MockNotificationRepository{}
	svc := NewNotificationService(repo, nil, nil, nil, nil)

	userID := uuid.New().String()

	_, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		UserID: &userID,
		Type:   NotificationTypeAnswerCreated,
		// Missing Title
	})

	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !errors.Is(err, ErrInvalidNotificationTitle) {
		t.Errorf("expected ErrInvalidNotificationTitle, got %v", err)
	}
}

// TestCreateNotification_DatabaseError tests CreateNotification with database error.
func TestCreateNotification_DatabaseError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			return nil, dbErr
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	userID := uuid.New().String()
	_, err := svc.CreateNotification(context.Background(), &CreateNotificationParams{
		UserID: &userID,
		Type:   NotificationTypeAnswerCreated,
		Title:  "New answer",
	})

	if err == nil {
		t.Fatal("expected error from database")
	}
}

// TestParseMentions tests parsing @username mentions from content.
func TestParseMentions(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "single mention",
			content:  "Hello @testuser, how are you?",
			expected: []string{"testuser"},
		},
		{
			name:     "multiple mentions",
			content:  "@user1 and @user2 should check this",
			expected: []string{"user1", "user2"},
		},
		{
			name:     "mention at start",
			content:  "@admin please review",
			expected: []string{"admin"},
		},
		{
			name:     "no mentions",
			content:  "Just a regular comment",
			expected: nil,
		},
		{
			name:     "email-like not a mention",
			content:  "Email me at test@example.com",
			expected: nil,
		},
		{
			name:     "mention with underscore",
			content:  "Thanks @claude_assistant!",
			expected: []string{"claude_assistant"},
		},
		{
			name:     "duplicate mentions deduplicated",
			content:  "@user1 @user1 @user1",
			expected: []string{"user1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMentions(tt.content)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d mentions, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected mention[%d] = %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

// TestNotificationTypes tests all notification type constants.
func TestNotificationTypes(t *testing.T) {
	types := []NotificationType{
		NotificationTypeAnswerCreated,
		NotificationTypeCommentCreated,
		NotificationTypeApproachUpdated,
		NotificationTypeProblemSolved,
		NotificationTypeMention,
		NotificationTypeAnswerAccepted,
		NotificationTypeUpvoteMilestone,
	}

	seen := make(map[NotificationType]bool)
	for _, nt := range types {
		if nt == "" {
			t.Error("notification type should not be empty")
		}
		if seen[nt] {
			t.Errorf("duplicate notification type: %s", nt)
		}
		seen[nt] = true
	}
}

// TestIsUpvoteMilestone tests milestone detection.
func TestIsUpvoteMilestone(t *testing.T) {
	tests := []struct {
		count    int
		expected bool
	}{
		{10, true},
		{50, true},
		{100, true},
		{9, false},
		{11, false},
		{49, false},
		{51, false},
		{99, false},
		{101, false},
		{0, false},
		{-1, false},
	}

	for _, tt := range tests {
		result := isUpvoteMilestone(tt.count)
		if result != tt.expected {
			t.Errorf("isUpvoteMilestone(%d) = %v, want %v", tt.count, result, tt.expected)
		}
	}
}

// TestNotifyOnModerationResult_Approved tests notification creation for approved posts.
func TestNotifyOnModerationResult_Approved(t *testing.T) {
	var captured *NotificationInput
	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			captured = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				AgentID:   n.AgentID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	err := svc.NotifyOnModerationResult(context.Background(), "post-uuid-123", "My Question", "question", "human", "user-uuid-456", true, "Content is appropriate")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if captured == nil {
		t.Fatal("expected notification to be created")
	}
	if captured.Type != NotificationTypePostApproved {
		t.Errorf("expected type %q, got %q", NotificationTypePostApproved, captured.Type)
	}
	if captured.Title != "Post approved" {
		t.Errorf("expected title 'Post approved', got %q", captured.Title)
	}
	expectedBody := `Your post "My Question" is now live on Solvr`
	if captured.Body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, captured.Body)
	}
	expectedLink := "/questions/post-uuid-123"
	if captured.Link != expectedLink {
		t.Errorf("expected link %q, got %q", expectedLink, captured.Link)
	}
}

// TestNotifyOnModerationResult_Rejected tests notification creation for rejected posts.
func TestNotifyOnModerationResult_Rejected(t *testing.T) {
	var captured *NotificationInput
	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			captured = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				AgentID:   n.AgentID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	err := svc.NotifyOnModerationResult(context.Background(), "post-uuid-789", "My Problem", "problem", "human", "user-uuid-456", false, "Content is not in English")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if captured == nil {
		t.Fatal("expected notification to be created")
	}
	if captured.Type != NotificationTypePostRejected {
		t.Errorf("expected type %q, got %q", NotificationTypePostRejected, captured.Type)
	}
	if captured.Title != "Post needs changes" {
		t.Errorf("expected title 'Post needs changes', got %q", captured.Title)
	}
	expectedBody := `Your post "My Problem" was not approved: Content is not in English. Edit and resubmit.`
	if captured.Body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, captured.Body)
	}
	expectedLink := "/problems/post-uuid-789"
	if captured.Link != expectedLink {
		t.Errorf("expected link %q, got %q", expectedLink, captured.Link)
	}
}

// TestNotifyOnModerationResult_AgentAuthor tests that AgentID is set and UserID is nil for agent authors.
func TestNotifyOnModerationResult_AgentAuthor(t *testing.T) {
	var captured *NotificationInput
	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			captured = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				AgentID:   n.AgentID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	err := svc.NotifyOnModerationResult(context.Background(), "post-uuid-100", "Agent Idea", "idea", "agent", "claude_assistant", true, "OK")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if captured == nil {
		t.Fatal("expected notification to be created")
	}
	if captured.AgentID == nil {
		t.Fatal("expected AgentID to be set")
	}
	if *captured.AgentID != "claude_assistant" {
		t.Errorf("expected AgentID 'claude_assistant', got %q", *captured.AgentID)
	}
	if captured.UserID != nil {
		t.Errorf("expected UserID to be nil, got %v", captured.UserID)
	}
}

// TestNotifyOnModerationResult_HumanAuthor tests that UserID is set and AgentID is nil for human authors.
func TestNotifyOnModerationResult_HumanAuthor(t *testing.T) {
	var captured *NotificationInput
	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			captured = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, nil, nil)

	err := svc.NotifyOnModerationResult(context.Background(), "post-uuid-200", "Human Question", "question", "human", "user-uuid-789", false, "Spam detected")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if captured == nil {
		t.Fatal("expected notification to be created")
	}
	if captured.UserID == nil {
		t.Fatal("expected UserID to be set")
	}
	if *captured.UserID != "user-uuid-789" {
		t.Errorf("expected UserID 'user-uuid-789', got %q", *captured.UserID)
	}
	if captured.AgentID != nil {
		t.Errorf("expected AgentID to be nil, got %v", captured.AgentID)
	}
}
