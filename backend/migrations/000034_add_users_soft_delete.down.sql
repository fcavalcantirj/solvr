-- Rollback migration 000034: Remove soft delete support for users

-- Drop the partial index
DROP INDEX IF EXISTS idx_users_deleted_at;

-- Drop the deleted_at column
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
