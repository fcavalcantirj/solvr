package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// EmailBroadcastRepository handles database operations for email broadcast logs.
type EmailBroadcastRepository struct {
	pool *Pool
}

// NewEmailBroadcastRepository creates a new EmailBroadcastRepository.
func NewEmailBroadcastRepository(pool *Pool) *EmailBroadcastRepository {
	return &EmailBroadcastRepository{pool: pool}
}

// CreateLog inserts a new email broadcast log entry and returns it with server-generated fields.
func (r *EmailBroadcastRepository) CreateLog(ctx context.Context, broadcast *models.EmailBroadcast) (*models.EmailBroadcast, error) {
	query := `
		INSERT INTO email_broadcast_logs (subject, body_html, body_text, total_recipients, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, subject, body_html, COALESCE(body_text, '') AS body_text,
		          total_recipients, sent_count, failed_count, status,
		          started_at, completed_at, created_at
	`

	var result models.EmailBroadcast
	err := r.pool.QueryRow(ctx, query,
		broadcast.Subject,
		broadcast.BodyHTML,
		broadcast.BodyText,
		broadcast.TotalRecipients,
		broadcast.Status,
	).Scan(
		&result.ID,
		&result.Subject,
		&result.BodyHTML,
		&result.BodyText,
		&result.TotalRecipients,
		&result.SentCount,
		&result.FailedCount,
		&result.Status,
		&result.StartedAt,
		&result.CompletedAt,
		&result.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create email broadcast log: %w", err)
	}
	return &result, nil
}

// UpdateStatusAndCounts updates the status, sent_count, failed_count, and completed_at
// for a broadcast log entry identified by id.
func (r *EmailBroadcastRepository) UpdateStatusAndCounts(ctx context.Context, id string, status string, sentCount, failedCount int, completedAt *time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE email_broadcast_logs
		SET status = $2, sent_count = $3, failed_count = $4, completed_at = $5
		WHERE id = $1
	`, id, status, sentCount, failedCount, completedAt)
	if err != nil {
		return fmt.Errorf("update email broadcast log: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HasRecentBroadcast checks if a broadcast (sending or completed) with the same
// subject exists within the given time window. Returns the broadcast if found.
// Catches both in-flight ("sending") and finished ("completed") broadcasts to
// prevent duplicates when a client retries after timeout.
func (r *EmailBroadcastRepository) HasRecentBroadcast(ctx context.Context, subject string, window time.Duration) (*models.EmailBroadcast, error) {
	cutoff := time.Now().Add(-window)
	query := `
		SELECT id, subject, total_recipients, sent_count, failed_count, status,
		       started_at, completed_at, created_at
		FROM email_broadcast_logs
		WHERE subject = $1
		  AND status IN ('sending', 'completed')
		  AND started_at >= $2
		ORDER BY started_at DESC
		LIMIT 1
	`

	var b models.EmailBroadcast
	err := r.pool.QueryRow(ctx, query, subject, cutoff).Scan(
		&b.ID, &b.Subject, &b.TotalRecipients, &b.SentCount, &b.FailedCount,
		&b.Status, &b.StartedAt, &b.CompletedAt, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No recent broadcast found
		}
		return nil, fmt.Errorf("check recent broadcast: %w", err)
	}
	return &b, nil
}

// List returns all email broadcast log entries ordered by started_at DESC.
// Returns an empty (non-nil) slice if no entries exist.
func (r *EmailBroadcastRepository) List(ctx context.Context) ([]models.EmailBroadcast, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, subject, body_html, COALESCE(body_text, '') AS body_text,
		       total_recipients, sent_count, failed_count, status,
		       started_at, completed_at, created_at
		FROM email_broadcast_logs
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list email broadcast logs: %w", err)
	}
	defer rows.Close()

	results := []models.EmailBroadcast{}
	for rows.Next() {
		var b models.EmailBroadcast
		if err := rows.Scan(
			&b.ID,
			&b.Subject,
			&b.BodyHTML,
			&b.BodyText,
			&b.TotalRecipients,
			&b.SentCount,
			&b.FailedCount,
			&b.Status,
			&b.StartedAt,
			&b.CompletedAt,
			&b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list email broadcast logs: %w", err)
		}
		results = append(results, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list email broadcast logs: %w", err)
	}
	return results, nil
}
