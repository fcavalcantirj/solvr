-- Webhooks deliver the agent's notification events of schema version 1 (task "Keep SDKs, CLI,
-- MCP, skills, and webhooks consistent with the redesigned product", step 4; SPEC.md Part
-- 12.3). The subscribed event names are the notification types of that contract.
--
-- signing_secret: the webhook secret sealed under a key derived from the server secret
-- (AES-256-GCM, the webhook id bound in), so the servers can sign every delivery with it and
-- the database alone cannot read it. secret_hash (bcrypt) cannot sign anything: a webhook
-- without a sealed secret is never queued.
ALTER TABLE webhooks ADD COLUMN signing_secret BYTEA;

COMMENT ON COLUMN webhooks.events IS 'Notification event types of schema version 1: post.approved, post.rejected, reply.removed, reply.flagged, blog_post_rejected';
COMMENT ON COLUMN webhooks.signing_secret IS 'Webhook secret sealed under the server secret (AES-256-GCM, webhook id as additional data); signs every delivery';

-- One delivery per webhook and notification, recorded in the statement that records the
-- notification. id is the delivery ID (X-Solvr-Delivery-ID and the payload "id"): every
-- attempt sends it with the same event, data and timestamp, so a receiver acts on it once.
-- A sender claims a due delivery with a lease (leased_until); a lease that runs out because
-- its sender died lets another instance send it again, with the same ID.
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    event VARCHAR(50) NOT NULL,
    schema_version SMALLINT NOT NULL,
    data JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    leased_until TIMESTAMPTZ,
    last_attempt_at TIMESTAMPTZ,
    last_status_code INT,
    last_error TEXT,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT webhook_deliveries_once UNIQUE (webhook_id, notification_id),
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('pending', 'delivered', 'failed')),
    CONSTRAINT webhook_deliveries_schema_version_check CHECK (schema_version = 1),
    CONSTRAINT webhook_deliveries_attempts_check CHECK (attempts >= 0)
);

-- The due queue, and the cascades from a deleted notification.
CREATE INDEX idx_webhook_deliveries_due ON webhook_deliveries(next_attempt_at) WHERE status = 'pending';
CREATE INDEX idx_webhook_deliveries_notification ON webhook_deliveries(notification_id);
