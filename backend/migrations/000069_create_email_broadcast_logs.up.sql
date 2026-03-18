-- Migration 000069: Create email_broadcast_logs table
-- Tracks admin email broadcasts for audit purposes (AUDIT-01)

CREATE TABLE IF NOT EXISTS email_broadcast_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT,
    total_recipients INT NOT NULL,
    sent_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'sending',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing broadcasts in reverse chronological order
CREATE INDEX IF NOT EXISTS idx_email_broadcast_logs_started_at ON email_broadcast_logs (started_at DESC);
