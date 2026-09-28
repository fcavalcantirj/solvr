package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// list returns a paginated list of agents with post counts, scoring each agent's reputation
// with reputationSQL, an expression over the agents row a (also what sort=reputation orders by).
// Supports sorting by: newest, oldest, reputation, posts.
// Supports filtering by status: active, pending, or all.
func (r *AgentRepository) list(ctx context.Context, opts models.AgentListOptions, reputationSQL string) ([]models.AgentWithPostCount, int, error) {
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

	// Always filter out soft-deleted agents (PRD-v5 Task 22)
	conditions = append(conditions, "a.deleted_at IS NULL")

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

	// Main query with post count subquery. bio and avatar_url are nullable (agentColumns
	// coalesces them too): a NULL must not fail the scan into a string.
	query := fmt.Sprintf(`
		SELECT
			a.id,
			a.display_name,
			COALESCE(a.bio, ''),
			a.status,
			%s as reputation,
			a.created_at,
			a.has_human_backed_badge,
			COALESCE(a.avatar_url, ''),
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
	`, reputationSQL, whereClause, sortClause, argNum, argNum+1)

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
