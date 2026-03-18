DROP TABLE IF EXISTS referrals;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_referral_code_key;
ALTER TABLE users DROP COLUMN IF EXISTS referral_code;
