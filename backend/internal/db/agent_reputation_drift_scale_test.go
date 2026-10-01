package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A drift check over every agent reads the agents table a fixed number of times, however many
// agents there are (idx 77 slice 24, migration 000132). 000121's agent_reputation_drift()
// called agent_activation_grants(a.id) once per agent, and that function's
// "p_agent_id IS NULL OR a.id = p_agent_id" filter cannot use the primary key, so every call
// scanned the whole table twice. Measured at 20,000 agents: 40,000 sequential scans of agents,
// 36M buffer hits and 44 s for a check that found nothing; the cutover's agent_reputation step
// (drift, a rebuild that runs it twice with every agent row locked, drift again) took minutes.

// seedScaleAgents inserts agents scale_<from>..scale_<to> with their activations granted, then
// breaks every tenth agent one of three ways and returns the drift those breaks must produce:
// g%10 == 1: stored reputation 7 above its grants;
// g%10 == 2 and claimed (g%3 == 0): its claim grant deleted (the trigger lowers the score), so
// only the activation is missing;
// g%10 == 3 and no model (g odd): a model set by SQL, with no activation.
func seedScaleAgents(ctx context.Context, t *testing.T, pool *Pool, from, to int) []agentReputationDrift {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	exec(`INSERT INTO agents (id, display_name, human_claimed_at, model)
		SELECT 'scale_' || lpad(g::text, 4, '0'), 'Scale ' || g,
		       CASE WHEN g % 3 = 0 THEN NOW() END, CASE WHEN g % 2 = 0 THEN 'model-' || g END
		  FROM generate_series($1::int, $2::int) g`, from, to)
	// Only the new agents' activations: a replay of every agent would repair the ones an
	// earlier call broke.
	exec(`SELECT replay_agent_activations('scale_' || lpad(g::text, 4, '0')) FROM generate_series($1::int, $2::int) g`, from, to)

	var want []agentReputationDrift
	for g := from; g <= to; g++ {
		id := fmt.Sprintf("scale_%04d", g)
		granted := 0
		if g%3 == 0 {
			granted += 50
		}
		if g%2 == 0 {
			granted += 10
		}
		switch {
		case g%10 == 1:
			exec(`UPDATE agents SET reputation = reputation + 7 WHERE id = $1`, id)
			want = append(want, agentReputationDrift{id, granted + 7, granted, nil})
		case g%10 == 2 && g%3 == 0:
			exec(`DELETE FROM agent_reputation_grants WHERE agent_id = $1 AND grant_key = 'human_claim'`, id)
			want = append(want, agentReputationDrift{id, granted - 50, granted - 50, []string{"human_claim"}})
		case g%10 == 3 && g%2 == 1:
			exec(`UPDATE agents SET model = 'set-by-sql' WHERE id = $1`, id)
			want = append(want, agentReputationDrift{id, granted, granted, []string{"model_declared"}})
		}
	}
	return want
}

// agentScansOfDrift runs the drift check over every agent and returns its rows and how many
// times it scanned the agents table. The scan counts a backend has not yet reported include
// its earlier transactions (it reports them only between transactions, at most once a
// second), so the check's own scans are the difference of two reads inside one transaction.
func agentScansOfDrift(ctx context.Context, t *testing.T, pool *Pool) (int64, []agentReputationDrift) {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	agentScans := func() int64 {
		t.Helper()
		var n int64
		require.NoError(t, tx.QueryRow(ctx, `SELECT COALESCE(seq_scan, 0) + COALESCE(idx_scan, 0)
			FROM pg_stat_xact_user_tables WHERE relname = 'agents'`).Scan(&n))
		return n
	}
	before := agentScans()
	rows, err := tx.Query(ctx, `SELECT agent_id, stored_reputation, granted_reputation, missing_activations
		FROM agent_reputation_drift()`)
	require.NoError(t, err)
	var out []agentReputationDrift
	for rows.Next() {
		var d agentReputationDrift
		require.NoError(t, rows.Scan(&d.agentID, &d.stored, &d.granted, &d.missing))
		if len(d.missing) == 0 {
			d.missing = nil
		}
		out = append(out, d)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	return agentScans() - before, out
}

func TestAgentReputationDrift_ScansTheAgentsTableAFixedNumberOfTimes(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	want := seedScaleAgents(ctx, t, pool, 1, 40)
	small, got := agentScansOfDrift(ctx, t, pool)
	require.Equal(t, want, got, "40 agents: exactly the broken ones, in agent id order")

	want = append(want, seedScaleAgents(ctx, t, pool, 41, 160)...)
	large, got := agentScansOfDrift(ctx, t, pool)
	require.Equal(t, want, got, "160 agents: exactly the broken ones, in agent id order")

	require.Equal(t, small, large, "four times the agents, the same number of agents scans")
	require.LessOrEqual(t, large, int64(4), "the agents table is read a fixed number of times, not once per agent")

	var nullKeys int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM agent_reputation_drift()
		WHERE missing_activations IS NULL`).Scan(&nullKeys))
	require.Zero(t, nullKeys, "an agent missing no activation reads '{}', as 000121 returned it")

	n, err := rebuildAgentReputation(ctx, pool, nil)
	require.NoError(t, err)
	require.Equal(t, len(want), n, "the rebuild repairs every agent the check named")
	_, got = agentScansOfDrift(ctx, t, pool)
	require.Empty(t, got)
}
