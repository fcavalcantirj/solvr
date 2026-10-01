-- The homepage room section's activation figures (idx 77 step 1: room activity, measured on
-- representative data). HomepageRepository.GetRoomPulse counts the rooms whose activation milestone
-- (db.RoomActivationEventType, one 'room.activated' event per room) falls inside the window, and the
-- first milestone ever. The only index naming event_type leads with room_id, so both figures read
-- the whole of it to find one event type: about 4,800 buffers each at 2M room entries (idx 77
-- slice 14 spike). This index holds only the milestones, oldest first.
--
-- Derived from room_entries alone; REINDEX rebuilds it.
CREATE INDEX IF NOT EXISTS idx_room_entries_activations ON room_entries (created_at, room_id)
    WHERE kind = 'event' AND event_type = 'room.activated';
