-- Restore room_entries_allocate as defined by 000094 (no latest-revision check).
CREATE OR REPLACE FUNCTION room_entries_allocate() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.sequence IS NULL THEN
        PERFORM 1 FROM rooms WHERE id = NEW.room_id FOR NO KEY UPDATE;
        IF NEW.reply_to_entry_id IS NOT NULL AND NOT EXISTS (
            SELECT 1 FROM room_entries WHERE id = NEW.reply_to_entry_id AND room_id = NEW.room_id
        ) THEN
            RAISE EXCEPTION 'reply_to_entry_id % is not an entry in room %', NEW.reply_to_entry_id, NEW.room_id
                USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'room_entries_same_room_reference';
        END IF;
        IF NEW.supersedes_entry_id IS NOT NULL AND NOT EXISTS (
            SELECT 1 FROM room_entries WHERE id = NEW.supersedes_entry_id AND room_id = NEW.room_id
        ) THEN
            RAISE EXCEPTION 'supersedes_entry_id % is not an entry in room %', NEW.supersedes_entry_id, NEW.room_id
                USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'room_entries_same_room_reference';
        END IF;
        IF NEW.addressed_member_ids IS NOT NULL AND NEW.addressed_member_ids <> 'null'::jsonb THEN
            IF jsonb_typeof(NEW.addressed_member_ids) <> 'array' THEN
                RAISE EXCEPTION 'addressed_member_ids must be an array of participant ids'
                    USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'room_entries_same_room_reference';
            END IF;
            IF EXISTS (
                SELECT 1 FROM jsonb_array_elements(NEW.addressed_member_ids) AS a(v)
                WHERE jsonb_typeof(a.v) <> 'string'
                   OR NOT (
                        EXISTS (SELECT 1 FROM room_members m
                                WHERE m.room_id = NEW.room_id AND m.agent_id = a.v #>> '{}')
                     OR EXISTS (SELECT 1 FROM agent_presence p
                                WHERE p.room_id = NEW.room_id AND p.agent_name = a.v #>> '{}')
                     OR EXISTS (SELECT 1 FROM room_entries e
                                WHERE e.room_id = NEW.room_id AND e.kind = 'message'
                                  AND (e.author_id = a.v #>> '{}' OR e.actor_label = a.v #>> '{}'))
                   )
            ) THEN
                RAISE EXCEPTION 'addressed_member_ids references a participant outside room %', NEW.room_id
                    USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'room_entries_same_room_reference';
            END IF;
        END IF;
        SELECT COALESCE(MAX(sequence), 0) + 1 INTO NEW.sequence
        FROM room_entries WHERE room_id = NEW.room_id;
        NEW.id := nextval('room_entries_id_seq');
    END IF;
    RETURN NEW;
END;
$$;
