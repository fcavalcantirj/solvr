package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// TranslationApplier applies a translation result to a post in the database.
type TranslationApplier interface {
	ApplyTranslation(ctx context.Context, postID, title, description string) error
	IncrementTranslationAttempts(ctx context.Context, postID string) error
}

// TranslationTriggerAdapter bridges the services and handlers packages,
// triggering immediate translation in a goroutine when a language-only
// rejection is detected. Follows the same adapter pattern as
// ContentModerationAdapter in moderation_adapter.go.
type TranslationTriggerAdapter struct {
	translator *services.TranslationService
	updater    TranslationApplier
	moderator  *handlers.ModerationTrigger
	logger     *slog.Logger
}

// NewTranslationTriggerAdapter creates a new TranslationTriggerAdapter.
func NewTranslationTriggerAdapter(
	translator *services.TranslationService,
	updater TranslationApplier,
	moderator *handlers.ModerationTrigger,
	logger *slog.Logger,
) *TranslationTriggerAdapter {
	return &TranslationTriggerAdapter{
		translator: translator,
		updater:    updater,
		moderator:  moderator,
		logger:     logger,
	}
}

// TranslateAndModerateAsync translates a post and triggers re-moderation in a goroutine.
// Uses context.Background() with its own 30s timeout — the calling goroutine's
// HTTP request context may be cancelled before translation completes.
func (a *TranslationTriggerAdapter) TranslateAndModerateAsync(
	postID, title, description string, tags []string,
	language, postType, authorType, authorID string,
) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error("panic in inline translation", "postID", postID, "panic", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		input := services.TranslationInput{
			Title:       title,
			Description: description,
			Language:    language,
		}

		result, err := a.translator.TranslateContent(ctx, input)
		if err != nil {
			// Translation failed — increment attempts. The hourly sweep will retry later.
			a.logger.Warn("inline translation failed, sweep will retry", "postID", postID, "error", err)
			if incrErr := a.updater.IncrementTranslationAttempts(ctx, postID); incrErr != nil {
				a.logger.Error("failed to increment translation attempts", "postID", postID, "error", incrErr)
			}
			return
		}

		// Apply translation (saves originals, overwrites with English, sets pending_review)
		if applyErr := a.updater.ApplyTranslation(ctx, postID, result.Title, result.Description); applyErr != nil {
			a.logger.Error("inline translation: failed to apply", "postID", postID, "error", applyErr)
			return
		}

		// Trigger re-moderation on the translated English content
		a.moderator.TriggerAsync(postID, result.Title, result.Description, tags, postType, authorType, authorID)

		a.logger.Info("inline translation complete", "postID", postID, "language", language)
	}()
}
