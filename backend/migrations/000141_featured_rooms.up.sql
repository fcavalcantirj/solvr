-- The homepage's featured rooms (owner decision 2026-10-04).
--
-- "Rooms worth reading" on the homepage is a pool the operator curates, not a
-- ranking: a room is shown because someone chose it, never because it is busy.
-- The API shows at most three public rooms from this pool and rotates through it
-- one day at a time; a room that turns private or is deleted drops out on the
-- next read. ask_seq / outcome_seq optionally name the messages (by their
-- sequence number in the room) the card quotes as what was asked and what came
-- out; NULL means the API picks them (pinned entries first, else the first and
-- last messages). Deleting a room removes it from the pool.
CREATE TABLE IF NOT EXISTS featured_rooms (
    room_id     UUID PRIMARY KEY REFERENCES rooms(id) ON DELETE CASCADE,
    featured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ask_seq     INTEGER,
    outcome_seq INTEGER
);
