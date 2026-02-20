// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// StaleContentRepository handles database operations for stale content cleanup.
// It implements the jobs.StaleApproachUpdater, jobs.StaleApproachWarner,
// and jobs.DormantPostUpdater interfaces.
type StaleContentRepository struct {
	pool      *Pool
	notifRepo *NotificationsRepository
}

// NewStaleContentRepository creates a new StaleContentRepository.
func NewStaleContentRepository(pool *Pool, notifRepo *NotificationsRepository) *StaleContentRepository {
	return &StaleContentRepository{pool: pool, notifRepo: notifRepo}
}

// AbandonStaleApproaches updates approaches in 'working' or 'starting' status
// that haven't been updated for longer than olderThan to 'abandoned' status.
// Returns the number of approaches abandoned.
func (r *StaleContentRepository) AbandonStaleApproaches(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result, err := r.pool.Exec(ctx, `
		UPDATE approaches
		SET status = 'abandoned', updated_at = NOW()
		WHERE status IN ('working', 'starting')
		  AND updated_at < $1
		  AND deleted_at IS NULL
	`, cutoff)
	if err != nil {
		LogQueryError(ctx, "AbandonStaleApproaches", "approaches", err)
		return 0, fmt.Errorf("failed to abandon stale approaches: %w", err)
	}

	return result.RowsAffected(), nil
}

// WarnApproachesApproachingAbandonment finds approaches in 'working' or 'starting'
// status that are between warningThreshold and abandonThreshold old, and creates
// a warning notification for each approach author.
// Returns the number of warnings sent.
func (r *StaleContentRepository) WarnApproachesApproachingAbandonment(ctx context.Context, warningThreshold, abandonThreshold time.Duration) (int64, error) {
	warningCutoff := time.Now().Add(-warningThreshold)
	abandonCutoff := time.Now().Add(-abandonThreshold)

	// Find approaches in the warning window (between warningThreshold and abandonThreshold old)
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.angle, a.author_type, a.author_id, a.problem_id, p.title
		FROM approaches a
		JOIN posts p ON p.id = a.problem_id
		WHERE a.status IN ('working', 'starting')
		  AND a.updated_at < $1
		  AND a.updated_at >= $2
		  AND a.deleted_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM notifications n
		    WHERE n.type = 'approach_abandonment_warning'
		      AND n.body LIKE '%' || a.id::text || '%'
		      AND n.created_at > $2
		  )
	`, warningCutoff, abandonCutoff)
	if err != nil {
		LogQueryError(ctx, "WarnApproachesApproachingAbandonment.Query", "approaches", err)
		return 0, fmt.Errorf("failed to query approaches for warning: %w", err)
	}
	defer rows.Close()

	var warned int64
	for rows.Next() {
		var approachID, angle, authorType, authorID, problemID, problemTitle string
		if err := rows.Scan(&approachID, &angle, &authorType, &authorID, &problemID, &problemTitle); err != nil {
			LogQueryError(ctx, "WarnApproachesApproachingAbandonment.Scan", "approaches", err)
			continue
		}

		// Create warning notification for the approach author
		notif := &models.Notification{
			Type:  "approach_abandonment_warning",
			Title: fmt.Sprintf("Your approach on \"%s\" will be auto-abandoned in 7 days", problemTitle),
			Body:  fmt.Sprintf("Approach %s has been inactive. Update it to prevent auto-abandonment.", approachID),
			Link:  fmt.Sprintf("/problems/%s", problemID),
		}

		// Set the correct recipient field based on author type
		if authorType == "agent" {
			notif.AgentID = &authorID
		} else {
			notif.UserID = &authorID
		}

		if _, err := r.notifRepo.Create(ctx, notif); err != nil {
			LogQueryError(ctx, "WarnApproachesApproachingAbandonment.CreateNotification", "notifications", err)
			continue
		}
		warned++
	}

	if err := rows.Err(); err != nil {
		return warned, fmt.Errorf("rows iteration error: %w", err)
	}

	return warned, nil
}

// MarkDormantPosts updates open problem posts that have no approaches
// and are older than olderThan to 'dormant' status.
// Returns the number of posts marked dormant.
func (r *StaleContentRepository) MarkDormantPosts(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result, err := r.pool.Exec(ctx, `
		UPDATE posts
		SET status = 'dormant', updated_at = NOW()
		WHERE type = 'problem'
		  AND status = 'open'
		  AND created_at < $1
		  AND deleted_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM approaches
		    WHERE approaches.problem_id = posts.id
		      AND approaches.deleted_at IS NULL
		  )
	`, cutoff)
	if err != nil {
		LogQueryError(ctx, "MarkDormantPosts", "posts", err)
		return 0, fmt.Errorf("failed to mark dormant posts: %w", err)
	}

	return result.RowsAffected(), nil
}
