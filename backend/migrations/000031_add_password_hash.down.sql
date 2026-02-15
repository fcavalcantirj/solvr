-- Migration rollback: Remove password_hash and restore OAuth NOT NULL constraints
--
-- WARNING: This rollback is potentially DESTRUCTIVE.
-- If any email/password users exist (auth_provider IS NULL), this will FAIL.
-- Do NOT run this in production if email/password users exist.

-- Safety check: Ensure no email/password users exist before rollback
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM users
        WHERE (auth_provider IS NULL OR auth_provider = 'email')
        AND password_hash IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'Cannot rollback: email/password users exist in database. Delete them first or keep the migration.';
    END IF;
END $$;

-- Remove password_hash column
ALTER TABLE users DROP COLUMN password_hash;

-- Restore NOT NULL constraints on OAuth fields
-- This will fail if any users have NULL auth_provider or auth_provider_id
ALTER TABLE users ALTER COLUMN auth_provider_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN auth_provider SET NOT NULL;
