-- An agent's stored reputation is a projection of its grant rows (idx 77: counters and
-- projections are rebuildable from authoritative records). agents.reputation holds only the
-- agent's bonus (earned points are scored at read time from reputation_history and votes);
-- agent_reputation_grants is now its record, one row per grant:
--
--   agents.reputation = SUM(points) of the agent's agent_reputation_grants rows
--
-- A keyed grant is an activation, granted at most once per agent by the primary key:
--   human_claim     50  the first human claim (ClaimTokenRepository.ClaimAgent, in the claim's
--                       transaction; handlers.ReputationBonusOnClaim)
--   model_declared  10  the agent declared a model (registration or its first PATCH with one;
--                       handlers.ReputationBonusOnModel)
-- An adjustment (AgentRepository.AddReputation) is keyed 'adjustment:<uuid>': each call adds.
--
-- The API used to add both bonuses straight to agents.reputation with no record of the grant.
-- Measured through the API on the code before this migration (idx 77 slice 5): an agent
-- claimed again after its first owner deleted their account (DELETE /v1/me unclaims) gained
-- +50 again (100), and clearing and re-setting the model gained +10 per cycle (60 after five).
-- The grants' trigger moves the reputation inside the grant's own transaction instead: a
-- committed grant counts once, a rolled-back one never, a repeated activation that inserts
-- nothing moves nothing, and a deleted, changed or moved grant is uncounted where it was.
--
-- The activations an agent row shows (a claim, a model) are recorded below BEFORE the
-- trigger exists, so this migration moves no stored reputation. On the restored production
-- dump every agent's reputation is exactly 50 per claim plus 10 per model (199 of 199), so its
-- drift is empty afterwards; an agent the old code double-counted shows up in the drift check.
--
-- REBUILD PATH (operator, READ COMMITTED):
--   SELECT * FROM agent_reputation_drift();            -- agents whose reputation differs
--   SELECT * FROM agent_reputation_drift('<agent id>'); -- one agent
--   SELECT rebuild_agent_reputation();                 -- repair every agent
--   SELECT rebuild_agent_reputation('<agent id>');     -- repair one agent
--   SELECT replay_agent_activations();                 -- record missing activations only
-- The rebuild locks the agents' rows, records each activation a row shows but the ledger
-- lacks (a claim or model written without its grant, e.g. by the API before this migration
-- during a deploy), then sets every drifted reputation to its grants' sum. A grant's trigger
-- takes the same row lock, so a grant in flight is counted once it commits, and one that
-- arrives during the rebuild waits and then lands on the rebuilt value. A consistent row is
-- not rewritten; a second rebuild returns 0 and a second replay records nothing.

CREATE TABLE IF NOT EXISTS agent_reputation_grants (
    agent_id   VARCHAR(50) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    grant_key  TEXT        NOT NULL,
    points     INTEGER     NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, grant_key)
);

-- The activations each agent row shows, with their points. One place for both the replay and
-- the drift check.
CREATE OR REPLACE FUNCTION agent_activation_grants(p_agent_id varchar DEFAULT NULL)
RETURNS TABLE (agent_id varchar, grant_key text, points integer)
LANGUAGE sql STABLE AS $$
    SELECT a.id, 'human_claim'::text, 50 FROM agents a
     WHERE a.human_claimed_at IS NOT NULL AND (p_agent_id IS NULL OR a.id = p_agent_id)
    UNION ALL
    SELECT a.id, 'model_declared'::text, 10 FROM agents a
     WHERE COALESCE(a.model, '') <> '' AND (p_agent_id IS NULL OR a.id = p_agent_id);
$$;

-- Record the activations the ledger lacks; returns how many it recorded. Idempotent: the
-- primary key makes a replay grant nothing twice.
CREATE OR REPLACE FUNCTION replay_agent_activations(p_agent_id varchar DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    recorded integer;
BEGIN
    INSERT INTO agent_reputation_grants (agent_id, grant_key, points)
    SELECT a.agent_id, a.grant_key, a.points FROM agent_activation_grants(p_agent_id) a
    ON CONFLICT (agent_id, grant_key) DO NOTHING;
    GET DIAGNOSTICS recorded = ROW_COUNT;
    RETURN recorded;
END;
$$;

-- Backfill: no trigger yet, so no stored reputation moves.
SELECT replay_agent_activations();

CREATE OR REPLACE FUNCTION agent_reputation_grants_projection() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        UPDATE agents SET reputation = reputation - OLD.points WHERE id = OLD.agent_id;
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
        UPDATE agents SET reputation = reputation + NEW.points WHERE id = NEW.agent_id;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS agent_reputation_grants_insert ON agent_reputation_grants;
CREATE TRIGGER agent_reputation_grants_insert
    AFTER INSERT ON agent_reputation_grants
    FOR EACH ROW
    EXECUTE FUNCTION agent_reputation_grants_projection();

DROP TRIGGER IF EXISTS agent_reputation_grants_update ON agent_reputation_grants;
CREATE TRIGGER agent_reputation_grants_update
    AFTER UPDATE OF agent_id, points ON agent_reputation_grants
    FOR EACH ROW
    WHEN (OLD.agent_id IS DISTINCT FROM NEW.agent_id OR OLD.points IS DISTINCT FROM NEW.points)
    EXECUTE FUNCTION agent_reputation_grants_projection();

DROP TRIGGER IF EXISTS agent_reputation_grants_delete ON agent_reputation_grants;
CREATE TRIGGER agent_reputation_grants_delete
    AFTER DELETE ON agent_reputation_grants
    FOR EACH ROW
    EXECUTE FUNCTION agent_reputation_grants_projection();

-- Read-only reconciliation: the agents (all, or one) whose stored reputation differs from
-- their grants' sum, or whose row shows an activation the ledger lacks.
CREATE OR REPLACE FUNCTION agent_reputation_drift(p_agent_id varchar DEFAULT NULL)
RETURNS TABLE (agent_id varchar, stored_reputation integer, granted_reputation integer, missing_activations text[])
LANGUAGE sql STABLE AS $$
    SELECT a.id, a.reputation, g.total, m.keys
    FROM agents a
    CROSS JOIN LATERAL (
        SELECT COALESCE(SUM(r.points), 0)::integer AS total
        FROM agent_reputation_grants r WHERE r.agent_id = a.id
    ) g
    CROSS JOIN LATERAL (
        SELECT COALESCE(array_agg(x.grant_key ORDER BY x.grant_key), '{}') AS keys
        FROM agent_activation_grants(a.id) x
        WHERE NOT EXISTS (SELECT 1 FROM agent_reputation_grants r
                           WHERE r.agent_id = x.agent_id AND r.grant_key = x.grant_key)
    ) m
    WHERE (p_agent_id IS NULL OR a.id = p_agent_id)
      AND (a.reputation IS DISTINCT FROM g.total OR cardinality(m.keys) > 0)
    ORDER BY a.id;
$$;

CREATE OR REPLACE FUNCTION rebuild_agent_reputation(p_agent_id varchar DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    repaired integer;
BEGIN
    -- Lock first, check after: the statements below run on snapshots taken once every
    -- in-flight grant on these agents has committed, and new ones wait for us.
    PERFORM 1 FROM agents
     WHERE p_agent_id IS NULL OR id = p_agent_id
     ORDER BY id
       FOR NO KEY UPDATE;
    SELECT COUNT(*) INTO repaired FROM agent_reputation_drift(p_agent_id);
    IF repaired = 0 THEN
        RETURN 0;
    END IF;
    PERFORM replay_agent_activations(p_agent_id);
    UPDATE agents a
       SET reputation = d.granted_reputation
      FROM agent_reputation_drift(p_agent_id) d
     WHERE a.id = d.agent_id;
    RETURN repaired;
END;
$$;

COMMENT ON TABLE agent_reputation_grants IS
    'Record of every agent reputation grant; agents.reputation is their sum, kept by trigger (000121).';
COMMENT ON FUNCTION agent_reputation_drift(varchar) IS
    'Agents whose stored reputation differs from their grants, or that lack an activation their row shows (000121). Read-only.';
COMMENT ON FUNCTION rebuild_agent_reputation(varchar) IS
    'Record missing activations and recompute agents.reputation from agent_reputation_grants (all agents when NULL); returns agents repaired (000121).';
COMMENT ON FUNCTION replay_agent_activations(varchar) IS
    'Record the claim and model activations the grants lack; idempotent, returns grants recorded (000121).';
