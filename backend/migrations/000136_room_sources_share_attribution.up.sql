-- Public collaborations bring in new participants (idx 88; SPEC.md Part 25.7).
--
-- rooms.source_room_id records the PUBLIC room whose task structure seeded a fresh room
-- ("Try this workflow" / "Start a new room"). It is provenance only: no membership,
-- credential, pin, event or result of the source room is ever copied. Deleting the source
-- keeps the new room and clears the pointer.
ALTER TABLE rooms ADD COLUMN source_room_id UUID REFERENCES rooms(id) ON DELETE SET NULL;
CREATE INDEX idx_rooms_source_room_id ON rooms (source_room_id) WHERE source_room_id IS NOT NULL;

-- Share attribution in the connection funnel. Two browser steps join the contract:
-- share_visit (a public room or post page opened from a share link) and share_link_copied
-- (a share link or outcome excerpt copied, after the clipboard write succeeded). Any step
-- may name the public source it is attributed to: source_kind + source_id, both or
-- neither, resolved by the API from a public room or post — never a raw client string.
-- Server steps of a room inherit the room_created step's source, as they inherit flow_id.
ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_event_name_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_event_name_check CHECK (event_name IN (
    'connection_started', 'starter_prompt_copied', 'room_created',
    'participant_joined', 'first_two_way_exchange', 'room_viewed',
    'join_prompt_copied', 'share_visit', 'share_link_copied'));

ALTER TABLE funnel_events
    ADD COLUMN source_kind TEXT,
    ADD COLUMN source_id UUID;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_source_check CHECK (
    (source_kind IS NULL AND source_id IS NULL)
    OR (source_kind IN ('room', 'post') AND source_id IS NOT NULL));

CREATE INDEX idx_funnel_source ON funnel_events (source_kind, source_id, occurred_at)
    WHERE source_id IS NOT NULL;

COMMENT ON COLUMN funnel_events.source_id IS
    'The public room or post (per source_kind) this step is attributed to; resolved by the API, never client text.';
