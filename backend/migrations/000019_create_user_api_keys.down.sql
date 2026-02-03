-- Drop user_api_keys table and indexes
DROP INDEX IF EXISTS idx_user_api_keys_active;
DROP INDEX IF EXISTS idx_user_api_keys_user_id;
DROP TABLE IF EXISTS user_api_keys;
