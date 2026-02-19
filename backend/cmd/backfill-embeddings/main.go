// Package main implements the backfill-embeddings CLI tool.
// It generates embeddings for existing posts that don't have one.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
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
	UpdatePostEmbedding(ctx context.Context, id string, embedding []float32) error
}

// backfillResult holds the summary of a backfill run.
type backfillResult struct {
	totalFound int
	embedded   int
	errors     int
}

// backfillWorker orchestrates the backfill process.
type backfillWorker struct {
	db               backfillDB
	embeddingService services.EmbeddingService
	batchSize        int
	dryRun           bool
	rateLimit        int // posts per second
}

// run executes the backfill process.
func (w *backfillWorker) run(ctx context.Context) (*backfillResult, error) {
	total, err := w.db.CountPostsWithoutEmbedding(ctx)
	if err != nil {
		return nil, fmt.Errorf("count posts: %w", err)
	}

	result := &backfillResult{totalFound: total}

	if total == 0 {
		slog.Info("No posts need embedding")
		return result, nil
	}

	if w.dryRun {
		slog.Info("Dry run mode",
			"total_posts", total,
			"batch_size", w.batchSize,
			"batches", (total+w.batchSize-1)/w.batchSize,
		)
		fmt.Printf("Dry run: would embed %d posts in batches of %d\n", total, w.batchSize)
		return result, nil
	}

	slog.Info("Starting backfill",
		"total_posts", total,
		"batch_size", w.batchSize,
		"rate_limit", w.rateLimit,
	)

	// Calculate delay between posts for rate limiting
	var delay time.Duration
	if w.rateLimit > 0 {
		delay = time.Second / time.Duration(w.rateLimit)
	}

	offset := 0
	for {
		if ctx.Err() != nil {
			slog.Info("Context canceled, stopping backfill")
			break
		}

		batch, err := w.db.GetPostsWithoutEmbedding(ctx, w.batchSize, offset)
		if err != nil {
			return nil, fmt.Errorf("fetch batch at offset %d: %w", offset, err)
		}
		if len(batch) == 0 {
			break
		}

		for _, post := range batch {
			if ctx.Err() != nil {
				slog.Info("Context canceled, stopping backfill")
				break
			}

			text := post.Title + " " + post.Description
			embedding, err := w.embeddingService.GenerateEmbedding(ctx, text)
			if err != nil {
				slog.Error("Failed to generate embedding",
					"post_id", post.ID,
					"error", err,
				)
				result.errors++
				continue
			}

			if err := w.db.UpdatePostEmbedding(ctx, post.ID, embedding); err != nil {
				slog.Error("Failed to update embedding",
					"post_id", post.ID,
					"error", err,
				)
				result.errors++
				continue
			}

			result.embedded++

			// Rate limiting
			if delay > 0 {
				time.Sleep(delay)
			}
		}

		processed := result.embedded + result.errors
		pct := 0
		if total > 0 {
			pct = processed * 100 / total
		}
		slog.Info(fmt.Sprintf("Processed %d/%d posts (%d%%)", processed, total, pct))

		offset += w.batchSize
	}

	return result, nil
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

func (d *pgBackfillDB) UpdatePostEmbedding(ctx context.Context, id string, embedding []float32) error {
	// Convert []float32 to PostgreSQL vector string format: [0.1,0.2,0.3]
	vecStr := float32SliceToVectorString(embedding)
	_, err := d.pool.Exec(ctx,
		`UPDATE posts SET embedding = $1::vector, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`,
		vecStr, id,
	)
	return err
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
	batchSize := flag.Int("batch-size", 100, "Number of posts to process per batch")
	dryRun := flag.Bool("dry-run", false, "Show what would be embedded without making changes")
	flag.Parse()

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

	worker := &backfillWorker{
		db:               &pgBackfillDB{pool: pool},
		embeddingService: embeddingService,
		batchSize:        *batchSize,
		dryRun:           *dryRun,
		rateLimit:        50, // 50 posts/second to respect Voyage API free tier limits
	}

	result, err := worker.run(ctx)
	if err != nil {
		log.Fatalf("Backfill failed: %v", err)
	}

	fmt.Printf("Backfill complete: %d posts embedded, %d errors\n", result.embedded, result.errors)
}
