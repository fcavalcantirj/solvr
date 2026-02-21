-- Auto-confirm all existing votes.
-- The "Vote → Confirm → Locked" workflow was designed but never implemented.
-- Votes were inserted with confirmed = FALSE (default), but reputation
-- calculations require confirmed = true. This fixes all existing votes.
UPDATE votes SET confirmed = true WHERE confirmed = false;
