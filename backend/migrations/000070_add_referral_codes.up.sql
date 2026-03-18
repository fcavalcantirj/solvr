-- Add referral_code column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(8);

-- Backfill existing users with unique random 8-char alphanumeric codes
-- Uses DO $$ block for atomicity with schema change
DO $$
DECLARE
    u RECORD;
    new_code VARCHAR(8);
    chars TEXT := 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    i INT;
BEGIN
    FOR u IN SELECT id FROM users WHERE referral_code IS NULL LOOP
        LOOP
            new_code := '';
            FOR i IN 1..8 LOOP
                new_code := new_code || substr(chars, floor(random() * 36 + 1)::int, 1);
            END LOOP;
            -- Check uniqueness before assigning
            EXIT WHEN NOT EXISTS (SELECT 1 FROM users WHERE referral_code = new_code);
        END LOOP;
        UPDATE users SET referral_code = new_code WHERE id = u.id;
    END LOOP;
END $$;

-- Now that all rows have values, add constraints
ALTER TABLE users ALTER COLUMN referral_code SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_referral_code_key UNIQUE (referral_code);

-- Create referrals tracking table
CREATE TABLE IF NOT EXISTS referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id UUID NOT NULL REFERENCES users(id),
    referred_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT referrals_referred_id_key UNIQUE (referred_id)
);

-- Index for counting referrals by referrer
CREATE INDEX IF NOT EXISTS idx_referrals_referrer_id ON referrals(referrer_id);
