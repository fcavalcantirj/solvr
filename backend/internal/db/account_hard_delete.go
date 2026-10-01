package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrAccountAuthorsContent is returned when an account cannot be hard-deleted because posts or
// replies still name it as their author (the {posts,replies}_author_{human,agent}_fkey keys).
var ErrAccountAuthorsContent = errors.New("account still authors posts or replies")

// authorsContent reports whether err is the database refusing to delete an account that
// content names as its author.
func authorsContent(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return false
	}
	switch pgErr.ConstraintName {
	case "posts_author_human_fkey", "posts_author_agent_fkey", "replies_author_human_fkey", "replies_author_agent_fkey":
		return true
	}
	return false
}

// HardDelete permanently removes a user from the database (admin-only).
// Per PRD-v5 Task 17: Admin hard-delete endpoints.
// This is IRREVERSIBLE - the user record is permanently deleted.
// Returns ErrNotFound if user doesn't exist, ErrAccountAuthorsContent if posts or replies name them.
func (r *UserRepository) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if authorsContent(err) {
		return ErrAccountAuthorsContent
	}
	if err != nil {
		LogQueryError(ctx, "HardDelete", "users", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// HardDelete permanently removes an agent from the database (admin-only).
// Per PRD-v5 Task 17: Admin hard-delete endpoints.
// This is IRREVERSIBLE - the agent record is permanently deleted.
// Returns ErrAgentNotFound if agent doesn't exist, ErrAccountAuthorsContent if posts or replies name it.
func (r *AgentRepository) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM agents WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if authorsContent(err) {
		return ErrAccountAuthorsContent
	}
	if err != nil {
		LogQueryError(ctx, "HardDelete", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrAgentNotFound
	}

	return nil
}
