// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// Common database errors.
var (
	ErrNotFound          = errors.New("record not found")
	ErrDuplicateUsername = errors.New("username already exists")
	ErrDuplicateEmail    = errors.New("email already exists")
)

// UserRepository handles database operations for users.
// Per SPEC.md Part 6: users table.
type UserRepository struct {
	pool *Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a new user into the database.
// Returns the created user with ID and timestamps set.
func (r *UserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	query := `
		INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, avatar_url, bio, role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, username, display_name, email, auth_provider, auth_provider_id, avatar_url, bio, role, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		user.Username,
		user.DisplayName,
		user.Email,
		user.AuthProvider,
		user.AuthProviderID,
		user.AvatarURL,
		user.Bio,
		user.Role,
	)

	created := &models.User{}
	err := row.Scan(
		&created.ID,
		&created.Username,
		&created.DisplayName,
		&created.Email,
		&created.AuthProvider,
		&created.AuthProviderID,
		&created.AvatarURL,
		&created.Bio,
		&created.Role,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		// Check for unique constraint violations
		if strings.Contains(err.Error(), "users_username_key") {
			slog.Info("duplicate key constraint", "op", "Create", "table", "users", "constraint", "username")
			return nil, ErrDuplicateUsername
		}
		if strings.Contains(err.Error(), "users_email_key") {
			slog.Info("duplicate key constraint", "op", "Create", "table", "users", "constraint", "email")
			return nil, ErrDuplicateEmail
		}
		LogQueryError(ctx, "Create", "users", err)
		return nil, err
	}

	return created, nil
}

// FindByID finds a user by their ID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id, avatar_url, bio, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanUser(row)
}

// FindByAuthProvider finds a user by their OAuth provider and provider ID.
// Per SPEC.md Part 5.2: Look up user by auth_provider and auth_provider_id.
func (r *UserRepository) FindByAuthProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id, avatar_url, bio, role, created_at, updated_at
		FROM users
		WHERE auth_provider = $1 AND auth_provider_id = $2
	`

	row := r.pool.QueryRow(ctx, query, provider, providerID)
	return r.scanUser(row)
}

// FindByEmail finds a user by their email address.
// Per SPEC.md Part 5.2: Link accounts if email matches.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id, avatar_url, bio, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	row := r.pool.QueryRow(ctx, query, email)
	return r.scanUser(row)
}

// Update updates an existing user.
// Only updates mutable fields: display_name, avatar_url, bio.
func (r *UserRepository) Update(ctx context.Context, user *models.User) (*models.User, error) {
	query := `
		UPDATE users
		SET display_name = $2, avatar_url = $3, bio = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, username, display_name, email, auth_provider, auth_provider_id, avatar_url, bio, role, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		user.ID,
		user.DisplayName,
		user.AvatarURL,
		user.Bio,
	)

	return r.scanUser(row)
}

// scanUser scans a user row into a User struct.
func (r *UserRepository) scanUser(row pgx.Row) (*models.User, error) {
	user := &models.User{}
	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.Email,
		&user.AuthProvider,
		&user.AuthProviderID,
		&user.AvatarURL,
		&user.Bio,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("user not found during scan", "table", "users")
			return nil, ErrNotFound
		}
		LogQueryError(context.Background(), "scanUser", "users", err)
		return nil, err
	}

	return user, nil
}

// GetUserStats returns computed statistics for a user.
// Per SPEC.md Part 2.8 and Part 10.3: Reputation algorithm.
func (r *UserRepository) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	// Query to compute user stats based on SPEC.md Part 10.3 reputation formula:
	// reputation = problems_solved * 100
	//            + problems_contributed * 25
	//            + answers_accepted * 50
	//            + answers_given * 10
	//            + ideas_posted * 15
	//            + responses_given * 5
	//            + upvotes_received * 2
	//            - downvotes_received * 1
	query := `
		WITH user_posts AS (
			SELECT
				COUNT(*) as posts_created,
				COUNT(*) FILTER (WHERE type = 'problem' AND status = 'solved') as problems_solved,
				COUNT(*) FILTER (WHERE type = 'problem') as problems_contributed,
				COUNT(*) FILTER (WHERE type = 'idea') as ideas_posted
			FROM posts
			WHERE posted_by_type = 'human' AND posted_by_id = $1 AND deleted_at IS NULL
		),
		user_answers AS (
			SELECT
				COUNT(*) as answers_given,
				COUNT(*) FILTER (WHERE is_accepted = true) as answers_accepted
			FROM answers
			WHERE author_type = 'human' AND author_id = $1 AND deleted_at IS NULL
		),
		user_responses AS (
			SELECT COUNT(*) as responses_given
			FROM responses
			WHERE author_type = 'human' AND author_id = $1
		),
		user_votes_received AS (
			SELECT
				COALESCE(SUM(CASE WHEN direction = 'up' THEN 1 ELSE 0 END), 0) as upvotes,
				COALESCE(SUM(CASE WHEN direction = 'down' THEN 1 ELSE 0 END), 0) as downvotes
			FROM votes v
			WHERE confirmed = true AND (
				(v.target_type = 'post' AND EXISTS (
					SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'human' AND p.posted_by_id = $1
				))
				OR (v.target_type = 'answer' AND EXISTS (
					SELECT 1 FROM answers a WHERE a.id = v.target_id AND a.author_type = 'human' AND a.author_id = $1
				))
				OR (v.target_type = 'response' AND EXISTS (
					SELECT 1 FROM responses r WHERE r.id = v.target_id AND r.author_type = 'human' AND r.author_id = $1
				))
			)
		)
		SELECT
			COALESCE(up.posts_created, 0)::int,
			COALESCE(ua.answers_given, 0)::int,
			COALESCE(ua.answers_accepted, 0)::int,
			COALESCE(uv.upvotes, 0)::int,
			(COALESCE(ua.answers_given, 0) + COALESCE(ur.responses_given, 0))::int as contributions,
			(COALESCE(up.problems_solved, 0) * 100 +
			 COALESCE(up.problems_contributed, 0) * 25 +
			 COALESCE(ua.answers_accepted, 0) * 50 +
			 COALESCE(ua.answers_given, 0) * 10 +
			 COALESCE(up.ideas_posted, 0) * 15 +
			 COALESCE(ur.responses_given, 0) * 5 +
			 COALESCE(uv.upvotes, 0) * 2 -
			 COALESCE(uv.downvotes, 0))::int as reputation
		FROM user_posts up, user_answers ua, user_responses ur, user_votes_received uv
	`

	row := r.pool.QueryRow(ctx, query, userID)
	stats := &models.UserStats{}
	err := row.Scan(
		&stats.PostsCreated,
		&stats.AnswersGiven,
		&stats.AnswersAccepted,
		&stats.UpvotesReceived,
		&stats.Contributions,
		&stats.Reputation,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No data, return zero stats
			return &models.UserStats{}, nil
		}
		LogQueryError(ctx, "GetUserStats", "users", err)
		return nil, err
	}

	return stats, nil
}

// List returns a paginated list of users with public info.
// Per prd-v4: GET /v1/users endpoint - includes agents_count via subquery.
// Supports sort: newest (created_at DESC), reputation, agents.
func (r *UserRepository) List(ctx context.Context, opts models.PublicUserListOptions) ([]models.UserListItem, int, error) {
	// Determine sort order
	var orderBy string
	switch opts.Sort {
	case models.PublicUserSortReputation:
		orderBy = "reputation DESC, u.created_at DESC"
	case models.PublicUserSortAgents:
		orderBy = "agents_count DESC, u.created_at DESC"
	default: // newest
		orderBy = "u.created_at DESC"
	}

	// Query with agents_count subquery
	query := `
		SELECT
			u.id,
			u.username,
			u.display_name,
			u.avatar_url,
			(
				(SELECT
					COUNT(*) FILTER (WHERE is_accepted = true) * 50 +
					COUNT(*) * 10
				 FROM answers WHERE author_type = 'human' AND author_id = u.id AND deleted_at IS NULL)
				+ COALESCE((SELECT
					SUM(CASE WHEN v.direction = 'up' THEN 2 ELSE -1 END)
				 FROM votes v WHERE v.confirmed = true AND (
					 (v.target_type = 'post' AND v.target_id IN (SELECT id FROM posts WHERE posted_by_type = 'human' AND posted_by_id = u.id))
					 OR (v.target_type = 'answer' AND v.target_id IN (SELECT id FROM answers WHERE author_type = 'human' AND author_id = u.id))
				 )), 0)
			) as reputation,
			(SELECT COUNT(*) FROM agents WHERE human_id = u.id) as agents_count,
			u.created_at
		FROM users u
		ORDER BY ` + orderBy + `
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		LogQueryError(ctx, "List", "users", err)
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.UserListItem
	for rows.Next() {
		var user models.UserListItem
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.AvatarURL,
			&user.Reputation,
			&user.AgentsCount,
			&user.CreatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "List.Scan", "users", err)
			return nil, 0, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "List.Rows", "users", err)
		return nil, 0, err
	}

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM users`
	err = r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "users", err)
		return nil, 0, err
	}

	return users, total, nil
}
