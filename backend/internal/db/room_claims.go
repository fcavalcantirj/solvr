package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrClaimNotHeld is returned by Renew/Release when the caller is not the current
// live holder of the claim (wrong holder, expired, or nonexistent).
var ErrClaimNotHeld = errors.New("claim not held by caller")

// RoomClaimRepository implements the atomic claim/lease primitive (mission #2).
type RoomClaimRepository struct {
	pool *Pool
}

// NewRoomClaimRepository creates a new RoomClaimRepository.
func NewRoomClaimRepository(pool *Pool) *RoomClaimRepository {
	return &RoomClaimRepository{pool: pool}
}

// Acquire attempts to take the lock (room_id, key) for holder with the given TTL.
//
// It returns (claim, won). won is true when the caller now holds the lock — either
// because the key was free or because a previous holder's lease had expired and was
// stolen. won is false when a live holder already owns the key; the returned claim
// then describes that current holder.
//
// Atomicity: the acquisition is a single INSERT ... ON CONFLICT DO UPDATE ... WHERE
// expired statement. PostgreSQL takes a row lock on the conflicting key, so under
// concurrent callers exactly one acquires a given key — no lost updates, no races.
func (r *RoomClaimRepository) Acquire(ctx context.Context, p models.AcquireClaimParams) (*models.RoomClaim, bool, error) {
	ttl := p.TTLSeconds
	if ttl <= 0 {
		ttl = 60
	}

	// The DO UPDATE fires only when the existing row has expired; otherwise the
	// conflicting live row is left untouched and RETURNING yields no row.
	query := `
		INSERT INTO room_claims (room_id, claim_key, holder, expires_at)
		VALUES ($1, $2, $3, NOW() + make_interval(secs => $4))
		ON CONFLICT (room_id, claim_key) DO UPDATE
			SET holder = EXCLUDED.holder,
			    expires_at = EXCLUDED.expires_at,
			    updated_at = NOW()
			WHERE room_claims.expires_at < NOW()
		RETURNING room_id, claim_key, holder, expires_at, created_at, updated_at
	`
	var c models.RoomClaim
	err := r.pool.QueryRow(ctx, query, p.RoomID, p.Key, p.Holder, ttl).Scan(
		&c.RoomID, &c.Key, &c.Holder, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == nil {
		return &c, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		LogQueryError(ctx, "Acquire", "room_claims", err)
		return nil, false, err
	}

	// No row updated -> a live holder already owns the key. Report who holds it.
	held, getErr := r.Get(ctx, p.RoomID, p.Key)
	if getErr != nil {
		return nil, false, getErr
	}
	return held, false, nil
}

// Renew extends a claim's TTL, but only for its current live holder. Returns
// ErrClaimNotHeld if the caller is not the live holder.
func (r *RoomClaimRepository) Renew(ctx context.Context, roomID uuid.UUID, key, holder string, ttlSeconds int) (*models.RoomClaim, error) {
	ttl := ttlSeconds
	if ttl <= 0 {
		ttl = 60
	}
	query := `
		UPDATE room_claims
		SET expires_at = NOW() + make_interval(secs => $4), updated_at = NOW()
		WHERE room_id = $1 AND claim_key = $2 AND holder = $3 AND expires_at >= NOW()
		RETURNING room_id, claim_key, holder, expires_at, created_at, updated_at
	`
	var c models.RoomClaim
	err := r.pool.QueryRow(ctx, query, roomID, key, holder, ttl).Scan(
		&c.RoomID, &c.Key, &c.Holder, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClaimNotHeld
		}
		LogQueryError(ctx, "Renew", "room_claims", err)
		return nil, err
	}
	return &c, nil
}

// Release deletes a claim, but only if held by the caller. Returns ErrClaimNotHeld
// if the caller is not the current holder. A live holder may release even a claim
// that has just expired (holder match is what matters for release).
func (r *RoomClaimRepository) Release(ctx context.Context, roomID uuid.UUID, key, holder string) error {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM room_claims WHERE room_id = $1 AND claim_key = $2 AND holder = $3`,
		roomID, key, holder,
	)
	if err != nil {
		LogQueryError(ctx, "Release", "room_claims", err)
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrClaimNotHeld
	}
	return nil
}

// Get returns the claim for (room_id, key) regardless of expiry, or ErrRoomClaimNotFound.
func (r *RoomClaimRepository) Get(ctx context.Context, roomID uuid.UUID, key string) (*models.RoomClaim, error) {
	query := `
		SELECT room_id, claim_key, holder, expires_at, created_at, updated_at
		FROM room_claims
		WHERE room_id = $1 AND claim_key = $2
	`
	var c models.RoomClaim
	err := r.pool.QueryRow(ctx, query, roomID, key).Scan(
		&c.RoomID, &c.Key, &c.Holder, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomClaimNotFound
		}
		LogQueryError(ctx, "Get", "room_claims", err)
		return nil, err
	}
	return &c, nil
}

// ListLive returns all currently-held (non-expired) claims for a room.
func (r *RoomClaimRepository) ListLive(ctx context.Context, roomID uuid.UUID) ([]models.RoomClaim, error) {
	query := `
		SELECT room_id, claim_key, holder, expires_at, created_at, updated_at
		FROM room_claims
		WHERE room_id = $1 AND expires_at >= NOW()
		ORDER BY claim_key ASC
	`
	rows, err := r.pool.Query(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "ListLive", "room_claims", err)
		return nil, err
	}
	defer rows.Close()

	claims := []models.RoomClaim{}
	for rows.Next() {
		var c models.RoomClaim
		if err := rows.Scan(&c.RoomID, &c.Key, &c.Holder, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			LogQueryError(ctx, "ListLive.Scan", "room_claims", err)
			return nil, fmt.Errorf("scan room claim: %w", err)
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

// ErrRoomClaimNotFound is returned when a claim row does not exist.
var ErrRoomClaimNotFound = errors.New("room claim not found")
