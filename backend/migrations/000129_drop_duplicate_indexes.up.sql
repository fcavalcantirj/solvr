-- Drop the plain indexes that duplicate a unique index of the same table (idx 77 step 1: indexes
-- chosen from measured query plans). Each pair has the same access method, columns, operator
-- classes, collations, orderings and predicate, so the unique index answers every lookup the plain
-- one did, and the plain one is only write cost. The largest is on the room timeline: at 600k room
-- entries idx_room_entries_room_seq was 37 MB, and 100k entry inserts took 1,257 / 1,193 ms with it
-- and 1,078 / 997 ms without it, writing 103-104 MB of WAL against 94 MB (idx 77 slice 17 spike).
-- Timeline replay, the resume cursor and the latest sequence read room_entries_room_seq_unique
-- instead, at the same cost (TestRoomTimeline_AtGrowthVolume...).
--
-- Kept: idx_users_username and idx_users_email (TestMigrations_UsersTable pins them by name).
-- Indexes are derived from their tables; the down migration rebuilds the dropped ones.
DROP INDEX IF EXISTS idx_room_entries_room_seq;
DROP INDEX IF EXISTS idx_refresh_tokens_token_hash;
DROP INDEX IF EXISTS idx_auth_methods_provider_lookup;
DROP INDEX IF EXISTS idx_tags_name;
DROP INDEX IF EXISTS idx_config_key;
DROP INDEX IF EXISTS idx_rate_limit_config_key;
