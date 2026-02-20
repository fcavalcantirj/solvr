package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// RecommendationRepository provides personalized post recommendations for agents.
type RecommendationRepository struct {
	pool *Pool
}

// NewRecommendationRepository creates a new RecommendationRepository.
func NewRecommendationRepository(pool *Pool) *RecommendationRepository {
	return &RecommendationRepository{pool: pool}
}

// GetYouMightLike returns personalized post recommendations for an agent.
// Uses a two-step strategy:
//  1. Tag affinity: posts matching tags from the agent's confirmed upvotes
//  2. Adjacent tags fallback: tags co-occurring with agent specialties (excluding specialties themselves)
//
// Excludes posts the agent has interacted with (voted, approached, answered) and the agent's own posts.
// Returns empty slice (not error) if agent has no history and no specialties.
func (r *RecommendationRepository) GetYouMightLike(ctx context.Context, agentID string, specialties []string, limit int) ([]models.RecommendedPost, error) {
	// Step A: Check if agent has confirmed upvotes
	var voteCount int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM votes WHERE voter_id = $1 AND direction = 'up' AND confirmed = true`,
		agentID,
	).Scan(&voteCount)
	if err != nil {
		LogQueryError(ctx, "GetYouMightLike.voteCount", "votes", err)
		return nil, err
	}

	var results []models.RecommendedPost

	// Step B: Tag affinity (if agent has upvotes)
	if voteCount > 0 {
		affinityResults, err := r.getTagAffinityResults(ctx, agentID, limit)
		if err != nil {
			return nil, err
		}
		results = append(results, affinityResults...)
	}

	// Step C: Adjacent tags fallback (if not enough results and agent has specialties)
	if len(results) < limit && len(specialties) > 0 {
		remaining := limit - len(results)
		// Collect IDs already in results for deduplication
		seen := make(map[string]bool, len(results))
		for _, r := range results {
			seen[r.ID] = true
		}

		adjacentResults, err := r.getAdjacentTagResults(ctx, agentID, specialties, remaining+len(results))
		if err != nil {
			return nil, err
		}

		for _, ar := range adjacentResults {
			if !seen[ar.ID] {
				results = append(results, ar)
				seen[ar.ID] = true
				if len(results) >= limit {
					break
				}
			}
		}
	}

	// Step D: Return empty slice if no results
	if results == nil {
		results = []models.RecommendedPost{}
	}
	return results, nil
}

// getTagAffinityResults finds posts matching tags from the agent's confirmed upvotes.
func (r *RecommendationRepository) getTagAffinityResults(ctx context.Context, agentID string, limit int) ([]models.RecommendedPost, error) {
	query := `
		WITH upvoted_tags AS (
			SELECT DISTINCT unnest(p.tags) AS tag
			FROM votes v
			JOIN posts p ON v.target_type = 'post' AND v.target_id = p.id AND p.deleted_at IS NULL
			WHERE v.voter_id = $1 AND v.direction = 'up' AND v.confirmed = true
		),
		interacted AS (
			SELECT target_id AS post_id FROM votes
			WHERE voter_id = $1 AND voter_type = 'agent'
			UNION
			SELECT problem_id FROM approaches
			WHERE author_id = $1 AND author_type = 'agent' AND deleted_at IS NULL
			UNION
			SELECT question_id FROM answers
			WHERE author_id = $1 AND author_type = 'agent' AND deleted_at IS NULL
		)
		SELECT p.id, p.title, p.type,
			(p.upvotes - p.downvotes) AS vote_score,
			p.tags,
			'voted_tags' AS match_reason,
			GREATEST(FLOOR(EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600)::int, 0) AS age_hours
		FROM posts p
		WHERE p.tags && (SELECT COALESCE(array_agg(tag), ARRAY[]::text[]) FROM upvoted_tags)
		  AND p.id NOT IN (SELECT post_id FROM interacted)
		  AND p.posted_by_id != $1
		  AND p.deleted_at IS NULL
		  AND p.status NOT IN ('draft', 'closed')
		ORDER BY (p.upvotes - p.downvotes) DESC, p.created_at DESC
		LIMIT $2`

	return r.scanRecommendations(ctx, "getTagAffinityResults", query, agentID, limit)
}

// getAdjacentTagResults finds posts matching tags that co-occur with agent specialties.
func (r *RecommendationRepository) getAdjacentTagResults(ctx context.Context, agentID string, specialties []string, limit int) ([]models.RecommendedPost, error) {
	query := `
		WITH adjacent_tags AS (
			SELECT DISTINCT unnest(p.tags) AS tag
			FROM posts p
			WHERE p.tags && $2::text[]
			  AND p.deleted_at IS NULL
			EXCEPT
			SELECT unnest($2::text[])
		),
		interacted AS (
			SELECT target_id AS post_id FROM votes
			WHERE voter_id = $1 AND voter_type = 'agent'
			UNION
			SELECT problem_id FROM approaches
			WHERE author_id = $1 AND author_type = 'agent' AND deleted_at IS NULL
			UNION
			SELECT question_id FROM answers
			WHERE author_id = $1 AND author_type = 'agent' AND deleted_at IS NULL
		)
		SELECT p.id, p.title, p.type,
			(p.upvotes - p.downvotes) AS vote_score,
			p.tags,
			'adjacent_tags' AS match_reason,
			GREATEST(FLOOR(EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600)::int, 0) AS age_hours
		FROM posts p
		WHERE p.tags && (SELECT COALESCE(array_agg(tag), ARRAY[]::text[]) FROM adjacent_tags)
		  AND p.id NOT IN (SELECT post_id FROM interacted)
		  AND p.posted_by_id != $1
		  AND p.deleted_at IS NULL
		  AND p.status NOT IN ('draft', 'closed')
		ORDER BY (p.upvotes - p.downvotes) DESC, p.created_at DESC
		LIMIT $3`

	return r.scanRecommendations(ctx, "getAdjacentTagResults", query, agentID, specialties, limit)
}

// scanRecommendations executes a query and scans the results into RecommendedPost slice.
func (r *RecommendationRepository) scanRecommendations(ctx context.Context, queryName, query string, args ...interface{}) ([]models.RecommendedPost, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "GetYouMightLike."+queryName, "posts", err)
		return nil, err
	}
	defer rows.Close()

	var results []models.RecommendedPost
	for rows.Next() {
		var rec models.RecommendedPost
		var tags []string
		err := rows.Scan(
			&rec.ID, &rec.Title, &rec.Type,
			&rec.VoteScore, &tags,
			&rec.MatchReason, &rec.AgeHours,
		)
		if err != nil {
			LogQueryError(ctx, "GetYouMightLike."+queryName+".Scan", "posts", err)
			return nil, err
		}
		if tags == nil {
			tags = []string{}
		}
		rec.Tags = tags
		results = append(results, rec)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetYouMightLike."+queryName+".Rows", "posts", err)
		return nil, err
	}

	return results, nil
}
