// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// AutoSolveRepository handles database operations for auto-solving problems
// with old succeeded approaches. It implements the jobs.AutoSolveWarner and
// jobs.AutoSolver interfaces.
type AutoSolveRepository struct {
	pool      *Pool
	notifRepo *NotificationsRepository
}

// NewAutoSolveRepository creates a new AutoSolveRepository.
func NewAutoSolveRepository(pool *Pool, notifRepo *NotificationsRepository) *AutoSolveRepository {
	return &AutoSolveRepository{pool: pool, notifRepo: notifRepo}
}

// autoSolveCandidate holds the data needed to warn or auto-solve a problem.
type autoSolveCandidate struct {
	ProblemID    string
	Title        string
	PostedByType string
	PostedByID   string
}

// WarnProblemsApproachingAutoSolve finds problems with succeeded approaches
// between warningThreshold and solveThreshold old, and sends a warning
// notification to the problem owner. Returns the number of warnings sent.
func (r *AutoSolveRepository) WarnProblemsApproachingAutoSolve(ctx context.Context, warningThreshold, solveThreshold time.Duration) (int64, error) {
	warningCutoff := time.Now().Add(-warningThreshold)
	solveCutoff := time.Now().Add(-solveThreshold)

	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (p.id) p.id, p.title, p.posted_by_type, p.posted_by_id
		FROM posts p
		JOIN approaches a ON a.problem_id = p.id
		WHERE p.type = 'problem'
		  AND p.status NOT IN ('solved', 'closed', 'dormant', 'pending_review', 'rejected')
		  AND p.deleted_at IS NULL
		  AND a.status = 'succeeded'
		  AND a.deleted_at IS NULL
		  AND a.updated_at < $1
		  AND a.updated_at >= $2
		  AND NOT EXISTS (
		    SELECT 1 FROM notifications n
		    WHERE n.type = 'auto_solve_warning'
		      AND n.body LIKE '%' || p.id::text || '%'
		      AND n.created_at > $2
		  )
		ORDER BY p.id, a.updated_at ASC
	`, warningCutoff, solveCutoff)
	if err != nil {
		LogQueryError(ctx, "WarnProblemsApproachingAutoSolve.Query", "posts", err)
		return 0, fmt.Errorf("failed to query problems for auto-solve warning: %w", err)
	}
	defer rows.Close()

	var warned int64
	for rows.Next() {
		var c autoSolveCandidate
		if err := rows.Scan(&c.ProblemID, &c.Title, &c.PostedByType, &c.PostedByID); err != nil {
			LogQueryError(ctx, "WarnProblemsApproachingAutoSolve.Scan", "posts", err)
			continue
		}

		notif := &models.Notification{
			Type:  "auto_solve_warning",
			Title: fmt.Sprintf("Your problem \"%s\" will be auto-solved in 7 days", c.Title),
			Body:  fmt.Sprintf("Problem %s has a succeeded approach. Verify or close it to prevent auto-solve.", c.ProblemID),
			Link:  fmt.Sprintf("/problems/%s", c.ProblemID),
		}

		if c.PostedByType == "agent" {
			notif.AgentID = &c.PostedByID
		} else {
			notif.UserID = &c.PostedByID
		}

		if _, err := r.notifRepo.Create(ctx, notif); err != nil {
			LogQueryError(ctx, "WarnProblemsApproachingAutoSolve.CreateNotification", "notifications", err)
			continue
		}
		warned++
	}

	if err := rows.Err(); err != nil {
		return warned, fmt.Errorf("rows iteration error: %w", err)
	}

	return warned, nil
}

// AutoSolveProblems finds problems with succeeded approaches older than
// olderThan and transitions them to "solved" status. Returns the number
// of problems auto-solved.
func (r *AutoSolveRepository) AutoSolveProblems(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	// Phase A: Find candidate problems
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (p.id) p.id, p.title, p.posted_by_type, p.posted_by_id
		FROM posts p
		JOIN approaches a ON a.problem_id = p.id
		WHERE p.type = 'problem'
		  AND p.status NOT IN ('solved', 'closed', 'dormant', 'pending_review', 'rejected')
		  AND p.deleted_at IS NULL
		  AND a.status = 'succeeded'
		  AND a.deleted_at IS NULL
		  AND a.updated_at < $1
		ORDER BY p.id
	`, cutoff)
	if err != nil {
		LogQueryError(ctx, "AutoSolveProblems.Query", "posts", err)
		return 0, fmt.Errorf("failed to query auto-solve candidates: %w", err)
	}
	defer rows.Close()

	var candidates []autoSolveCandidate
	for rows.Next() {
		var c autoSolveCandidate
		if err := rows.Scan(&c.ProblemID, &c.Title, &c.PostedByType, &c.PostedByID); err != nil {
			LogQueryError(ctx, "AutoSolveProblems.Scan", "posts", err)
			continue
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration error: %w", err)
	}

	if len(candidates) == 0 {
		return 0, nil
	}

	// Phase B: Update status for all candidates
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ProblemID
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE posts
		SET status = 'solved', updated_at = NOW()
		WHERE id = ANY($1)
		  AND status NOT IN ('solved', 'closed')
		  AND deleted_at IS NULL
	`, ids)
	if err != nil {
		LogQueryError(ctx, "AutoSolveProblems.Update", "posts", err)
		return 0, fmt.Errorf("failed to auto-solve problems: %w", err)
	}

	// Phase C: Send notifications to problem owners
	for _, c := range candidates {
		notif := &models.Notification{
			Type:  "auto_solved",
			Title: fmt.Sprintf("Your problem \"%s\" was auto-solved", c.Title),
			Body:  fmt.Sprintf("Problem %s had a succeeded approach for 14+ days and was automatically marked as solved.", c.ProblemID),
			Link:  fmt.Sprintf("/problems/%s", c.ProblemID),
		}

		if c.PostedByType == "agent" {
			notif.AgentID = &c.PostedByID
		} else {
			notif.UserID = &c.PostedByID
		}

		if _, err := r.notifRepo.Create(ctx, notif); err != nil {
			LogQueryError(ctx, "AutoSolveProblems.CreateNotification", "notifications", err)
		}
	}

	return result.RowsAffected(), nil
}
