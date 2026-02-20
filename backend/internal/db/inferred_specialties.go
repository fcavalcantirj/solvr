// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
	"log/slog"
)

// InferredSpecialtiesRepository computes specialties from activity for agents
// and users who have empty explicit specialties.
// Per PRD-v5: uses weighted UNION ALL across posts, approaches, answers, and upvotes.
type InferredSpecialtiesRepository struct {
	pool *Pool
}

// NewInferredSpecialtiesRepository creates a new InferredSpecialtiesRepository.
func NewInferredSpecialtiesRepository(pool *Pool) *InferredSpecialtiesRepository {
	return &InferredSpecialtiesRepository{pool: pool}
}

// InferSpecialtiesForAgent computes the top 5 tags from an agent's activity.
// Weights: posts authored (2), approaches on problems (2, inherits parent post tags),
// answers on questions (1), confirmed upvotes on posts (1).
func (r *InferredSpecialtiesRepository) InferSpecialtiesForAgent(ctx context.Context, agentID string) ([]string, error) {
	query := `
		WITH weighted_tags AS (
			-- Posts authored by agent (weight 2)
			SELECT unnest(p.tags) AS tag, 2 AS weight
			FROM posts p
			WHERE p.posted_by_id = $1
			  AND p.posted_by_type = 'agent'
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL

			UNION ALL

			-- Approaches by agent: inherit tags from parent problem (weight 2)
			SELECT unnest(p.tags) AS tag, 2 AS weight
			FROM approaches a
			JOIN posts p ON p.id = a.problem_id
			WHERE a.author_id = $1
			  AND a.author_type = 'agent'
			  AND a.deleted_at IS NULL
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL

			UNION ALL

			-- Answers by agent: inherit tags from parent question (weight 1)
			SELECT unnest(p.tags) AS tag, 1 AS weight
			FROM answers ans
			JOIN posts p ON p.id = ans.question_id
			WHERE ans.author_id = $1
			  AND ans.author_type = 'agent'
			  AND ans.deleted_at IS NULL
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL

			UNION ALL

			-- Confirmed upvotes by agent on posts (weight 1)
			SELECT unnest(p.tags) AS tag, 1 AS weight
			FROM votes v
			JOIN posts p ON p.id = v.target_id
			WHERE v.voter_id = $1
			  AND v.voter_type = 'agent'
			  AND v.direction = 'up'
			  AND v.confirmed = true
			  AND v.target_type = 'post'
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL
		)
		SELECT tag
		FROM weighted_tags
		GROUP BY tag
		ORDER BY SUM(weight) DESC, tag ASC
		LIMIT 5
	`

	rows, err := r.pool.Query(ctx, query, agentID)
	if err != nil {
		slog.Error("InferSpecialtiesForAgent query failed", "agent_id", agentID, "error", err)
		return nil, fmt.Errorf("infer specialties for agent: %w", err)
	}
	defer rows.Close()

	var specialties []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan inferred specialty: %w", err)
		}
		specialties = append(specialties, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inferred specialties: %w", err)
	}

	if specialties == nil {
		specialties = []string{}
	}

	return specialties, nil
}

// InferSpecialtiesForUser computes the top 5 tags from a user's activity.
// Same pattern as agent but using human author type.
// Weights: posts authored (2), answers on questions (1), confirmed upvotes on posts (1).
func (r *InferredSpecialtiesRepository) InferSpecialtiesForUser(ctx context.Context, userID string) ([]string, error) {
	query := `
		WITH weighted_tags AS (
			-- Posts authored by user (weight 2)
			SELECT unnest(p.tags) AS tag, 2 AS weight
			FROM posts p
			WHERE p.posted_by_id = $1
			  AND p.posted_by_type = 'human'
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL

			UNION ALL

			-- Answers by user: inherit tags from parent question (weight 1)
			SELECT unnest(p.tags) AS tag, 1 AS weight
			FROM answers ans
			JOIN posts p ON p.id = ans.question_id
			WHERE ans.author_id = $1
			  AND ans.author_type = 'human'
			  AND ans.deleted_at IS NULL
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL

			UNION ALL

			-- Confirmed upvotes by user on posts (weight 1)
			SELECT unnest(p.tags) AS tag, 1 AS weight
			FROM votes v
			JOIN posts p ON p.id = v.target_id
			WHERE v.voter_id = $1
			  AND v.voter_type = 'human'
			  AND v.direction = 'up'
			  AND v.confirmed = true
			  AND v.target_type = 'post'
			  AND p.deleted_at IS NULL
			  AND p.tags IS NOT NULL
		)
		SELECT tag
		FROM weighted_tags
		GROUP BY tag
		ORDER BY SUM(weight) DESC, tag ASC
		LIMIT 5
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		slog.Error("InferSpecialtiesForUser query failed", "user_id", userID, "error", err)
		return nil, fmt.Errorf("infer specialties for user: %w", err)
	}
	defer rows.Close()

	var specialties []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan inferred specialty: %w", err)
		}
		specialties = append(specialties, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inferred specialties: %w", err)
	}

	if specialties == nil {
		specialties = []string{}
	}

	return specialties, nil
}
