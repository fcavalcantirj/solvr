-- Reverts 000123: post and reply writes stop announcing overview changes; the snapshots fall
-- back to their 30 s TTL (room changes still drop them through Pool.OverviewChanged).
DROP TRIGGER IF EXISTS replies_overview_changed_delete ON replies;
DROP TRIGGER IF EXISTS replies_overview_changed_update ON replies;
DROP TRIGGER IF EXISTS posts_overview_changed_delete ON posts;
DROP TRIGGER IF EXISTS posts_overview_changed_update ON posts;
DROP FUNCTION IF EXISTS overview_changed_notify();
