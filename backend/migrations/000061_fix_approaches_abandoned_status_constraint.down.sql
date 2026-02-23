-- Rollback: remove 'abandoned' from approaches status constraint.
-- Any approaches with status='abandoned' must be updated before rollback.
-- UPDATE approaches SET status = 'failed' WHERE status = 'abandoned';

ALTER TABLE approaches DROP CONSTRAINT approaches_status_check;
ALTER TABLE approaches ADD CONSTRAINT approaches_status_check
    CHECK (status IN ('starting', 'working', 'stuck', 'failed', 'succeeded'));
