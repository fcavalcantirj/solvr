package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrAccountSuspended is returned when an identity is banned or belongs to a tombstoned
// (soft-deleted) account. The users_refuse_tombstoned_email trigger raises the same message.
var ErrAccountSuspended = errors.New("account suspended")

// IdentityQuery names the identity an entry point is about to admit. Empty fields are not
// checked.
type IdentityQuery struct {
	Email      string
	Provider   string // github, google
	ProviderID string
	AgentID    string
}

// BannedIdentityRepository reads and writes the ban list (migration 000114).
type BannedIdentityRepository struct {
	pool *Pool
}

// NewBannedIdentityRepository creates a BannedIdentityRepository.
func NewBannedIdentityRepository(pool *Pool) *BannedIdentityRepository {
	return &BannedIdentityRepository{pool: pool}
}

// IsRefused reports whether any part of q is on the ban list or belongs to a tombstoned
// account: a soft-deleted user with that email (case-insensitive) or that OAuth identity, or
// a soft-deleted agent with that id. Until a ban list existed, a tombstone was the only ban,
// so a tombstone keeps refusing even without a ban row.
func (r *BannedIdentityRepository) IsRefused(ctx context.Context, q IdentityQuery) (bool, error) {
	if r == nil || r.pool == nil {
		return false, nil // no database: nothing to check against (router wired without a pool)
	}
	email := strings.ToLower(strings.TrimSpace(q.Email))
	var refused bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM banned_identities b
		    WHERE ($1 <> '' AND b.kind = 'email' AND b.value = $1::text COLLATE "C")
		       OR ($2 <> '' AND $3 <> '' AND b.kind = 'oauth' AND b.provider = $2::text COLLATE "C" AND b.value = $3::text COLLATE "C")
		       OR ($4 <> '' AND b.kind = 'agent_id' AND b.value = $4::text COLLATE "C")
		) OR ($1 <> '' AND EXISTS (
		    SELECT 1 FROM users u WHERE u.deleted_at IS NOT NULL AND lower(u.email) = $1
		)) OR ($2 <> '' AND $3 <> '' AND EXISTS (
		    SELECT 1 FROM users u
		    WHERE u.deleted_at IS NOT NULL
		      AND ((u.auth_provider = $2 AND u.auth_provider_id = $3)
		        OR EXISTS (SELECT 1 FROM auth_methods am
		                   WHERE am.user_id = u.id AND am.auth_provider = $2 AND am.auth_provider_id = $3))
		)) OR ($4 <> '' AND EXISTS (
		    SELECT 1 FROM agents a WHERE a.id = $4 AND a.deleted_at IS NOT NULL
		))`,
		email, q.Provider, q.ProviderID, q.AgentID).Scan(&refused)
	if err != nil {
		LogQueryError(ctx, "IsRefused", "banned_identities", err)
		return false, fmt.Errorf("check banned identity: %w", err)
	}
	return refused, nil
}
