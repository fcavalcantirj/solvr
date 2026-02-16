-- Rollback optimization indexes
-- Migration 000033 rollback

DROP INDEX IF EXISTS idx_auth_methods_last_used;
