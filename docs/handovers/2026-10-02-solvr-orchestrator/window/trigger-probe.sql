-- Cutover window G5 and rollback: does users_refuse_tombstoned_email refuse a tombstoned email? Everything is rolled back.
-- Emails are read inside the block and never printed; other errors print only their SQLSTATE.
BEGIN;
DO $$
DECLARE
  probe record;
  e text;
  outcome text;
BEGIN
  FOR probe IN SELECT * FROM (VALUES
      ('nan_song', 'tombstone only (2026-03-19), not in any ban list'),
      ('xu_wei', 'purge tombstone (2026-09-29), also a banned email after 000114'),
      ('<fresh>', 'control: an email no account has')) v(username, note)
  LOOP
    IF probe.username = '<fresh>' THEN
      e := 'window-probe-' || md5(random()::text) || '@localhost';
    ELSE
      SELECT email INTO e FROM users WHERE username = probe.username;
    END IF;
    BEGIN
      INSERT INTO users (username, display_name, email, referral_code)
      VALUES ('window_probe_' || left(md5(random()::text), 8), 'window probe', upper(e), upper(left(md5(random()::text), 8)));
      outcome := 'ACCEPTED';
    EXCEPTION
      WHEN SQLSTATE 'P0001' THEN
        outcome := CASE WHEN SQLERRM = 'account suspended' THEN 'REFUSED P0001 account suspended' ELSE 'P0001 other message' END;
      WHEN OTHERS THEN
        outcome := 'OTHER ERROR ' || SQLSTATE;
    END;
    RAISE NOTICE 'trigger probe % (%): %', probe.username, probe.note, outcome;
  END LOOP;
END $$;
ROLLBACK;
