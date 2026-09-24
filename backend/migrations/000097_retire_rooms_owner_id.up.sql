-- Retire rooms.owner_id: room_members (000095/000096) is the only ownership store.
--
-- Every writer now records human ownership as an active 'owner' membership directly
-- (room creation, claim backfill), so the compatibility mirror rooms_sync_human_owner is
-- no longer needed. A room's owner_id in API responses is derived at read time from its
-- earliest active human owner membership. Dropping the column also drops its foreign key
-- and idx_rooms_owner_id.

DROP TRIGGER IF EXISTS rooms_sync_human_owner ON rooms;
DROP FUNCTION IF EXISTS rooms_sync_human_owner();

ALTER TABLE rooms DROP COLUMN IF EXISTS owner_id;
