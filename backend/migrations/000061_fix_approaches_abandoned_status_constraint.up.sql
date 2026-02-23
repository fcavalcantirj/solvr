-- Fix: actually enforce 'abandoned' as a valid approaches status.
-- Migration 000049 documented this intent but did not alter the constraint.
-- Drop the old constraint and recreate it with 'abandoned' included.

ALTER TABLE approaches DROP CONSTRAINT approaches_status_check;
ALTER TABLE approaches ADD CONSTRAINT approaches_status_check
    CHECK (status IN ('starting', 'working', 'stuck', 'failed', 'succeeded', 'abandoned'));
