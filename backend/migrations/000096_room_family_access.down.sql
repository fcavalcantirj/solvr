DROP TRIGGER IF EXISTS users_end_family_memberships ON users;
DROP FUNCTION IF EXISTS users_end_family_memberships();
DROP TRIGGER IF EXISTS room_members_end_family_on_owner_loss ON room_members;
DROP FUNCTION IF EXISTS room_members_end_family_on_owner_loss();
DROP TRIGGER IF EXISTS agents_end_family_memberships ON agents;
DROP FUNCTION IF EXISTS agents_end_family_memberships();
DROP INDEX IF EXISTS idx_room_members_family;
ALTER TABLE room_members DROP COLUMN IF EXISTS access_source;
