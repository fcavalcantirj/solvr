-- Email delivery (SPEC.md 16.6): one row per message sent, the provider's events about it,
-- and the addresses Solvr no longer mails after a hard bounce or a complaint.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_suppressed_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_suppressed_reason TEXT;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_suppressed_reason_check;
ALTER TABLE users ADD CONSTRAINT users_email_suppressed_reason_check
    CHECK (email_suppressed_reason IS NULL OR email_suppressed_reason IN ('hard_bounce', 'complaint'));

CREATE TABLE IF NOT EXISTS email_messages (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL,
    broadcast_id UUID REFERENCES email_broadcast_logs(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    provider_message_id TEXT UNIQUE,
    subject TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'sent',
    last_event_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_messages_kind_check CHECK (kind IN ('broadcast', 'transactional')),
    CONSTRAINT email_messages_status_check CHECK (status IN ('sent', 'delivered', 'delivery_delayed', 'bounced', 'complained', 'failed', 'suppressed'))
);

CREATE TABLE IF NOT EXISTS email_events (
    id BIGSERIAL PRIMARY KEY,
    svix_id TEXT NOT NULL UNIQUE,
    provider_message_id TEXT,
    event_type TEXT NOT NULL,
    bounce_type TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_email_events_provider_message_id ON email_events (provider_message_id);
