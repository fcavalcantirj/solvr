package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// BanRequest bans one account (POST /admin/bans).
type BanRequest struct {
	AccountType  string // "human" or "agent"
	AccountID    string
	Reason       string
	IncludeOwner bool // also ban the human who claimed the agent
	DryRun       bool // report what would be banned, write nothing
}

// BannedIdentity is one row of the ban list.
type BannedIdentity struct {
	Kind     string `json:"kind"`
	Provider string `json:"provider,omitempty"`
	Value    string `json:"value"`
}

// BanResult reports what a ban did (or, in a dry run, would do) to one account.
type BanResult struct {
	AccountType    string           `json:"account_type"`
	AccountID      string           `json:"account_id"`
	AlreadyDeleted bool             `json:"already_deleted"`
	Identities     []BannedIdentity `json:"identities"`
	Owner          *BanResult       `json:"owner,omitempty"`
}

// ErrInvalidBanRequest is returned for an unknown account type or an empty id or reason.
var ErrInvalidBanRequest = errors.New("invalid ban request")

// AccountBanRepository tombstones an account and bans its identities in one transaction.
type AccountBanRepository struct {
	pool *Pool
}

// NewAccountBanRepository creates an AccountBanRepository.
func NewAccountBanRepository(pool *Pool) *AccountBanRepository {
	return &AccountBanRepository{pool: pool}
}

// BanAccount soft-deletes the account (keeping an existing deleted_at), removes its
// credentials, and records its email, OAuth identities or agent id on the ban list. With
// IncludeOwner it does the same for the human who claimed the agent. A dry run rolls back.
func (r *AccountBanRepository) BanAccount(ctx context.Context, req BanRequest) (*BanResult, error) {
	if (req.AccountType != "human" && req.AccountType != "agent") || req.AccountID == "" || strings.TrimSpace(req.Reason) == "" {
		return nil, ErrInvalidBanRequest
	}
	var result *BanResult
	errDryRun := errors.New("dry run")
	err := r.pool.WithTx(ctx, func(tx Tx) error {
		var err error
		if req.AccountType == "human" {
			result, err = banHuman(ctx, tx, req.AccountID, req.Reason)
		} else {
			result, err = banAgent(ctx, tx, req.AccountID, req.Reason, req.IncludeOwner)
		}
		if err != nil {
			return err
		}
		if req.DryRun {
			return errDryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, errDryRun) {
		return nil, err
	}
	return result, nil
}

func banHuman(ctx context.Context, tx Tx, userID, reason string) (*BanResult, error) {
	res := &BanResult{AccountType: "human", AccountID: userID}
	var email string
	err := tx.QueryRow(ctx, `SELECT COALESCE(email, ''), deleted_at IS NOT NULL FROM users WHERE id::text = $1 FOR UPDATE`, userID).
		Scan(&email, &res.AlreadyDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	if email != "" {
		res.Identities = append(res.Identities, BannedIdentity{Kind: "email", Value: strings.ToLower(email)})
	}
	rows, err := tx.Query(ctx, `
		SELECT auth_provider, auth_provider_id FROM auth_methods
		WHERE user_id::text = $1 AND auth_provider IN ('github', 'google') AND COALESCE(auth_provider_id, '') <> ''
		UNION
		SELECT auth_provider, auth_provider_id FROM users
		WHERE id::text = $1 AND auth_provider IN ('github', 'google') AND COALESCE(auth_provider_id, '') <> ''`, userID)
	if err != nil {
		return nil, fmt.Errorf("load auth methods: %w", err)
	}
	for rows.Next() {
		var id BannedIdentity
		if err := rows.Scan(&id.Provider, &id.Value); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan auth method: %w", err)
		}
		id.Kind = "oauth"
		res.Identities = append(res.Identities, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load auth methods: %w", err)
	}

	for _, stmt := range []string{
		`UPDATE users SET deleted_at = COALESCE(deleted_at, NOW()) WHERE id::text = $1`,
		`DELETE FROM user_api_keys WHERE user_id::text = $1`,
		`DELETE FROM refresh_tokens WHERE user_id::text = $1`,
	} {
		if _, err := tx.Exec(ctx, stmt, userID); err != nil {
			return nil, fmt.Errorf("tombstone user: %w", err)
		}
	}
	return res, insertBans(ctx, tx, res, reason)
}

func banAgent(ctx context.Context, tx Tx, agentID, reason string, includeOwner bool) (*BanResult, error) {
	res := &BanResult{AccountType: "agent", AccountID: agentID}
	var ownerID *string
	err := tx.QueryRow(ctx, `SELECT human_id::text, deleted_at IS NOT NULL FROM agents WHERE id = $1 FOR UPDATE`, agentID).
		Scan(&ownerID, &res.AlreadyDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load agent: %w", err)
	}
	res.Identities = []BannedIdentity{{Kind: "agent_id", Value: agentID}}
	if _, err := tx.Exec(ctx, `
		UPDATE agents SET deleted_at = COALESCE(deleted_at, NOW()), api_key_hash = NULL, key_sha256 = NULL
		WHERE id = $1`, agentID); err != nil {
		return nil, fmt.Errorf("tombstone agent: %w", err)
	}
	if err := insertBans(ctx, tx, res, reason); err != nil {
		return nil, err
	}
	if includeOwner && ownerID != nil {
		owner, err := banHuman(ctx, tx, *ownerID, reason)
		if err != nil {
			return nil, fmt.Errorf("ban owner: %w", err)
		}
		res.Owner = owner
	}
	return res, nil
}

func insertBans(ctx context.Context, tx Tx, res *BanResult, reason string) error {
	for _, id := range res.Identities {
		if _, err := tx.Exec(ctx, `
			INSERT INTO banned_identities (kind, provider, value, reason, source_type, source_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, 'admin')
			ON CONFLICT DO NOTHING`, id.Kind, id.Provider, id.Value, reason, res.AccountType, res.AccountID); err != nil {
			return fmt.Errorf("insert ban %s: %w", id.Kind, err)
		}
	}
	return nil
}
