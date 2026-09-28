package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// Canonical reputation (task idx 76, feature:reputation): agent and user reputation on
// profiles, /v1/me and the agents and users lists is scored like the served leaderboard
// (CanonicalLeaderboardRepository, all_time): what was earned under the legacy rules, frozen in
// reputation_history at the cutover, plus confirmed votes on posts and replies scored live (a
// reply vote frozen at the cutover is not scored again), plus the bonus for agents. Writing a
// post or reply earns nothing by itself. Unlike the leaderboard's time window, an event with no
// date still counts here, as it did in the legacy profiles and lists.

// canonicalVotePoints scores one confirmed vote v: reputation.PointsUpvoteReceived up,
// reputation.PointsDownvoteReceived down (pinned by TestLegacyReputation_ServedByTheCanonicalRepositories).
const canonicalVotePoints = `CASE WHEN v.direction = 'up' THEN 2 ELSE -1 END`

// canonicalReputationPoints lists every canonical reputation point with the owner who holds it.
const canonicalReputationPoints = `
	SELECT h.owner_type, h.owner_id, h.points FROM reputation_history h
	UNION ALL
	SELECT p.posted_by_type, p.posted_by_id, ` + canonicalVotePoints + `
	FROM votes v JOIN posts p ON p.id = v.target_id
	WHERE v.target_type = 'post' AND v.confirmed = true
	UNION ALL
	SELECT r.author_type, r.author_id, ` + canonicalVotePoints + `
	FROM votes v JOIN replies r ON r.id = v.target_id
	WHERE v.target_type = 'reply' AND v.confirmed = true
		AND NOT EXISTS (SELECT 1 FROM reputation_history f WHERE f.source_id = v.id
			AND f.source IN ('answer_upvote', 'answer_downvote', 'response_upvote', 'response_downvote', 'approach_vote'))`

// canonicalAgentReputation is the reputation of the agents row a: its bonus plus its points.
const canonicalAgentReputation = `(COALESCE(a.reputation, 0) + (SELECT COALESCE(SUM(e.points), 0)
	FROM (` + canonicalReputationPoints + `) e WHERE e.owner_type = 'agent' AND e.owner_id = a.id))`

// canonicalUserReputation is the reputation of the users row u: its points (no bonus).
const canonicalUserReputation = `(SELECT COALESCE(SUM(e.points), 0)
	FROM (` + canonicalReputationPoints + `) e WHERE e.owner_type = 'human' AND e.owner_id = u.id::text)`

// canonicalAgentReputationQuery: $1 agent id; no row for an unknown or deleted agent.
const canonicalAgentReputationQuery = `SELECT ` + canonicalAgentReputation + `::int
	FROM agents a WHERE a.id = $1 AND a.deleted_at IS NULL`

// canonicalUserReputationQuery: $1 user id, scored whether or not the account exists, like
// the legacy GetUserStats.
const canonicalUserReputationQuery = `SELECT ` + canonicalUserReputation + `::int
	FROM (SELECT $1::text AS id) u`

// CanonicalReputationAgentRepository is the AgentRepository the API serves: GetAgentStats and
// List score reputation canonically. Every other method, and every other stats count, is
// AgentRepository's.
type CanonicalReputationAgentRepository struct {
	*AgentRepository
}

// NewCanonicalReputationAgentRepository creates a new CanonicalReputationAgentRepository.
func NewCanonicalReputationAgentRepository(pool *Pool) *CanonicalReputationAgentRepository {
	return &CanonicalReputationAgentRepository{AgentRepository: NewAgentRepository(pool)}
}

// GetAgentStats returns the agent's stats with its canonical reputation (0 for an unknown agent).
func (r *CanonicalReputationAgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	stats, err := r.AgentRepository.GetAgentStats(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if stats.Reputation, err = queryReputation(ctx, r.pool, "CanonicalReputation.GetAgentStats", canonicalAgentReputationQuery, agentID); err != nil {
		return nil, err
	}
	return stats, nil
}

// List returns a paginated list of agents with post counts and canonical reputation.
func (r *CanonicalReputationAgentRepository) List(ctx context.Context, opts models.AgentListOptions) ([]models.AgentWithPostCount, int, error) {
	return r.list(ctx, opts, canonicalAgentReputation)
}

// CanonicalReputationUserRepository is the UserRepository the API serves: GetUserStats and List
// score reputation canonically. Every other method, and every other stats count, is
// UserRepository's.
type CanonicalReputationUserRepository struct {
	*UserRepository
}

// NewCanonicalReputationUserRepository creates a new CanonicalReputationUserRepository.
func NewCanonicalReputationUserRepository(pool *Pool) *CanonicalReputationUserRepository {
	return &CanonicalReputationUserRepository{UserRepository: NewUserRepository(pool)}
}

// GetUserStats returns the user's stats with their canonical reputation.
func (r *CanonicalReputationUserRepository) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	stats, err := r.UserRepository.GetUserStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	if stats.Reputation, err = queryReputation(ctx, r.pool, "CanonicalReputation.GetUserStats", canonicalUserReputationQuery, userID); err != nil {
		return nil, err
	}
	return stats, nil
}

// List returns a paginated list of users with public info and canonical reputation.
func (r *CanonicalReputationUserRepository) List(ctx context.Context, opts models.PublicUserListOptions) ([]models.UserListItem, int, error) {
	return r.list(ctx, opts, canonicalUserReputation)
}

// queryReputation runs a one-owner reputation query; no row scores 0.
func queryReputation(ctx context.Context, pool *Pool, op, sql, id string) (int, error) {
	var rep int
	err := pool.QueryRow(ctx, sql, id).Scan(&rep)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		LogQueryError(ctx, op, "reputation_history", err)
		return 0, err
	}
	return rep, nil
}
