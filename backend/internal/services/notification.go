// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// NotificationType represents the type of notification.
type NotificationType string

// Notification types per SPEC.md Part 12.3 and notifications category.
const (
	NotificationTypeAnswerCreated   NotificationType = "answer.created"
	NotificationTypeCommentCreated  NotificationType = "comment.created"
	NotificationTypeApproachUpdated NotificationType = "approach.updated"
	NotificationTypeProblemSolved   NotificationType = "problem.solved"
	NotificationTypeMention         NotificationType = "mention"
	NotificationTypeAnswerAccepted  NotificationType = "answer.accepted"
	NotificationTypeUpvoteMilestone NotificationType = "upvote.milestone"
)

// Errors for notification service.
var (
	ErrInvalidRecipient         = errors.New("invalid recipient: must specify either userID or agentID, not both or neither")
	ErrInvalidNotificationType  = errors.New("invalid notification type")
	ErrInvalidNotificationTitle = errors.New("notification title is required")
)

// NotificationInput represents input data for creating a notification.
type NotificationInput struct {
	UserID  *string
	AgentID *string
	Type    NotificationType
	Title   string
	Body    string
	Link    string
}

// NotificationRecord represents a stored notification.
type NotificationRecord struct {
	ID        string
	UserID    *string
	AgentID   *string
	Type      NotificationType
	Title     string
	Body      string
	Link      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// CreateNotificationParams contains parameters for creating a notification.
type CreateNotificationParams struct {
	UserID  *string
	AgentID *string
	Type    NotificationType
	Title   string
	Body    string
	Link    string
}

// NewAnswerEvent represents an event when a new answer is created.
type NewAnswerEvent struct {
	AnswerID     string
	QuestionID   string
	AnswererID   string
	AnswererType string // "human" or "agent"
}

// ApproachUpdateEvent represents an event when an approach status changes.
type ApproachUpdateEvent struct {
	ApproachID string
	NewStatus  string
	OldStatus  string
}

// CommentEvent represents an event when a comment is created.
type CommentEvent struct {
	CommentID         string
	TargetType        string // "approach", "answer", "response"
	TargetID          string
	CommentAuthorID   string
	CommentAuthorType string // "human" or "agent"
}

// AcceptedAnswerEvent represents an event when an answer is accepted.
type AcceptedAnswerEvent struct {
	AnswerID   string
	QuestionID string
}

// UpvoteEvent represents an upvote event.
type UpvoteEvent struct {
	TargetType string // "post", "answer", "approach", "response"
	TargetID   string
}

// MentionEvent represents a mention event.
type MentionEvent struct {
	MentionedUsernames []string
	MentionerID        string
	MentionerType      string
	ContentType        string
	ContentID          string
	Link               string
}

// NotificationRepository defines database operations for notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *NotificationInput) (*NotificationRecord, error)
	FindByUserID(ctx context.Context, userID string, limit int) ([]NotificationRecord, error)
	FindByAgentID(ctx context.Context, agentID string, limit int) ([]NotificationRecord, error)
	GetUpvoteCount(ctx context.Context, targetType, targetID string) (int, error)
}

// UserLookup defines lookup operations for users.
type UserLookup interface {
	FindByID(ctx context.Context, id string) (*UserInfo, error)
}

// UserInfo contains basic user information.
type UserInfo struct {
	ID       string
	Username string
}

// AgentLookup defines lookup operations for agents.
type AgentLookup interface {
	FindByID(ctx context.Context, id string) (*AgentInfo, error)
}

// AgentInfo contains basic agent information.
type AgentInfo struct {
	ID          string
	DisplayName string
	HumanID     *string
}

// PostLookup defines lookup operations for posts.
type PostLookup interface {
	FindByID(ctx context.Context, id string) (*PostInfo, error)
}

// PostInfo contains basic post information.
type PostInfo struct {
	ID         string
	Type       string
	Title      string
	AuthorType string
	AuthorID   string
}

// AnswerLookup defines lookup operations for answers.
type AnswerLookup interface {
	FindByID(ctx context.Context, id string) (*AnswerInfo, error)
}

// AnswerInfo contains basic answer information.
type AnswerInfo struct {
	ID         string
	QuestionID string
	AuthorType string
	AuthorID   string
}

// ApproachLookup defines lookup operations for approaches.
type ApproachLookup interface {
	FindByID(ctx context.Context, id string) (*ApproachInfo, error)
}

// ApproachInfo contains basic approach information.
type ApproachInfo struct {
	ID         string
	ProblemID  string
	AuthorType string
	AuthorID   string
	Status     string
}

// NotificationService handles notification creation and triggers.
type NotificationService struct {
	repo           NotificationRepository
	userLookup     UserLookup
	answerLookup   AnswerLookup
	postLookup     PostLookup
	approachLookup ApproachLookup
}

// NewNotificationService creates a new notification service.
func NewNotificationService(
	repo NotificationRepository,
	userLookup UserLookup,
	answerLookup AnswerLookup,
	postLookup PostLookup,
	approachLookup ApproachLookup,
) *NotificationService {
	return &NotificationService{
		repo:           repo,
		userLookup:     userLookup,
		answerLookup:   answerLookup,
		postLookup:     postLookup,
		approachLookup: approachLookup,
	}
}

// CreateNotification creates a notification for a user or agent.
// Per SPEC.md Part 6: Either user_id OR agent_id must be set (not both, not neither).
func (s *NotificationService) CreateNotification(ctx context.Context, params *CreateNotificationParams) (*NotificationRecord, error) {
	// Validate recipient - must have exactly one of UserID or AgentID
	hasUser := params.UserID != nil && *params.UserID != ""
	hasAgent := params.AgentID != nil && *params.AgentID != ""

	if (!hasUser && !hasAgent) || (hasUser && hasAgent) {
		return nil, ErrInvalidRecipient
	}

	// Validate type
	if params.Type == "" {
		return nil, ErrInvalidNotificationType
	}

	// Validate title
	if params.Title == "" {
		return nil, ErrInvalidNotificationTitle
	}

	input := &NotificationInput{
		UserID:  params.UserID,
		AgentID: params.AgentID,
		Type:    params.Type,
		Title:   params.Title,
		Body:    params.Body,
		Link:    params.Link,
	}

	return s.repo.Create(ctx, input)
}

// NotifyOnNewAnswer sends notification to question author when answer created.
// Per PRD: "Notify on new answer" - Trigger when answer created, notify question author.
func (s *NotificationService) NotifyOnNewAnswer(ctx context.Context, event *NewAnswerEvent) error {
	// Look up the question to get the author
	question, err := s.postLookup.FindByID(ctx, event.QuestionID)
	if err != nil {
		return fmt.Errorf("failed to find question: %w", err)
	}

	// Don't notify if answering own question
	if question.AuthorType == event.AnswererType && question.AuthorID == event.AnswererID {
		return nil
	}

	// Create notification for question author
	params := &CreateNotificationParams{
		Type:  NotificationTypeAnswerCreated,
		Title: "New answer to your question",
		Body:  fmt.Sprintf("Someone answered your question: %s", question.Title),
		Link:  fmt.Sprintf("/questions/%s#answer-%s", event.QuestionID, event.AnswerID),
	}

	// Set recipient based on author type
	if question.AuthorType == "human" {
		params.UserID = &question.AuthorID
	} else {
		params.AgentID = &question.AuthorID
	}

	_, err = s.CreateNotification(ctx, params)
	return err
}

// NotifyOnApproachUpdate sends notification to problem author when approach status changes.
// Per PRD: "Notify on approach update" - Trigger on approach status change, notify problem author.
func (s *NotificationService) NotifyOnApproachUpdate(ctx context.Context, event *ApproachUpdateEvent) error {
	// Look up the approach to get problem ID
	approach, err := s.approachLookup.FindByID(ctx, event.ApproachID)
	if err != nil {
		return fmt.Errorf("failed to find approach: %w", err)
	}

	// Look up the problem to get the author
	problem, err := s.postLookup.FindByID(ctx, approach.ProblemID)
	if err != nil {
		return fmt.Errorf("failed to find problem: %w", err)
	}

	// Don't notify if updating own approach on own problem
	if problem.AuthorType == approach.AuthorType && problem.AuthorID == approach.AuthorID {
		return nil
	}

	// Create notification for problem author
	var title, body string
	switch event.NewStatus {
	case "succeeded":
		title = "Approach succeeded!"
		body = fmt.Sprintf("An approach to your problem '%s' has succeeded", problem.Title)
	case "stuck":
		title = "Approach is stuck"
		body = fmt.Sprintf("An approach to your problem '%s' needs help", problem.Title)
	case "failed":
		title = "Approach failed"
		body = fmt.Sprintf("An approach to your problem '%s' has failed", problem.Title)
	default:
		title = "Approach status updated"
		body = fmt.Sprintf("An approach to your problem '%s' changed to %s", problem.Title, event.NewStatus)
	}

	params := &CreateNotificationParams{
		Type:  NotificationTypeApproachUpdated,
		Title: title,
		Body:  body,
		Link:  fmt.Sprintf("/problems/%s#approach-%s", approach.ProblemID, event.ApproachID),
	}

	// Set recipient based on author type
	if problem.AuthorType == "human" {
		params.UserID = &problem.AuthorID
	} else {
		params.AgentID = &problem.AuthorID
	}

	_, err = s.CreateNotification(ctx, params)
	return err
}

// NotifyOnComment sends notification to content author when comment created.
// Per PRD: "Notify on comment" - Trigger on comment created, notify post/answer author.
func (s *NotificationService) NotifyOnComment(ctx context.Context, event *CommentEvent) error {
	var authorType, authorID string

	// Look up target to get author
	switch event.TargetType {
	case "answer":
		answer, err := s.answerLookup.FindByID(ctx, event.TargetID)
		if err != nil {
			return fmt.Errorf("failed to find answer: %w", err)
		}
		authorType = answer.AuthorType
		authorID = answer.AuthorID

	case "approach":
		approach, err := s.approachLookup.FindByID(ctx, event.TargetID)
		if err != nil {
			return fmt.Errorf("failed to find approach: %w", err)
		}
		authorType = approach.AuthorType
		authorID = approach.AuthorID

	default:
		// For other types (response, etc.), we'd need additional lookups
		// For now, skip notification
		return nil
	}

	// Don't notify if commenting on own content
	if authorType == event.CommentAuthorType && authorID == event.CommentAuthorID {
		return nil
	}

	params := &CreateNotificationParams{
		Type:  NotificationTypeCommentCreated,
		Title: "New comment on your content",
		Body:  "Someone commented on your content",
		Link:  fmt.Sprintf("/%ss/%s#comment-%s", event.TargetType, event.TargetID, event.CommentID),
	}

	// Set recipient based on author type
	if authorType == "human" {
		params.UserID = &authorID
	} else {
		params.AgentID = &authorID
	}

	_, err := s.CreateNotification(ctx, params)
	return err
}

// NotifyOnAcceptedAnswer sends notification to answer author when answer is accepted.
// Per PRD: "Notify on accepted answer" - Trigger when answer accepted, notify answer author.
func (s *NotificationService) NotifyOnAcceptedAnswer(ctx context.Context, event *AcceptedAnswerEvent) error {
	// Look up the answer to get author
	answer, err := s.answerLookup.FindByID(ctx, event.AnswerID)
	if err != nil {
		return fmt.Errorf("failed to find answer: %w", err)
	}

	// Look up question for title
	question, err := s.postLookup.FindByID(ctx, event.QuestionID)
	if err != nil {
		return fmt.Errorf("failed to find question: %w", err)
	}

	params := &CreateNotificationParams{
		Type:  NotificationTypeAnswerAccepted,
		Title: "Your answer was accepted!",
		Body:  fmt.Sprintf("Your answer to '%s' was accepted", question.Title),
		Link:  fmt.Sprintf("/questions/%s#answer-%s", event.QuestionID, event.AnswerID),
	}

	// Set recipient based on author type
	if answer.AuthorType == "human" {
		params.UserID = &answer.AuthorID
	} else {
		params.AgentID = &answer.AuthorID
	}

	_, err = s.CreateNotification(ctx, params)
	return err
}

// NotifyOnUpvoteMilestone sends notification at upvote milestones (10, 50, 100).
// Per PRD: "Notify on upvote milestone" - Trigger at 10, 50, 100 upvotes, notify content author.
func (s *NotificationService) NotifyOnUpvoteMilestone(ctx context.Context, event *UpvoteEvent) error {
	// Get current upvote count
	count, err := s.repo.GetUpvoteCount(ctx, event.TargetType, event.TargetID)
	if err != nil {
		return fmt.Errorf("failed to get upvote count: %w", err)
	}

	// Check if this is a milestone
	if !isUpvoteMilestone(count) {
		return nil
	}

	// For posts, look up the author
	if event.TargetType == "post" {
		post, err := s.postLookup.FindByID(ctx, event.TargetID)
		if err != nil {
			return fmt.Errorf("failed to find post: %w", err)
		}

		params := &CreateNotificationParams{
			Type:  NotificationTypeUpvoteMilestone,
			Title: fmt.Sprintf("Your post reached %d upvotes!", count),
			Body:  fmt.Sprintf("'%s' has reached %d upvotes", post.Title, count),
			Link:  fmt.Sprintf("/posts/%s", event.TargetID),
		}

		if post.AuthorType == "human" {
			params.UserID = &post.AuthorID
		} else {
			params.AgentID = &post.AuthorID
		}

		_, err = s.CreateNotification(ctx, params)
		return err
	}

	return nil
}

// NotifyOnMention sends notification when @username is mentioned in content.
// Per PRD: "Notify on mention" - Parse @username in content, notify mentioned user.
func (s *NotificationService) NotifyOnMention(ctx context.Context, event *MentionEvent, userLookup UserLookup) error {
	for _, username := range event.MentionedUsernames {
		// Look up user by username
		user, err := userLookup.FindByID(ctx, username)
		if err != nil {
			// User not found, skip
			continue
		}

		// Don't notify if mentioning self
		if event.MentionerType == "human" && user.ID == event.MentionerID {
			continue
		}

		params := &CreateNotificationParams{
			UserID: &user.ID,
			Type:   NotificationTypeMention,
			Title:  "You were mentioned",
			Body:   "Someone mentioned you in their content",
			Link:   event.Link,
		}

		if _, err := s.CreateNotification(ctx, params); err != nil {
			return fmt.Errorf("failed to create mention notification: %w", err)
		}
	}

	return nil
}

// isUpvoteMilestone checks if count is a milestone (10, 50, or 100).
func isUpvoteMilestone(count int) bool {
	return count == 10 || count == 50 || count == 100
}

// mentionRegex matches @username patterns.
// Matches @ followed by alphanumeric and underscores, not preceded by alphanumeric
// (to avoid matching emails).
var mentionRegex = regexp.MustCompile(`(?:^|[^a-zA-Z0-9])@([a-zA-Z0-9_]+)`)

// ParseMentions extracts @username mentions from content.
// Returns unique usernames in order of first occurrence.
func ParseMentions(content string) []string {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}

	seen := make(map[string]bool)
	var usernames []string

	for _, match := range matches {
		if len(match) > 1 {
			username := match[1]
			if !seen[username] {
				seen[username] = true
				usernames = append(usernames, username)
			}
		}
	}

	return usernames
}
