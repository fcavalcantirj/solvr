-- skill_fetched leaves the funnel: its rows are removed (the step has no meaning in the
-- older contract, and they are the only rows on the web_server channel), then both CHECKs
-- return to what 000136 left.
DELETE FROM funnel_events WHERE event_name = 'skill_fetched' OR source_channel = 'web_server';

ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_source_channel_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_source_channel_check
    CHECK (source_channel IN ('browser', 'server'));

ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_event_name_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_event_name_check CHECK (event_name IN (
    'connection_started', 'starter_prompt_copied', 'room_created',
    'participant_joined', 'first_two_way_exchange', 'room_viewed',
    'join_prompt_copied', 'share_visit', 'share_link_copied'));
