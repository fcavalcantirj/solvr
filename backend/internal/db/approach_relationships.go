// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ApproachRelationshipsRepository handles database operations for approach relationships.
type ApproachRelationshipsRepository struct {
	pool *Pool
}

// NewApproachRelationshipsRepository creates a new ApproachRelationshipsRepository.
func NewApproachRelationshipsRepository(pool *Pool) *ApproachRelationshipsRepository {
	return &ApproachRelationshipsRepository{pool: pool}
}

// CreateRelationship creates a relationship between two approaches.
// When relation_type is "updates", the target (to) approach is marked as is_latest=false.
func (r *ApproachRelationshipsRepository) CreateRelationship(ctx context.Context, rel *models.ApproachRelationship) (*models.ApproachRelationship, error) {
	var id string
	var createdAt pgtype.Timestamptz

	err := r.pool.WithTx(ctx, func(tx Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO approach_relationships (from_approach_id, to_approach_id, relation_type)
			VALUES ($1, $2, $3)
			RETURNING id, created_at
		`, rel.FromApproachID, rel.ToApproachID, rel.RelationType).Scan(&id, &createdAt)
		if err != nil {
			return fmt.Errorf("insert relationship: %w", err)
		}

		// If this is an "updates" relationship, mark the old approach as not latest
		if rel.RelationType == models.RelationTypeUpdates {
			_, err = tx.Exec(ctx, `
				UPDATE approaches SET is_latest = false WHERE id = $1
			`, rel.ToApproachID)
			if err != nil {
				return fmt.Errorf("mark old approach not latest: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	rel.ID = id
	rel.CreatedAt = createdAt.Time
	return rel, nil
}

// GetRelationships returns all relationships for an approach (both directions).
func (r *ApproachRelationshipsRepository) GetRelationships(ctx context.Context, approachID string) ([]models.ApproachRelationship, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, from_approach_id, to_approach_id, relation_type, created_at
		FROM approach_relationships
		WHERE from_approach_id = $1 OR to_approach_id = $1
		ORDER BY created_at ASC
	`, approachID)
	if err != nil {
		return nil, fmt.Errorf("get relationships: %w", err)
	}
	defer rows.Close()

	var rels []models.ApproachRelationship
	for rows.Next() {
		var rel models.ApproachRelationship
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&rel.ID, &rel.FromApproachID, &rel.ToApproachID, &rel.RelationType, &createdAt); err != nil {
			return nil, fmt.Errorf("scan relationship: %w", err)
		}
		rel.CreatedAt = createdAt.Time
		rels = append(rels, rel)
	}

	return rels, nil
}

// GetVersionChain traverses the relationship graph backwards from the given approach
// to find its full version history. depth=0 means no limit.
// Returns the current approach, ordered history (oldest first), and all relationships.
func (r *ApproachRelationshipsRepository) GetVersionChain(ctx context.Context, approachID string, depth int) (*models.ApproachVersionHistory, error) {
	// Get the current approach with author info
	current, err := r.getApproachWithAuthor(ctx, approachID)
	if err != nil {
		return nil, fmt.Errorf("get current approach: %w", err)
	}

	// Traverse backwards through "updates" relationships
	var history []models.ApproachWithAuthor
	var allRels []models.ApproachRelationship
	currentID := approachID
	steps := 0

	for {
		if depth > 0 && steps >= depth {
			break
		}

		// Find what this approach updates (from=current, to=parent)
		var parentID string
		var rel models.ApproachRelationship
		var relCreatedAt pgtype.Timestamptz

		err := r.pool.QueryRow(ctx, `
			SELECT id, from_approach_id, to_approach_id, relation_type, created_at
			FROM approach_relationships
			WHERE from_approach_id = $1
			ORDER BY created_at DESC
			LIMIT 1
		`, currentID).Scan(&rel.ID, &rel.FromApproachID, &rel.ToApproachID, &rel.RelationType, &relCreatedAt)

		if err != nil {
			if err == pgx.ErrNoRows {
				break // No more parents
			}
			return nil, fmt.Errorf("traverse chain: %w", err)
		}

		rel.CreatedAt = relCreatedAt.Time
		allRels = append(allRels, rel)
		parentID = rel.ToApproachID

		parent, err := r.getApproachWithAuthor(ctx, parentID)
		if err != nil {
			break // Parent might be deleted
		}

		history = append(history, *parent)
		currentID = parentID
		steps++
	}

	// Reverse history so oldest is first
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	return &models.ApproachVersionHistory{
		Current:       *current,
		History:       history,
		Relationships: allRels,
	}, nil
}

// MarkForForgetting sets the forget_after timestamp on an approach.
func (r *ApproachRelationshipsRepository) MarkForForgetting(ctx context.Context, approachID string, forgetAfter pgtype.Timestamptz) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE approaches SET forget_after = $1 WHERE id = $2
	`, forgetAfter, approachID)
	if err != nil {
		return fmt.Errorf("mark for forgetting: %w", err)
	}
	return nil
}

// ArchiveApproach marks an approach as archived with the given IPFS CID.
func (r *ApproachRelationshipsRepository) ArchiveApproach(ctx context.Context, approachID string, cid string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE approaches SET archived_at = NOW(), archived_cid = $1 WHERE id = $2
	`, cid, approachID)
	if err != nil {
		return fmt.Errorf("archive approach: %w", err)
	}
	return nil
}

// ListStaleApproaches finds approaches eligible for archival.
// Criteria: failed approaches older than failedDays, superseded (is_latest=false) older than supersededDays.
// Excludes already-archived approaches.
func (r *ApproachRelationshipsRepository) ListStaleApproaches(ctx context.Context, failedDays int, supersededDays int) ([]models.Approach, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, problem_id, author_type, author_id, angle, method, status, is_latest,
		       created_at, updated_at
		FROM approaches
		WHERE deleted_at IS NULL
		  AND archived_at IS NULL
		  AND (
		    (status = 'failed' AND updated_at < NOW() - make_interval(days => $1))
		    OR
		    (is_latest = false AND updated_at < NOW() - make_interval(days => $2))
		  )
		ORDER BY updated_at ASC
	`, failedDays, supersededDays)
	if err != nil {
		return nil, fmt.Errorf("list stale approaches: %w", err)
	}
	defer rows.Close()

	var approaches []models.Approach
	for rows.Next() {
		var a models.Approach
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(
			&a.ID, &a.ProblemID, &a.AuthorType, &a.AuthorID,
			&a.Angle, &a.Method, &a.Status, &a.IsLatest,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale approach: %w", err)
		}
		a.CreatedAt = createdAt.Time
		a.UpdatedAt = updatedAt.Time
		approaches = append(approaches, a)
	}

	return approaches, nil
}

// getApproachWithAuthor is a helper to fetch a single approach with author info.
func (r *ApproachRelationshipsRepository) getApproachWithAuthor(ctx context.Context, id string) (*models.ApproachWithAuthor, error) {
	var a models.ApproachWithAuthor
	var createdAt, updatedAt pgtype.Timestamptz
	var deletedAt pgtype.Timestamptz
	var forgetAfter pgtype.Timestamptz
	var archivedAt pgtype.Timestamptz
	var authorDisplayName, authorAvatarURL pgtype.Text
	var archivedCID pgtype.Text

	err := r.pool.QueryRow(ctx, `
		SELECT
			a.id, a.problem_id, a.author_type, a.author_id,
			a.angle, a.method, a.status, a.is_latest,
			a.outcome, a.solution,
			a.created_at, a.updated_at, a.deleted_at,
			a.forget_after, a.archived_at, a.archived_cid,
			COALESCE(ag.display_name, u.display_name, a.author_id) as author_display_name,
			COALESCE(ag.avatar_url, u.avatar_url, '') as author_avatar_url
		FROM approaches a
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		WHERE a.id = $1 AND a.deleted_at IS NULL
	`, id).Scan(
		&a.ID, &a.ProblemID, &a.AuthorType, &a.AuthorID,
		&a.Angle, &a.Method, &a.Status, &a.IsLatest,
		&a.Outcome, &a.Solution,
		&createdAt, &updatedAt, &deletedAt,
		&forgetAfter, &archivedAt, &archivedCID,
		&authorDisplayName, &authorAvatarURL,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrApproachNotFound
		}
		return nil, fmt.Errorf("get approach with author: %w", err)
	}

	a.CreatedAt = createdAt.Time
	a.UpdatedAt = updatedAt.Time
	if deletedAt.Valid {
		a.DeletedAt = &deletedAt.Time
	}
	if forgetAfter.Valid {
		a.ForgetAfter = &forgetAfter.Time
	}
	if archivedAt.Valid {
		a.ArchivedAt = &archivedAt.Time
	}
	if archivedCID.Valid {
		a.ArchivedCID = archivedCID.String
	}
	a.Author = models.ApproachAuthor{
		Type:        a.AuthorType,
		ID:          a.AuthorID,
		DisplayName: authorDisplayName.String,
		AvatarURL:   authorAvatarURL.String,
	}

	return &a, nil
}
