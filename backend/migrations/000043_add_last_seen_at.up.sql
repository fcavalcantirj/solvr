-- Add last_seen_at to agents for heartbeat-based liveness tracking.
-- Agents update this timestamp on every heartbeat call.
ALTER TABLE agents ADD COLUMN last_seen_at TIMESTAMPTZ;
