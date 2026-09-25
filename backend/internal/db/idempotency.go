package db

import (
	"context"
	"errors"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

const (
	// IdempotencyRetention is how long a create result stays replayable (idx 73 step 4:
	// "retain create results for at least 24 hours").
	IdempotencyRetention = 24 * time.Hour
	// IdempotencyPendingLease is how long an in-flight reservation blocks its key. A
	// pending row older than this belongs to a request that died with its server.
	IdempotencyPendingLease = 5 * time.Minute
)

// IdempotencyRepository stores Idempotency-Key reservations and create results.
type IdempotencyRepository struct {
	pool *Pool
}

// NewIdempotencyRepository creates a new IdempotencyRepository.
func NewIdempotencyRepository(pool *Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

// Reserve claims the key for a new request in one statement: a missing row is
// inserted, and an expired row (past retention) or a dead pending row (past the lease)
// is taken over. Only one concurrent caller can win. When the key is held, the held
// record is returned with reserved=false.
func (r *IdempotencyRepository) Reserve(ctx context.Context, scope models.IdempotencyScope, requestHash string) (*models.IdempotencyRecord, bool, error) {
	// A row released between the failed claim and the read is simply claimed again.
	for attempt := 0; attempt < 3; attempt++ {
		var won bool
		err := r.pool.QueryRow(ctx, `
			INSERT INTO idempotency_keys (actor_type, actor_id, operation, idempotency_key, request_hash)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (actor_type, actor_id, operation, idempotency_key) DO UPDATE
			SET request_hash = EXCLUDED.request_hash, status = 'pending', response_status = NULL,
			    response_content_type = NULL, response_body = NULL, created_at = NOW(), completed_at = NULL
			WHERE idempotency_keys.created_at < NOW() - make_interval(secs => $6)
			   OR (idempotency_keys.status = 'pending' AND idempotency_keys.created_at < NOW() - make_interval(secs => $7))
			RETURNING true`,
			scope.ActorType, scope.ActorID, scope.Operation, scope.Key, requestHash,
			IdempotencyRetention.Seconds(), IdempotencyPendingLease.Seconds(),
		).Scan(&won)
		if err == nil {
			return nil, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, err
		}

		rec := &models.IdempotencyRecord{}
		var respStatus *int
		var respType *string
		err = r.pool.QueryRow(ctx, `
			SELECT request_hash, status, response_status, response_content_type, response_body, created_at
			FROM idempotency_keys
			WHERE actor_type = $1 AND actor_id = $2 AND operation = $3 AND idempotency_key = $4`,
			scope.ActorType, scope.ActorID, scope.Operation, scope.Key,
		).Scan(&rec.RequestHash, &rec.Status, &respStatus, &respType, &rec.ResponseBody, &rec.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		if respStatus != nil {
			rec.ResponseStatus = *respStatus
		}
		if respType != nil {
			rec.ResponseContentType = *respType
		}
		return rec, false, nil
	}
	return nil, false, errors.New("idempotency key contended: reservation kept disappearing")
}

// Complete stores the response of a reserved key so retries replay it.
func (r *IdempotencyRepository) Complete(ctx context.Context, scope models.IdempotencyScope, status int, contentType string, body []byte) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed', response_status = $5, response_content_type = $6,
		    response_body = $7, completed_at = NOW()
		WHERE actor_type = $1 AND actor_id = $2 AND operation = $3 AND idempotency_key = $4
		  AND status = 'pending'`,
		scope.ActorType, scope.ActorID, scope.Operation, scope.Key, status, contentType, body)
	return err
}

// Release drops a pending reservation whose request did not succeed.
func (r *IdempotencyRepository) Release(ctx context.Context, scope models.IdempotencyScope) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM idempotency_keys
		WHERE actor_type = $1 AND actor_id = $2 AND operation = $3 AND idempotency_key = $4
		  AND status = 'pending'`,
		scope.ActorType, scope.ActorID, scope.Operation, scope.Key)
	return err
}
