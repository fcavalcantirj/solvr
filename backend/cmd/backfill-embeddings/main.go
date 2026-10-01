// Package main implements the backfill-embeddings CLI tool.
// It generates embeddings for existing posts and replies that don't have one. Answers and
// approaches are replies after the contribution cutover, which copies their vectors.
// It is the rebuild path of the search documents (migration 000122): it embeds the rows
// search_document_drift() lists, from their current text.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/config"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// postRow holds the minimal fields needed for embedding generation.
type postRow struct {
	ID          string
	Title       string
	Description string
}

// backfillDB abstracts database operations for testing.
type backfillDB interface {
	GetPostsWithoutEmbedding(ctx context.Context, limit, offset int) ([]postRow, error)
	CountPostsWithoutEmbedding(ctx context.Context) (int, error)
	// UpdatePostEmbedding stores the vector computed from post's text; written is false when
	// the row no longer holds that text, already has a vector, or was deleted since the read.
	UpdatePostEmbedding(ctx context.Context, post postRow, embedding []float32) (written bool, err error)
	GetRepliesWithoutEmbedding(ctx context.Context, limit, offset int) ([]replyRow, error)
	CountRepliesWithoutEmbedding(ctx context.Context) (int, error)
	UpdateReplyEmbedding(ctx context.Context, reply replyRow, embedding []float32) (written bool, err error)
}

// backfillResult holds the summary of a backfill run.
type backfillResult struct {
	totalFound      int
	embedded        int
	errors          int
	skipped         int
	postsFound      int
	postsEmbedded   int
	postsErrors     int
	postsSkipped    int
	repliesFound    int
	repliesEmbedded int
	repliesErrors   int
	repliesSkipped  int
}

// backfillWorker orchestrates the backfill process.
type backfillWorker struct {
	db                backfillDB
	embeddingService  services.EmbeddingService
	batchSize         int
	dryRun            bool
	delayBetweenItems time.Duration
	contentTypes      []string // which content types to process
}

// parseContentTypes parses a comma-separated content types string.
// Valid values: "posts", "replies", "all" (default). The retired "answers" and "approaches"
// types are an error: those contributions are replies now, embedded by the "replies" type.
func parseContentTypes(s string) ([]string, error) {
	all := []string{"posts", "replies"}
	if s == "" || s == "all" {
		return all, nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "posts", "replies":
			result = append(result, p)
		case "answers", "approaches":
			return nil, fmt.Errorf("content type %q was retired: answers and approaches are replies now, use -content-types=replies", p)
		}
	}
	if len(result) == 0 {
		return all, nil
	}
	return result, nil
}

// shouldProcess returns true if the given content type is in the worker's contentTypes list.
func (w *backfillWorker) shouldProcess(contentType string) bool {
	for _, ct := range w.contentTypes {
		if ct == contentType {
			return true
		}
	}
	return false
}

// run executes the backfill process for all configured content types.
func (w *backfillWorker) run(ctx context.Context) (*backfillResult, error) {
	result := &backfillResult{}

	if w.shouldProcess("posts") {
		if err := w.runPosts(ctx, result); err != nil {
			return result, err
		}
	}

	if w.shouldProcess("replies") {
		if err := w.runReplies(ctx, result); err != nil {
			return result, err
		}
	}

	// Aggregate totals
	result.totalFound = result.postsFound + result.repliesFound
	result.embedded = result.postsEmbedded + result.repliesEmbedded
	result.errors = result.postsErrors + result.repliesErrors
	result.skipped = result.postsSkipped + result.repliesSkipped

	return result, nil
}

// runPosts embeds posts without embeddings.
func (w *backfillWorker) runPosts(ctx context.Context, result *backfillResult) error {
	total, err := w.db.CountPostsWithoutEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("count posts: %w", err)
	}
	result.postsFound = total

	if total == 0 {
		slog.Info("No posts need embedding")
		return nil
	}

	if w.dryRun {
		slog.Info("Dry run: posts",
			"total", total,
			"batch_size", w.batchSize,
		)
		fmt.Printf("Dry run: would embed %d posts in batches of %d\n", total, w.batchSize)
		return nil
	}

	slog.Info("Starting posts backfill", "total", total, "batch_size", w.batchSize, "delay", w.delayBetweenItems)

	// Track IDs attempted this run so failed items don't loop forever.
	attempted := make(map[string]bool)
	for {
		if ctx.Err() != nil {
			slog.Info("Context canceled, stopping posts backfill")
			break
		}

		// Always fetch from OFFSET 0: successfully embedded items drop out of the query.
		batch, err := w.db.GetPostsWithoutEmbedding(ctx, w.batchSize, 0)
		if err != nil {
			return fmt.Errorf("fetch posts batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		madeProgress := false
		for _, post := range batch {
			if ctx.Err() != nil {
				break
			}
			if attempted[post.ID] {
				continue
			}
			attempted[post.ID] = true
			madeProgress = true

			text := post.Title + " " + post.Description
			embedding, err := w.embeddingService.GenerateEmbedding(ctx, text)
			if err != nil {
				slog.Error("Failed to generate embedding", "post_id", post.ID, "error", err)
				result.postsErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			written, err := w.db.UpdatePostEmbedding(ctx, post, embedding)
			if err != nil {
				slog.Error("Failed to update embedding", "post_id", post.ID, "error", err)
				result.postsErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			if written {
				result.postsEmbedded++
			} else {
				// Edited, embedded by another writer or deleted since the read: a row still
				// without a vector is embedded from its new text by the next run.
				slog.Info("Post changed since it was read; vector not stored", "post_id", post.ID)
				result.postsSkipped++
			}

			if w.delayBetweenItems > 0 {
				time.Sleep(w.delayBetweenItems)
			}
		}

		if !madeProgress {
			break // All remaining items already attempted; exit cleanly.
		}

		processed := result.postsEmbedded + result.postsErrors + result.postsSkipped
		pct := 0
		if total > 0 {
			pct = processed * 100 / total
		}
		slog.Info(fmt.Sprintf("Processed %d/%d posts (%d%%)", processed, total, pct))
	}

	return nil
}

// pgBackfillDB implements backfillDB using a real PostgreSQL connection.
type pgBackfillDB struct {
	pool *db.Pool
}

func (d *pgBackfillDB) GetPostsWithoutEmbedding(ctx context.Context, limit, offset int) ([]postRow, error) {
	query := `SELECT id, title, description FROM posts
		WHERE deleted_at IS NULL AND embedding IS NULL
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
		if err := rows.Scan(&p.ID, &p.Title, &p.Description); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (d *pgBackfillDB) CountPostsWithoutEmbedding(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND embedding IS NULL`,
	).Scan(&count)
	return count, err
}

// UpdatePostEmbedding stores the vector only onto the text it was computed from and only
// where no vector exists yet, without touching updated_at: that column is the post's ETag
// validator (If-Match), and a backfill is not an edit.
func (d *pgBackfillDB) UpdatePostEmbedding(ctx context.Context, post postRow, embedding []float32) (bool, error) {
	tag, err := d.pool.Exec(ctx,
		`UPDATE posts SET embedding = $1::vector
		  WHERE id = $2 AND deleted_at IS NULL AND embedding IS NULL AND title = $3 AND description = $4`,
		float32SliceToVectorString(embedding), post.ID, post.Title, post.Description,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// float32SliceToVectorString converts a float32 slice to PostgreSQL vector literal format.
// Example: [0.1, 0.2, 0.3] -> "[0.1,0.2,0.3]"
func float32SliceToVectorString(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	s := "["
	for i, f := range v {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%g", f)
	}
	s += "]"
	return s
}

func main() {
	batchSize := flag.Int("batch-size", 100, "Number of items to process per batch")
	dryRun := flag.Bool("dry-run", false, "Show what would be embedded without making changes")
	delayMs := flag.Int("delay-ms", 20, "Delay in milliseconds between each embedding API call (default 20ms ≈ 50/sec; use 22000 for ~3 RPM free tier)")
	contentTypesFlag := flag.String("content-types", "all", "Content types to embed: posts, replies, all (comma-separated)")
	flag.Parse()

	contentTypes, err := parseContentTypes(*contentTypesFlag)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Connect to database
	ctx := context.Background()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.NewPool(connectCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize embedding service
	var embeddingService services.EmbeddingService
	provider := cfg.EmbeddingProvider
	if provider == "" {
		provider = "voyage"
	}
	switch provider {
	case "ollama":
		embeddingService = services.NewOllamaEmbeddingService(cfg.OllamaBaseURL)
		log.Printf("Embedding service: ollama (base URL: %s)", cfg.OllamaBaseURL)
	default:
		if cfg.VoyageAPIKey == "" {
			log.Fatal("VOYAGE_API_KEY is required for voyage embedding provider")
		}
		embeddingService = services.NewVoyageEmbeddingService(cfg.VoyageAPIKey)
		log.Println("Embedding service: voyage")
	}

	log.Printf("Content types: %s", strings.Join(contentTypes, ", "))

	worker := &backfillWorker{
		db:                &pgBackfillDB{pool: pool},
		embeddingService:  embeddingService,
		batchSize:         *batchSize,
		dryRun:            *dryRun,
		delayBetweenItems: time.Duration(*delayMs) * time.Millisecond,
		contentTypes:      contentTypes,
	}

	result, err := worker.run(ctx)
	if err != nil {
		log.Fatalf("Backfill failed: %v", err)
	}

	fmt.Printf("Backfill complete: %d posts, %d replies embedded\n", result.postsEmbedded, result.repliesEmbedded)
	if result.skipped > 0 {
		fmt.Printf("Changed while embedding (run again): %d posts, %d replies\n", result.postsSkipped, result.repliesSkipped)
	}
	if result.errors > 0 {
		fmt.Printf("Errors: %d posts, %d replies\n", result.postsErrors, result.repliesErrors)
	}
}
