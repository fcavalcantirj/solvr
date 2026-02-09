package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Agent-related errors.
var (
	ErrDuplicateAgentID    = errors.New("agent ID already exists")
	ErrAgentNotFound       = errors.New("agent not found")
	ErrAgentAlreadyClaimed = errors.New("agent is already claimed by a human")
)

// AgentRepository handles database operations for agents.
// Per SPEC.md Part 6: agents table.
type AgentRepository struct {
	pool *Pool
}

// agentColumns defines the standard columns returned when querying agents.
// Used to keep queries consistent and DRY.
// Note: COALESCE handles NULL values for nullable columns scanned into non-pointer Go types.
// Without COALESCE, pgx fails when scanning NULL into string/[]string.
const agentColumns = `id, display_name, human_id, COALESCE(bio, '') as bio, COALESCE(specialties, '{}') as specialties, COALESCE(avatar_url, '') as avatar_url, COALESCE(api_key_hash, '') as api_key_hash, COALESCE(moltbook_id, '') as moltbook_id, COALESCE(model, '') as model, COALESCE(email, '') as email, COALESCE(external_links, '{}') as external_links, status, reputation, human_claimed_at, has_human_backed_badge, created_at, updated_at`

// NewAgentRepository creates a new AgentRepository.
func NewAgentRepository(pool *Pool) *AgentRepository {
	return &AgentRepository{pool: pool}
}

// Create inserts a new agent into the database.
// The agent struct is populated with timestamps after successful creation.
func (r *AgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	query := `
		INSERT INTO agents (id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, moltbook_id, model, email, external_links)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING ` + agentColumns

	row := r.pool.QueryRow(ctx, query,
		agent.ID,
		agent.DisplayName,
		agent.HumanID,
		agent.Bio,
		agent.Specialties,
		agent.AvatarURL,
		agent.APIKeyHash,
		agent.MoltbookID,
		agent.Model,
		agent.Email,
		agent.ExternalLinks,
	)

	err := row.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			slog.Info("duplicate key constraint", "op", "Create", "table", "agents", "constraint", "id", "agent_id", agent.ID)
			return ErrDuplicateAgentID
		}
		LogQueryError(ctx, "Create", "agents", err)
		return err
	}

	return nil
}

// FindByID finds an agent by their ID.
func (r *AgentRepository) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanAgent(row)
}

// FindByHumanID finds all agents owned by a human user.
func (r *AgentRepository) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE human_id = $1 ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, humanID)
	if err != nil {
		LogQueryError(ctx, "FindByHumanID", "agents", err)
		return nil, err
	}
	defer rows.Close()

	var agents []*models.Agent
	for rows.Next() {
		agent, err := r.scanAgentRows(rows)
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
	query := `SELECT ` + agentColumns + ` FROM agents WHERE api_key_hash = $1`

	row := r.pool.QueryRow(ctx, query, hash)
	return r.scanAgent(row)
}

// Update updates an existing agent.
// Updates display_name, bio, specialties, avatar_url, model, email, external_links.
// The agent struct is updated with new values after successful update.
func (r *AgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	query := `
		UPDATE agents
		SET display_name = $2, bio = $3, specialties = $4, avatar_url = $5, model = $6, email = $7, external_links = $8, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + agentColumns

	row := r.pool.QueryRow(ctx, query,
		agent.ID,
		agent.DisplayName,
		agent.Bio,
		agent.Specialties,
		agent.AvatarURL,
		agent.Model,
		agent.Email,
		agent.ExternalLinks,
	)

	err := row.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("agent not found", "op", "Update", "table", "agents", "id", agent.ID)
			return ErrAgentNotFound
		}
		LogQueryError(ctx, "Update", "agents", err)
		return err
	}

	return nil
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
		LogQueryError(ctx, "UpdateAPIKeyHash", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("agent not found", "op", "UpdateAPIKeyHash", "table", "agents", "id", agentID)
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
		LogQueryError(ctx, "RevokeAPIKey", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("agent not found", "op", "RevokeAPIKey", "table", "agents", "id", agentID)
		return ErrAgentNotFound
	}

	return nil
}

// GetAgentStats returns computed statistics for an agent.
// Per SPEC.md Part 2.7 and Part 10.3: Reputation algorithm.
func (r *AgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	// Query to compute agent stats based on SPEC.md Part 10.3 reputation formula:
	// reputation = bonus_points (from agents.reputation column)
	//            + problems_solved * 100
	//            + problems_contributed * 25
	//            + answers_accepted * 50
	//            + answers_given * 10
	//            + ideas_posted * 15
	//            + responses_given * 5
	//            + upvotes_received * 2
	//            - downvotes_received * 1
	query := `
		WITH agent_bonus AS (
			SELECT COALESCE(reputation, 0) as bonus FROM agents WHERE id = $1
		),
		agent_posts AS (
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
			(ab.bonus +
			 COALESCE(ap.problems_solved, 0) * 100 +
			 COALESCE(ap.problems_contributed, 0) * 25 +
			 COALESCE(aa.answers_accepted, 0) * 50 +
			 COALESCE(aa.questions_answered, 0) * 10 +
			 COALESCE(ap.ideas_posted, 0) * 15 +
			 COALESCE(ar.responses_given, 0) * 5 +
			 COALESCE(av.upvotes, 0) * 2 -
			 COALESCE(av.downvotes, 0))::int as reputation
		FROM agent_bonus ab, agent_posts ap, agent_answers aa, agent_responses ar, agent_votes_received av
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
		LogQueryError(ctx, "GetAgentStats", "agents", err)
		return nil, err
	}

	return stats, nil
}

// scanAgent scans an agent row into an Agent struct.
// Expects columns in order defined by agentColumns constant.
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
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
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

// scanAgentRows scans a rows result into an Agent struct.
// Used for queries that return multiple rows.
func (r *AgentRepository) scanAgentRows(rows pgx.Rows) (*models.Agent, error) {
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
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// GetActivity returns the activity history for an agent.
// Per SPEC.md Part 4.9 and Part 5.6.
// Returns posts, answers, approaches created by the agent, ordered by created_at DESC.
func (r *AgentRepository) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	// First verify agent exists
	_, err := r.FindByID(ctx, agentID)
	if err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * perPage

	// Query to get activity items - combines posts, answers, approaches
	// Uses UNION ALL to combine results from different tables
	query := `
		WITH activity AS (
			-- Posts created by agent
			SELECT
				p.id::text,
				'post' as type,
				'created' as action,
				p.title,
				p.type as post_type,
				p.status,
				p.created_at,
				'' as target_id,
				'' as target_title
			FROM posts p
			WHERE p.posted_by_type = 'agent' AND p.posted_by_id = $1 AND p.deleted_at IS NULL

			UNION ALL

			-- Answers by agent
			SELECT
				a.id::text,
				'answer' as type,
				'answered' as action,
				LEFT(a.content, 100) as title,
				'' as post_type,
				CASE WHEN a.is_accepted THEN 'accepted' ELSE 'pending' END as status,
				a.created_at,
				p.id::text as target_id,
				p.title as target_title
			FROM answers a
			JOIN posts p ON a.question_id = p.id
			WHERE a.author_type = 'agent' AND a.author_id = $1 AND a.deleted_at IS NULL

			UNION ALL

			-- Approaches by agent
			SELECT
				ap.id::text,
				'approach' as type,
				'started_approach' as action,
				ap.angle as title,
				'' as post_type,
				ap.status,
				ap.created_at,
				p.id::text as target_id,
				p.title as target_title
			FROM approaches ap
			JOIN posts p ON ap.problem_id = p.id
			WHERE ap.author_type = 'agent' AND ap.author_id = $1 AND ap.deleted_at IS NULL

			UNION ALL

			-- Responses to ideas by agent
			SELECT
				r.id::text,
				'response' as type,
				'responded' as action,
				LEFT(r.content, 100) as title,
				'' as post_type,
				'' as status,
				r.created_at,
				p.id::text as target_id,
				p.title as target_title
			FROM responses r
			JOIN posts p ON r.idea_id = p.id
			WHERE r.author_type = 'agent' AND r.author_id = $1
		)
		SELECT id, type, action, title, post_type, status, created_at, target_id, target_title
		FROM activity
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, agentID, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "GetActivity", "activity", err)
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.ActivityItem
	for rows.Next() {
		var item models.ActivityItem
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Action,
			&item.Title,
			&item.PostType,
			&item.Status,
			&item.CreatedAt,
			&item.TargetID,
			&item.TargetTitle,
		)
		if err != nil {
			LogQueryError(ctx, "GetActivity.Scan", "activity", err)
			return nil, 0, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetActivity.Rows", "activity", err)
		return nil, 0, err
	}

	// Count total items
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM posts WHERE posted_by_type = 'agent' AND posted_by_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM answers WHERE author_type = 'agent' AND author_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM approaches WHERE author_type = 'agent' AND author_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM responses WHERE author_type = 'agent' AND author_id = $1
		) as counts
	`

	var total int
	err = r.pool.QueryRow(ctx, countQuery, agentID).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "GetActivity.Count", "activity", err)
		return nil, 0, err
	}

	return items, total, nil
}

// ============================================================================
// Agent-Human Linking methods (AGENT-LINKING requirement)
// ============================================================================

// LinkHuman links an agent to a human user.
// Per AGENT-LINKING requirement: "CHECK constraint: human_id can only be set once"
// The database trigger prevents_agent_reclaim enforces this at DB level.
// Returns ErrAgentAlreadyClaimed if the agent is already linked to a human.
func (r *AgentRepository) LinkHuman(ctx context.Context, agentID, humanID string) error {
	query := `
		UPDATE agents
		SET human_id = $2, human_claimed_at = NOW(), updated_at = NOW()
		WHERE id = $1
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

// FindByName finds an agent by their display name.
// Used for name uniqueness checks during registration.
func (r *AgentRepository) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE display_name = $1`

	row := r.pool.QueryRow(ctx, query, name)
	return r.scanAgent(row)
}

// GetAgentByAPIKeyHash finds an agent by validating the raw API key against stored hashes.
// Uses bcrypt.CompareHashAndPassword to securely compare the key against stored hashes.
// Returns the matching agent if found, or (nil, nil) if no agent matches.
// Per SPEC.md Part 8.1: API keys are hashed with bcrypt and never stored plain.
func (r *AgentRepository) GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error) {
	// Query all agents that have an API key hash set
	query := `SELECT ` + agentColumns + ` FROM agents WHERE api_key_hash IS NOT NULL AND api_key_hash != ''`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "GetAgentByAPIKeyHash", "agents", err)
		return nil, err
	}
	defer rows.Close()

	// Iterate through agents and compare the key against each hash
	for rows.Next() {
		agent, err := r.scanAgentRows(rows)
		if err != nil {
			return nil, err
		}

		// Use bcrypt to compare the raw key against the stored hash
		if err := bcrypt.CompareHashAndPassword([]byte(agent.APIKeyHash), []byte(key)); err == nil {
			// Match found
			return agent, nil
		}
		// If comparison fails, continue to next agent
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetAgentByAPIKeyHash.Rows", "agents", err)
		return nil, err
	}

	// No matching agent found
	return nil, nil
}

// List returns a paginated list of agents with post counts.
// Per API-001: GET /v1/agents - list registered agents.
// Supports sorting by: newest, oldest, reputation, posts.
// Supports filtering by status: active, pending, or all.
func (r *AgentRepository) List(ctx context.Context, opts models.AgentListOptions) ([]models.AgentWithPostCount, int, error) {
	// Build dynamic query with filters
	var conditions []string
	var args []any
	argNum := 1

	// Filter by status
	if opts.Status != "" && opts.Status != "all" {
		conditions = append(conditions, fmt.Sprintf("a.status = $%d", argNum))
		args = append(args, opts.Status)
		argNum++
	}

	// Filter by owner
	if opts.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("a.human_id = $%d", argNum))
		args = append(args, opts.OwnerID.String())
		argNum++
	}

	// Search query (searches in display_name and bio)
	if opts.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(a.display_name ILIKE $%d OR a.bio ILIKE $%d)", argNum, argNum))
		args = append(args, "%"+opts.Query+"%")
		argNum++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Calculate pagination
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	// Determine sort order
	sortClause := "ORDER BY a.created_at DESC" // default: newest
	switch opts.Sort {
	case "oldest":
		sortClause = "ORDER BY a.created_at ASC"
	case "reputation":
		sortClause = "ORDER BY reputation DESC, a.created_at DESC"
	case "posts":
		sortClause = "ORDER BY post_count DESC, a.created_at DESC"
	case "newest", "":
		sortClause = "ORDER BY a.created_at DESC"
	}

	// Query for total count
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM agents a %s`, whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "agents", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Main query with post count subquery
	query := fmt.Sprintf(`
		SELECT
			a.id,
			a.display_name,
			a.bio,
			a.status,
			(
				COALESCE(a.reputation, 0)
				+ (SELECT
					COUNT(*) FILTER (WHERE type = 'problem' AND status = 'solved') * 100 +
					COUNT(*) FILTER (WHERE type = 'problem') * 25 +
					COUNT(*) FILTER (WHERE type = 'idea') * 15
				   FROM posts WHERE posted_by_type = 'agent' AND posted_by_id = a.id AND deleted_at IS NULL)
				+ (SELECT
					COUNT(*) FILTER (WHERE is_accepted = true) * 50 +
					COUNT(*) * 10
				   FROM answers WHERE author_type = 'agent' AND author_id = a.id AND deleted_at IS NULL)
				+ (SELECT COUNT(*) * 5 FROM responses WHERE author_type = 'agent' AND author_id = a.id)
				+ COALESCE((SELECT
					SUM(CASE WHEN v.direction = 'up' THEN 2 ELSE -1 END)
				   FROM votes v WHERE v.confirmed = true AND (
					   (v.target_type = 'post' AND v.target_id IN (SELECT id FROM posts WHERE posted_by_type = 'agent' AND posted_by_id = a.id))
					   OR (v.target_type = 'answer' AND v.target_id IN (SELECT id FROM answers WHERE author_type = 'agent' AND author_id = a.id))
					   OR (v.target_type = 'response' AND v.target_id IN (SELECT id FROM responses WHERE author_type = 'agent' AND author_id = a.id))
				   )), 0)
			) as reputation,
			a.created_at,
			a.has_human_backed_badge,
			a.avatar_url,
			COALESCE((
				SELECT COUNT(*)
				FROM posts p
				WHERE p.posted_by_type = 'agent'
				AND p.posted_by_id = a.id
				AND p.deleted_at IS NULL
			), 0) as post_count
		FROM agents a
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereClause, sortClause, argNum, argNum+1)

	args = append(args, perPage, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "List", "agents", err)
		return nil, 0, fmt.Errorf("list query failed: %w", err)
	}
	defer rows.Close()

	var agents []models.AgentWithPostCount
	for rows.Next() {
		var agent models.AgentWithPostCount
		err := rows.Scan(
			&agent.ID,
			&agent.DisplayName,
			&agent.Bio,
			&agent.Status,
			&agent.Reputation,
			&agent.CreatedAt,
			&agent.HasHumanBackedBadge,
			&agent.AvatarURL,
			&agent.PostCount,
		)
		if err != nil {
			LogQueryError(ctx, "List.Scan", "agents", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "List.Rows", "agents", err)
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	// Return empty slice instead of nil
	if agents == nil {
		agents = []models.AgentWithPostCount{}
	}

	return agents, total, nil
}
