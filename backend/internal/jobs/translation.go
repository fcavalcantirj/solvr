package jobs

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// Default translation job configuration.
const (
	// DefaultTranslationInterval runs the job twice daily to stay within Groq limits.
	DefaultTranslationInterval = 12 * time.Hour

	// DefaultTranslationBatchSize is the max posts to translate per run.
	DefaultTranslationBatchSize = 5

	// DefaultTranslationDelayMs is the milliseconds to sleep between API calls.
	DefaultTranslationDelayMs = 10_000 // 10 seconds
)

// TranslationPostLister lists draft posts that need translation.
type TranslationPostLister interface {
	ListPostsNeedingTranslation(ctx context.Context, limit int) ([]*models.Post, error)
}

// TranslationPostUpdater applies or records translation results.
type TranslationPostUpdater interface {
	ApplyTranslation(ctx context.Context, postID, title, description string) error
	IncrementTranslationAttempts(ctx context.Context, postID string) error
}

// PostTranslator translates post content.
type PostTranslator interface {
	TranslateContent(ctx context.Context, input services.TranslationInput) (*services.TranslationResult, error)
}

// PostModerationTrigger triggers async content moderation for a post.
type PostModerationTrigger interface {
	TriggerAsync(postID, title, description string, tags []string, postType, authorType, authorID string)
}

// TranslationJob handles periodic translation of non-English draft posts.
type TranslationJob struct {
	lister    TranslationPostLister
	updater   TranslationPostUpdater
	translator PostTranslator
	trigger   PostModerationTrigger
	batchSize int
	delayMs   int
}

// NewTranslationJob creates a new TranslationJob.
func NewTranslationJob(
	lister TranslationPostLister,
	updater TranslationPostUpdater,
	translator PostTranslator,
	trigger PostModerationTrigger,
	batchSize, delayMs int,
) *TranslationJob {
	return &TranslationJob{
		lister:    lister,
		updater:   updater,
		translator: translator,
		trigger:   trigger,
		batchSize: batchSize,
		delayMs:   delayMs,
	}
}

// RunOnce fetches the next batch of posts needing translation and processes them.
// Returns the number of successfully translated and failed posts.
func (j *TranslationJob) RunOnce(ctx context.Context) (translated, failed int) {
	posts, err := j.lister.ListPostsNeedingTranslation(ctx, j.batchSize)
	if err != nil {
		log.Printf("Translation job: failed to list candidates: %v", err)
		return 0, 0
	}

	if len(posts) == 0 {
		return 0, 0
	}

	log.Printf("Translation job: found %d posts needing translation", len(posts))

	for i, post := range posts {
		if i > 0 && j.delayMs > 0 {
			time.Sleep(time.Duration(j.delayMs) * time.Millisecond)
		}

		input := services.TranslationInput{
			Title:       post.Title,
			Description: post.Description,
			Language:    post.OriginalLanguage,
		}

		result, err := j.translator.TranslateContent(ctx, input)
		if err != nil {
			var rlErr *services.TranslationRateLimitError
			if errors.As(err, &rlErr) {
				// Rate limited: increment attempts and stop the batch
				log.Printf("Translation job: rate limited on post %s, retry after %v", post.ID, rlErr.RetryAfter)
				if incrErr := j.updater.IncrementTranslationAttempts(ctx, post.ID); incrErr != nil {
					log.Printf("Translation job: failed to increment attempts for %s: %v", post.ID, incrErr)
				}
				// Stop processing the rest of the batch
				break
			}

			// Non-rate-limit error: increment attempts and continue
			log.Printf("Translation job: failed to translate post %s: %v", post.ID, err)
			if incrErr := j.updater.IncrementTranslationAttempts(ctx, post.ID); incrErr != nil {
				log.Printf("Translation job: failed to increment attempts for %s: %v", post.ID, incrErr)
			}
			failed++
			continue
		}

		// Apply translation (sets title/description + saves originals + sets pending_review)
		if applyErr := j.updater.ApplyTranslation(ctx, post.ID, result.Title, result.Description); applyErr != nil {
			log.Printf("Translation job: failed to apply translation for %s: %v", post.ID, applyErr)
			failed++
			continue
		}

		// Trigger moderation for the now-translated post
		j.trigger.TriggerAsync(
			post.ID,
			result.Title,
			result.Description,
			post.Tags,
			string(post.Type),
			string(post.PostedByType),
			post.PostedByID,
		)

		log.Printf("Translation job: translated post %s (%s → English)", post.ID, post.OriginalLanguage)
		translated++
	}

	return translated, failed
}

// RunScheduled runs the translation job on a schedule.
// It runs immediately on start, then repeats at the given interval.
// The job stops when the context is cancelled.
func (j *TranslationJob) RunScheduled(ctx context.Context, interval time.Duration) {
	translated, failed := j.RunOnce(ctx)
	if translated > 0 || failed > 0 {
		log.Printf("Translation job: %d translated, %d failed", translated, failed)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Translation job stopped")
			return
		case <-ticker.C:
			translated, failed := j.RunOnce(ctx)
			if translated > 0 || failed > 0 {
				log.Printf("Translation job: %d translated, %d failed", translated, failed)
			}
		}
	}
}
