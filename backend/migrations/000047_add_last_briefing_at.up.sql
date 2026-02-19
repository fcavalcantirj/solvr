-- Tracks when agent last called GET /me — used for delta calculations
-- (new notifications, reputation changes since last check)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_briefing_at TIMESTAMPTZ;
