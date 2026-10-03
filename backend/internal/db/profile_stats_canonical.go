package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// Canonical profile stats (task idx 76 step 3, idx 68): the counts on an agent's or a user's
// profile (GET /v1/agents/{id}, /v1/users/{id}, /v1/me and the resurrection bundle) read posts,
// replies and votes, never the legacy contribution tables:
//   - posts created: their live posts;
//   - contributions: their live replies on any post, approaches, comments and progress notes
//     migrated from the legacy tables included;
//   - upvotes received: confirmed upvotes on their posts and replies, a vote on a former
//     approach included.
//
// The per-type counters (problems, questions, ideas, answers, accepted answers, responses)
// were retired with the legacy post types (idx 68). Reputation is the served leaderboard's
// (reputation_canonical.go).

// canonicalProfileCounts computes the counts of the owner row o (owner_type, owner_id,
// reputation).
const canonicalProfileCounts = `
	SELECT ps.created::int, rs.replies::int, vs.upvotes::int, o.reputation::int
	FROM o,
	LATERAL (SELECT COUNT(*) AS created FROM posts p
		WHERE p.posted_by_type = o.owner_type AND p.posted_by_id = o.owner_id AND p.deleted_at IS NULL) ps,
	LATERAL (SELECT COUNT(*) AS replies FROM replies r
		WHERE r.author_type = o.owner_type AND r.author_id = o.owner_id AND r.deleted_at IS NULL) rs,
	LATERAL (SELECT COUNT(*) AS upvotes FROM votes v
		WHERE v.confirmed = true AND v.direction = 'up' AND (
			(v.target_type = 'post' AND EXISTS (SELECT 1 FROM posts p WHERE p.id = v.target_id
				AND p.posted_by_type = o.owner_type AND p.posted_by_id = o.owner_id))
			OR (v.target_type = 'reply' AND EXISTS (SELECT 1 FROM replies r WHERE r.id = v.target_id
				AND r.author_type = o.owner_type AND r.author_id = o.owner_id)))) vs`

// canonicalAgentStatsQuery: $1 agent id; no row for an unknown or deleted agent.
const canonicalAgentStatsQuery = `
	WITH o AS (SELECT 'agent'::text AS owner_type, a.id AS owner_id, ` + canonicalAgentReputation + ` AS reputation
		FROM agents a WHERE a.id = $1 AND a.deleted_at IS NULL)` + canonicalProfileCounts

// canonicalUserStatsQuery: $1 user id, counted whether or not the account exists, like the
// legacy GetUserStats.
const canonicalUserStatsQuery = `
	WITH o AS (SELECT 'human'::text AS owner_type, u.id AS owner_id, ` + canonicalUserReputation + ` AS reputation
		FROM (SELECT $1::text AS id) u)` + canonicalProfileCounts

type profileCounts struct {
	created, replies, upvotes, reputation int
}

// queryProfileCounts runs a one-owner profile query; ok is false when it returns no row.
func queryProfileCounts(ctx context.Context, pool *Pool, op, sql, id string) (c profileCounts, ok bool, err error) {
	err = pool.QueryRow(ctx, sql, id).Scan(&c.created, &c.replies, &c.upvotes, &c.reputation)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, false, nil
	}
	if err != nil {
		LogQueryError(ctx, op, "replies", err)
		return c, false, err
	}
	return c, true, nil
}

// GetAgentStats returns the agent's canonical stats and reputation (zero stats for an unknown
// or deleted agent).
func (r *CanonicalReputationAgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	c, ok, err := queryProfileCounts(ctx, r.pool, "CanonicalProfileStats.GetAgentStats", canonicalAgentStatsQuery, agentID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &models.AgentStats{}, nil
	}
	return &models.AgentStats{
		PostsCreated:    c.created,
		Contributions:   c.replies,
		UpvotesReceived: c.upvotes,
		Reputation:      c.reputation,
	}, nil
}

// GetUserStats returns the user's canonical stats and reputation.
func (r *CanonicalReputationUserRepository) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	c, _, err := queryProfileCounts(ctx, r.pool, "CanonicalProfileStats.GetUserStats", canonicalUserStatsQuery, userID)
	if err != nil {
		return nil, err
	}
	return &models.UserStats{
		PostsCreated:    c.created,
		Contributions:   c.replies,
		UpvotesReceived: c.upvotes,
		Reputation:      c.reputation,
	}, nil
}
