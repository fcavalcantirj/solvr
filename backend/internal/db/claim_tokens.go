package db

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// ClaimTokenRepository handles database operations for claim tokens.
// Claim tokens are used for agent-human linking flow.
// See SPEC.md Part 12.3 and PRD AGENT-LINKING category.
//
// A claim token is a credential, so a row never holds it: a row keeps token_hash (SHA-256,
// how a claim is looked up) and, when the repository has a seal secret, token_sealed (the
// token under a key derived from that secret, see claimTokenSealer).
type ClaimTokenRepository struct {
	pool   *Pool
	sealer *claimTokenSealer
}

// NewClaimTokenRepository creates a new ClaimTokenRepository. Without WithSealSecret it
// keeps only hashes: a live token cannot be shown again (FindActiveByAgentID returns it
// with an empty Token).
func NewClaimTokenRepository(pool *Pool) *ClaimTokenRepository {
	return &ClaimTokenRepository{pool: pool}
}

// WithSealSecret lets the repository keep a sealed copy of each token it creates and open
// the copies it wrote, so a repeat request for a claim link gets the same link back. An
// empty secret keeps the repository hash-only. Call it before the repository serves.
func (r *ClaimTokenRepository) WithSealSecret(secret string) *ClaimTokenRepository {
	r.sealer = newClaimTokenSealer(secret)
	return r
}

// ErrDuplicateClaimToken is returned when attempting to create a token with a duplicate value.
var ErrDuplicateClaimToken = errors.New("claim token already exists")

// ErrClaimTokenNotFound is returned when a claim token is not found.
var ErrClaimTokenNotFound = errors.New("claim token not found")

// Create inserts a new claim token into the database.
// The token's ID and CreatedAt fields are populated from the database after insertion.
// Returns ErrDuplicateClaimToken if the token value already exists.
func (r *ClaimTokenRepository) Create(ctx context.Context, token *models.ClaimToken) error {
	query := `
		INSERT INTO claim_tokens (token_hash, token_sealed, agent_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	hash := hashClaimToken(token.Token)
	sealed, err := r.sealer.seal(token.Token, hash)
	if err != nil {
		LogQueryError(ctx, "Create", "claim_tokens", err)
		return err
	}

	err = r.pool.QueryRow(ctx, query, hash, sealed, token.AgentID, token.ExpiresAt).
		Scan(&token.ID, &token.CreatedAt)

	if err != nil {
		// Check for unique constraint violation
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			slog.Info("duplicate key constraint", "op", "Create", "table", "claim_tokens", "constraint", "token")
			return ErrDuplicateClaimToken
		}
		LogQueryError(ctx, "Create", "claim_tokens", err)
		return err
	}

	return nil
}

// FindByToken retrieves a claim token by its token value, matched on its SHA-256.
// Returns nil, nil if no token is found (not an error). The row's Token is the value the
// caller presented: the database does not hold it.
func (r *ClaimTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*models.ClaimToken, error) {
	query := `
		SELECT id, agent_id, expires_at, used_at, used_by_human_id, created_at
		FROM claim_tokens
		WHERE token_hash = $1
	`

	token := &models.ClaimToken{Token: tokenValue}
	err := r.pool.QueryRow(ctx, query, hashClaimToken(tokenValue)).Scan(
		&token.ID,
		&token.AgentID,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.UsedByHumanID,
		&token.CreatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		LogQueryError(ctx, "FindByToken", "claim_tokens", err)
		return nil, err
	}

	return token, nil
}

// FindActiveByAgentID retrieves the active (unexpired, unused) claim token for an agent.
// Returns nil, nil if no active token exists (not an error).
// Token is opened from the sealed copy; it is empty when that copy cannot be opened (the
// row predates sealing, or the seal secret is not the one it was sealed under), and the
// caller must then replace the token rather than show it.
func (r *ClaimTokenRepository) FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error) {
	query := `
		SELECT id, token_hash, token_sealed, agent_id, expires_at, used_at, used_by_human_id, created_at
		FROM claim_tokens
		WHERE agent_id = $1 AND used_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`

	token := &models.ClaimToken{}
	var hash string
	var sealed []byte
	err := r.pool.QueryRow(ctx, query, agentID).Scan(
		&token.ID,
		&hash,
		&sealed,
		&token.AgentID,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.UsedByHumanID,
		&token.CreatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		LogQueryError(ctx, "FindActiveByAgentID", "claim_tokens", err)
		return nil, err
	}

	if value, err := r.sealer.open(sealed, hash); err == nil {
		token.Token = value
	}

	return token, nil
}

// DeleteUnusedByAgentID deletes every unused claim token of an agent, expired or not, and
// returns how many. Used tokens stay: they record who claimed the agent. It clears the
// way (the partial unique index allows one unused token per agent) for replacing a live
// token whose sealed copy can no longer be opened.
func (r *ClaimTokenRepository) DeleteUnusedByAgentID(ctx context.Context, agentID string) (int64, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM claim_tokens WHERE agent_id = $1 AND used_at IS NULL`, agentID)
	if err != nil {
		LogQueryError(ctx, "DeleteUnusedByAgentID", "claim_tokens", err)
		return 0, err
	}
	return result.RowsAffected(), nil
}

// MarkUsed marks a claim token as used by a human.
// Per prd-v2.json: UPDATE claim_tokens SET used_at, used_by_human_id
// Returns ErrClaimTokenNotFound if the token doesn't exist.
func (r *ClaimTokenRepository) MarkUsed(ctx context.Context, tokenID, humanID string) error {
	query := `
		UPDATE claim_tokens
		SET used_at = NOW(), used_by_human_id = $2
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, tokenID, humanID)
	if err != nil {
		LogQueryError(ctx, "MarkUsed", "claim_tokens", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("claim token not found", "op", "MarkUsed", "table", "claim_tokens", "id", tokenID)
		return ErrClaimTokenNotFound
	}

	return nil
}

// ClaimAgent atomically links an unclaimed agent, grants the claim rewards, and consumes
// the claim token. A failure in any write rolls the whole claim back, so clients can safely
// retry without observing a linked agent with a reusable token or missing rewards.
func (r *ClaimTokenRepository) ClaimAgent(ctx context.Context, tokenID, agentID, humanID string, reputationBonus int) error {
	return r.pool.WithTx(ctx, func(tx Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE agents
			SET human_id = $2,
				human_claimed_at = NOW(),
				reputation = reputation + $3,
				has_human_backed_badge = true,
				updated_at = NOW()
			WHERE id = $1 AND human_id IS NULL
		`, agentID, humanID, reputationBonus)
		if err != nil {
			if strings.Contains(err.Error(), "agent_already_claimed") {
				return ErrAgentAlreadyClaimed
			}
			return err
		}
		if result.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agents WHERE id = $1)`, agentID).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return ErrAgentAlreadyClaimed
			}
			return ErrAgentNotFound
		}

		result, err = tx.Exec(ctx, `
			UPDATE claim_tokens
			SET used_at = NOW(), used_by_human_id = $3
			WHERE id = $1
				AND agent_id = $2
				AND used_at IS NULL
				AND expires_at > NOW()
		`, tokenID, agentID, humanID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrClaimTokenNotFound
		}
		return nil
	})
}

// DeleteExpiredByAgentID deletes expired unused claim tokens for a specific agent.
// This unblocks the partial unique index (one active token per agent) after expiry.
func (r *ClaimTokenRepository) DeleteExpiredByAgentID(ctx context.Context, agentID string) (int64, error) {
	query := `
		DELETE FROM claim_tokens
		WHERE agent_id = $1 AND expires_at < NOW() AND used_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, agentID)
	if err != nil {
		LogQueryError(ctx, "DeleteExpiredByAgentID", "claim_tokens", err)
		return 0, err
	}

	return result.RowsAffected(), nil
}

// DeleteExpiredTokens deletes all claim tokens that have expired and are unused.
// Per prd-v2.json requirement: "Delete where expires_at < NOW() AND used_at IS NULL"
// Returns the number of deleted tokens.
func (r *ClaimTokenRepository) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM claim_tokens
		WHERE expires_at < NOW() AND used_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		LogQueryError(ctx, "DeleteExpiredTokens", "claim_tokens", err)
		return 0, err
	}

	return result.RowsAffected(), nil
}
