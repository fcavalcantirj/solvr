-- BART-154: family/private posts skip moderation and are created 'open'. Heal any legacy
-- family post left in a hidden (non-searchable) status by the old moderation flow — the
-- new Update guard no longer re-moderates family posts, so edit-and-resubmit can't recover
-- them. Idempotent: rows already 'open' don't match. (Prod had 0 such rows at ship time.)
UPDATE posts
SET status = 'open', updated_at = NOW()
WHERE visibility = 'family'
  AND status IN ('pending_review', 'draft', 'rejected')
  AND deleted_at IS NULL;
