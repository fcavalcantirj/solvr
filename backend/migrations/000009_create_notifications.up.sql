-- Notifications table
-- Stores notifications for both humans (users) and AI agents
-- Either user_id OR agent_id should be set, not both

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Recipient: either a user or an agent (one should be NULL)
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    agent_id VARCHAR(50) REFERENCES agents(id) ON DELETE CASCADE,

    -- Notification content
    type VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    body TEXT,
    link VARCHAR(500),

    -- Status
    read_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Ensure at least one recipient is set
    CONSTRAINT notifications_recipient_check CHECK (
        (user_id IS NOT NULL AND agent_id IS NULL) OR
        (user_id IS NULL AND agent_id IS NOT NULL)
    )
);

-- Indexes for efficient queries
-- Index for fetching user notifications
CREATE INDEX idx_notifications_user_id ON notifications(user_id) WHERE user_id IS NOT NULL;

-- Index for fetching agent notifications
CREATE INDEX idx_notifications_agent_id ON notifications(agent_id) WHERE agent_id IS NOT NULL;

-- Index for filtering unread notifications
CREATE INDEX idx_notifications_unread_user ON notifications(user_id, created_at DESC)
    WHERE user_id IS NOT NULL AND read_at IS NULL;

CREATE INDEX idx_notifications_unread_agent ON notifications(agent_id, created_at DESC)
    WHERE agent_id IS NOT NULL AND read_at IS NULL;

-- Index for sorting by created_at
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);

-- Index on type for filtering by notification type
CREATE INDEX idx_notifications_type ON notifications(type);
