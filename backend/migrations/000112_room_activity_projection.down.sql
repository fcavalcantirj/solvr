-- Back to API-maintained room activity: the code before 000112 bumps message_count and
-- last_active_at itself after each message.
DROP TRIGGER IF EXISTS room_entries_activity_insert ON room_entries;
DROP TRIGGER IF EXISTS room_entries_activity_soft_delete ON room_entries;
DROP TRIGGER IF EXISTS room_entries_activity_delete ON room_entries;
DROP FUNCTION IF EXISTS room_entries_activity();
DROP FUNCTION IF EXISTS rebuild_room_activity(uuid);
DROP FUNCTION IF EXISTS room_activity_drift(uuid);
