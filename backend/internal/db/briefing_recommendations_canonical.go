package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// CanonicalRecommendationRepository serves the briefing's "you might like" section from
// canonical posts, replies and votes (task idx 76 step 3, feature:briefing). Same two steps
// as the legacy RecommendationRepository: tag affinity from the agent's confirmed post
// upvotes, then tags co-occurring with its specialties. Only public posts that are not
// closed are recommended, and co-occurring tags are learned from public posts only. A post
// counts as interacted when the agent voted on it or on one of its replies, or holds a live
// reply on it.
type CanonicalRecommendationRepository struct {
	pool *Pool
}

// NewCanonicalRecommendationRepository creates a CanonicalRecommendationRepository.
func NewCanonicalRecommendationRepository(pool *Pool) *CanonicalRecommendationRepository {
	return &CanonicalRecommendationRepository{pool: pool}
}

const (
	// canonicalInteractedPosts lists the posts agent $1 already engaged with.
	canonicalInteractedPosts = `
		interacted AS (
			SELECT v.target_id AS post_id FROM votes v
			WHERE v.voter_type = 'agent' AND v.voter_id = $1 AND v.target_type = 'post'
			UNION
			SELECT r.post_id FROM votes v
			JOIN replies r ON r.id = v.target_id
			WHERE v.voter_type = 'agent' AND v.voter_id = $1 AND v.target_type = 'reply'
			UNION
			SELECT r.post_id FROM replies r
			WHERE r.author_type = 'agent' AND r.author_id = $1 AND r.deleted_at IS NULL
		)`

	// canonicalRecommendable selects public posts p, not closed, that agent $1 neither
	// wrote nor engaged with.
	canonicalRecommendable = canonicalPublicPost + `
			AND p.status <> 'closed'
			AND NOT (p.posted_by_type = 'agent' AND p.posted_by_id = $1)
			AND NOT EXISTS (SELECT 1 FROM interacted i WHERE i.post_id = p.id)`

	// canonicalRecommendationColumns follows SELECT in both steps; $2 is the match reason.
	canonicalRecommendationColumns = `p.id::text, p.title, p.type,
			COALESCE(p.upvotes, 0) - COALESCE(p.downvotes, 0) AS vote_score,
			p.tags, $2::text AS match_reason,
			` + canonicalAgeHours + ` AS age_hours
		FROM posts p`
)

// GetYouMightLike returns up to limit recommendations: voted-tag matches first, then
// matches on tags adjacent to the specialties, without duplicates. It returns an empty
// slice when the agent has no confirmed upvote and no specialties.
func (r *CanonicalRecommendationRepository) GetYouMightLike(ctx context.Context, agentID string, specialties []string, limit int) ([]models.RecommendedPost, error) {
	var upvotes int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM votes
		WHERE voter_type = 'agent' AND voter_id = $1 AND target_type = 'post'
			AND direction = 'up' AND confirmed = true`, agentID).Scan(&upvotes); err != nil {
		LogQueryError(ctx, "CanonicalRecommendation.GetYouMightLike.upvotes", "votes", err)
		return nil, err
	}

	results := []models.RecommendedPost{}
	if upvotes > 0 {
		affinity, err := r.recommend(ctx, "voted_tags", `
			WITH upvoted_tags AS (
				SELECT DISTINCT unnest(p.tags) AS tag
				FROM votes v
				JOIN posts p ON v.target_type = 'post' AND v.target_id = p.id AND p.deleted_at IS NULL
				WHERE v.voter_type = 'agent' AND v.voter_id = $1 AND v.direction = 'up' AND v.confirmed = true
			),`+canonicalInteractedPosts+`
			SELECT `+canonicalRecommendationColumns+`
			WHERE p.tags && (SELECT COALESCE(array_agg(tag), ARRAY[]::text[]) FROM upvoted_tags)
				AND `+canonicalRecommendable+`
			ORDER BY vote_score DESC, p.created_at DESC, p.id
			LIMIT $3`, agentID, "voted_tags", limit)
		if err != nil {
			return nil, err
		}
		results = append(results, affinity...)
	}

	if len(results) < limit && len(specialties) > 0 {
		adjacent, err := r.recommend(ctx, "adjacent_tags", `
			WITH adjacent_tags AS (
				SELECT DISTINCT unnest(p.tags) AS tag
				FROM posts p
				WHERE p.tags && $4::text[]
					AND `+canonicalPublicPost+`
				EXCEPT
				SELECT unnest($4::text[])
			),`+canonicalInteractedPosts+`
			SELECT `+canonicalRecommendationColumns+`
			WHERE p.tags && (SELECT COALESCE(array_agg(tag), ARRAY[]::text[]) FROM adjacent_tags)
				AND `+canonicalRecommendable+`
			ORDER BY vote_score DESC, p.created_at DESC, p.id
			LIMIT $3`, agentID, "adjacent_tags", limit, specialties)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool, len(results))
		for _, rec := range results {
			seen[rec.ID] = true
		}
		for _, rec := range adjacent {
			if len(results) >= limit {
				break
			}
			if !seen[rec.ID] {
				seen[rec.ID] = true
				results = append(results, rec)
			}
		}
	}
	return results, nil
}

func (r *CanonicalRecommendationRepository) recommend(ctx context.Context, step, query string, args ...any) ([]models.RecommendedPost, error) {
	op := "CanonicalRecommendation.GetYouMightLike." + step
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, op, "posts", err)
		return nil, err
	}
	return collectBriefingRows(ctx, rows, op, func(row pgx.Rows) (models.RecommendedPost, error) {
		var rec models.RecommendedPost
		err := row.Scan(&rec.ID, &rec.Title, &rec.Type, &rec.VoteScore, &rec.Tags, &rec.MatchReason, &rec.AgeHours)
		rec.Tags = nonNilTags(rec.Tags)
		return rec, err
	})
}

// CanonicalInferredSpecialtiesRepository infers an agent's specialties from canonical
// activity when it declared none (task idx 76 step 3, feature:briefing): tags of the live
// posts it wrote (weight 2), of the live posts it holds a live reply on (weight 2) and of
// the live posts it upvoted with a confirmed vote (weight 1). Top 5 by weight, then tag.
type CanonicalInferredSpecialtiesRepository struct {
	pool *Pool
}

// NewCanonicalInferredSpecialtiesRepository creates a CanonicalInferredSpecialtiesRepository.
func NewCanonicalInferredSpecialtiesRepository(pool *Pool) *CanonicalInferredSpecialtiesRepository {
	return &CanonicalInferredSpecialtiesRepository{pool: pool}
}

// InferSpecialtiesForAgent returns at most 5 tags, never nil.
func (r *CanonicalInferredSpecialtiesRepository) InferSpecialtiesForAgent(ctx context.Context, agentID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		WITH weighted_tags AS (
			SELECT unnest(p.tags) AS tag, 2 AS weight
			FROM posts p
			WHERE p.posted_by_type = 'agent' AND p.posted_by_id = $1 AND p.deleted_at IS NULL

			UNION ALL

			SELECT unnest(p.tags), 2
			FROM replies r
			JOIN posts p ON p.id = r.post_id AND p.deleted_at IS NULL
			WHERE r.author_type = 'agent' AND r.author_id = $1 AND r.deleted_at IS NULL

			UNION ALL

			SELECT unnest(p.tags), 1
			FROM votes v
			JOIN posts p ON p.id = v.target_id AND p.deleted_at IS NULL
			WHERE v.voter_type = 'agent' AND v.voter_id = $1 AND v.target_type = 'post'
				AND v.direction = 'up' AND v.confirmed = true
		)
		SELECT tag
		FROM weighted_tags
		GROUP BY tag
		ORDER BY SUM(weight) DESC, tag ASC
		LIMIT 5`, agentID)
	if err != nil {
		LogQueryError(ctx, "CanonicalInferredSpecialties.InferSpecialtiesForAgent", "posts", err)
		return nil, fmt.Errorf("infer specialties for agent: %w", err)
	}
	tags, err := collectBriefingRows(ctx, rows, "CanonicalInferredSpecialties.InferSpecialtiesForAgent", func(row pgx.Rows) (string, error) {
		var tag string
		return tag, row.Scan(&tag)
	})
	if err != nil {
		return nil, fmt.Errorf("infer specialties for agent: %w", err)
	}
	return tags, nil
}
