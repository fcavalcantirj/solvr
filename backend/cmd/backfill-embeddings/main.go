// Package main implements the backfill-embeddings CLI tool.
// It generates embeddings for existing posts, answers, and approaches that don't have one.
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

// answerRow holds the minimal fields needed for answer embedding generation.
type answerRow struct {
	ID      string
	Content string
}

// approachRow holds the minimal fields needed for approach embedding generation.
type approachRow struct {
	ID       string
	Angle    string
	Method   string
	Outcome  string
	Solution string
}

// backfillDB abstracts database operations for testing.
type backfillDB interface {
	GetPostsWithoutEmbedding(ctx context.Context, limit, offset int) ([]postRow, error)
	CountPostsWithoutEmbedding(ctx context.Context) (int, error)
	UpdatePostEmbedding(ctx context.Context, id string, embedding []float32) error
	GetAnswersWithoutEmbedding(ctx context.Context, limit, offset int) ([]answerRow, error)
	CountAnswersWithoutEmbedding(ctx context.Context) (int, error)
	UpdateAnswerEmbedding(ctx context.Context, id string, embedding []float32) error
	GetApproachesWithoutEmbedding(ctx context.Context, limit, offset int) ([]approachRow, error)
	CountApproachesWithoutEmbedding(ctx context.Context) (int, error)
	UpdateApproachEmbedding(ctx context.Context, id string, embedding []float32) error
}

// backfillResult holds the summary of a backfill run.
type backfillResult struct {
	totalFound         int
	embedded           int
	errors             int
	postsFound         int
	postsEmbedded      int
	postsErrors        int
	answersFound       int
	answersEmbedded    int
	answersErrors      int
	approachesFound    int
	approachesEmbedded int
	approachesErrors   int
}

// backfillWorker orchestrates the backfill process.
type backfillWorker struct {
	db               backfillDB
	embeddingService services.EmbeddingService
	batchSize        int
	dryRun           bool
	delayBetweenItems time.Duration
	contentTypes     []string // which content types to process
}

// parseContentTypes parses a comma-separated content types string.
// Valid values: "posts", "answers", "approaches", "all" (default).
func parseContentTypes(s string) []string {
	if s == "" || s == "all" {
		return []string{"posts", "answers", "approaches"}
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "posts" || p == "answers" || p == "approaches" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{"posts", "answers", "approaches"}
	}
	return result
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

	if w.shouldProcess("answers") {
		if err := w.runAnswers(ctx, result); err != nil {
			return result, err
		}
	}

	if w.shouldProcess("approaches") {
		if err := w.runApproaches(ctx, result); err != nil {
			return result, err
		}
	}

	// Aggregate totals
	result.totalFound = result.postsFound + result.answersFound + result.approachesFound
	result.embedded = result.postsEmbedded + result.answersEmbedded + result.approachesEmbedded
	result.errors = result.postsErrors + result.answersErrors + result.approachesErrors

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

			if err := w.db.UpdatePostEmbedding(ctx, post.ID, embedding); err != nil {
				slog.Error("Failed to update embedding", "post_id", post.ID, "error", err)
				result.postsErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			result.postsEmbedded++

			if w.delayBetweenItems > 0 {
				time.Sleep(w.delayBetweenItems)
			}
		}

		if !madeProgress {
			break // All remaining items already attempted; exit cleanly.
		}

		processed := result.postsEmbedded + result.postsErrors
		pct := 0
		if total > 0 {
			pct = processed * 100 / total
		}
		slog.Info(fmt.Sprintf("Processed %d/%d posts (%d%%)", processed, total, pct))
	}

	return nil
}

// runAnswers embeds answers without embeddings.
func (w *backfillWorker) runAnswers(ctx context.Context, result *backfillResult) error {
	total, err := w.db.CountAnswersWithoutEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("count answers: %w", err)
	}
	result.answersFound = total

	if total == 0 {
		slog.Info("No answers need embedding")
		return nil
	}

	if w.dryRun {
		slog.Info("Dry run: answers",
			"total", total,
			"batch_size", w.batchSize,
		)
		fmt.Printf("Dry run: would embed %d answers in batches of %d\n", total, w.batchSize)
		return nil
	}

	slog.Info("Starting answers backfill", "total", total, "batch_size", w.batchSize, "delay", w.delayBetweenItems)

	attempted := make(map[string]bool)
	for {
		if ctx.Err() != nil {
			slog.Info("Context canceled, stopping answers backfill")
			break
		}

		batch, err := w.db.GetAnswersWithoutEmbedding(ctx, w.batchSize, 0)
		if err != nil {
			return fmt.Errorf("fetch answers batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		madeProgress := false
		for _, answer := range batch {
			if ctx.Err() != nil {
				break
			}
			if attempted[answer.ID] {
				continue
			}
			attempted[answer.ID] = true
			madeProgress = true

			embedding, err := w.embeddingService.GenerateEmbedding(ctx, answer.Content)
			if err != nil {
				slog.Error("Failed to generate embedding", "answer_id", answer.ID, "error", err)
				result.answersErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			if err := w.db.UpdateAnswerEmbedding(ctx, answer.ID, embedding); err != nil {
				slog.Error("Failed to update embedding", "answer_id", answer.ID, "error", err)
				result.answersErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			result.answersEmbedded++

			if w.delayBetweenItems > 0 {
				time.Sleep(w.delayBetweenItems)
			}
		}

		if !madeProgress {
			break
		}

		processed := result.answersEmbedded + result.answersErrors
		pct := 0
		if total > 0 {
			pct = processed * 100 / total
		}
		slog.Info(fmt.Sprintf("Processed %d/%d answers (%d%%)", processed, total, pct))
	}

	return nil
}

// buildApproachText combines approach fields into embedding input text.
// Empty outcome and solution fields are omitted.
func buildApproachText(a approachRow) string {
	parts := []string{a.Angle, a.Method}
	if a.Outcome != "" {
		parts = append(parts, a.Outcome)
	}
	if a.Solution != "" {
		parts = append(parts, a.Solution)
	}
	return strings.Join(parts, " ")
}

// runApproaches embeds approaches without embeddings.
func (w *backfillWorker) runApproaches(ctx context.Context, result *backfillResult) error {
	total, err := w.db.CountApproachesWithoutEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("count approaches: %w", err)
	}
	result.approachesFound = total

	if total == 0 {
		slog.Info("No approaches need embedding")
		return nil
	}

	if w.dryRun {
		slog.Info("Dry run: approaches",
			"total", total,
			"batch_size", w.batchSize,
		)
		fmt.Printf("Dry run: would embed %d approaches in batches of %d\n", total, w.batchSize)
		return nil
	}

	slog.Info("Starting approaches backfill", "total", total, "batch_size", w.batchSize, "delay", w.delayBetweenItems)

	attempted := make(map[string]bool)
	for {
		if ctx.Err() != nil {
			slog.Info("Context canceled, stopping approaches backfill")
			break
		}

		batch, err := w.db.GetApproachesWithoutEmbedding(ctx, w.batchSize, 0)
		if err != nil {
			return fmt.Errorf("fetch approaches batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		madeProgress := false
		for _, approach := range batch {
			if ctx.Err() != nil {
				break
			}
			if attempted[approach.ID] {
				continue
			}
			attempted[approach.ID] = true
			madeProgress = true

			text := buildApproachText(approach)
			embedding, err := w.embeddingService.GenerateEmbedding(ctx, text)
			if err != nil {
				slog.Error("Failed to generate embedding", "approach_id", approach.ID, "error", err)
				result.approachesErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			if err := w.db.UpdateApproachEmbedding(ctx, approach.ID, embedding); err != nil {
				slog.Error("Failed to update embedding", "approach_id", approach.ID, "error", err)
				result.approachesErrors++
				if w.delayBetweenItems > 0 {
					time.Sleep(w.delayBetweenItems)
				}
				continue
			}

			result.approachesEmbedded++

			if w.delayBetweenItems > 0 {
				time.Sleep(w.delayBetweenItems)
			}
		}

		if !madeProgress {
			break
		}

		processed := result.approachesEmbedded + result.approachesErrors
		pct := 0
		if total > 0 {
			pct = processed * 100 / total
		}
		slog.Info(fmt.Sprintf("Processed %d/%d approaches (%d%%)", processed, total, pct))
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

func (d *pgBackfillDB) UpdatePostEmbedding(ctx context.Context, id string, embedding []float32) error {
	vecStr := float32SliceToVectorString(embedding)
	_, err := d.pool.Exec(ctx,
		`UPDATE posts SET embedding = $1::vector, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`,
		vecStr, id,
	)
	return err
}

func (d *pgBackfillDB) GetAnswersWithoutEmbedding(ctx context.Context, limit, offset int) ([]answerRow, error) {
	query := `SELECT id, content FROM answers
		WHERE deleted_at IS NULL AND embedding IS NULL
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2`

	rows, err := d.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query answers: %w", err)
	}
	defer rows.Close()

	var answers []answerRow
	for rows.Next() {
		var a answerRow
		if err := rows.Scan(&a.ID, &a.Content); err != nil {
			return nil, fmt.Errorf("scan answer: %w", err)
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func (d *pgBackfillDB) CountAnswersWithoutEmbedding(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM answers WHERE deleted_at IS NULL AND embedding IS NULL`,
	).Scan(&count)
	return count, err
}

func (d *pgBackfillDB) UpdateAnswerEmbedding(ctx context.Context, id string, embedding []float32) error {
	vecStr := float32SliceToVectorString(embedding)
	_, err := d.pool.Exec(ctx,
		`UPDATE answers SET embedding = $1::vector WHERE id = $2 AND deleted_at IS NULL`,
		vecStr, id,
	)
	return err
}

func (d *pgBackfillDB) GetApproachesWithoutEmbedding(ctx context.Context, limit, offset int) ([]approachRow, error) {
	query := `SELECT id, angle, COALESCE(method, ''), COALESCE(outcome, ''), COALESCE(solution, '')
		FROM approaches
		WHERE deleted_at IS NULL AND embedding IS NULL
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2`

	rows, err := d.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query approaches: %w", err)
	}
	defer rows.Close()

	var approaches []approachRow
	for rows.Next() {
		var a approachRow
		if err := rows.Scan(&a.ID, &a.Angle, &a.Method, &a.Outcome, &a.Solution); err != nil {
			return nil, fmt.Errorf("scan approach: %w", err)
		}
		approaches = append(approaches, a)
	}
	return approaches, rows.Err()
}

func (d *pgBackfillDB) CountApproachesWithoutEmbedding(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM approaches WHERE deleted_at IS NULL AND embedding IS NULL`,
	).Scan(&count)
	return count, err
}

func (d *pgBackfillDB) UpdateApproachEmbedding(ctx context.Context, id string, embedding []float32) error {
	vecStr := float32SliceToVectorString(embedding)
	_, err := d.pool.Exec(ctx,
		`UPDATE approaches SET embedding = $1::vector, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`,
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
	batchSize := flag.Int("batch-size", 100, "Number of items to process per batch")
	dryRun := flag.Bool("dry-run", false, "Show what would be embedded without making changes")
	delayMs := flag.Int("delay-ms", 20, "Delay in milliseconds between each embedding API call (default 20ms ≈ 50/sec; use 22000 for ~3 RPM free tier)")
	contentTypesFlag := flag.String("content-types", "all", "Content types to embed: posts, answers, approaches, all (comma-separated)")
	flag.Parse()

	contentTypes := parseContentTypes(*contentTypesFlag)

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
		db:               &pgBackfillDB{pool: pool},
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

	fmt.Printf("Backfill complete: %d posts, %d answers, %d approaches embedded\n",
		result.postsEmbedded, result.answersEmbedded, result.approachesEmbedded)
	if result.errors > 0 {
		fmt.Printf("Errors: %d posts, %d answers, %d approaches\n",
			result.postsErrors, result.answersErrors, result.approachesErrors)
	}
}
