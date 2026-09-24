-- Unified room timeline CUTOVER (task: "Store room messages and coordination events in
-- one ordered room timeline"). Migration 000093 created room_entries and backfilled it
-- next to the parallel messages / room_events tables. This migration makes room_entries
-- the ONLY storage:
--
--   1. Rebuild the backfill so every message entry keeps its legacy messages.id. Old deep
--      links (/rooms/<slug>/messages/<id>), SSE Last-Event-ID cursors and ?after= cursors
--      therefore resolve by identity; legacy event ids resolve through
--      room_entry_legacy_map. room_entries was only ever written by the 000093 backfill,
--      so the rebuild loses nothing.
--   2. Freeze the old tables as legacy_messages / legacy_room_events (read-only archive
--      kept for the down migration and for re-running the idempotent backfill).
--   3. Replace the messages and room_events names with views over room_entries whose
--      INSTEAD OF triggers write exactly one entry per action, so every existing reader
--      (transcript, event filters, homepage activity, analytics) derives from the
--      timeline and no compatibility path can insert a duplicate.
--   4. Allocate the per-room sequence (and the entry id) inside the inserting
--      transaction while holding the room row lock, so concurrent commits keep a stable,
--      gap-free order in which id order matches sequence order.
--
-- room_claims (expiring leases) and agent_presence (expiring presence) intentionally
-- stay separate: neither is a permanent timeline entry.

-- 1. Archive the parallel tables.
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_result_message_id_fkey;
ALTER TABLE messages RENAME TO legacy_messages;
ALTER TABLE room_events RENAME TO legacy_room_events;
-- Archive ids share the timeline id space so a late archived row can never collide
-- with a live entry when it is backfilled by identity.
ALTER TABLE legacy_messages ALTER COLUMN id SET DEFAULT nextval('room_entries_id_seq');

-- 2. Idempotent, incremental backfill from the archive. Message entries keep their
-- legacy id; event entries get fresh ids in chronological order. Rows are merged per
-- room by created_at with a stable tie breaker (messages before events, then legacy
-- id) and sequenced after the room's current MAX(sequence). Reply / supersede links are
-- copied only when they stay inside the room; no other causal link is invented.
CREATE FUNCTION room_entries_backfill_legacy() RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    inserted integer;
BEGIN
    PERFORM setval('room_entries_id_seq', GREATEST(
        (SELECT COALESCE(MAX(id), 0) FROM room_entries),
        (SELECT COALESCE(MAX(id), 0) FROM legacy_messages),
        (SELECT last_value FROM room_entries_id_seq)
    ));

    WITH unified AS (
        SELECT m.room_id, 'message'::text AS kind,
               COALESCE(m.author_type, 'agent') AS author_type, m.author_id,
               m.agent_name::varchar(200) AS actor_label,
               m.content AS body, m.content_type,
               NULL::text AS event_type, ''::text AS issue,
               m.metadata AS extension,
               CASE WHEN EXISTS (SELECT 1 FROM legacy_messages t
                                 WHERE t.id = m.reply_to_entry_id AND t.room_id = m.room_id)
                    THEN m.reply_to_entry_id END AS reply_to_entry_id,
               m.addressed_member_ids,
               CASE WHEN EXISTS (SELECT 1 FROM legacy_messages t
                                 WHERE t.id = m.supersedes_entry_id AND t.room_id = m.room_id)
                    THEN m.supersedes_entry_id END AS supersedes_entry_id,
               m.client_entry_id, m.pinned_at, m.created_at, m.deleted_at,
               'message'::text AS legacy_kind, m.id AS legacy_id, 0 AS source_rank
        FROM legacy_messages m
        WHERE NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                          WHERE lm.legacy_kind = 'message' AND lm.legacy_id = m.id)
        UNION ALL
        SELECT e.room_id, 'event'::text,
               NULL, NULL, e.actor::varchar(200),
               NULL, 'text',
               e.event_type, e.issue,
               e.payload,
               NULL, NULL, NULL,
               NULL, NULL, e.created_at, NULL,
               'event'::text, e.id, 1
        FROM legacy_room_events e
        WHERE NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                          WHERE lm.legacy_kind = 'event' AND lm.legacy_id = e.id)
    ),
    base AS (
        SELECT room_id, MAX(sequence) AS maxseq FROM room_entries GROUP BY room_id
    ),
    seq AS (
        SELECT u.*,
            COALESCE(b.maxseq, 0) + ROW_NUMBER() OVER (
                PARTITION BY u.room_id ORDER BY u.created_at ASC, u.source_rank ASC, u.legacy_id ASC
            ) AS sequence
        FROM unified u LEFT JOIN base b ON b.room_id = u.room_id
    ),
    ins AS (
        INSERT INTO room_entries
            (id, room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
             event_type, issue, extension, reply_to_entry_id, addressed_member_ids,
             supersedes_entry_id, client_entry_id, pinned_at, created_at, deleted_at)
        SELECT CASE WHEN kind = 'message' THEN legacy_id ELSE nextval('room_entries_id_seq') END,
               room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
               event_type, issue, extension, reply_to_entry_id, addressed_member_ids,
               supersedes_entry_id, client_entry_id, pinned_at, created_at, deleted_at
        FROM seq
        ORDER BY created_at ASC, source_rank ASC, legacy_id ASC
        RETURNING id AS entry_id, room_id, sequence
    )
    INSERT INTO room_entry_legacy_map (legacy_kind, legacy_id, entry_id)
    SELECT s.legacy_kind, s.legacy_id, ins.entry_id
    FROM ins JOIN seq s ON s.room_id = ins.room_id AND s.sequence = ins.sequence;

    GET DIAGNOSTICS inserted = ROW_COUNT;
    RETURN inserted;
END;
$$;

-- Rebuild the 000093 backfill with identity message ids.
DELETE FROM room_entry_legacy_map;
DELETE FROM room_entries;
SELECT room_entries_backfill_legacy();

-- rooms.result_message_id now points at the timeline (ids are unchanged).
ALTER TABLE rooms ADD CONSTRAINT rooms_result_message_id_fkey
    FOREIGN KEY (result_message_id) REFERENCES room_entries(id) ON DELETE SET NULL;

-- 3. Live writes: sequence + id allocated under the room row lock; same-room reply,
-- supersede and addressing references enforced (constraint name
-- room_entries_same_room_reference, which the API maps to a 400). An addressed
-- participant must already be known to THIS room by one of the identities the room
-- records: an admitted member (room_members.agent_id), a present agent
-- (agent_presence.agent_name), or the author id / display label of one of its message
-- entries. Addressing stays a routing hint, not a permission. Rows that arrive with an
-- explicit sequence (the backfill) have already been validated and keep their ids.
CREATE FUNCTION room_entries_allocate() RETURNS trigger
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

CREATE TRIGGER room_entries_allocate_before_insert
    BEFORE INSERT ON room_entries
    FOR EACH ROW EXECUTE FUNCTION room_entries_allocate();

-- Indexes the legacy readers relied on, now on the timeline.
CREATE INDEX idx_room_entries_room_created  ON room_entries (room_id, created_at);
CREATE INDEX idx_room_entries_message_id    ON room_entries (room_id, id) WHERE kind = 'message';
CREATE INDEX idx_room_entries_event_recent  ON room_entries (room_id, id DESC) WHERE kind = 'event';
CREATE INDEX idx_room_entries_room_pinned   ON room_entries (room_id, id DESC)
    WHERE pinned_at IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_room_entries_supersedes    ON room_entries (supersedes_entry_id)
    WHERE supersedes_entry_id IS NOT NULL AND deleted_at IS NULL;

-- 4. Compatibility views: the old names read from and write to the timeline.
CREATE VIEW messages AS
SELECT id, room_id, author_type, author_id,
       actor_label AS agent_name, body AS content, content_type,
       extension AS metadata, sequence AS sequence_num,
       created_at, deleted_at, reply_to_entry_id, addressed_member_ids,
       client_entry_id, pinned_at, supersedes_entry_id
FROM room_entries
WHERE kind = 'message';

ALTER VIEW messages ALTER COLUMN author_type  SET DEFAULT 'agent';
ALTER VIEW messages ALTER COLUMN content_type SET DEFAULT 'text';
ALTER VIEW messages ALTER COLUMN metadata     SET DEFAULT '{}';
ALTER VIEW messages ALTER COLUMN created_at   SET DEFAULT NOW();

CREATE FUNCTION messages_view_write() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM room_entries WHERE id = OLD.id AND kind = 'message';
        IF NOT FOUND THEN RETURN NULL; END IF;
        RETURN OLD;
    END IF;

    IF length(NEW.agent_name) > 100 THEN
        RAISE EXCEPTION 'value too long for type character varying(100)'
            USING ERRCODE = 'string_data_right_truncation';
    END IF;

    IF TG_OP = 'INSERT' THEN
        -- sequence_num is always allocated by the timeline; a caller-supplied value is
        -- ignored so events and messages can never collide on a position.
        INSERT INTO room_entries
            (room_id, kind, author_type, author_id, actor_label, body, content_type, extension,
             reply_to_entry_id, addressed_member_ids, client_entry_id, pinned_at,
             supersedes_entry_id, created_at, deleted_at)
        VALUES
            (NEW.room_id, 'message', COALESCE(NEW.author_type, 'agent'), NEW.author_id, NEW.agent_name,
             NEW.content, COALESCE(NEW.content_type, 'text'), COALESCE(NEW.metadata, '{}'),
             NEW.reply_to_entry_id, NEW.addressed_member_ids, NEW.client_entry_id, NEW.pinned_at,
             NEW.supersedes_entry_id, COALESCE(NEW.created_at, NOW()), NEW.deleted_at)
        RETURNING id, sequence, created_at INTO NEW.id, NEW.sequence_num, NEW.created_at;
        NEW.author_type  := COALESCE(NEW.author_type, 'agent');
        NEW.content_type := COALESCE(NEW.content_type, 'text');
        NEW.metadata     := COALESCE(NEW.metadata, '{}');
        RETURN NEW;
    END IF;

    UPDATE room_entries SET
        author_type = NEW.author_type, author_id = NEW.author_id, actor_label = NEW.agent_name,
        body = NEW.content, content_type = NEW.content_type, extension = NEW.metadata,
        reply_to_entry_id = NEW.reply_to_entry_id, addressed_member_ids = NEW.addressed_member_ids,
        client_entry_id = NEW.client_entry_id, pinned_at = NEW.pinned_at,
        supersedes_entry_id = NEW.supersedes_entry_id,
        created_at = NEW.created_at, deleted_at = NEW.deleted_at
    WHERE id = OLD.id AND kind = 'message'
    RETURNING id, room_id, sequence INTO NEW.id, NEW.room_id, NEW.sequence_num;
    IF NOT FOUND THEN RETURN NULL; END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER messages_view_write
    INSTEAD OF INSERT OR UPDATE OR DELETE ON messages
    FOR EACH ROW EXECUTE FUNCTION messages_view_write();

CREATE VIEW room_events AS
SELECT id, room_id, event_type, issue, actor_label::text AS actor,
       extension AS payload, created_at
FROM room_entries
WHERE kind = 'event';

ALTER VIEW room_events ALTER COLUMN issue      SET DEFAULT '';
ALTER VIEW room_events ALTER COLUMN payload    SET DEFAULT '{}';
ALTER VIEW room_events ALTER COLUMN created_at SET DEFAULT NOW();

CREATE FUNCTION room_events_view_write() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM room_entries WHERE id = OLD.id AND kind = 'event';
        IF NOT FOUND THEN RETURN NULL; END IF;
        RETURN OLD;
    END IF;

    IF NEW.actor IS NULL OR length(NEW.actor) < 1 OR length(NEW.actor) > 200 THEN
        RAISE EXCEPTION 'room event actor must be 1..200 characters'
            USING ERRCODE = 'check_violation';
    END IF;

    IF TG_OP = 'INSERT' THEN
        INSERT INTO room_entries (room_id, kind, actor_label, event_type, issue, extension, created_at)
        VALUES (NEW.room_id, 'event', NEW.actor, NEW.event_type, COALESCE(NEW.issue, ''),
                COALESCE(NEW.payload, '{}'), COALESCE(NEW.created_at, NOW()))
        RETURNING id, issue, extension, created_at INTO NEW.id, NEW.issue, NEW.payload, NEW.created_at;
        RETURN NEW;
    END IF;

    UPDATE room_entries SET
        event_type = NEW.event_type, issue = NEW.issue, actor_label = NEW.actor,
        extension = NEW.payload, created_at = NEW.created_at
    WHERE id = OLD.id AND kind = 'event'
    RETURNING id, room_id INTO NEW.id, NEW.room_id;
    IF NOT FOUND THEN RETURN NULL; END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER room_events_view_write
    INSTEAD OF INSERT OR UPDATE OR DELETE ON room_events
    FOR EACH ROW EXECUTE FUNCTION room_events_view_write();
