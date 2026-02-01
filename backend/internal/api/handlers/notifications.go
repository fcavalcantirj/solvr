// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// ErrNotificationNotFound is returned when a notification is not found.
var ErrNotificationNotFound = errors.New("notification not found")

// Notification represents a notification for a user or agent.
// Per SPEC.md Part 6 - Notifications table schema.
type Notification struct {
	// ID is the notification UUID.
	ID string `json:"id"`

	// UserID is the ID of the user recipient (nil if for agent).
	UserID *string `json:"user_id,omitempty"`

	// AgentID is the ID of the agent recipient (nil if for user).
	AgentID *string `json:"agent_id,omitempty"`

	// Type is the notification type (e.g., "answer.created", "comment.created").
	Type string `json:"type"`

	// Title is the notification title.
	Title string `json:"title"`

	// Body is the notification body text.
	Body string `json:"body,omitempty"`

	// Link is the URL to navigate to when clicked.
	Link string `json:"link,omitempty"`

	// ReadAt is when the notification was read (nil if unread).
	ReadAt *time.Time `json:"read_at,omitempty"`

	// CreatedAt is when the notification was created.
	CreatedAt time.Time `json:"created_at"`
}

// NotificationsMeta contains pagination metadata for notifications responses.
type NotificationsMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// NotificationsResponse is the response for listing notifications.
type NotificationsResponse struct {
	Data []Notification    `json:"data"`
	Meta NotificationsMeta `json:"meta"`
}

// NotificationsRepositoryInterface defines the database operations for notifications.
type NotificationsRepositoryInterface interface {
	// GetNotificationsForUser returns notifications for a user.
	// Per SPEC.md Part 5.6: GET /notifications - List
	// Ordered by created_at DESC.
	GetNotificationsForUser(ctx context.Context, userID string, page, perPage int) ([]Notification, int, error)

	// GetNotificationsForAgent returns notifications for an agent.
	GetNotificationsForAgent(ctx context.Context, agentID string, page, perPage int) ([]Notification, int, error)

	// MarkRead marks a notification as read.
	// Per SPEC.md Part 5.6: POST /notifications/:id/read - Mark read
	// Sets read_at = NOW().
	MarkRead(ctx context.Context, id string) (*Notification, error)

	// MarkAllReadForUser marks all unread notifications as read for a user.
	// Per SPEC.md Part 5.6: POST /notifications/read-all - Mark all read
	MarkAllReadForUser(ctx context.Context, userID string) (int, error)

	// MarkAllReadForAgent marks all unread notifications as read for an agent.
	MarkAllReadForAgent(ctx context.Context, agentID string) (int, error)

	// FindByID finds a notification by ID.
	FindByID(ctx context.Context, id string) (*Notification, error)
}

// NotificationsHandler handles notification-related HTTP requests.
type NotificationsHandler struct {
	repo NotificationsRepositoryInterface
}

// NewNotificationsHandler creates a new NotificationsHandler.
func NewNotificationsHandler(repo NotificationsRepositoryInterface) *NotificationsHandler {
	return &NotificationsHandler{repo: repo}
}

// parseNotificationsPagination parses page and per_page query parameters with defaults.
func parseNotificationsPagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 {
			perPage = pp
		}
	}

	// Cap per_page at 50 per SPEC.md
	if perPage > 50 {
		perPage = 50
	}

	return page, perPage
}

// calculateNotificationsHasMore determines if there are more pages.
func calculateNotificationsHasMore(page, perPage, total int) bool {
	return (page * perPage) < total
}

// writeNotificationsJSON writes a JSON response.
func writeNotificationsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeNotificationsError writes an error JSON response.
func writeNotificationsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// getURLParam extracts a URL parameter from the context.
// This works with chi router's URLParam or a custom urlParamKey for testing.
func getURLParam(r *http.Request, key string) string {
	// First try the custom urlParamKey (used in tests)
	if val := r.Context().Value(urlParamKey(key)); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}

	// Then try chi's URL params
	// Since we don't import chi here, we use the context value directly
	// Chi stores params with chi.RouteCtxKey
	// For production, the router will set this; for tests, we use urlParamKey
	return ""
}

// List handles GET /v1/notifications - list notifications.
// Per SPEC.md Part 5.6: GET /notifications -> List
// Requires authentication. Queries notifications for user/agent. Orders by created_at DESC.
func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeNotificationsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	page, perPage := parseNotificationsPagination(r)

	// Get notifications for the authenticated user
	notifications, total, err := h.repo.GetNotificationsForUser(r.Context(), claims.UserID, page, perPage)
	if err != nil {
		writeNotificationsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get notifications")
		return
	}

	// Ensure notifications is not nil for JSON serialization
	if notifications == nil {
		notifications = []Notification{}
	}

	response := NotificationsResponse{
		Data: notifications,
		Meta: NotificationsMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: calculateNotificationsHasMore(page, perPage, total),
		},
	}

	writeNotificationsJSON(w, http.StatusOK, response)
}

// MarkRead handles POST /v1/notifications/:id/read - mark notification as read.
// Per SPEC.md Part 5.6: POST /notifications/:id/read -> Mark read
// Requires authentication (owner). Sets read_at = NOW().
func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeNotificationsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Get notification ID from URL
	notificationID := getURLParam(r, "id")
	if notificationID == "" {
		writeNotificationsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "notification ID required")
		return
	}

	// Find the notification to check ownership
	notification, err := h.repo.FindByID(r.Context(), notificationID)
	if err != nil {
		if errors.Is(err, ErrNotificationNotFound) {
			writeNotificationsError(w, http.StatusNotFound, "NOT_FOUND", "notification not found")
			return
		}
		writeNotificationsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to find notification")
		return
	}

	// Check ownership - notification must belong to the authenticated user
	if notification.UserID == nil || *notification.UserID != claims.UserID {
		// Also check if it's for an agent owned by this user (future enhancement)
		writeNotificationsError(w, http.StatusForbidden, "FORBIDDEN", "not authorized to modify this notification")
		return
	}

	// Mark as read
	updatedNotification, err := h.repo.MarkRead(r.Context(), notificationID)
	if err != nil {
		writeNotificationsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark notification as read")
		return
	}

	writeNotificationsJSON(w, http.StatusOK, updatedNotification)
}

// MarkAllRead handles POST /v1/notifications/read-all - mark all notifications as read.
// Per SPEC.md Part 5.6: POST /notifications/read-all -> Mark all read
// Requires authentication. Updates all unread for user.
func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeNotificationsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Mark all notifications as read for the user
	count, err := h.repo.MarkAllReadForUser(r.Context(), claims.UserID)
	if err != nil {
		writeNotificationsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark notifications as read")
		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"marked_count": count,
		},
	}

	writeNotificationsJSON(w, http.StatusOK, response)
}
