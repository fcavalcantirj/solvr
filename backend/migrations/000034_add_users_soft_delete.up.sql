-- Migration 000034: Add soft delete support for users
-- Per PRD-v5 Task 11: User self-deletion feature
-- Adds deleted_at column and partial index for performance

-- Add deleted_at column to users table
-- NULL = user is active, NOT NULL = user is soft-deleted
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Create partial index to optimize queries filtering active users
-- Most queries will use WHERE deleted_at IS NULL
-- Partial index is much smaller and faster than full index
CREATE INDEX IF NOT EXISTS idx_users_deleted_at
ON users(deleted_at)
WHERE deleted_at IS NULL;
