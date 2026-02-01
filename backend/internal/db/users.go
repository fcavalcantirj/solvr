// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
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
			return nil, ErrDuplicateUsername
		}
		if strings.Contains(err.Error(), "users_email_key") {
			return nil, ErrDuplicateEmail
		}
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
			return nil, ErrNotFound
		}
		return nil, err
	}

	return user, nil
}
