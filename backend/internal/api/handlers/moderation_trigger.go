package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// ModerationTrigger implements jobs.PostModerationTrigger with retry logic,
// rate-limit handling, and flag creation. Used by the translation job to
// moderate translated content before publishing.
//
// Unlike PostsHandler.moderatePostAsync, this trigger does NOT detect
// language-only rejections — post-translation should only approve or reject.
type ModerationTrigger struct {
	modSvc       ContentModerationServiceInterface
	statusUpdate PostStatusUpdaterInterface
	flagCreator  FlagCreatorInterface
	commentRepo  CommentCreatorInterface
	notifService NotificationServiceInterface
	retryDelays  []time.Duration
	timeout      time.Duration
	logger       *slog.Logger
}

// NewModerationTrigger creates a ModerationTrigger with the given dependencies.
func NewModerationTrigger(
	modSvc ContentModerationServiceInterface,
	statusUpdater PostStatusUpdaterInterface,
	logger *slog.Logger,
) *ModerationTrigger {
	return &ModerationTrigger{
		modSvc:       modSvc,
		statusUpdate: statusUpdater,
		retryDelays:  defaultRetryDelays,
		timeout:      60 * time.Second,
		logger:       logger,
	}
}

// SetFlagCreator sets the flag creator for admin alerts on moderation failures.
func (t *ModerationTrigger) SetFlagCreator(fc FlagCreatorInterface) {
	t.flagCreator = fc
}

// SetCommentRepo sets the comment creator for system moderation comments.
func (t *ModerationTrigger) SetCommentRepo(repo CommentCreatorInterface) {
	t.commentRepo = repo
}

// SetNotificationService sets the notification service for author notifications.
func (t *ModerationTrigger) SetNotificationService(svc NotificationServiceInterface) {
	t.notifService = svc
}

// SetRetryDelays overrides retry delays (useful for testing).
func (t *ModerationTrigger) SetRetryDelays(delays []time.Duration) {
	t.retryDelays = delays
}

// SetTimeout overrides the context timeout (useful for testing).
func (t *ModerationTrigger) SetTimeout(d time.Duration) {
	t.timeout = d
}

// TriggerAsync implements jobs.PostModerationTrigger.
// Fires moderation in a goroutine with retry logic.
func (t *ModerationTrigger) TriggerAsync(postID, title, description string, tags []string, postType, authorType, authorID string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.logger.Error("panic in post-translation moderation", "postID", postID, "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
		defer cancel()
		t.moderate(ctx, postID, title, description, tags, postType, authorType, authorID)
	}()
}

// moderate runs content moderation with retries and updates status/comments/notifications.
func (t *ModerationTrigger) moderate(ctx context.Context, postID, title, description string, tags []string, postType, authorType, authorID string) {
	input := ModerationInput{
		Title:       title,
		Description: description,
		Tags:        tags,
	}

	maxAttempts := len(t.retryDelays)
	attempt := 0

	for attempt < maxAttempts {
		result, err := t.modSvc.ModerateContent(ctx, input)
		if err != nil {
			var rateLimitErr RateLimitError
			if errors.As(err, &rateLimitErr) {
				retryAfter := rateLimitErr.GetRetryAfter()
				t.logger.Warn("translation moderation rate limited, retrying", "postID", postID, "retryAfter", retryAfter)
				time.Sleep(retryAfter)
				continue
			}

			attempt++
			t.logger.Warn("translation moderation attempt failed", "postID", postID, "attempt", attempt, "error", err)
			if attempt < maxAttempts {
				time.Sleep(t.retryDelays[attempt-1])
				continue
			}

			t.logger.Error("translation moderation failed after all retries", "postID", postID, "attempts", attempt)
			if t.flagCreator != nil {
				parsedID, parseErr := uuid.Parse(postID)
				if parseErr != nil {
					t.logger.Error("invalid post ID for flag creation", "postID", postID, "error", parseErr)
					return
				}
				flag := &models.Flag{
					TargetType:   "post",
					TargetID:     parsedID,
					ReporterType: "system",
					ReporterID:   "translation-moderation",
					Reason:       "moderation_failed",
					Details:      fmt.Sprintf("Post-translation moderation failed after %d attempts: %v", attempt, err),
					Status:       "pending",
				}
				if _, flagErr := t.flagCreator.CreateFlag(ctx, flag); flagErr != nil {
					t.logger.Error("failed to create moderation failure flag", "postID", postID, "error", flagErr)
				}
			}
			return
		}

		// Moderation succeeded — only approve or reject (no language-only detection)
		status := models.PostStatusRejected
		if result.Approved {
			status = models.PostStatusOpen
		}

		if updateErr := t.statusUpdate.UpdateStatus(ctx, postID, status); updateErr != nil {
			t.logger.Error("failed to update post status after translation moderation", "postID", postID, "status", status, "error", updateErr)
			return
		}
		t.logger.Info("translation moderation complete", "postID", postID, "approved", result.Approved, "language", result.LanguageDetected)

		// Create system comment
		if t.commentRepo != nil {
			var commentContent string
			if result.Approved {
				commentContent = "Post approved by Solvr moderation. Your post is now visible in the feed."
			} else {
				commentContent = fmt.Sprintf("Post rejected by Solvr moderation.\n\nReason: %s\n\nYou can edit your post and resubmit for review.", result.Explanation)
			}
			comment := &models.Comment{
				TargetType: models.CommentTargetPost,
				TargetID:   postID,
				AuthorType: models.AuthorTypeSystem,
				AuthorID:   "solvr-moderator",
				Content:    commentContent,
			}
			if _, commentErr := t.commentRepo.Create(ctx, comment); commentErr != nil {
				t.logger.Error("failed to create moderation comment", "postID", postID, "error", commentErr)
			}
		}

		// Send notification
		if t.notifService != nil {
			if notifErr := t.notifService.NotifyOnModerationResult(ctx, postID, title, postType, authorType, authorID, result.Approved, result.Explanation); notifErr != nil {
				t.logger.Error("failed to send moderation notification", "postID", postID, "error", notifErr)
			}
		}
		return
	}
}
