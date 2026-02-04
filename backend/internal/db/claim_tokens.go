package db

import (
	"context"
	"errors"
	"log/slog"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// ClaimTokenRepository handles database operations for claim tokens.
// Claim tokens are used for agent-human linking flow.
// See SPEC.md Part 12.3 and PRD AGENT-LINKING category.
type ClaimTokenRepository struct {
	pool *Pool
}

// NewClaimTokenRepository creates a new ClaimTokenRepository.
func NewClaimTokenRepository(pool *Pool) *ClaimTokenRepository {
	return &ClaimTokenRepository{pool: pool}
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
		INSERT INTO claim_tokens (token, agent_id, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(ctx, query, token.Token, token.AgentID, token.ExpiresAt).
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

// FindByToken retrieves a claim token by its token value.
// Returns nil, nil if no token is found (not an error).
// Per prd-v2.json: SELECT * FROM claim_tokens WHERE token = $1
func (r *ClaimTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*models.ClaimToken, error) {
	query := `
		SELECT id, token, agent_id, expires_at, used_at, used_by_human_id, created_at
		FROM claim_tokens
		WHERE token = $1
	`

	token := &models.ClaimToken{}
	err := r.pool.QueryRow(ctx, query, tokenValue).Scan(
		&token.ID,
		&token.Token,
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
// Per prd-v2.json: Query for unexpired, unused tokens.
func (r *ClaimTokenRepository) FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error) {
	query := `
		SELECT id, token, agent_id, expires_at, used_at, used_by_human_id, created_at
		FROM claim_tokens
		WHERE agent_id = $1 AND used_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`

	token := &models.ClaimToken{}
	err := r.pool.QueryRow(ctx, query, agentID).Scan(
		&token.ID,
		&token.Token,
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

	return token, nil
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
