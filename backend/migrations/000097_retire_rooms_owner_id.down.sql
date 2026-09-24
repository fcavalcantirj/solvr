-- Restore rooms.owner_id (000073 shape) from the membership authority: the earliest
-- active human owner membership of each room. Re-creates the 000095 compatibility mirror.

ALTER TABLE rooms ADD COLUMN owner_id UUID REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX idx_rooms_owner_id ON rooms (owner_id);

UPDATE rooms r SET owner_id = (
    SELECT rm.user_id FROM room_members rm
    WHERE rm.room_id = r.id AND rm.user_id IS NOT NULL
      AND rm.role = 'owner' AND rm.revoked_at IS NULL
    ORDER BY rm.created_at, rm.id
    LIMIT 1
);

CREATE FUNCTION rooms_sync_human_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO room_members (room_id, user_id, role, added_by)
    VALUES (NEW.id, NEW.owner_id, 'owner', 'system')
    ON CONFLICT (room_id, user_id) DO UPDATE SET role = 'owner', revoked_at = NULL;
    RETURN NULL;
END;
$$;

CREATE TRIGGER rooms_sync_human_owner
    AFTER INSERT OR UPDATE OF owner_id ON rooms
    FOR EACH ROW WHEN (NEW.owner_id IS NOT NULL)
    EXECUTE FUNCTION rooms_sync_human_owner();
