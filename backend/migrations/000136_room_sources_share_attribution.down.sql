-- The share attribution leaves the funnel: its two browser steps are removed (they have
-- no meaning in the older contract) and the source columns dropped.
DROP INDEX IF EXISTS idx_funnel_source;
ALTER TABLE funnel_events DROP CONSTRAINT IF EXISTS funnel_events_source_check;
ALTER TABLE funnel_events DROP COLUMN IF EXISTS source_id, DROP COLUMN IF EXISTS source_kind;

DELETE FROM funnel_events WHERE event_name IN ('share_visit', 'share_link_copied');
ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_event_name_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_event_name_check CHECK (event_name IN (
    'connection_started', 'starter_prompt_copied', 'room_created',
    'participant_joined', 'first_two_way_exchange', 'room_viewed',
    'join_prompt_copied'));

DROP INDEX IF EXISTS idx_rooms_source_room_id;
ALTER TABLE rooms DROP COLUMN IF EXISTS source_room_id;
