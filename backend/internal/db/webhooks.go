package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// webhookSecretSealDomain separates the webhook signing-secret key from every other key
// derived from the server secret.
const webhookSecretSealDomain = "solvr/webhook-signing-secret/v1"

var (
	// ErrWebhookNotFound is returned when a webhook does not exist.
	ErrWebhookNotFound = errors.New("webhook not found")
	// ErrWebhookSecretUnsealable is returned when a signing secret cannot be sealed or opened:
	// the repository has no seal secret, or the stored seal is from another server secret.
	ErrWebhookSecretUnsealable = errors.New("webhook signing secret cannot be sealed or opened")
)

// WebhookRepository stores an agent's webhooks (SPEC.md Part 12.3) and their delivery queue
// (webhook_deliveries.go). A webhook's signing secret is kept sealed under a key derived from
// the server secret, the webhook id bound in, so the servers can sign deliveries with it and
// the database alone cannot read it.
type WebhookRepository struct {
	pool   *Pool
	sealer *claimTokenSealer
}

// NewWebhookRepository creates a WebhookRepository. Without WithSealSecret it cannot store a
// webhook or open one for delivery.
func NewWebhookRepository(pool *Pool) *WebhookRepository {
	return &WebhookRepository{pool: pool}
}

// WithSealSecret sets the server secret the signing secrets are sealed under. Call it before
// the repository serves.
func (r *WebhookRepository) WithSealSecret(secret string) *WebhookRepository {
	r.sealer = newSealerForDomain(secret, webhookSecretSealDomain)
	return r
}

const webhookColumns = `id, agent_id, url, events, secret_hash, status, consecutive_failures,
	last_failure_at, last_success_at, created_at, updated_at`

func scanWebhook(row pgx.Row) (*models.Webhook, error) {
	var w models.Webhook
	err := row.Scan(&w.ID, &w.AgentID, &w.URL, &w.Events, &w.SecretHash, &w.Status, &w.ConsecutiveFailures,
		&w.LastFailureAt, &w.LastSuccessAt, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWebhookNotFound
	}
	return &w, err
}

func (r *WebhookRepository) sealSecret(id uuid.UUID, secret string) ([]byte, error) {
	if r.sealer == nil || secret == "" {
		return nil, ErrWebhookSecretUnsealable
	}
	return r.sealer.seal(secret, id.String())
}

func (r *WebhookRepository) openSecret(id uuid.UUID, sealed []byte) (string, error) {
	secret, err := r.sealer.open(sealed, id.String())
	if err != nil {
		return "", ErrWebhookSecretUnsealable
	}
	return secret, nil
}

// Create stores w with its Secret sealed, setting w.ID when it is unset.
func (r *WebhookRepository) Create(ctx context.Context, w *models.Webhook) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	sealed, err := r.sealSecret(w.ID, w.Secret)
	if err != nil {
		return err
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO webhooks (id, agent_id, url, events, secret_hash, signing_secret, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+webhookColumns,
		w.ID, w.AgentID, w.URL, w.Events, w.SecretHash, sealed, w.Status)
	created, err := scanWebhook(row)
	if err != nil {
		LogQueryError(ctx, "Create", "webhooks", err)
		return fmt.Errorf("create webhook: %w", err)
	}
	created.Secret = w.Secret
	*w = *created
	return nil
}

// FindByID returns the webhook with the given id, its secret not opened.
func (r *WebhookRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	w, err := scanWebhook(r.pool.QueryRow(ctx, `SELECT `+webhookColumns+` FROM webhooks WHERE id = $1`, id))
	if err != nil && !errors.Is(err, ErrWebhookNotFound) {
		LogQueryError(ctx, "FindByID", "webhooks", err)
	}
	return w, err
}

// List returns the agent's webhooks, oldest first.
func (r *WebhookRepository) List(ctx context.Context, agentID string) ([]models.Webhook, error) {
	return r.list(ctx, `SELECT `+webhookColumns+` FROM webhooks WHERE agent_id = $1 ORDER BY created_at, id`, agentID)
}

// FindByAgentAndEvent returns the agent's active or failing webhooks subscribed to event.
func (r *WebhookRepository) FindByAgentAndEvent(ctx context.Context, agentID, event string) ([]*models.Webhook, error) {
	list, err := r.list(ctx, `SELECT `+webhookColumns+` FROM webhooks
		WHERE agent_id = $1 AND $2 = ANY(events) AND status IN ('active', 'failing') ORDER BY created_at, id`, agentID, event)
	if err != nil {
		return nil, err
	}
	out := make([]*models.Webhook, len(list))
	for i := range list {
		out[i] = &list[i]
	}
	return out, nil
}

func (r *WebhookRepository) list(ctx context.Context, query string, args ...any) ([]models.Webhook, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "List", "webhooks", err)
		return nil, err
	}
	defer rows.Close()
	webhooks := []models.Webhook{}
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, *w)
	}
	return webhooks, rows.Err()
}

// Update stores w's URL, events, status and secret hash; a non-empty Secret is sealed in
// place of the stored one.
func (r *WebhookRepository) Update(ctx context.Context, w *models.Webhook) error {
	var sealed []byte
	if w.Secret != "" {
		var err error
		if sealed, err = r.sealSecret(w.ID, w.Secret); err != nil {
			return err
		}
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE webhooks SET url = $2, events = $3, status = $4, secret_hash = $5,
			signing_secret = COALESCE($6, signing_secret), updated_at = NOW()
		WHERE id = $1`, w.ID, w.URL, w.Events, w.Status, w.SecretHash, sealed)
	if err != nil {
		LogQueryError(ctx, "Update", "webhooks", err)
		return fmt.Errorf("update webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// Delete removes the webhook and its queued deliveries.
func (r *WebhookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM webhooks WHERE id = $1`, id)
	if err != nil {
		LogQueryError(ctx, "Delete", "webhooks", err)
		return fmt.Errorf("delete webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// FindAgent returns the agent a webhook belongs to (ErrAgentNotFound when absent).
func (r *WebhookRepository) FindAgent(ctx context.Context, agentID string) (*models.Agent, error) {
	return NewAgentRepository(r.pool).FindByID(ctx, agentID)
}

// UpdateDeliveryStatus records a webhook's health after a delivery attempt: its consecutive
// failures, the time of the last failure or success, and a new status when one is given.
func (r *WebhookRepository) UpdateDeliveryStatus(ctx context.Context, id uuid.UUID, failures int, lastFailure, lastSuccess *time.Time, status *models.WebhookStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE webhooks SET consecutive_failures = $2,
			last_failure_at = COALESCE($3, last_failure_at), last_success_at = COALESCE($4, last_success_at),
			status = COALESCE($5, status), updated_at = NOW()
		WHERE id = $1`, id, failures, lastFailure, lastSuccess, status)
	if err != nil {
		LogQueryError(ctx, "UpdateDeliveryStatus", "webhooks", err)
	}
	return err
}
