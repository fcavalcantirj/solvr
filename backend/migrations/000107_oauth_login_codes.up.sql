-- One-time login codes for the browser OAuth flow (idx 75 step 5).
--
-- The OAuth callback used to redirect to {frontend}/auth/callback?token=<JWT>, which put a bearer
-- credential in a URL: browser history, the frontend host's access log and Google Analytics all
-- record the page URL. It now redirects with ?code=<login code>; the frontend POSTs the code to
-- /v1/auth/oauth/exchange and receives the token in a response body. The code is single-use and
-- lives 60 seconds, so a copy of the URL that leaks is worthless. Only the SHA-256 of a code is
-- stored, and the row is shared by every API instance.

CREATE TABLE oauth_login_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash CHAR(64) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_login_codes_expires_at ON oauth_login_codes(expires_at);

COMMENT ON TABLE oauth_login_codes IS 'Single-use, 60-second codes that the OAuth callback hands to the browser instead of a JWT';
COMMENT ON COLUMN oauth_login_codes.code_hash IS 'SHA-256 (hex) of the solvr_lc_ code; the plaintext is never stored';
COMMENT ON COLUMN oauth_login_codes.consumed_at IS 'Set by the one redeem that wins; a consumed code never redeems again';
