package db

import (
	"context"
	"log/slog"
	"strings"
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

// AddReputation adds reputation points to an agent.
// Per AGENT-LINKING: +50 reputation on human claim.
func (r *AgentRepository) AddReputation(ctx context.Context, agentID string, amount int) error {
	query := `
		UPDATE agents
		SET reputation = reputation + $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, agentID, amount)
	if err != nil {
		LogQueryError(ctx, "AddReputation", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("agent not found", "op", "AddReputation", "table", "agents", "id", agentID)
		return ErrAgentNotFound
	}

	return nil
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
