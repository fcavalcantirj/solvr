-- The connection funnel, measured across browser and agent activity.
--
-- One row per funnel step. The funnel joins two channels a visitor's connection
-- crosses: the BROWSER (a panel opened, a prompt copied, a room viewed) and the
-- SERVER (a room created, a participant joined, the first two-way exchange). A
-- non-secret flow_id issued by GET /v1/connect travels in the copied prompt and
-- the create-room call, so a browser step and the server steps of the same
-- attempt share one flow_id even though no browser event fires for an
-- agent-generated invite.
--
-- What is deliberately NOT stored, because analytics must never expose it:
-- credentials, private message bodies, task text, and raw private room titles.
-- actor_ref is a pseudonymous hash, never a raw account or agent id. room_id is
-- an opaque UUID used only to derive ordinals, dedupe milestones and link server
-- steps of one room — it is internal analytics, never returned to a public
-- surface.
CREATE TABLE IF NOT EXISTS funnel_events (
    id                  BIGSERIAL PRIMARY KEY,
    flow_id             TEXT,
    event_name          TEXT NOT NULL,
    source_channel      TEXT NOT NULL,
    actor_type          TEXT NOT NULL,
    actor_ref           TEXT,
    room_id             UUID,
    preset              TEXT,
    role                TEXT,
    ordinal             SMALLINT,
    entry_surface       TEXT,
    instruction_version TEXT,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT funnel_events_event_name_check CHECK (event_name IN (
        'connection_started', 'starter_prompt_copied', 'room_created',
        'participant_joined', 'first_two_way_exchange', 'room_viewed',
        'join_prompt_copied')),
    CONSTRAINT funnel_events_source_channel_check CHECK (source_channel IN ('browser', 'server')),
    CONSTRAINT funnel_events_actor_type_check CHECK (actor_type IN ('agent', 'human', 'anonymous')),
    CONSTRAINT funnel_events_flow_id_len CHECK (flow_id IS NULL OR length(flow_id) <= 64),
    CONSTRAINT funnel_events_actor_ref_len CHECK (actor_ref IS NULL OR length(actor_ref) <= 128),
    CONSTRAINT funnel_events_preset_len CHECK (preset IS NULL OR length(preset) <= 50),
    CONSTRAINT funnel_events_role_len CHECK (role IS NULL OR length(role) <= 50),
    CONSTRAINT funnel_events_entry_surface_len CHECK (entry_surface IS NULL OR length(entry_surface) <= 60),
    CONSTRAINT funnel_events_instruction_version_len CHECK (instruction_version IS NULL OR length(instruction_version) <= 20)
);

COMMENT ON TABLE funnel_events IS
    'One connection-funnel step, browser or server. Internal analytics only: no credentials, no bodies, no task text, no private titles; actor_ref is pseudonymous.';
COMMENT ON COLUMN funnel_events.flow_id IS
    'Non-secret flow identifier issued by GET /v1/connect. Links a browser step to the server steps of the same connection attempt.';
COMMENT ON COLUMN funnel_events.actor_ref IS
    'Pseudonymous hash of the authenticated actor, never a raw account/agent id. NULL for anonymous browser steps.';
COMMENT ON COLUMN funnel_events.ordinal IS
    'For participant_joined: the 1-based order this actor joined the room, so first/second participant milestones can be derived without capping the participant count.';

-- A room is created once, its first two-way exchange happens once, and one actor
-- joins a room once: these milestones dedupe so a retry, a reconnect or a replayed
-- write cannot inflate a funnel count.
CREATE UNIQUE INDEX idx_funnel_room_created_unique
    ON funnel_events (room_id)
    WHERE event_name = 'room_created' AND room_id IS NOT NULL;

CREATE UNIQUE INDEX idx_funnel_first_two_way_unique
    ON funnel_events (room_id)
    WHERE event_name = 'first_two_way_exchange' AND room_id IS NOT NULL;

CREATE UNIQUE INDEX idx_funnel_participant_joined_unique
    ON funnel_events (room_id, actor_ref)
    WHERE event_name = 'participant_joined' AND room_id IS NOT NULL AND actor_ref IS NOT NULL;

-- Analysis reads a flow end to end, a room's server steps, and windows ending now.
CREATE INDEX idx_funnel_flow ON funnel_events (flow_id) WHERE flow_id IS NOT NULL;
CREATE INDEX idx_funnel_room ON funnel_events (room_id) WHERE room_id IS NOT NULL;
CREATE INDEX idx_funnel_event_occurred ON funnel_events (event_name, occurred_at DESC);
