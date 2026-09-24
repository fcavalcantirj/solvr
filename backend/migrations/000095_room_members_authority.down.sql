-- Restore the agent-only allowlist of 000076. Human memberships (owner_id still holds
-- human ownership) and revoked rows (the old model had no revoked state) are dropped.
DROP TRIGGER IF EXISTS room_members_revoke_credentials ON room_members;
DROP FUNCTION IF EXISTS room_members_revoke_credentials();
DROP TRIGGER IF EXISTS room_members_keep_owner ON room_members;
DROP FUNCTION IF EXISTS room_members_keep_owner();
DROP TRIGGER IF EXISTS rooms_sync_human_owner ON rooms;
DROP FUNCTION IF EXISTS rooms_sync_human_owner();

DELETE FROM room_members WHERE user_id IS NOT NULL OR revoked_at IS NOT NULL;

DROP INDEX IF EXISTS idx_room_members_active_owners;
DROP INDEX IF EXISTS idx_room_members_user;
ALTER TABLE room_members DROP CONSTRAINT IF EXISTS room_members_exactly_one_actor;
ALTER TABLE room_members DROP CONSTRAINT IF EXISTS room_members_room_user_key;
ALTER TABLE room_members DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE room_members DROP COLUMN IF EXISTS display_role;
ALTER TABLE room_members DROP COLUMN IF EXISTS user_id;

ALTER TABLE room_members DROP CONSTRAINT IF EXISTS room_members_room_agent_key;
ALTER TABLE room_members DROP CONSTRAINT room_members_pkey;
ALTER TABLE room_members ALTER COLUMN agent_id SET NOT NULL;
ALTER TABLE room_members ADD CONSTRAINT room_members_pkey PRIMARY KEY (room_id, agent_id);
ALTER TABLE room_members DROP COLUMN IF EXISTS id;
