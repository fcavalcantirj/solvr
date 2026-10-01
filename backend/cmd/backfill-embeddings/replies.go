package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// replyRow holds the minimal fields needed for reply embedding generation.
type replyRow struct {
	ID   string
	Body string
}

// runReplies embeds canonical replies without embeddings (replies.embedding, searched
// by hybrid_search_replies). Same batching contract as the other content types: always
// fetch from OFFSET 0 and skip IDs already attempted this run.
func (w *backfillWorker) runReplies(ctx context.Context, result *backfillResult) error {
	total, err := w.db.CountRepliesWithoutEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("count replies: %w", err)
	}
	result.repliesFound = total

	if total == 0 {
		slog.Info("No replies need embedding")
		return nil
	}

	if w.dryRun {
		slog.Info("Dry run: replies", "total", total, "batch_size", w.batchSize)
		fmt.Printf("Dry run: would embed %d replies in batches of %d\n", total, w.batchSize)
		return nil
	}

	slog.Info("Starting replies backfill", "total", total, "batch_size", w.batchSize, "delay", w.delayBetweenItems)

	attempted := make(map[string]bool)
	for ctx.Err() == nil {
		batch, err := w.db.GetRepliesWithoutEmbedding(ctx, w.batchSize, 0)
		if err != nil {
			return fmt.Errorf("fetch replies batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		madeProgress := false
		for _, reply := range batch {
			if ctx.Err() != nil {
				break
			}
			if attempted[reply.ID] {
				continue
			}
			attempted[reply.ID] = true
			madeProgress = true

			embedding, err := w.embeddingService.GenerateEmbedding(ctx, reply.Body)
			if err != nil {
				slog.Error("Failed to generate embedding", "reply_id", reply.ID, "error", err)
				result.repliesErrors++
			} else if written, err := w.db.UpdateReplyEmbedding(ctx, reply, embedding); err != nil {
				slog.Error("Failed to update embedding", "reply_id", reply.ID, "error", err)
				result.repliesErrors++
			} else if written {
				result.repliesEmbedded++
			} else {
				slog.Info("Reply changed since it was read; vector not stored", "reply_id", reply.ID)
				result.repliesSkipped++
			}

			if w.delayBetweenItems > 0 {
				time.Sleep(w.delayBetweenItems)
			}
		}

		if !madeProgress {
			break
		}

		processed := result.repliesEmbedded + result.repliesErrors + result.repliesSkipped
		slog.Info(fmt.Sprintf("Processed %d/%d replies (%d%%)", processed, total, processed*100/total))
	}

	return nil
}

// repliesPendingEmbedding selects live human and agent replies without a vector. System
// replies (moderation verdicts) are notes, not knowledge, and are never embedded.
const repliesPendingEmbedding = `FROM replies
		WHERE deleted_at IS NULL AND embedding IS NULL AND author_type <> 'system'`

func (d *pgBackfillDB) GetRepliesWithoutEmbedding(ctx context.Context, limit, offset int) ([]replyRow, error) {
	rows, err := d.pool.Query(ctx, `SELECT id, body `+repliesPendingEmbedding+`
		ORDER BY created_at ASC, id ASC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query replies: %w", err)
	}
	defer rows.Close()

	var replies []replyRow
	for rows.Next() {
		var r replyRow
		if err := rows.Scan(&r.ID, &r.Body); err != nil {
			return nil, fmt.Errorf("scan reply: %w", err)
		}
		replies = append(replies, r)
	}
	return replies, rows.Err()
}

func (d *pgBackfillDB) CountRepliesWithoutEmbedding(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) `+repliesPendingEmbedding).Scan(&count)
	return count, err
}

// UpdateReplyEmbedding stores the vector only onto the body it was computed from and only
// where no vector exists yet, without touching updated_at: that column is the reply's ETag
// validator, and a backfill is not an edit.
func (d *pgBackfillDB) UpdateReplyEmbedding(ctx context.Context, reply replyRow, embedding []float32) (bool, error) {
	tag, err := d.pool.Exec(ctx,
		`UPDATE replies SET embedding = $1::vector
		  WHERE id = $2 AND deleted_at IS NULL AND embedding IS NULL AND body = $3`,
		float32SliceToVectorString(embedding), reply.ID, reply.Body,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
