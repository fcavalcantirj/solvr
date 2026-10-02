package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// queueWebhookDeliveries is the statement part NotificationsRepository.Create runs after its
// INSERT (as the CTE "created"): the event, when it is under the contract (version 1 or 2) and
// addressed to an agent, is queued for each of the agent's active or failing webhooks subscribed to its type
// that has a sealed signing secret. Its data is fixed here, so every attempt sends the same.
const queueWebhookDeliveries = `
	INSERT INTO webhook_deliveries (webhook_id, notification_id, event, schema_version, data, occurred_at)
	SELECT w.id, c.id, c.type, c.schema_version,
		jsonb_build_object(
			'notification_id', c.id,
			'agent_id', c.agent_id,
			'subject', jsonb_strip_nulls(jsonb_build_object('post_id', c.post_id, 'reply_id', c.reply_id, 'room_id', c.room_id)),
			'title', c.title,
			'body', COALESCE(c.body, ''),
			'link', COALESCE(c.link, '')),
		COALESCE(c.created_at, NOW())
	FROM created c
	JOIN webhooks w ON w.agent_id = c.agent_id
	WHERE c.schema_version IN (1, 2) AND c.type = ANY(w.events)
	  AND w.status IN ('active', 'failing') AND w.signing_secret IS NOT NULL
	ON CONFLICT (webhook_id, notification_id) DO NOTHING`

// ClaimDueDeliveries leases up to limit due deliveries to this sender for lease and returns
// them with their webhook, its secret opened. A delivery leased to another sender — another
// API instance — is skipped, so each is sent by one sender at a time; a lease that runs out
// (its sender died) makes it due again, with the same ID. A delivery whose secret this server
// cannot open is failed rather than sent unsigned.
func (r *WebhookRepository) ClaimDueDeliveries(ctx context.Context, limit int, lease time.Duration) ([]*models.WebhookDelivery, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE webhook_deliveries d SET leased_until = NOW() + make_interval(secs => $2)
		FROM webhooks w
		WHERE w.id = d.webhook_id AND d.id IN (
			SELECT q.id FROM webhook_deliveries q JOIN webhooks qw ON qw.id = q.webhook_id
			WHERE q.status = 'pending' AND q.next_attempt_at <= NOW()
			  AND (q.leased_until IS NULL OR q.leased_until < NOW())
			  AND qw.status IN ('active', 'failing')
			ORDER BY q.next_attempt_at, q.id
			LIMIT $1
			FOR UPDATE OF q SKIP LOCKED)
		RETURNING d.id, d.webhook_id, d.notification_id, d.event, d.schema_version, d.data::text, d.occurred_at,
			d.attempts, d.status, w.agent_id, w.url, w.events, w.status, w.consecutive_failures,
			w.last_failure_at, w.last_success_at, w.signing_secret`, limit, lease.Seconds())
	if err != nil {
		LogQueryError(ctx, "ClaimDueDeliveries", "webhook_deliveries", err)
		return nil, fmt.Errorf("claim webhook deliveries: %w", err)
	}
	type claimed struct {
		delivery *models.WebhookDelivery
		sealed   []byte
	}
	var all []claimed
	for rows.Next() {
		d := &models.WebhookDelivery{}
		var data string
		var sealed []byte
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.NotificationID, &d.Event, &d.SchemaVersion, &data, &d.OccurredAt,
			&d.Attempts, &d.Status, &d.Webhook.AgentID, &d.Webhook.URL, &d.Webhook.Events, &d.Webhook.Status,
			&d.Webhook.ConsecutiveFailures, &d.Webhook.LastFailureAt, &d.Webhook.LastSuccessAt, &sealed); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}
		d.Data = []byte(data)
		d.Webhook.ID = d.WebhookID
		all = append(all, claimed{d, sealed})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim webhook deliveries: %w", err)
	}

	deliveries := make([]*models.WebhookDelivery, 0, len(all))
	for _, c := range all {
		secret, err := r.openSecret(c.delivery.WebhookID, c.sealed)
		if err != nil {
			if err := r.RecordDeliveryAttempt(ctx, c.delivery.ID, models.WebhookDeliveryAttempt{
				Error: "the webhook signing secret cannot be opened under this server's secret; set the secret again"}); err != nil {
				return nil, err
			}
			continue
		}
		c.delivery.Webhook.Secret = secret
		deliveries = append(deliveries, c.delivery)
	}
	return deliveries, nil
}

// RecordDeliveryAttempt records one send of a delivery and releases its lease: delivered on
// a 2xx, due again at NextAttemptAt, or failed when there is no next attempt.
func (r *WebhookRepository) RecordDeliveryAttempt(ctx context.Context, id uuid.UUID, a models.WebhookDeliveryAttempt) error {
	status := models.WebhookDeliveryFailed
	switch {
	case a.Delivered:
		status = models.WebhookDeliveryDelivered
	case a.NextAttemptAt != nil:
		status = models.WebhookDeliveryPending
	}
	var lastError *string
	if a.Error != "" {
		lastError = &a.Error
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status = $2, attempts = attempts + 1, last_attempt_at = NOW(),
			last_status_code = $3, last_error = $4, leased_until = NULL,
			next_attempt_at = COALESCE($5, next_attempt_at),
			delivered_at = CASE WHEN $6 THEN NOW() ELSE delivered_at END
		WHERE id = $1`, id, string(status), a.StatusCode, lastError, a.NextAttemptAt, a.Delivered)
	if err != nil {
		LogQueryError(ctx, "RecordDeliveryAttempt", "webhook_deliveries", err)
		return fmt.Errorf("record webhook delivery attempt: %w", err)
	}
	return nil
}
