// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Pin-related errors.
var (
	ErrPinNotFound  = errors.New("pin not found")
	ErrDuplicatePin = errors.New("pin already exists for this owner and CID")
)

// pinColumns defines the columns returned by pin queries.
const pinColumns = `id, cid, status, COALESCE(name, '') as name, origins, meta, delegates, owner_id, owner_type, size_bytes, created_at, updated_at, pinned_at`

// PinRepository handles database operations for IPFS pins.
type PinRepository struct {
	pool *Pool
}

// NewPinRepository creates a new PinRepository.
func NewPinRepository(pool *Pool) *PinRepository {
	return &PinRepository{pool: pool}
}

// Create inserts a new pin record and populates the pin with DB-generated values.
func (r *PinRepository) Create(ctx context.Context, pin *models.Pin) error {
	query := `
		INSERT INTO pins (cid, status, name, origins, meta, delegates, owner_id, owner_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + pinColumns

	err := r.scanPin(
		r.pool.QueryRow(ctx, query,
			pin.CID,
			pin.Status,
			nullableString(pin.Name),
			pin.Origins,
			pin.Meta,
			pin.Delegates,
			pin.OwnerID,
			pin.OwnerType,
		),
		pin,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			LogDuplicateKeyError(ctx, "Create", "pins", "pins_cid_owner_unique")
			return ErrDuplicatePin
		}
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			LogDuplicateKeyError(ctx, "Create", "pins", "pins_cid_owner_unique")
			return ErrDuplicatePin
		}
		LogQueryError(ctx, "Create", "pins", err)
		return err
	}

	return nil
}

// GetByID retrieves a pin by its UUID (requestid).
func (r *PinRepository) GetByID(ctx context.Context, id string) (*models.Pin, error) {
	query := `SELECT ` + pinColumns + ` FROM pins WHERE id = $1`

	pin := &models.Pin{}
	err := r.scanPin(r.pool.QueryRow(ctx, query, id), pin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPinNotFound
		}
		LogQueryError(ctx, "GetByID", "pins", err)
		return nil, err
	}

	return pin, nil
}

// GetByCID retrieves a pin by CID and owner ID.
func (r *PinRepository) GetByCID(ctx context.Context, cid, ownerID string) (*models.Pin, error) {
	query := `SELECT ` + pinColumns + ` FROM pins WHERE cid = $1 AND owner_id = $2`

	pin := &models.Pin{}
	err := r.scanPin(r.pool.QueryRow(ctx, query, cid, ownerID), pin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPinNotFound
		}
		LogQueryError(ctx, "GetByCID", "pins", err)
		return nil, err
	}

	return pin, nil
}

// ListByOwner returns pins for a specific owner with optional filters and pagination.
func (r *PinRepository) ListByOwner(ctx context.Context, ownerID, ownerType string, opts models.PinListOptions) ([]models.Pin, int, error) {
	// Apply defaults and bounds for limit
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	// Build dynamic WHERE conditions
	conditions := []string{"owner_id = $1", "owner_type = $2"}
	args := []any{ownerID, ownerType}
	argNum := 3

	if opts.CID != "" {
		conditions = append(conditions, fmt.Sprintf("cid = $%d", argNum))
		args = append(args, opts.CID)
		argNum++
	}
	if opts.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name = $%d", argNum))
		args = append(args, opts.Name)
		argNum++
	}
	if opts.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, string(opts.Status))
		argNum++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count total matching records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM pins WHERE %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "ListByOwner-count", "pins", err)
		return nil, 0, err
	}

	// Fetch paginated results
	dataQuery := fmt.Sprintf(
		"SELECT %s FROM pins WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		pinColumns, whereClause, argNum, argNum+1,
	)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		LogQueryError(ctx, "ListByOwner", "pins", err)
		return nil, 0, err
	}
	defer rows.Close()

	var pins []models.Pin
	for rows.Next() {
		var pin models.Pin
		err := r.scanPinRow(rows, &pin)
		if err != nil {
			return nil, 0, err
		}
		pins = append(pins, pin)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if pins == nil {
		pins = []models.Pin{}
	}

	return pins, total, nil
}

// UpdateStatus updates the status of a pin. When status becomes "pinned", pinned_at is set.
func (r *PinRepository) UpdateStatus(ctx context.Context, id string, status models.PinStatus) error {
	var query string

	if status == models.PinStatusPinned {
		query = `
			UPDATE pins
			SET status = $2, pinned_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE pins
			SET status = $2, updated_at = NOW()
			WHERE id = $1
		`
	}

	result, err := r.pool.Exec(ctx, query, id, string(status))
	if err != nil {
		LogQueryError(ctx, "UpdateStatus", "pins", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrPinNotFound
	}

	return nil
}

// UpdateStatusAndSize updates a pin's status and size_bytes atomically.
func (r *PinRepository) UpdateStatusAndSize(ctx context.Context, id string, status models.PinStatus, sizeBytes int64) error {
	query := `
		UPDATE pins
		SET status = $2, size_bytes = $3, pinned_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, string(status), sizeBytes)
	if err != nil {
		LogQueryError(ctx, "UpdateStatusAndSize", "pins", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrPinNotFound
	}

	return nil
}

// Delete removes a pin record (hard delete).
func (r *PinRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM pins WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		LogQueryError(ctx, "Delete", "pins", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrPinNotFound
	}

	return nil
}

// scanPin scans a single row into a Pin struct.
func (r *PinRepository) scanPin(row pgx.Row, pin *models.Pin) error {
	return row.Scan(
		&pin.ID,
		&pin.CID,
		&pin.Status,
		&pin.Name,
		&pin.Origins,
		&pin.Meta,
		&pin.Delegates,
		&pin.OwnerID,
		&pin.OwnerType,
		&pin.SizeBytes,
		&pin.CreatedAt,
		&pin.UpdatedAt,
		&pin.PinnedAt,
	)
}

// scanPinRow scans a pgx.Rows row into a Pin struct.
func (r *PinRepository) scanPinRow(rows pgx.Rows, pin *models.Pin) error {
	return rows.Scan(
		&pin.ID,
		&pin.CID,
		&pin.Status,
		&pin.Name,
		&pin.Origins,
		&pin.Meta,
		&pin.Delegates,
		&pin.OwnerID,
		&pin.OwnerType,
		&pin.SizeBytes,
		&pin.CreatedAt,
		&pin.UpdatedAt,
		&pin.PinnedAt,
	)
}

// nullableString converts an empty string to nil for nullable TEXT columns.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
