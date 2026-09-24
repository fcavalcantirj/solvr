-- Account lifecycle leaves every room in a documented, manageable or archived state.
--
-- 1. Agent unlinking. SPEC Part 20.2: DELETE /v1/me unclaims the user's agents
--    (human_id -> NULL) and they "can be claimed by other humans". 000018's
--    prevent_agent_reclaim refused ANY change of a non-null human_id, so that unclaim
--    failed and account deletion returned 500 for every user with a claimed agent.
--    Only a direct re-claim X -> Y stays forbidden; X -> NULL (unlink) is allowed, and
--    NULL -> Y was always allowed. Unlinking touches no direct membership: the agent keeps
--    the rows it holds itself, and 000096's agents_end_family_memberships revokes the
--    family-derived ones.
-- 2. Human soft deletion (users.deleted_at set). The account can no longer authenticate,
--    so it leaves its rooms:
--      * a live room in which it was the last active owner whose account is live
--        (users/agents.deleted_at IS NULL) is ARCHIVED, the same terminal state 000095
--        gives a room whose owner account is hard-deleted; that owner row is KEPT active
--        so an admin account recovery restores ownership, and hard deletion later
--        cascades it away without further effect;
--      * every other active membership of the account (member rows, owner rows in rooms
--        that keep a live owner, rows in deleted rooms) is revoked.

CREATE OR REPLACE FUNCTION prevent_agent_reclaim()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.human_id IS NOT NULL AND NEW.human_id IS NOT NULL
       AND NEW.human_id IS DISTINCT FROM OLD.human_id THEN
        RAISE EXCEPTION 'agent_already_claimed: Agent is already linked to a human and cannot be re-claimed'
            USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION users_leave_rooms() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    WITH sole AS (
        SELECT m.room_id
        FROM room_members m
        JOIN rooms r ON r.id = m.room_id
        WHERE m.user_id = NEW.id AND m.role = 'owner' AND m.revoked_at IS NULL
          AND r.deleted_at IS NULL
          AND NOT EXISTS (
              SELECT 1
              FROM room_members o
              LEFT JOIN users u  ON u.id = o.user_id
              LEFT JOIN agents a ON a.id = o.agent_id
              WHERE o.room_id = m.room_id AND o.role = 'owner' AND o.revoked_at IS NULL
                AND o.user_id IS DISTINCT FROM NEW.id
                AND COALESCE(u.deleted_at, a.deleted_at) IS NULL)
    ), archived AS (
        UPDATE rooms SET archived_at = COALESCE(archived_at, NOW()), updated_at = NOW()
        WHERE id IN (SELECT room_id FROM sole)
        RETURNING id
    )
    UPDATE room_members SET revoked_at = NOW()
    WHERE user_id = NEW.id AND revoked_at IS NULL
      AND room_id NOT IN (SELECT room_id FROM sole);
    RETURN NULL;
END;
$$;

CREATE TRIGGER users_leave_rooms
    AFTER UPDATE OF deleted_at ON users
    FOR EACH ROW WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION users_leave_rooms();
