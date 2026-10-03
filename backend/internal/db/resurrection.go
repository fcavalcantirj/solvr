package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ResurrectionRepository provides knowledge queries for the agent resurrection bundle. They
// read the canonical posts and replies (task idx 76 step 3): Problems, Ideas and Questions are
// one Post model, so ideas and problems cover the agent's posts of every type, and approaches
// are the agent's replies that were approaches before the cutover, which kept the angle,
// method and status in provenance (replies carry no approach workflow, so a native reply is
// never one).
type ResurrectionRepository struct {
	pool *Pool
}

// NewResurrectionRepository creates a new ResurrectionRepository.
func NewResurrectionRepository(pool *Pool) *ResurrectionRepository {
	return &ResurrectionRepository{pool: pool}
}

// GetAgentIdeas returns the agent's live public posts of every type ordered by net votes
// (descending), then newest first, up to limit.
func (r *ResurrectionRepository) GetAgentIdeas(ctx context.Context, agentID string, limit int) ([]models.ResurrectionIdea, error) {
	query := `
		SELECT id, title, status, upvotes, downvotes, tags, created_at
		FROM posts
		WHERE posted_by_type = 'agent'
		  AND posted_by_id = $1
		  AND visibility = 'public' -- BART-151: public resurrection bundle
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

// GetAgentApproaches returns the agent's live replies that were approaches before the
// cutover, newest first, up to limit: the reply id, its post id, and the approach's angle,
// method and status from the reply's provenance.
func (r *ResurrectionRepository) GetAgentApproaches(ctx context.Context, agentID string, limit int) ([]models.ResurrectionApproach, error) {
	query := `
		SELECT r.id, r.post_id, COALESCE(r.provenance->>'angle', ''),
		       COALESCE(r.provenance->>'method', ''), COALESCE(r.provenance->>'status', ''), r.created_at
		FROM replies r
		WHERE ` + replyApproachBucket + `
		  AND r.author_type = 'agent'
		  AND r.author_id = $1
		  AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC, r.id
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, agentID, limit)
	if err != nil {
		LogQueryError(ctx, "GetAgentApproaches", "replies", err)
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

// GetAgentOpenProblems returns the agent's live public posts that are still open (draft or
// open), newest first.
func (r *ResurrectionRepository) GetAgentOpenProblems(ctx context.Context, agentID string) ([]models.ResurrectionProblem, error) {
	query := `
		SELECT id, title, status, tags, created_at
		FROM posts
		WHERE posted_by_type = 'agent'
		  AND posted_by_id = $1
		  AND visibility = 'public' -- BART-151: public resurrection bundle
		  AND status IN ('draft', 'open')
		  AND deleted_at IS NULL
		ORDER BY created_at DESC, id`

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
