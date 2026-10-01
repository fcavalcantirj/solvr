-- agent_reputation_drift() reads every agent's missing activations in one pass (idx 77 step 5:
-- counters reconciled after migration and periodic rebuilds). 000121 defined it with a
-- correlated agent_activation_grants(a.id) per agent; that function's
-- "p_agent_id IS NULL OR a.id = p_agent_id" filter cannot use the primary key, so each call
-- scanned the agents table twice and a check of every agent cost O(agents^2).
--
-- Measured (idx 77 slice 24 spike: 20,000 agents, a third claimed, half with a model, every
-- activation granted), before -> after:
--   SELECT count(*) FROM agent_reputation_drift()   44-60 s -> 0.12-0.18 s
--     (40,000 sequential scans of agents and 36M buffer hits -> 3 scans)
--   rebuild_agent_reputation(), nothing to repair    51.2 s -> 0.18 s
--   rebuild_agent_reputation(), 347 agents drifted   99.5 s -> 0.37 s (347 repaired by both)
-- The rebuild holds every agent row locked while it runs, so the cutover's agent_reputation
-- step blocked every agent write (heartbeats, claims, profile edits) for minutes.
--
-- Same rows in the same order: on that copy with 551 agents broken three ways (stored score
-- off, an activation missing, both), both definitions returned the same 551 rows (EXCEPT ALL
-- empty both ways, identical agent_id order). agent_activation_grants() stays the one place
-- that says which activations an agent row shows; it is now called once, with the caller's
-- p_agent_id, instead of once per agent.

CREATE OR REPLACE FUNCTION agent_reputation_drift(p_agent_id varchar DEFAULT NULL)
RETURNS TABLE (agent_id varchar, stored_reputation integer, granted_reputation integer, missing_activations text[])
LANGUAGE sql STABLE AS $$
    SELECT a.id, a.reputation, g.total, COALESCE(m.keys, '{}')
    FROM agents a
    CROSS JOIN LATERAL (
        SELECT COALESCE(SUM(r.points), 0)::integer AS total
        FROM agent_reputation_grants r WHERE r.agent_id = a.id
    ) g
    LEFT JOIN (
        SELECT x.agent_id, array_agg(x.grant_key ORDER BY x.grant_key) AS keys
        FROM agent_activation_grants(p_agent_id) x
        WHERE NOT EXISTS (SELECT 1 FROM agent_reputation_grants r
                           WHERE r.agent_id = x.agent_id AND r.grant_key = x.grant_key)
        GROUP BY x.agent_id
    ) m ON m.agent_id = a.id
    WHERE (p_agent_id IS NULL OR a.id = p_agent_id)
      AND (a.reputation IS DISTINCT FROM g.total OR m.agent_id IS NOT NULL)
    ORDER BY a.id;
$$;
