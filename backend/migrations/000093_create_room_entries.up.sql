-- Unified room timeline (task: "Store room messages and coordination events in one
-- ordered room timeline"). room_entries replaces the parallel messages and room_events
-- tables: every action is stored once as an ordered per-room entry that is either a
-- freeform 'message' (Body/ContentType, optional reply/addressing) or a typed
-- coordination 'event' (EventType/Issue with an Extension payload).
--
-- This migration is the EXPAND step: it creates the new storage and backfills existing
-- messages and room_events into it, keeping a legacy-ID map so old message links and
-- SSE cursors resolve during the transition. The parallel tables are left in place for a
-- later cutover step; nothing here removes them.
CREATE TABLE room_entries (
    id                   BIGSERIAL PRIMARY KEY,
    room_id              UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sequence             INT  NOT NULL,
    kind                 TEXT NOT NULL CHECK (kind IN ('message', 'event')),

    -- Authenticated actor (NULL for shared-token/system writes) + historical label.
    author_type          VARCHAR(10) CHECK (author_type IN ('human', 'agent', 'system')),
    author_id            VARCHAR(255),
    actor_label          VARCHAR(200) NOT NULL,

    -- Message fields.
    body                 TEXT CHECK (body IS NULL OR length(body) <= 65536),
    content_type         VARCHAR(20) NOT NULL DEFAULT 'text'
                             CHECK (content_type IN ('text', 'markdown', 'json')),
    reply_to_entry_id    BIGINT REFERENCES room_entries(id) ON DELETE SET NULL,
    addressed_member_ids JSONB,
    supersedes_entry_id  BIGINT REFERENCES room_entries(id) ON DELETE SET NULL,
    pinned_at            TIMESTAMPTZ,

    -- Event fields.
    event_type           TEXT CHECK (event_type IS NULL OR length(event_type) BETWEEN 1 AND 50),
    issue                TEXT NOT NULL DEFAULT '' CHECK (length(issue) <= 200),

    -- Bounded extension data: message metadata or event payload.
    extension            JSONB NOT NULL DEFAULT '{}'
                             CHECK (length(extension::text) <= 16384),

    client_entry_id      VARCHAR(255),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ,

    -- A message must carry a body; an event must carry an event_type. This keeps a
    -- single table honest without splitting it back into two.
    CONSTRAINT room_entries_kind_fields CHECK (
        (kind = 'message' AND body IS NOT NULL) OR
        (kind = 'event'   AND event_type IS NOT NULL)
    ),
    -- One ordered timeline per room: (room_id, sequence) is unique and gap-tolerant.
    CONSTRAINT room_entries_room_seq_unique UNIQUE (room_id, sequence)
);

-- Transcript read (messages, oldest->newest) and full-timeline replay.
CREATE INDEX idx_room_entries_room_seq     ON room_entries (room_id, sequence);
CREATE INDEX idx_room_entries_room_active  ON room_entries (room_id, created_at)
    WHERE deleted_at IS NULL;
-- Event views retain the original room_events filters.
CREATE INDEX idx_room_entries_room_type    ON room_entries (room_id, event_type, id DESC)
    WHERE kind = 'event';
CREATE INDEX idx_room_entries_room_issue   ON room_entries (room_id, issue, id DESC)
    WHERE kind = 'event';
CREATE INDEX idx_room_entries_reply_to     ON room_entries (reply_to_entry_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_room_entries_addressed    ON room_entries USING GIN (addressed_member_ids);

-- Idempotent write key mirrors messages: a retry from the same authenticated author
-- with the same client_entry_id resolves to the existing entry instead of a duplicate.
CREATE UNIQUE INDEX idx_room_entries_client_entry
    ON room_entries (room_id, author_id, client_entry_id)
    WHERE client_entry_id IS NOT NULL AND author_id IS NOT NULL AND deleted_at IS NULL;

-- Legacy-ID map: resolves an old messages.id / room_events.id to its new entry id so
-- deep links, SSE cursors and accepted references survive the transition.
CREATE TABLE room_entry_legacy_map (
    legacy_kind TEXT   NOT NULL CHECK (legacy_kind IN ('message', 'event')),
    legacy_id   BIGINT NOT NULL,
    entry_id    BIGINT NOT NULL REFERENCES room_entries(id) ON DELETE CASCADE,
    PRIMARY KEY (legacy_kind, legacy_id)
);
CREATE INDEX idx_room_entry_legacy_map_entry ON room_entry_legacy_map (entry_id);

-- Backfill existing rows. Idempotent and incremental: only legacy rows not already in
-- the map are inserted, sequenced after the room's current MAX(sequence), merged by
-- created_at with a stable tie breaker (messages before events, then legacy id).
WITH unified AS (
    SELECT m.room_id, 'message'::text AS kind,
           m.author_type, m.author_id, m.agent_name AS actor_label,
           m.content AS body, m.content_type,
           NULL::text AS event_type, ''::text AS issue,
           m.metadata AS extension,
           m.addressed_member_ids, m.client_entry_id, m.pinned_at,
           m.created_at, m.deleted_at,
           'message'::text AS legacy_kind, m.id AS legacy_id, 0 AS source_rank
    FROM messages m
    WHERE NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                      WHERE lm.legacy_kind = 'message' AND lm.legacy_id = m.id)
    UNION ALL
    SELECT e.room_id, 'event'::text,
           NULL, NULL, e.actor,
           NULL, 'text',
           e.event_type, e.issue,
           e.payload,
           NULL, NULL, NULL,
           e.created_at, NULL,
           'event'::text, e.id, 1
    FROM room_events e
    WHERE NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                      WHERE lm.legacy_kind = 'event' AND lm.legacy_id = e.id)
),
base AS (
    SELECT room_id, COALESCE(MAX(sequence), 0) AS maxseq FROM room_entries GROUP BY room_id
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
        (room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
         event_type, issue, extension, addressed_member_ids, client_entry_id, pinned_at, created_at, deleted_at)
    SELECT room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
           event_type, issue, extension, addressed_member_ids, client_entry_id, pinned_at, created_at, deleted_at
    FROM seq
    RETURNING id AS entry_id, room_id, sequence
)
INSERT INTO room_entry_legacy_map (legacy_kind, legacy_id, entry_id)
SELECT s.legacy_kind, s.legacy_id, ins.entry_id
FROM ins JOIN seq s ON s.room_id = ins.room_id AND s.sequence = ins.sequence;

-- Remap reply and supersede references from legacy message ids to new entry ids.
UPDATE room_entries re
SET reply_to_entry_id = tgt.entry_id
FROM room_entry_legacy_map src
JOIN messages m ON m.id = src.legacy_id AND src.legacy_kind = 'message'
JOIN room_entry_legacy_map tgt ON tgt.legacy_kind = 'message' AND tgt.legacy_id = m.reply_to_entry_id
WHERE re.id = src.entry_id
  AND m.reply_to_entry_id IS NOT NULL
  AND re.reply_to_entry_id IS DISTINCT FROM tgt.entry_id;

UPDATE room_entries re
SET supersedes_entry_id = tgt.entry_id
FROM room_entry_legacy_map src
JOIN messages m ON m.id = src.legacy_id AND src.legacy_kind = 'message'
JOIN room_entry_legacy_map tgt ON tgt.legacy_kind = 'message' AND tgt.legacy_id = m.supersedes_entry_id
WHERE re.id = src.entry_id
  AND m.supersedes_entry_id IS NOT NULL
  AND re.supersedes_entry_id IS DISTINCT FROM tgt.entry_id;
