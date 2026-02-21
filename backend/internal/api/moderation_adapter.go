package api

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// ContentModerationAdapter adapts services.ContentModerationService to
// handlers.ContentModerationServiceInterface, bridging the type gap
// between packages without introducing an import cycle.
type ContentModerationAdapter struct {
	svc *services.ContentModerationService
}

// NewContentModerationAdapter wraps a ContentModerationService.
func NewContentModerationAdapter(svc *services.ContentModerationService) *ContentModerationAdapter {
	return &ContentModerationAdapter{svc: svc}
}

// ModerateContent delegates to the underlying service, converting types.
func (a *ContentModerationAdapter) ModerateContent(ctx context.Context, input handlers.ModerationInput) (*handlers.ModerationResult, error) {
	result, err := a.svc.ModerateContent(ctx, services.ModerationInput{
		Title:       input.Title,
		Description: input.Description,
		Tags:        input.Tags,
	})
	if err != nil {
		return nil, err
	}

	return &handlers.ModerationResult{
		Approved:         result.Approved,
		LanguageDetected: result.LanguageDetected,
		RejectionReasons: result.RejectionReasons,
		Confidence:       result.Confidence,
		Explanation:      result.Explanation,
	}, nil
}

// notifRepoForService adapts db.NotificationsRepository to services.NotificationRepository.
type notifRepoForService struct {
	create func(ctx context.Context, n *models.Notification) (*models.Notification, error)
}

func (a *notifRepoForService) Create(ctx context.Context, n *services.NotificationInput) (*services.NotificationRecord, error) {
	notif := &models.Notification{
		UserID:  n.UserID,
		AgentID: n.AgentID,
		Type:    string(n.Type),
		Title:   n.Title,
		Body:    n.Body,
		Link:    n.Link,
	}
	created, err := a.create(ctx, notif)
	if err != nil {
		return nil, err
	}
	return &services.NotificationRecord{
		ID:        created.ID,
		UserID:    created.UserID,
		AgentID:   created.AgentID,
		Type:      services.NotificationType(created.Type),
		Title:     created.Title,
		Body:      created.Body,
		Link:      created.Link,
		ReadAt:    created.ReadAt,
		CreatedAt: created.CreatedAt,
	}, nil
}

func (a *notifRepoForService) FindByUserID(ctx context.Context, userID string, limit int) ([]services.NotificationRecord, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *notifRepoForService) FindByAgentID(ctx context.Context, agentID string, limit int) ([]services.NotificationRecord, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *notifRepoForService) GetUpvoteCount(ctx context.Context, targetType, targetID string) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

// NewModerationNotificationService creates a NotificationService that only supports
// moderation notifications, wired to the given DB repo's Create method.
func NewModerationNotificationService(createFunc func(ctx context.Context, n *models.Notification) (*models.Notification, error)) *services.NotificationService {
	repo := &notifRepoForService{create: createFunc}
	return services.NewNotificationService(repo, nil, nil, nil, nil)
}
