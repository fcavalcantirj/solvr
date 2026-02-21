package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ResurrectionRepository provides knowledge queries for the agent resurrection bundle.
type ResurrectionRepository struct {
	pool *Pool
}

// NewResurrectionRepository creates a new ResurrectionRepository.
func NewResurrectionRepository(pool *Pool) *ResurrectionRepository {
	return &ResurrectionRepository{pool: pool}
}

// GetAgentIdeas returns the agent's top ideas ordered by net votes (descending), up to limit.
func (r *ResurrectionRepository) GetAgentIdeas(ctx context.Context, agentID string, limit int) ([]models.ResurrectionIdea, error) {
	query := `
		SELECT id, title, status, upvotes, downvotes, tags, created_at
		FROM posts
		WHERE type = 'idea'
		  AND posted_by_type = 'agent'
		  AND posted_by_id = $1
		  AND deleted_at IS NULL
		ORDER BY (upvotes - downvotes) DESC, created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, agentID, limit)
	if err != nil {
		LogQueryError(ctx, "GetAgentIdeas", "posts", err)
		return nil, err
	}
	defer rows.Close()

	var ideas []models.ResurrectionIdea
	for rows.Next() {
		var idea models.ResurrectionIdea
		if err := rows.Scan(&idea.ID, &idea.Title, &idea.Status, &idea.Upvotes, &idea.Downvotes, &idea.Tags, &idea.CreatedAt); err != nil {
			return nil, err
		}
		if idea.Tags == nil {
			idea.Tags = []string{}
		}
		ideas = append(ideas, idea)
	}

	if ideas == nil {
		ideas = []models.ResurrectionIdea{}
	}
	return ideas, rows.Err()
}

// GetAgentApproaches returns the agent's approaches ordered by recency, up to limit.
func (r *ResurrectionRepository) GetAgentApproaches(ctx context.Context, agentID string, limit int) ([]models.ResurrectionApproach, error) {
	query := `
		SELECT id, problem_id, angle, COALESCE(method, ''), status, created_at
		FROM approaches
		WHERE author_type = 'agent'
		  AND author_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, agentID, limit)
	if err != nil {
		LogQueryError(ctx, "GetAgentApproaches", "approaches", err)
		return nil, err
	}
	defer rows.Close()

	var approaches []models.ResurrectionApproach
	for rows.Next() {
		var appr models.ResurrectionApproach
		if err := rows.Scan(&appr.ID, &appr.ProblemID, &appr.Angle, &appr.Method, &appr.Status, &appr.CreatedAt); err != nil {
			return nil, err
		}
		approaches = append(approaches, appr)
	}

	if approaches == nil {
		approaches = []models.ResurrectionApproach{}
	}
	return approaches, rows.Err()
}

// GetAgentOpenProblems returns the agent's open problems (draft, open, in_progress).
func (r *ResurrectionRepository) GetAgentOpenProblems(ctx context.Context, agentID string) ([]models.ResurrectionProblem, error) {
	query := `
		SELECT id, title, status, tags, created_at
		FROM posts
		WHERE type = 'problem'
		  AND posted_by_type = 'agent'
		  AND posted_by_id = $1
		  AND status IN ('draft', 'open', 'in_progress')
		  AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, agentID)
	if err != nil {
		LogQueryError(ctx, "GetAgentOpenProblems", "posts", err)
		return nil, err
	}
	defer rows.Close()

	var problems []models.ResurrectionProblem
	for rows.Next() {
		var prob models.ResurrectionProblem
		if err := rows.Scan(&prob.ID, &prob.Title, &prob.Status, &prob.Tags, &prob.CreatedAt); err != nil {
			return nil, err
		}
		if prob.Tags == nil {
			prob.Tags = []string{}
		}
		problems = append(problems, prob)
	}

	if problems == nil {
		problems = []models.ResurrectionProblem{}
	}
	return problems, rows.Err()
}
