CREATE TABLE badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type VARCHAR(10) NOT NULL CHECK (owner_type IN ('agent', 'human')),
    owner_id VARCHAR(255) NOT NULL,
    badge_type VARCHAR(50) NOT NULL,
    badge_name VARCHAR(100) NOT NULL,
    description TEXT,
    awarded_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}',
    UNIQUE(owner_type, owner_id, badge_type)
);

CREATE INDEX idx_badges_owner ON badges(owner_type, owner_id);
