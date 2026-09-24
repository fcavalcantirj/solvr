-- Restore 000018's prevent_agent_reclaim (any change of a non-null human_id refused) and
-- drop the soft-deletion room exit. Archived rooms and revoked rows are left as they are.
DROP TRIGGER IF EXISTS users_leave_rooms ON users;
DROP FUNCTION IF EXISTS users_leave_rooms();

CREATE OR REPLACE FUNCTION prevent_agent_reclaim()
RETURNS TRIGGER AS $$
BEGIN
    -- If human_id was already set (not null) and we're trying to change it
    IF OLD.human_id IS NOT NULL AND NEW.human_id IS DISTINCT FROM OLD.human_id THEN
        RAISE EXCEPTION 'agent_already_claimed: Agent is already linked to a human and cannot be re-claimed'
            USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
