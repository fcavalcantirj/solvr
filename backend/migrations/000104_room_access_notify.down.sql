DROP TRIGGER IF EXISTS rooms_access_notify_delete ON rooms;
DROP TRIGGER IF EXISTS rooms_access_notify_update ON rooms;
DROP TRIGGER IF EXISTS room_agent_tokens_access_notify_delete ON room_agent_tokens;
DROP TRIGGER IF EXISTS room_agent_tokens_access_notify_update ON room_agent_tokens;
DROP TRIGGER IF EXISTS room_members_access_notify_delete ON room_members;
DROP TRIGGER IF EXISTS room_members_access_notify_update ON room_members;
DROP FUNCTION IF EXISTS room_access_notify();
