// Package db provides database access for Solvr.
package db

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/reputation"
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
		INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		user.Username,
		user.DisplayName,
		user.Email,
		user.AuthProvider,
		user.AuthProviderID,
		user.PasswordHash,
		user.AvatarURL,
		user.Bio,
		user.Role,
	)

	created := &models.User{}

	// Use sql.NullString for nullable fields
	var passwordHash, avatarURL, bio, authProvider, authProviderID, role sql.NullString

	err := row.Scan(
		&created.ID,
		&created.Username,
		&created.DisplayName,
		&created.Email,
		&authProvider,
		&authProviderID,
		&passwordHash,
		&avatarURL,
		&bio,
		&role,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	// Convert nullable fields to strings (empty if NULL)
	created.AuthProvider = authProvider.String
	created.AuthProviderID = authProviderID.String
	created.PasswordHash = passwordHash.String
	created.AvatarURL = avatarURL.String
	created.Bio = bio.String
	created.Role = role.String

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
// Filters out soft-deleted users (WHERE deleted_at IS NULL).
func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanUser(row)
}

// FindByAuthProvider finds a user by their OAuth provider and provider ID.
// Per SPEC.md Part 5.2: Look up user by auth_provider and auth_provider_id.
//
// This method queries the auth_methods table (introduced in migration 000032)
// rather than the deprecated users.auth_provider columns. This allows:
// - Users to have multiple auth providers (GitHub + Google)
// - Proper auth method linking/unlinking
// - Clean separation of auth data from user profile data
//
// Returns ErrNotFound if no user has this OAuth provider linked.
// Filters out soft-deleted users (WHERE deleted_at IS NULL).
func (r *UserRepository) FindByAuthProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	query := `
		SELECT u.id, u.username, u.display_name, u.email, u.auth_provider,
		       u.auth_provider_id, u.password_hash, u.avatar_url, u.bio,
		       u.role, u.created_at, u.updated_at
		FROM users u
		INNER JOIN auth_methods am ON u.id = am.user_id
		WHERE am.auth_provider = $1 AND am.auth_provider_id = $2
		  AND u.deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, provider, providerID)
	return r.scanUser(row)
}

// FindByEmail finds a user by their email address.
// Per SPEC.md Part 5.2: Link accounts if email matches.
// Filters out soft-deleted users (WHERE deleted_at IS NULL).
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, created_at, updated_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, email)
	return r.scanUser(row)
}

// FindByUsername finds a user by username.
// Returns db.ErrNotFound if no user exists with that username.
// Filters out soft-deleted users (WHERE deleted_at IS NULL).
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, created_at, updated_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, username)
	return r.scanUser(row)
}

// Update updates an existing user.
// Only updates mutable fields: display_name, avatar_url, bio.
func (r *UserRepository) Update(ctx context.Context, user *models.User) (*models.User, error) {
	query := `
		UPDATE users
		SET display_name = $2, avatar_url = $3, bio = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		user.ID,
		user.DisplayName,
		user.AvatarURL,
		user.Bio,
	)

	return r.scanUser(row)
}

// Delete soft-deletes a user by setting deleted_at to NOW().
// Per PRD-v5 Task 12: User self-deletion (soft delete).
// Returns ErrNotFound if user doesn't exist or is already deleted.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// HardDelete permanently removes a user from the database (admin-only).
// Per PRD-v5 Task 17: Admin hard-delete endpoints.
// This is IRREVERSIBLE - the user record is permanently deleted.
// Returns ErrNotFound if user doesn't exist.
func (r *UserRepository) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		LogQueryError(ctx, "HardDelete", "users", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// ListDeleted returns soft-deleted users with pagination.
// Per PRD-v5 Task 17: Admin endpoints to review deleted accounts before permanent deletion.
// Returns users ordered by deleted_at DESC (most recently deleted first).
func (r *UserRepository) ListDeleted(ctx context.Context, page, perPage int) ([]models.User, int, error) {
	offset := (page - 1) * perPage

	query := `
		SELECT id, username, display_name, email, auth_provider, auth_provider_id,
		       password_hash, avatar_url, bio, role, created_at, updated_at, deleted_at
		FROM users
		WHERE deleted_at IS NOT NULL
		ORDER BY deleted_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "ListDeleted", "users", err)
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var passwordHash, avatarURL, bio, authProvider, authProviderID, role sql.NullString
		var deletedAt sql.NullTime

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.Email,
			&authProvider,
			&authProviderID,
			&passwordHash,
			&avatarURL,
			&bio,
			&role,
			&user.CreatedAt,
			&user.UpdatedAt,
			&deletedAt,
		)
		if err != nil {
			LogQueryError(ctx, "ListDeleted.Scan", "users", err)
			return nil, 0, err
		}

		// Convert nullable fields
		user.AuthProvider = authProvider.String
		user.AuthProviderID = authProviderID.String
		user.PasswordHash = passwordHash.String
		user.AvatarURL = avatarURL.String
		user.Bio = bio.String
		user.Role = role.String
		if deletedAt.Valid {
			user.DeletedAt = &deletedAt.Time
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "ListDeleted.Rows", "users", err)
		return nil, 0, err
	}

	// Count total deleted users
	var total int
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NOT NULL`
	err = r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "ListDeleted.Count", "users", err)
		return nil, 0, err
	}

	return users, total, nil
}

// scanUser scans a user row into a User struct.
func (r *UserRepository) scanUser(row pgx.Row) (*models.User, error) {
	user := &models.User{}

	// Use sql.NullString for nullable fields (per schema: auth_provider, auth_provider_id,
	// password_hash, avatar_url, bio, role are all nullable)
	var passwordHash, avatarURL, bio, authProvider, authProviderID, role sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.Email,
		&authProvider,
		&authProviderID,
		&passwordHash,
		&avatarURL,
		&bio,
		&role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	// Convert nullable fields to strings (empty if NULL)
	user.AuthProvider = authProvider.String
	user.AuthProviderID = authProviderID.String
	user.PasswordHash = passwordHash.String
	user.AvatarURL = avatarURL.String
	user.Bio = bio.String
	user.Role = role.String

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
	// Filters out soft-deleted users (WHERE deleted_at IS NULL)
	// Reputation calculated using centralized reputation.BuildReputationSQL
	reputationSQL := reputation.BuildReputationSQL(reputation.SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "", // All-time for list
		IncludeBonus:   false,
	})

	query := `
		SELECT
			u.id,
			u.username,
			u.display_name,
			u.avatar_url,
			` + reputationSQL + ` as reputation,
			(SELECT COUNT(*) FROM agents WHERE human_id = u.id) as agents_count,
			u.created_at
		FROM users u
		WHERE u.deleted_at IS NULL
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

	// Get total count (exclude soft-deleted users)
	var total int
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	err = r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "users", err)
		return nil, 0, err
	}

	return users, total, nil
}
