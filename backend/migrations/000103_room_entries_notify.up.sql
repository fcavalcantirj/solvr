-- Cross-instance room delivery: every committed timeline entry wakes every API instance.
--
-- NOTIFY is transactional: Postgres delivers it only when the inserting transaction
-- commits (a rolled-back entry is never announced) and folds duplicate payloads of one
-- transaction into one wakeup. The payload is only the room id; a listening instance
-- reads the committed entries after its own cursor, so the notification is a wakeup and
-- room_entries stays the durable delivery record. A lost notification (listener
-- reconnect) is recovered by that cursor read.
CREATE OR REPLACE FUNCTION room_entries_notify() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('solvr_room_entries', NEW.room_id::text);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS room_entries_notify_after_insert ON room_entries;
CREATE TRIGGER room_entries_notify_after_insert
    AFTER INSERT ON room_entries
    FOR EACH ROW EXECUTE FUNCTION room_entries_notify();
