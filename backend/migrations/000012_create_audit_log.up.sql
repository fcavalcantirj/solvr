-- Audit log table for admin actions
-- Per SPEC.md Part 16 (Admin Tools) and PRD database requirements
-- Tracks all administrative actions for accountability and debugging

CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    target_type VARCHAR(50),
    target_id UUID,
    details JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for querying by admin
CREATE INDEX idx_audit_log_admin_id ON audit_log(admin_id);

-- Index for querying by action type
CREATE INDEX idx_audit_log_action ON audit_log(action);

-- Index for querying by target
CREATE INDEX idx_audit_log_target ON audit_log(target_type, target_id);

-- Index for time-based queries (recent actions, date range filters)
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC);

-- Comments
COMMENT ON TABLE audit_log IS 'Audit log for all administrative actions';
COMMENT ON COLUMN audit_log.id IS 'Unique identifier for the audit entry';
COMMENT ON COLUMN audit_log.admin_id IS 'Reference to the admin user who performed the action';
COMMENT ON COLUMN audit_log.action IS 'Type of action performed (e.g., delete, ban, suspend, warn, dismiss)';
COMMENT ON COLUMN audit_log.target_type IS 'Type of entity targeted (e.g., post, user, agent, flag)';
COMMENT ON COLUMN audit_log.target_id IS 'ID of the targeted entity';
COMMENT ON COLUMN audit_log.details IS 'Additional details about the action in JSON format';
COMMENT ON COLUMN audit_log.ip_address IS 'IP address from which the action was performed';
COMMENT ON COLUMN audit_log.created_at IS 'Timestamp when the action was performed';
