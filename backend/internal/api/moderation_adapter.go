package api

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
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
