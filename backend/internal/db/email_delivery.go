package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// EmailDeliveryRepository stores what Solvr sent and what the provider reported about it
// (SPEC.md 16.6): email_messages, email_events and the suppression columns of users.
type EmailDeliveryRepository struct {
	pool *Pool
}

// NewEmailDeliveryRepository creates a new EmailDeliveryRepository.
func NewEmailDeliveryRepository(pool *Pool) *EmailDeliveryRepository {
	return &EmailDeliveryRepository{pool: pool}
}

// RecordMessage stores one message sent or tried: a failed send has no provider id, and a
// notice belongs to no broadcast.
func (r *EmailDeliveryRepository) RecordMessage(ctx context.Context, m *models.EmailMessage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO email_messages (kind, broadcast_id, user_id, provider_message_id, subject, status)
		VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, ''), $5, $6)`,
		m.Kind, m.BroadcastID, m.UserID, m.ProviderMessageID, m.Subject, m.Status)
	if err != nil {
		LogQueryError(ctx, "RecordMessage", "email_messages", err)
		return fmt.Errorf("record email message: %w", err)
	}
	return nil
}

// ApplyEvent stores a provider event once by its svix-id and applies its effect in the same
// transaction: the message takes the event's status unless a later event already set it, and a
// suppression marks every user with one of the addresses (case-insensitively), keeping the
// first time and reason. A stored svix-id changes nothing and reports false.
func (r *EmailDeliveryRepository) ApplyEvent(ctx context.Context, ev *models.EmailEvent, effect models.EmailEventEffect) (bool, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin email event: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			LogQueryError(ctx, "ApplyEvent.Rollback", "email_events", rbErr)
		}
	}()

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO email_events (svix_id, provider_message_id, event_type, bounce_type, occurred_at)
		VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''), $5)
		ON CONFLICT (svix_id) DO NOTHING
		RETURNING id`,
		ev.SvixID, ev.ProviderMessageID, ev.EventType, ev.BounceType, ev.OccurredAt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		LogQueryError(ctx, "ApplyEvent.insert", "email_events", err)
		return false, fmt.Errorf("store email event: %w", err)
	}

	if effect.Status != "" && ev.ProviderMessageID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE email_messages SET status = $2, last_event_at = $3
			WHERE provider_message_id = $1 AND (last_event_at IS NULL OR last_event_at <= $3)`,
			ev.ProviderMessageID, effect.Status, ev.OccurredAt); err != nil {
			LogQueryError(ctx, "ApplyEvent.status", "email_messages", err)
			return false, fmt.Errorf("update email message status: %w", err)
		}
	}

	if effect.Suppress != "" && len(effect.Recipients) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE users SET
				email_suppressed_at = COALESCE(email_suppressed_at, NOW()),
				email_suppressed_reason = COALESCE(email_suppressed_reason, $2),
				email_unsubscribed_at = CASE WHEN $3 THEN COALESCE(email_unsubscribed_at, NOW()) ELSE email_unsubscribed_at END
			WHERE lower(email) = ANY($1)`,
			effect.Recipients, effect.Suppress, effect.Unsubscribe); err != nil {
			LogQueryError(ctx, "ApplyEvent.suppress", "users", err)
			return false, fmt.Errorf("suppress email addresses: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit email event: %w", err)
	}
	return true, nil
}

// FindNoticeRecipient returns the live user with this email, and whether Solvr no longer mails
// the address. A user who unsubscribed from broadcasts is still found: a notice is not one.
func (r *EmailDeliveryRepository) FindNoticeRecipient(ctx context.Context, email string) (*models.EmailRecipient, bool, error) {
	var rec models.EmailRecipient
	var suppressed bool
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, display_name, COALESCE(referral_code, ''), email_suppressed_at IS NOT NULL
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`, email).
		Scan(&rec.ID, &rec.Email, &rec.DisplayName, &rec.ReferralCode, &suppressed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		LogQueryError(ctx, "FindNoticeRecipient", "users", err)
		return nil, false, fmt.Errorf("find notice recipient: %w", err)
	}
	return &rec, suppressed, nil
}
