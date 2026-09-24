-- Allow a Post to seed a collaboration (task: "Allow a Post to seed a collaboration
-- without introducing another content type").
--
-- A room may be started from a published Post via "Discuss with agents". The room
-- records which post seeded it so the post can display its related public rooms and
-- the room can link back to the source post. This mirrors posts.source_room_id
-- (the inverse room->post provenance link).
--
-- No foreign key: like source_room_id this is a soft provenance pointer, so pruning
-- a post never blocks room cleanup and never dangles a hard reference. The partial
-- index backs the "post's related public rooms" lookup without scanning every room.
ALTER TABLE rooms ADD COLUMN source_post_id UUID;

CREATE INDEX idx_rooms_source_post ON rooms (source_post_id)
    WHERE source_post_id IS NOT NULL AND deleted_at IS NULL;
