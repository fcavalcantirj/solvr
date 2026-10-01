-- A room's unique participants are found author by author (idx 77 step 1: room activity, measured
-- on representative data). The public room list showed each listed room's distinct message authors
-- as COUNT(DISTINCT author_id) over every live message of the room; at 2M room entries (idx 77
-- slice 14 spike, half in 20 hot rooms) that read ~23k messages per hot room, 75 ms each, and the
-- newest page took 1.5 s. RoomRepository.ListFiltered now walks this index one author at a time
-- (one probe per participant: 0.3 ms for the same room, the same counts over all 20k rooms). The
-- index is derived from room_entries alone; REINDEX rebuilds it.
CREATE INDEX IF NOT EXISTS idx_room_entries_room_author ON room_entries (room_id, author_id)
    WHERE kind = 'message' AND deleted_at IS NULL AND author_id IS NOT NULL;
