// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// NotificationsRepository handles database operations for notifications.
type NotificationsRepository struct {
	pool *Pool
}

// NewNotificationsRepository creates a new NotificationsRepository.
func NewNotificationsRepository(pool *Pool) *NotificationsRepository {
	return &NotificationsRepository{pool: pool}
}

// GetNotificationsForUser returns notifications for a user, paginated, ordered by created_at DESC.
func (r *NotificationsRepository) GetNotificationsForUser(ctx context.Context, userID string, page, perPage int) ([]models.Notification, int, error) {
	return r.getNotifications(ctx, "user_id", userID, page, perPage)
}

// GetNotificationsForAgent returns notifications for an agent, paginated, ordered by created_at DESC.
func (r *NotificationsRepository) GetNotificationsForAgent(ctx context.Context, agentID string, page, perPage int) ([]models.Notification, int, error) {
	return r.getNotifications(ctx, "agent_id", agentID, page, perPage)
}

// getNotifications is the shared implementation for user/agent notification queries.
func (r *NotificationsRepository) getNotifications(ctx context.Context, column, id string, page, perPage int) ([]models.Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Count total
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM notifications WHERE %s = $1`, column)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, id).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "GetNotifications.Count", "notifications", err)
		return nil, 0, err
	}

	// Get paginated notifications
	query := fmt.Sprintf(`
		SELECT id, user_id, agent_id, type, title, COALESCE(body, '') as body, COALESCE(link, '') as link, read_at, created_at
		FROM notifications
		WHERE %s = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, column)

	rows, err := r.pool.Query(ctx, query, id, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "GetNotifications.Query", "notifications", err)
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		err := rows.Scan(
			&n.ID,
			&n.UserID,
			&n.AgentID,
			&n.Type,
			&n.Title,
			&n.Body,
			&n.Link,
			&n.ReadAt,
			&n.CreatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "GetNotifications.Scan", "notifications", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if notifications == nil {
		notifications = []models.Notification{}
	}

	return notifications, total, nil
}

// MarkRead marks a notification as read by setting read_at = NOW().
func (r *NotificationsRepository) MarkRead(ctx context.Context, id string) (*models.Notification, error) {
	query := `
		UPDATE notifications
		SET read_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, agent_id, type, title, COALESCE(body, '') as body, COALESCE(link, '') as link, read_at, created_at
	`

	var n models.Notification
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&n.ID,
		&n.UserID,
		&n.AgentID,
		&n.Type,
		&n.Title,
		&n.Body,
		&n.Link,
		&n.ReadAt,
		&n.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotificationNotFound
		}
		LogQueryError(ctx, "MarkRead", "notifications", err)
		return nil, err
	}

	return &n, nil
}

// MarkAllReadForUser marks all unread notifications as read for a user.
func (r *NotificationsRepository) MarkAllReadForUser(ctx context.Context, userID string) (int, error) {
	return r.markAllRead(ctx, "user_id", userID)
}

// MarkAllReadForAgent marks all unread notifications as read for an agent.
func (r *NotificationsRepository) MarkAllReadForAgent(ctx context.Context, agentID string) (int, error) {
	return r.markAllRead(ctx, "agent_id", agentID)
}

// markAllRead is the shared implementation for marking all notifications as read.
func (r *NotificationsRepository) markAllRead(ctx context.Context, column, id string) (int, error) {
	query := fmt.Sprintf(`
		UPDATE notifications
		SET read_at = NOW()
		WHERE %s = $1 AND read_at IS NULL
	`, column)

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		LogQueryError(ctx, "MarkAllRead", "notifications", err)
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// GetUnreadCountForAgent returns the number of unread notifications for an agent.
func (r *NotificationsRepository) GetUnreadCountForAgent(ctx context.Context, agentID string) (int, error) {
	return r.getUnreadCount(ctx, "agent_id", agentID)
}

// GetUnreadCountForUser returns the number of unread notifications for a user.
func (r *NotificationsRepository) GetUnreadCountForUser(ctx context.Context, userID string) (int, error) {
	return r.getUnreadCount(ctx, "user_id", userID)
}

func (r *NotificationsRepository) getUnreadCount(ctx context.Context, column, id string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM notifications WHERE %s = $1 AND read_at IS NULL`, column)
	var count int
	err := r.pool.QueryRow(ctx, query, id).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "GetUnreadCount", "notifications", err)
		return 0, err
	}
	return count, nil
}

// GetRecentUnreadForAgent returns the most recent unread notifications for an agent,
// limited by the given count, along with the total unread count.
// Used by the agent briefing /me endpoint.
func (r *NotificationsRepository) GetRecentUnreadForAgent(ctx context.Context, agentID string, limit int) ([]models.Notification, int, error) {
	if limit < 1 {
		limit = 10
	}

	// Get total unread count
	var totalUnread int
	countQuery := `SELECT COUNT(*) FROM notifications WHERE agent_id = $1 AND read_at IS NULL`
	err := r.pool.QueryRow(ctx, countQuery, agentID).Scan(&totalUnread)
	if err != nil {
		LogQueryError(ctx, "GetRecentUnreadForAgent.Count", "notifications", err)
		return nil, 0, err
	}

	// Get recent unread notifications
	query := `
		SELECT id, user_id, agent_id, type, title, COALESCE(body, '') as body, COALESCE(link, '') as link, read_at, created_at
		FROM notifications
		WHERE agent_id = $1 AND read_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, agentID, limit)
	if err != nil {
		LogQueryError(ctx, "GetRecentUnreadForAgent.Query", "notifications", err)
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		err := rows.Scan(
			&n.ID,
			&n.UserID,
			&n.AgentID,
			&n.Type,
			&n.Title,
			&n.Body,
			&n.Link,
			&n.ReadAt,
			&n.CreatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "GetRecentUnreadForAgent.Scan", "notifications", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if notifications == nil {
		notifications = []models.Notification{}
	}

	return notifications, totalUnread, nil
}

// FindByID finds a notification by ID.
func (r *NotificationsRepository) FindByID(ctx context.Context, id string) (*models.Notification, error) {
	query := `
		SELECT id, user_id, agent_id, type, title, COALESCE(body, '') as body, COALESCE(link, '') as link, read_at, created_at
		FROM notifications
		WHERE id = $1
	`

	var n models.Notification
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&n.ID,
		&n.UserID,
		&n.AgentID,
		&n.Type,
		&n.Title,
		&n.Body,
		&n.Link,
		&n.ReadAt,
		&n.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotificationNotFound
		}
		LogQueryError(ctx, "FindByID", "notifications", err)
		return nil, err
	}

	return &n, nil
}
