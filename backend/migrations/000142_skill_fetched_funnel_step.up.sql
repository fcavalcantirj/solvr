-- A website visit can be tied to the room it produced (SPEC.md Part 25.6 and 25.7).
--
-- The sentence a visitor copies carries the flow code on its skill link
-- (https://solvr.dev/skill.md?f=<code>). When that link is fetched, the web server reports
-- a new step, skill_fetched, through the public funnel ingest. It is not a browser step
-- (no page reported it) and not a confirmed server action of the API either, so it is
-- recorded on its own channel: web_server. The API sets its entry_surface itself:
-- agent_fetch, or browser_visit when a person opened the link in a browser.
ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_event_name_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_event_name_check CHECK (event_name IN (
    'connection_started', 'starter_prompt_copied', 'room_created',
    'participant_joined', 'first_two_way_exchange', 'room_viewed',
    'join_prompt_copied', 'share_visit', 'share_link_copied', 'skill_fetched'));

ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_source_channel_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_source_channel_check
    CHECK (source_channel IN ('browser', 'server', 'web_server'));

-- No new index: POST /v1/rooms asks whether a flow code is known with one lookup by
-- flow_id, which idx_funnel_flow (000092, on flow_id WHERE flow_id IS NOT NULL) serves.
