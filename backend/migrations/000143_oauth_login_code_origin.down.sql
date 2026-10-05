-- The exchange stops telling a sign-up from a login: both facts leave with their columns.
-- Codes live 60 seconds, so nothing that outlasts this migration is lost.
ALTER TABLE oauth_login_codes
    DROP COLUMN IF EXISTS auth_provider,
    DROP COLUMN IF EXISTS is_new_user;
