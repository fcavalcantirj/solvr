package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// IncidentRepository handles persistence of incidents and their updates.
type IncidentRepository struct {
	pool *Pool
}

// NewIncidentRepository creates a new IncidentRepository.
func NewIncidentRepository(pool *Pool) *IncidentRepository {
	return &IncidentRepository{pool: pool}
}

// Create inserts a new incident.
func (r *IncidentRepository) Create(ctx context.Context, incident models.Incident) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO incidents (id, title, status, severity, affected_services)
		VALUES ($1, $2, $3, $4, $5)
	`, incident.ID, incident.Title, string(incident.Status), string(incident.Severity), incident.AffectedServices)
	if err != nil {
		return fmt.Errorf("create incident: %w", err)
	}
	return nil
}

// UpdateStatus changes the status of an incident. If the new status is "resolved",
// resolved_at is set to NOW().
func (r *IncidentRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	var query string
	if status == string(models.IncidentStatusResolved) {
		query = `UPDATE incidents SET status = $2, updated_at = NOW(), resolved_at = NOW() WHERE id = $1`
	} else {
		query = `UPDATE incidents SET status = $2, updated_at = NOW() WHERE id = $1`
	}

	tag, err := r.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update incident status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("incident %s not found", id)
	}
	return nil
}

// AddUpdate inserts a timeline entry for an incident.
func (r *IncidentRepository) AddUpdate(ctx context.Context, update models.IncidentUpdate) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO incident_updates (incident_id, status, message)
		VALUES ($1, $2, $3)
	`, update.IncidentID, string(update.Status), update.Message)
	if err != nil {
		return fmt.Errorf("add incident update: %w", err)
	}
	return nil
}

// ListRecent returns the most recent incidents with their updates, ordered by created_at DESC.
func (r *IncidentRepository) ListRecent(ctx context.Context, limit int) ([]models.IncidentWithUpdates, error) {
	// First get incidents
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, status, severity, affected_services, created_at, updated_at, resolved_at
		FROM incidents
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []models.IncidentWithUpdates
	for rows.Next() {
		var inc models.IncidentWithUpdates
		if err := rows.Scan(
			&inc.ID, &inc.Title, &inc.Status, &inc.Severity,
			&inc.AffectedServices, &inc.CreatedAt, &inc.UpdatedAt, &inc.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, inc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Then get updates for each incident
	for i := range incidents {
		updates, err := r.getUpdatesForIncident(ctx, incidents[i].ID)
		if err != nil {
			return nil, err
		}
		incidents[i].Updates = updates
	}

	if incidents == nil {
		incidents = []models.IncidentWithUpdates{}
	}
	return incidents, nil
}

// getUpdatesForIncident fetches all updates for a single incident.
func (r *IncidentRepository) getUpdatesForIncident(ctx context.Context, incidentID string) ([]models.IncidentUpdate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, status, message, created_at
		FROM incident_updates
		WHERE incident_id = $1
		ORDER BY created_at DESC
	`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("get incident updates: %w", err)
	}
	defer rows.Close()

	var updates []models.IncidentUpdate
	for rows.Next() {
		var u models.IncidentUpdate
		if err := rows.Scan(&u.ID, &u.IncidentID, &u.Status, &u.Message, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan incident update: %w", err)
		}
		updates = append(updates, u)
	}
	if updates == nil {
		updates = []models.IncidentUpdate{}
	}
	return updates, rows.Err()
}
