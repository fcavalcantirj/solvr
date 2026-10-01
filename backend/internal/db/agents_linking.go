package db

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// ============================================================================
// Agent-Human Linking methods (AGENT-LINKING requirement)
// ============================================================================

// LinkHuman links an agent to a human user.
// Per AGENT-LINKING requirement: "CHECK constraint: human_id can only be set once"
// The database trigger prevents_agent_reclaim enforces this at DB level.
// Returns ErrAgentAlreadyClaimed if the agent is already linked to a human.
// Only an unclaimed agent is linked (human_id IS NULL), so the NULL -> human transition
// happens once: a concurrent or retried claim, even by the same human, gets
// ErrAgentAlreadyClaimed instead of linking again.
func (r *AgentRepository) LinkHuman(ctx context.Context, agentID, humanID string) error {
	query := `
		UPDATE agents
		SET human_id = $2, human_claimed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND human_id IS NULL
	`

	result, err := r.pool.Exec(ctx, query, agentID, humanID)
	if err != nil {
		// Check for the trigger exception (agent_already_claimed)
		if strings.Contains(err.Error(), "agent_already_claimed") {
			slog.Info("agent already claimed", "op", "LinkHuman", "table", "agents", "agent_id", agentID)
			return ErrAgentAlreadyClaimed
		}
		LogQueryError(ctx, "LinkHuman", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agents WHERE id = $1)`, agentID).Scan(&exists); err != nil {
			LogQueryError(ctx, "LinkHuman.Exists", "agents", err)
			return err
		}
		if exists {
			slog.Info("agent already claimed", "op", "LinkHuman", "table", "agents", "agent_id", agentID)
			return ErrAgentAlreadyClaimed
		}
		slog.Debug("agent not found", "op", "LinkHuman", "table", "agents", "id", agentID)
		return ErrAgentNotFound
	}

	return nil
}

// Reputation grant keys (migration 000121). An agent's stored reputation is the sum of its
// agent_reputation_grants rows, moved by a trigger in the grant's own transaction. A keyed
// activation is granted at most once per agent, whatever repeats it: a second claim after the
// first owner left, a model cleared and set again, a retry, a replayed worker.
const (
	// ReputationGrantHumanClaim is the claim bonus, granted by ClaimTokenRepository.ClaimAgent.
	ReputationGrantHumanClaim = "human_claim"
	// ReputationGrantModelDeclared is the bonus for declaring a model.
	ReputationGrantModelDeclared = "model_declared"
)

// AddReputation adds reputation points to an agent as an adjustment: every call is its own
// grant, so two calls add twice. Activations use GrantReputationOnce instead.
func (r *AgentRepository) AddReputation(ctx context.Context, agentID string, amount int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_reputation_grants (agent_id, grant_key, points)
		VALUES ($1, 'adjustment:' || gen_random_uuid(), $2)`, agentID, amount)
	return reputationGrantError(ctx, "AddReputation", agentID, err)
}

// GrantReputationOnce grants points under grantKey unless the agent already holds that grant,
// and reports whether it did. Concurrent and repeated calls grant once.
func (r *AgentRepository) GrantReputationOnce(ctx context.Context, agentID, grantKey string, points int) (bool, error) {
	result, err := r.pool.Exec(ctx, `
		INSERT INTO agent_reputation_grants (agent_id, grant_key, points)
		VALUES ($1, $2, $3)
		ON CONFLICT (agent_id, grant_key) DO NOTHING`, agentID, grantKey, points)
	if err := reputationGrantError(ctx, "GrantReputationOnce", agentID, err); err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

// reputationGrantError maps a grant for an agent that does not exist to ErrAgentNotFound.
func reputationGrantError(ctx context.Context, op, agentID string, err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		slog.Debug("agent not found", "op", op, "table", "agent_reputation_grants", "id", agentID)
		return ErrAgentNotFound
	}
	LogQueryError(ctx, op, "agent_reputation_grants", err)
	return err
}

// GrantHumanBackedBadge grants the Human-Backed badge to an agent.
// Per AGENT-LINKING: granted on successful claim.
func (r *AgentRepository) GrantHumanBackedBadge(ctx context.Context, agentID string) error {
	query := `
		UPDATE agents
		SET has_human_backed_badge = true, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, agentID)
	if err != nil {
		LogQueryError(ctx, "GrantHumanBackedBadge", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("agent not found", "op", "GrantHumanBackedBadge", "table", "agents", "id", agentID)
		return ErrAgentNotFound
	}

	return nil
}
