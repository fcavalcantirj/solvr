-- Restores agent_reputation_drift as 000121 defined it (one agent_activation_grants call per agent).

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
