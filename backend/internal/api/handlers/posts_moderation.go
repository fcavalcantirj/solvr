package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// moderatePostAsync runs content moderation asynchronously with retry logic.
// Uses context.Background() with 30s timeout (not request context).
func (h *PostsHandler) moderatePostAsync(postID, title, description string, tags []string, postType, authorType, authorID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := ModerationInput{
		Title:       title,
		Description: description,
		Tags:        tags,
	}

	maxAttempts := len(h.retryDelays)
	attempt := 0

	for attempt < maxAttempts {
		result, err := h.contentModService.ModerateContent(ctx, input)
		if err != nil {
			// Rate limit errors: sleep and retry without counting as attempt
			var rateLimitErr RateLimitError
			if errors.As(err, &rateLimitErr) {
				retryAfter := rateLimitErr.GetRetryAfter()
				h.logger.Warn("moderation rate limited, retrying", "postID", postID, "retryAfter", retryAfter)
				time.Sleep(retryAfter)
				continue
			}

			// Other errors: count as attempt and use exponential backoff
			attempt++
			h.logger.Warn("moderation attempt failed", "postID", postID, "attempt", attempt, "error", err)
			if attempt < maxAttempts {
				time.Sleep(h.retryDelays[attempt-1])
				continue
			}

			// All retries exhausted
			h.logger.Error("moderation failed after all retries", "postID", postID, "attempts", attempt)
			if h.flagCreator != nil {
				parsedID, parseErr := uuid.Parse(postID)
				if parseErr != nil {
					h.logger.Error("invalid post ID for flag creation", "postID", postID, "error", parseErr)
					return
				}
				flag := &models.Flag{
					TargetType:   "post",
					TargetID:     parsedID,
					ReporterType: "system",
					ReporterID:   "content-moderation",
					Reason:       "moderation_failed",
					Details:      fmt.Sprintf("Content moderation failed after %d attempts: %v", attempt, err),
					Status:       "pending",
				}
				if _, flagErr := h.flagCreator.CreateFlag(ctx, flag); flagErr != nil {
					h.logger.Error("failed to create moderation failure flag", "postID", postID, "error", flagErr)
				}
			}
			return
		}

		// Moderation succeeded - update status
		if h.statusUpdater == nil {
			h.logger.Error("no status updater configured", "postID", postID)
			return
		}

		var newStatus models.PostStatus
		var languageOnlyRejection bool
		if result.Approved {
			newStatus = models.PostStatusOpen
		} else if isLanguageOnlyRejection(result) {
			// Language-only rejection → save as draft for auto-translation
			newStatus = models.PostStatusDraft
			languageOnlyRejection = true
		} else {
			newStatus = models.PostStatusRejected
		}

		if languageOnlyRejection {
			// Use UpdateOriginalLanguage to set status=draft and record the language in one call.
			if err := h.statusUpdater.UpdateOriginalLanguage(ctx, postID, result.LanguageDetected); err != nil {
				h.logger.Error("failed to set original language after language rejection", "postID", postID, "language", result.LanguageDetected, "error", err)
			}
		} else {
			if err := h.statusUpdater.UpdateStatus(ctx, postID, newStatus); err != nil {
				h.logger.Error("failed to update post status after moderation", "postID", postID, "status", newStatus, "error", err)
			}
		}

		// Create system comment explaining the moderation decision
		if h.commentRepo != nil {
			var commentContent string
			if result.Approved {
				commentContent = "Post approved by Solvr moderation. Your post is now visible in the feed."
			} else if languageOnlyRejection {
				commentContent = fmt.Sprintf(
					"Your post appears to be in %s. We'll automatically translate it to English and resubmit for review — this typically takes up to 24 hours.",
					result.LanguageDetected,
				)
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
			if _, commentErr := h.commentRepo.Create(ctx, comment); commentErr != nil {
				h.logger.Error("failed to create moderation comment", "postID", postID, "error", commentErr)
			}
		}

		// Send notification to post author about moderation result
		if h.notifService != nil {
			if notifErr := h.notifService.NotifyOnModerationResult(ctx, postID, title, postType, authorType, authorID, result.Approved, result.Explanation); notifErr != nil {
				h.logger.Error("failed to send moderation notification", "postID", postID, "error", notifErr)
			}
		}
		return
	}
}

// isLanguageOnlyRejection returns true when the post was rejected exclusively
// because of language (not spam, injection, or relevance).
// Only triggers auto-translation when LANGUAGE is the sole rejection reason.
func isLanguageOnlyRejection(result *ModerationResult) bool {
	if result.Approved || result.LanguageDetected == "" {
		return false
	}
	lang := strings.ToLower(result.LanguageDetected)
	if lang == "en" || lang == "english" {
		return false
	}
	// Only trigger translation if LANGUAGE is the sole rejection reason
	return len(result.RejectionReasons) == 1 &&
		strings.ToUpper(result.RejectionReasons[0]) == "LANGUAGE"
}
