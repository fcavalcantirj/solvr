-- Back to an API-maintained agent bonus: the code before 000121 adds the claim and model
-- bonuses to agents.reputation itself. agents.reputation keeps its value (the grants' sum);
-- the per-grant breakdown is dropped with the table.
DROP TRIGGER IF EXISTS agent_reputation_grants_insert ON agent_reputation_grants;
DROP TRIGGER IF EXISTS agent_reputation_grants_update ON agent_reputation_grants;
DROP TRIGGER IF EXISTS agent_reputation_grants_delete ON agent_reputation_grants;
DROP FUNCTION IF EXISTS rebuild_agent_reputation(varchar);
DROP FUNCTION IF EXISTS agent_reputation_drift(varchar);
DROP FUNCTION IF EXISTS replay_agent_activations(varchar);
DROP FUNCTION IF EXISTS agent_reputation_grants_projection();
DROP FUNCTION IF EXISTS agent_activation_grants(varchar);
DROP TABLE IF EXISTS agent_reputation_grants;
