// Package main implements the moderate-existing CLI tool.
// It scans existing open posts and runs them through Groq content moderation,
// rejecting non-English or otherwise violating posts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// postRow holds the minimal fields needed for moderation.
type postRow struct {
	ID          string
	Title       string
	Description string
	Tags        []string
	PostedByType string
	PostedByID   string
}

// moderationResult holds the summary of a moderation run.
type moderationResult struct {
	total    int
	approved int
	rejected int
	errors   int
}

// moderationDB abstracts database operations for testing.
type moderationDB interface {
	GetOpenPosts(ctx context.Context, limit, offset int) ([]postRow, error)
	CountOpenPosts(ctx context.Context) (int, error)
	RejectPost(ctx context.Context, postID string) error
	CreateSystemComment(ctx context.Context, postID, content string) error
}

// moderationWorker orchestrates the moderation process.
type moderationWorker struct {
	db        moderationDB
	moderator *services.ContentModerationService
	batchSize int
	delay     time.Duration
	dryRun    bool
}

// truncateTitle returns the first maxLen characters of title, appending "..." if truncated.
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen] + "..."
}

// run executes the moderation process for all open posts.
func (w *moderationWorker) run(ctx context.Context) (*moderationResult, error) {
	result := &moderationResult{}

	total, err := w.db.CountOpenPosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("count open posts: %w", err)
	}
	result.total = total

	if total == 0 {
		slog.Info("No open posts to moderate")
		return result, nil
	}

	mode := "LIVE"
	if w.dryRun {
		mode = "DRY RUN"
	}
	slog.Info(fmt.Sprintf("[%s] Found %d open posts to moderate", mode, total))

	offset := 0
	for {
		if ctx.Err() != nil {
			slog.Info("Context canceled, stopping moderation")
			break
		}

		batch, err := w.db.GetOpenPosts(ctx, w.batchSize, offset)
		if err != nil {
			return result, fmt.Errorf("fetch posts batch at offset %d: %w", offset, err)
		}
		if len(batch) == 0 {
			break
		}

		for _, post := range batch {
			if ctx.Err() != nil {
				break
			}

			modResult, err := w.moderatePost(ctx, post)
			if err != nil {
				slog.Error("Moderation failed",
					"post_id", post.ID,
					"title", truncateTitle(post.Title, 50),
					"error", err,
				)
				result.errors++
				continue
			}

			status := "APPROVED"
			if !modResult.Approved {
				status = "REJECTED"
			}

			slog.Info(fmt.Sprintf("[%s] %s | %s | lang=%s | reasons=%v",
				mode,
				post.ID,
				status,
				modResult.LanguageDetected,
				modResult.RejectionReasons,
			),
				"title", truncateTitle(post.Title, 50),
			)

			if modResult.Approved {
				result.approved++
			} else {
				result.rejected++
				if !w.dryRun {
					if err := w.rejectPost(ctx, post.ID, modResult); err != nil {
						slog.Error("Failed to reject post",
							"post_id", post.ID,
							"error", err,
						)
						result.errors++
					}
				}
			}
		}

		// Delay between batches to respect rate limits
		if w.delay > 0 && offset+w.batchSize < total {
			time.Sleep(w.delay)
		}

		offset += w.batchSize
	}

	return result, nil
}

// moderatePost sends a single post through Groq moderation, handling rate limits with retries.
func (w *moderationWorker) moderatePost(ctx context.Context, post postRow) (*services.ModerationResult, error) {
	input := services.ModerationInput{
		Title:       post.Title,
		Description: post.Description,
		Tags:        post.Tags,
	}

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := w.moderator.ModerateContent(ctx, input)
		if err == nil {
			return result, nil
		}

		var rateLimitErr *services.RateLimitError
		if errors.As(err, &rateLimitErr) && attempt < maxRetries {
			retryAfter := rateLimitErr.GetRetryAfter()
			if retryAfter < time.Second {
				retryAfter = time.Second * time.Duration(attempt*30)
			}
			slog.Warn("Rate limited, waiting before retry",
				"post_id", post.ID,
				"retry_after", retryAfter,
				"attempt", attempt,
			)
			time.Sleep(retryAfter)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("max retries exceeded for post %s", post.ID)
}

// rejectPost updates the post status to rejected and creates a system comment.
func (w *moderationWorker) rejectPost(ctx context.Context, postID string, result *services.ModerationResult) error {
	if err := w.db.RejectPost(ctx, postID); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	content := fmt.Sprintf(services.ModerationRejectedFormat, result.Explanation)
	if err := w.db.CreateSystemComment(ctx, postID, content); err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	return nil
}

// pgModerationDB implements moderationDB using a real PostgreSQL connection.
type pgModerationDB struct {
	pool *db.Pool
}

func (d *pgModerationDB) GetOpenPosts(ctx context.Context, limit, offset int) ([]postRow, error) {
	query := `SELECT id, title, description, COALESCE(tags, '{}'), posted_by_type, posted_by_id
		FROM posts
		WHERE status = 'open' AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2`

	rows, err := d.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	var posts []postRow
	for rows.Next() {
		var p postRow
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Tags, &p.PostedByType, &p.PostedByID); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (d *pgModerationDB) CountOpenPosts(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE status = 'open' AND deleted_at IS NULL`,
	).Scan(&count)
	return count, err
}

func (d *pgModerationDB) RejectPost(ctx context.Context, postID string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE posts SET status = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`,
		string(models.PostStatusRejected), postID,
	)
	return err
}

func (d *pgModerationDB) CreateSystemComment(ctx context.Context, postID, content string) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		 VALUES ($1, $2, $3, $4, $5)`,
		string(models.CommentTargetPost),
		postID,
		string(models.AuthorTypeSystem),
		services.ModerationAuthorID,
		content,
	)
	return err
}

// Ensure pgModerationDB implements moderationDB at compile time.
var _ moderationDB = (*pgModerationDB)(nil)

func main() {
	databaseURL := flag.String("database-url", "", "PostgreSQL database URL (required)")
	groqAPIKey := flag.String("groq-api-key", "", "Groq API key for content moderation (required)")
	groqModel := flag.String("groq-model", "", "Groq model to use (optional, defaults to service default)")
	batchSize := flag.Int("batch-size", 10, "Number of posts to process per batch")
	delay := flag.Duration("delay", time.Second, "Delay between batches to respect rate limits")
	dryRun := flag.Bool("dry-run", true, "Preview moderation results without making changes (default: true)")
	flag.Parse()

	if *databaseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --database-url is required")
		flag.Usage()
		os.Exit(1)
	}

	if *groqAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --groq-api-key is required")
		flag.Usage()
		os.Exit(1)
	}

	// Connect to database
	ctx := context.Background()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.NewPool(connectCtx, *databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize moderation service
	var opts []services.Option
	if *groqModel != "" {
		opts = append(opts, services.WithGroqModel(*groqModel))
	}
	moderator := services.NewContentModerationService(*groqAPIKey, opts...)

	mode := "LIVE"
	if *dryRun {
		mode = "DRY RUN"
	}
	log.Printf("[%s] Starting moderation of existing posts (batch_size=%d, delay=%v)", mode, *batchSize, *delay)

	worker := &moderationWorker{
		db:        &pgModerationDB{pool: pool},
		moderator: moderator,
		batchSize: *batchSize,
		delay:     *delay,
		dryRun:    *dryRun,
	}

	result, err := worker.run(ctx)
	if err != nil {
		log.Fatalf("Moderation failed: %v", err)
	}

	// Print summary
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Moderation Summary (%s)\n", mode)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total posts scanned: %d\n", result.total)
	fmt.Printf("Approved:            %d\n", result.approved)
	fmt.Printf("Rejected:            %d\n", result.rejected)
	fmt.Printf("Errors:              %d\n", result.errors)
	fmt.Println(strings.Repeat("=", 60))
}
