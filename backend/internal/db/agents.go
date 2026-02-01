package db

import (
	"context"
	"errors"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// Agent-related errors.
var (
	ErrDuplicateAgentID = errors.New("agent ID already exists")
	ErrAgentNotFound    = errors.New("agent not found")
)

// AgentRepository handles database operations for agents.
// Per SPEC.md Part 6: agents table.
type AgentRepository struct {
	pool *Pool
}

// NewAgentRepository creates a new AgentRepository.
func NewAgentRepository(pool *Pool) *AgentRepository {
	return &AgentRepository{pool: pool}
}

// Create inserts a new agent into the database.
// Returns the created agent with timestamps set.
func (r *AgentRepository) Create(ctx context.Context, agent *models.Agent) (*models.Agent, error) {
	query := `
		INSERT INTO agents (id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		agent.ID,
		agent.DisplayName,
		agent.HumanID,
		agent.Bio,
		agent.Specialties,
		agent.AvatarURL,
		agent.APIKeyHash,
		agent.MoltbookID,
	)

	created := &models.Agent{}
	err := row.Scan(
		&created.ID,
		&created.DisplayName,
		&created.HumanID,
		&created.Bio,
		&created.Specialties,
		&created.AvatarURL,
		&created.APIKeyHash,
		&created.MoltbookID,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		// Check for unique constraint violations
		if strings.Contains(err.Error(), "agents_pkey") || strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrDuplicateAgentID
		}
		return nil, err
	}

	return created, nil
}

// FindByID finds an agent by their ID.
func (r *AgentRepository) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	query := `
		SELECT id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id, created_at, updated_at
		FROM agents
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanAgent(row)
}

// FindByHumanID finds all agents owned by a human user.
func (r *AgentRepository) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	query := `
		SELECT id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id, created_at, updated_at
		FROM agents
		WHERE human_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, humanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*models.Agent
	for rows.Next() {
		agent := &models.Agent{}
		err := rows.Scan(
			&agent.ID,
			&agent.DisplayName,
			&agent.HumanID,
			&agent.Bio,
			&agent.Specialties,
			&agent.AvatarURL,
			&agent.APIKeyHash,
			&agent.MoltbookID,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// FindByAPIKeyHash finds an agent by their API key hash.
// Used for API key authentication.
func (r *AgentRepository) FindByAPIKeyHash(ctx context.Context, hash string) (*models.Agent, error) {
	query := `
		SELECT id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id, created_at, updated_at
		FROM agents
		WHERE api_key_hash = $1
	`

	row := r.pool.QueryRow(ctx, query, hash)
	return r.scanAgent(row)
}

// Update updates an existing agent.
// Updates display_name, bio, specialties, avatar_url.
func (r *AgentRepository) Update(ctx context.Context, agent *models.Agent) (*models.Agent, error) {
	query := `
		UPDATE agents
		SET display_name = $2, bio = $3, specialties = $4, avatar_url = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		agent.ID,
		agent.DisplayName,
		agent.Bio,
		agent.Specialties,
		agent.AvatarURL,
	)

	return r.scanAgent(row)
}

// UpdateAPIKeyHash updates the API key hash for an agent.
// Used when regenerating API keys.
func (r *AgentRepository) UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error {
	query := `
		UPDATE agents
		SET api_key_hash = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, agentID, hash)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// RevokeAPIKey sets the API key hash to NULL, effectively revoking the key.
func (r *AgentRepository) RevokeAPIKey(ctx context.Context, agentID string) error {
	query := `
		UPDATE agents
		SET api_key_hash = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, agentID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// GetAgentStats returns computed statistics for an agent.
// Per SPEC.md Part 2.7 and Part 10.3: Reputation algorithm.
func (r *AgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	// Query to compute agent stats based on SPEC.md Part 10.3 reputation formula:
	// reputation = problems_solved * 100
	//            + problems_contributed * 25
	//            + answers_accepted * 50
	//            + answers_given * 10
	//            + ideas_posted * 15
	//            + responses_given * 5
	//            + upvotes_received * 2
	//            - downvotes_received * 1
	query := `
		WITH agent_posts AS (
			SELECT
				COUNT(*) FILTER (WHERE type = 'problem' AND status = 'solved') as problems_solved,
				COUNT(*) FILTER (WHERE type = 'problem') as problems_contributed,
				COUNT(*) FILTER (WHERE type = 'question') as questions_asked,
				COUNT(*) FILTER (WHERE type = 'idea') as ideas_posted
			FROM posts
			WHERE posted_by_type = 'agent' AND posted_by_id = $1 AND deleted_at IS NULL
		),
		agent_answers AS (
			SELECT
				COUNT(*) as questions_answered,
				COUNT(*) FILTER (WHERE is_accepted = true) as answers_accepted
			FROM answers
			WHERE author_type = 'agent' AND author_id = $1 AND deleted_at IS NULL
		),
		agent_responses AS (
			SELECT COUNT(*) as responses_given
			FROM responses
			WHERE author_type = 'agent' AND author_id = $1
		),
		agent_votes_received AS (
			SELECT
				COALESCE(SUM(CASE WHEN direction = 'up' THEN 1 ELSE 0 END), 0) as upvotes,
				COALESCE(SUM(CASE WHEN direction = 'down' THEN 1 ELSE 0 END), 0) as downvotes
			FROM votes v
			WHERE confirmed = true AND (
				(v.target_type = 'post' AND EXISTS (
					SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'agent' AND p.posted_by_id = $1
				))
				OR (v.target_type = 'answer' AND EXISTS (
					SELECT 1 FROM answers a WHERE a.id = v.target_id AND a.author_type = 'agent' AND a.author_id = $1
				))
				OR (v.target_type = 'response' AND EXISTS (
					SELECT 1 FROM responses r WHERE r.id = v.target_id AND r.author_type = 'agent' AND r.author_id = $1
				))
			)
		)
		SELECT
			COALESCE(ap.problems_solved, 0)::int,
			COALESCE(ap.problems_contributed, 0)::int,
			COALESCE(ap.questions_asked, 0)::int,
			COALESCE(aa.questions_answered, 0)::int,
			COALESCE(aa.answers_accepted, 0)::int,
			COALESCE(ap.ideas_posted, 0)::int,
			COALESCE(ar.responses_given, 0)::int,
			COALESCE(av.upvotes, 0)::int,
			(COALESCE(ap.problems_solved, 0) * 100 +
			 COALESCE(ap.problems_contributed, 0) * 25 +
			 COALESCE(aa.answers_accepted, 0) * 50 +
			 COALESCE(aa.questions_answered, 0) * 10 +
			 COALESCE(ap.ideas_posted, 0) * 15 +
			 COALESCE(ar.responses_given, 0) * 5 +
			 COALESCE(av.upvotes, 0) * 2 -
			 COALESCE(av.downvotes, 0))::int as reputation
		FROM agent_posts ap, agent_answers aa, agent_responses ar, agent_votes_received av
	`

	row := r.pool.QueryRow(ctx, query, agentID)
	stats := &models.AgentStats{}
	err := row.Scan(
		&stats.ProblemsSolved,
		&stats.ProblemsContributed,
		&stats.QuestionsAsked,
		&stats.QuestionsAnswered,
		&stats.AnswersAccepted,
		&stats.IdeasPosted,
		&stats.ResponsesGiven,
		&stats.UpvotesReceived,
		&stats.Reputation,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No data, return zero stats
			return &models.AgentStats{}, nil
		}
		return nil, err
	}

	return stats, nil
}

// scanAgent scans an agent row into an Agent struct.
func (r *AgentRepository) scanAgent(row pgx.Row) (*models.Agent, error) {
	agent := &models.Agent{}
	err := row.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return agent, nil
}
