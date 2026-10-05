-- The OAuth code exchange can tell a sign-up from a login (SPEC.md Part 5.2).
--
-- The callback that issues a login code knows two things the exchange cannot find out later:
-- whether this sign-in created the account, and which provider it went through. Both are
-- stored with the code and answered by POST /v1/auth/oauth/exchange as is_new_user and
-- provider. The web client reads them only to report a sign-up or a login, with its method,
-- to analytics (SPEC.md Part 27.7); neither is a credential or a permission.
--
-- Both columns have a default, so a code written by an API instance from before this
-- migration (it does not name them) is a login through no named provider.
--
-- Apply this migration BEFORE the API that reads the columns is deployed: that API names
-- them when it issues a code, and both callbacks answer login_failed while they are missing.
--
-- IF NOT EXISTS: it can be applied again over a schema that already has the columns (the
-- legacy-archive test re-applies every migration after the archive).
ALTER TABLE oauth_login_codes
    ADD COLUMN IF NOT EXISTS is_new_user BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(20) NOT NULL DEFAULT '';

COMMENT ON COLUMN oauth_login_codes.is_new_user IS 'True only when the sign-in that issued this code created the account';
COMMENT ON COLUMN oauth_login_codes.auth_provider IS 'The provider that sign-in went through: github or google; empty when it was not recorded';
