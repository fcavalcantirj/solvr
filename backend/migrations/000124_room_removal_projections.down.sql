-- Rooms changed outside RoomHandler wait for the overview snapshot again, and a removed agent's
-- presence lasts until its TTL (000123 behavior). overview_changed_notify() belongs to 000123.
DROP TRIGGER IF EXISTS agents_removed_presence_end ON agents;
DROP FUNCTION IF EXISTS agent_removed_presence_end();
DROP TRIGGER IF EXISTS rooms_overview_changed_delete ON rooms;
DROP TRIGGER IF EXISTS rooms_overview_changed_update ON rooms;
