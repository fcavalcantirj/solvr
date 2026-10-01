-- The public room activity feed reads its newest entries first (idx 77 step 1: room activity,
-- measured on representative data). HomepageRepository.ListPublicRoomFeed and
-- CountPublicRoomFeedSince read every eligible live message and allow-listed event of every public
-- room and sorted them all to keep a handful: 1.6-2.0 s per page and 225 ms per New activity count
-- at 2M room entries (idx 77 slice 14 spike), a Seq Scan of room_entries at 600k (slice 15 test).
-- These two indexes hold exactly the entries the feed may show, in the feed's order, so a page
-- merges the two newest-first walks and stops after the rows it returns.
--
-- The event index carries the feed's allow-list (db.PublicFeedEventTypes) in its predicate. In the
-- 2026-09-29 production dump most events were ENGINE_TICK, ACK and other types the feed never shows;
-- an index of every event would walk through all of them. The feed query names the same list as
-- constants so the planner can prove the predicate; a type added to the Go list and not here still
-- reads correctly, just without this index, until a migration widens it.
--
-- Both are derived from room_entries alone; REINDEX rebuilds them.
CREATE INDEX IF NOT EXISTS idx_room_entries_feed_messages ON room_entries (created_at, id)
    WHERE kind = 'message' AND deleted_at IS NULL AND author_type <> 'system';
CREATE INDEX IF NOT EXISTS idx_room_entries_feed_events ON room_entries (created_at, id)
    WHERE kind = 'event' AND upper(event_type) IN ('CLAIM', 'RELEASE', 'BUILDING', 'PLAN', 'DIRECTIVE',
        'EVIDENCE', 'REVIEW', 'BLOCKED', 'PR', 'MERGED', 'DONE');
