package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// An agent's stored reputation (its bonus; earned points are scored at read time) is a
// projection of its grant rows (idx 77, migration 000121). Before it, the claim bonus and
// the model bonus were added straight to agents.reputation with no record of the grant:
// claiming an agent again after its first owner deleted their account added +50 again, and
// clearing and re-setting the model added +10 per cycle (measured on HEAD through the API,
// idx 77 slice 5: 100 after one re-claim, 60 after five model cycles). These tests run on
// scratch databases so a drift check or rebuild of every agent sees only rows they created.

type agentGrant struct {
	key    string
	points int
}

func readAgentReputation(ctx context.Context, t *testing.T, pool *Pool, agentID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT reputation FROM agents WHERE id = $1`, agentID).Scan(&n))
	return n
}

// readAgentGrants lists an agent's grants with every adjustment key read as "adjustment".
func readAgentGrants(ctx context.Context, t *testing.T, pool *Pool, agentID string) []agentGrant {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT CASE WHEN grant_key LIKE 'adjustment:%' THEN 'adjustment' ELSE grant_key END, points
		FROM agent_reputation_grants WHERE agent_id = $1`, agentID)
	require.NoError(t, err)
	defer rows.Close()
	var out []agentGrant
	for rows.Next() {
		var g agentGrant
		require.NoError(t, rows.Scan(&g.key, &g.points))
		out = append(out, g)
	}
	require.NoError(t, rows.Err())
	return out
}

type agentReputationDrift struct {
	agentID         string
	stored, granted int
	missing         []string
}

func readAgentReputationDrift(ctx context.Context, t *testing.T, pool *Pool, agentID *string) []agentReputationDrift {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT agent_id, stored_reputation, granted_reputation, missing_activations
		FROM agent_reputation_drift($1)`, agentID)
	require.NoError(t, err)
	defer rows.Close()
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
	return out
}

func rebuildAgentReputation(ctx context.Context, pool *Pool, agentID *string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT rebuild_agent_reputation($1)`, agentID).Scan(&n)
	return n, err
}

func replayAgentActivations(ctx context.Context, t *testing.T, pool *Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT replay_agent_activations()`).Scan(&n))
	return n
}

// bonusAgent registers an agent with no model and no owner.
func bonusAgent(ctx context.Context, t *testing.T, pool *Pool, label string) string {
	t.Helper()
	id := "bonus_" + label + "_" + uuid.NewString()[:8]
	require.NoError(t, NewAgentRepository(pool).Create(ctx, &models.Agent{ID: id, DisplayName: id}))
	return id
}

// claimWithNewToken claims agentID for humanID through a fresh claim token, as
// POST /v1/agents/claim does.
func claimWithNewToken(ctx context.Context, t *testing.T, pool *Pool, agentID, humanID string) {
	t.Helper()
	tokens := NewClaimTokenRepository(pool)
	token := &models.ClaimToken{Token: "bonus_" + uuid.NewString(), AgentID: agentID, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, tokens.Create(ctx, token))
	require.NoError(t, tokens.ClaimAgent(ctx, token.ID, agentID, humanID, 50))
}

func TestAgentReputation_EachActivationIsGrantedOncePerAgent(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agents := NewAgentRepository(pool)
	agentID := bonusAgent(ctx, t, pool, "once")
	require.Equal(t, 0, readAgentReputation(ctx, t, pool, agentID))

	granted, err := agents.GrantReputationOnce(ctx, agentID, ReputationGrantModelDeclared, 10)
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 10, readAgentReputation(ctx, t, pool, agentID))
	// The model is cleared and set again (the measured +10 per cycle): already granted.
	granted, err = agents.GrantReputationOnce(ctx, agentID, ReputationGrantModelDeclared, 10)
	require.NoError(t, err)
	require.False(t, granted, "a second model activation grants nothing")
	require.Equal(t, 10, readAgentReputation(ctx, t, pool, agentID))

	// Claimed, unclaimed when the first owner deletes their account, claimed by another human.
	claimWithNewToken(ctx, t, pool, agentID, authorHuman(ctx, t, pool, "first owner"))
	require.Equal(t, 60, readAgentReputation(ctx, t, pool, agentID), "the claim bonus")
	_, err = pool.Exec(ctx, `UPDATE agents SET human_id = NULL WHERE id = $1`, agentID)
	require.NoError(t, err)
	claimWithNewToken(ctx, t, pool, agentID, authorHuman(ctx, t, pool, "second owner"))
	require.Equal(t, 60, readAgentReputation(ctx, t, pool, agentID), "a second claim of the same agent grants nothing")

	// Adjustments are not activations: each one is its own grant.
	require.NoError(t, agents.AddReputation(ctx, agentID, 7))
	require.NoError(t, agents.AddReputation(ctx, agentID, 7))
	require.Equal(t, 74, readAgentReputation(ctx, t, pool, agentID))

	require.ElementsMatch(t, []agentGrant{{"model_declared", 10}, {"human_claim", 50}, {"adjustment", 7}, {"adjustment", 7}},
		readAgentGrants(ctx, t, pool, agentID))
	require.Empty(t, readAgentReputationDrift(ctx, t, pool, nil))
	require.Equal(t, 0, replayAgentActivations(ctx, t, pool), "replaying the activations grants nothing again")
	require.Equal(t, 74, readAgentReputation(ctx, t, pool, agentID))

	_, err = agents.GrantReputationOnce(ctx, "no_such_agent", ReputationGrantModelDeclared, 10)
	require.ErrorIs(t, err, ErrAgentNotFound)
	require.ErrorIs(t, agents.AddReputation(ctx, "no_such_agent", 5), ErrAgentNotFound)
}

func TestAgentReputation_EveryGrantRowMovesTheReputationInItsOwnTransaction(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agentID, otherID := bonusAgent(ctx, t, pool, "rows"), bonusAgent(ctx, t, pool, "other")
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}

	// Whoever writes the grant rows keeps the reputation: a fixture, an operator, a cleanup.
	exec(`INSERT INTO agent_reputation_grants (agent_id, grant_key, points) VALUES ($1, 'operator:a', 30), ($1, 'operator:b', 5)`, agentID)
	require.Equal(t, 35, readAgentReputation(ctx, t, pool, agentID))
	exec(`UPDATE agent_reputation_grants SET points = 20 WHERE agent_id = $1 AND grant_key = 'operator:a'`, agentID)
	require.Equal(t, 25, readAgentReputation(ctx, t, pool, agentID), "a changed grant moves by the difference")
	exec(`UPDATE agent_reputation_grants SET agent_id = $1 WHERE agent_id = $2 AND grant_key = 'operator:b'`, otherID, agentID)
	require.Equal(t, 20, readAgentReputation(ctx, t, pool, agentID), "a grant moved away is removed here")
	require.Equal(t, 5, readAgentReputation(ctx, t, pool, otherID), "and added where it moved")
	exec(`UPDATE agent_reputation_grants SET granted_at = granted_at - interval '1 day' WHERE agent_id = $1`, agentID)
	require.Equal(t, 20, readAgentReputation(ctx, t, pool, agentID), "an update that keeps agent and points moves nothing")
	exec(`DELETE FROM agent_reputation_grants WHERE agent_id = $1`, agentID)
	require.Equal(t, 0, readAgentReputation(ctx, t, pool, agentID), "a deleted grant is removed")

	// A grant whose transaction rolls back never counted.
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO agent_reputation_grants (agent_id, grant_key, points) VALUES ($1, 'human_claim', 50)`, agentID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, 0, readAgentReputation(ctx, t, pool, agentID), "a rolled-back grant is not counted")

	// A claim whose token turns out to be spent rolls its bonus back with it.
	tokens := NewClaimTokenRepository(pool)
	token := &models.ClaimToken{Token: "bonus_" + uuid.NewString(), AgentID: agentID, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, tokens.Create(ctx, token))
	exec(`UPDATE claim_tokens SET used_at = NOW() WHERE id = $1`, token.ID)
	err = tokens.ClaimAgent(ctx, token.ID, agentID, authorHuman(ctx, t, pool, "spent token"), 50)
	require.ErrorIs(t, err, ErrClaimTokenNotFound)
	require.Equal(t, 0, readAgentReputation(ctx, t, pool, agentID), "the failed claim granted nothing")
	require.Empty(t, readAgentGrants(ctx, t, pool, agentID))
	require.Empty(t, readAgentReputationDrift(ctx, t, pool, nil))
}

func TestAgentReputation_ConcurrentActivationsGrantOnce(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agents := NewAgentRepository(pool)
	agentID := bonusAgent(ctx, t, pool, "concurrent")

	var wg sync.WaitGroup
	results := make(chan bool, 40)
	errs := make(chan error, 60)
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			granted, err := agents.GrantReputationOnce(ctx, agentID, ReputationGrantModelDeclared, 10)
			results <- granted
			errs <- err
		}()
	}
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- agents.AddReputation(ctx, agentID, 1)
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	granted := 0
	for g := range results {
		if g {
			granted++
		}
	}
	require.Equal(t, 1, granted, "exactly one of 40 concurrent model activations is granted")
	require.Equal(t, 30, readAgentReputation(ctx, t, pool, agentID), "10 once plus 20 adjustments of 1")
	require.Empty(t, readAgentReputationDrift(ctx, t, pool, nil))
}

func TestAgentReputation_DriftNamesBrokenAgentsAndTheRebuildRepairsThem(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agents := NewAgentRepository(pool)
	healthy, oldClaim, inflated, modelBySQL := bonusAgent(ctx, t, pool, "healthy"), bonusAgent(ctx, t, pool, "oldclaim"),
		bonusAgent(ctx, t, pool, "inflated"), bonusAgent(ctx, t, pool, "sqlmodel")
	_, err := agents.GrantReputationOnce(ctx, healthy, ReputationGrantModelDeclared, 10)
	require.NoError(t, err)
	_, err = agents.GrantReputationOnce(ctx, inflated, ReputationGrantModelDeclared, 10)
	require.NoError(t, err)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	// Reputation the grants do not support: a claim written the pre-000121 way (bonus added to
	// the column, no grant), a value written by hand, and a model set with no activation.
	exec(`UPDATE agents SET human_id = $2, human_claimed_at = NOW(), reputation = reputation + 50 WHERE id = $1`,
		oldClaim, authorHuman(ctx, t, pool, "old claim"))
	exec(`UPDATE agents SET reputation = 999 WHERE id = $1`, inflated)
	exec(`UPDATE agents SET model = 'gpt-x' WHERE id = $1`, modelBySQL)
	xmin := func(id string) string {
		var x string
		require.NoError(t, pool.QueryRow(ctx, `SELECT xmin::text FROM agents WHERE id = $1`, id).Scan(&x))
		return x
	}
	healthyXmin := xmin(healthy)

	require.ElementsMatch(t, []agentReputationDrift{
		{oldClaim, 50, 0, []string{"human_claim"}},
		{inflated, 999, 10, nil},
		{modelBySQL, 0, 0, []string{"model_declared"}},
	}, readAgentReputationDrift(ctx, t, pool, nil), "exactly the three broken agents")
	require.Equal(t, []agentReputationDrift{{inflated, 999, 10, nil}}, readAgentReputationDrift(ctx, t, pool, &inflated), "filtered by agent")
	require.Empty(t, readAgentReputationDrift(ctx, t, pool, &healthy))

	n, err := rebuildAgentReputation(ctx, pool, &oldClaim)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 50, readAgentReputation(ctx, t, pool, oldClaim), "the claim bonus once, not twice")
	require.Equal(t, []agentGrant{{"human_claim", 50}}, readAgentGrants(ctx, t, pool, oldClaim))
	require.Len(t, readAgentReputationDrift(ctx, t, pool, nil), 2, "a one-agent rebuild leaves the other broken agents alone")

	n, err = rebuildAgentReputation(ctx, pool, nil)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, 10, readAgentReputation(ctx, t, pool, inflated))
	require.Equal(t, 10, readAgentReputation(ctx, t, pool, modelBySQL), "the missing model activation is granted")
	require.Empty(t, readAgentReputationDrift(ctx, t, pool, nil))
	require.Equal(t, healthyXmin, xmin(healthy), "a consistent agent is not rewritten")

	n, err = rebuildAgentReputation(ctx, pool, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n, "replaying the rebuild changes nothing")
	require.Equal(t, 0, replayAgentActivations(ctx, t, pool), "replaying the activations grants nothing again")
	require.Equal(t, []int{10, 50, 10, 10}, []int{readAgentReputation(ctx, t, pool, healthy), readAgentReputation(ctx, t, pool, oldClaim),
		readAgentReputation(ctx, t, pool, inflated), readAgentReputation(ctx, t, pool, modelBySQL)})
}

// A rebuild and a grant that meet must not lose the grant, whichever holds the agent first.
func TestAgentReputation_ARebuildAndAGrantNeverLoseEachOther(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agents := NewAgentRepository(pool)
	agentID := bonusAgent(ctx, t, pool, "inflight")
	_, err := agents.GrantReputationOnce(ctx, agentID, ReputationGrantModelDeclared, 10)
	require.NoError(t, err)
	breakReputation := func() {
		_, err := pool.Exec(ctx, `UPDATE agents SET reputation = 500 WHERE id = $1`, agentID)
		require.NoError(t, err)
	}

	// 1. The grant is in flight: the rebuild waits for it, then counts it.
	breakReputation()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background()) //nolint:errcheck
	_, err = tx.Exec(ctx, `INSERT INTO agent_reputation_grants (agent_id, grant_key, points) VALUES ($1, 'adjustment:in_flight', 3)`, agentID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := rebuildAgentReputation(ctx, pool, &agentID); done <- err }()
	waitForLockWait(ctx, t, pool, "%rebuild_agent_reputation%", done)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	require.Equal(t, 13, readAgentReputation(ctx, t, pool, agentID), "the in-flight grant is counted")

	// 2. The rebuild is in flight: the grant waits for it, then lands on the rebuilt value.
	breakReputation()
	tx2, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx2.Rollback(context.Background()) //nolint:errcheck
	var repaired int
	require.NoError(t, tx2.QueryRow(ctx, `SELECT rebuild_agent_reputation($1)`, agentID).Scan(&repaired))
	require.Equal(t, 1, repaired)
	added := make(chan error, 1)
	go func() { added <- agents.AddReputation(ctx, agentID, 4) }()
	waitForLockWait(ctx, t, pool, "%INSERT INTO agent_reputation_grants%", added)
	require.NoError(t, tx2.Commit(ctx))
	require.NoError(t, <-added)
	require.Equal(t, 17, readAgentReputation(ctx, t, pool, agentID), "the late grant lands on the rebuilt value")
	require.Empty(t, readAgentReputationDrift(ctx, t, pool, nil))
}

// Migration 000121 records the activations each existing agent row already shows (a claim, a
// model) without moving any stored reputation; what the rows do not explain is left to the
// drift check. Measured on the restored production dump: every agent's reputation is exactly
// 50 per claim plus 10 per model, so its drift is empty after the migration.
func TestAgentReputation_TheMigrationRecordsExistingActivationsWithoutMovingAScore(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	migration := func(direction string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join(backendRoot(t), "migrations", "000121_agent_reputation_grants."+direction+".sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, "000121 %s", direction)
	}
	migration("down")
	owner := authorHuman(ctx, t, pool, "legacy owner")
	legacy := func(label string, reputation int, claimed bool, model string) string {
		t.Helper()
		id := fmt.Sprintf("legacy_%s_%s", label, uuid.NewString()[:8])
		var humanID any
		var claimedAt any
		if claimed {
			humanID, claimedAt = owner, time.Now()
		}
		_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, reputation, human_id, human_claimed_at, model)
			VALUES ($1, $1, $2, $3, $4, NULLIF($5, ''))`, id, reputation, humanID, claimedAt, model)
		require.NoError(t, err)
		return id
	}
	plain, modelOnly, claimOnly, both := legacy("plain", 0, false, ""), legacy("model", 10, false, "m"),
		legacy("claim", 50, true, ""), legacy("both", 60, true, "m")
	reclaimed := legacy("reclaimed", 110, true, "m") // the measured double claim bonus
	migration("up")

	for id, want := range map[string]int{plain: 0, modelOnly: 10, claimOnly: 50, both: 60, reclaimed: 110} {
		require.Equal(t, want, readAgentReputation(ctx, t, pool, id), "the migration moves no stored reputation (%s)", id)
	}
	require.Empty(t, readAgentGrants(ctx, t, pool, plain))
	require.Equal(t, []agentGrant{{"model_declared", 10}}, readAgentGrants(ctx, t, pool, modelOnly))
	require.Equal(t, []agentGrant{{"human_claim", 50}}, readAgentGrants(ctx, t, pool, claimOnly))
	require.ElementsMatch(t, []agentGrant{{"human_claim", 50}, {"model_declared", 10}}, readAgentGrants(ctx, t, pool, both))
	require.Equal(t, []agentReputationDrift{{reclaimed, 110, 60, nil}}, readAgentReputationDrift(ctx, t, pool, nil),
		"only the double-counted agent drifts")
	n, err := rebuildAgentReputation(ctx, pool, nil)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 60, readAgentReputation(ctx, t, pool, reclaimed))
	require.Equal(t, 0, replayAgentActivations(ctx, t, pool))
}
