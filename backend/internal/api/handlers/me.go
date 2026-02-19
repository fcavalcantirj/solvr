// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// MeUserRepositoryInterface defines the interface for user repository operations
// needed by the Me handler.
type MeUserRepositoryInterface interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
	GetUserStats(ctx context.Context, userID string) (*models.UserStats, error)
	Delete(ctx context.Context, id string) error
}

// MeAgentStatsInterface defines the interface for fetching computed agent stats.
type MeAgentStatsInterface interface {
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
}

// AuthMethodRepositoryInterface defines the interface for auth method repository operations.
type AuthMethodRepositoryInterface interface {
	FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error)
}

// PoolInterface defines the interface for database pool operations needed by MeHandler.
type PoolInterface interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// BriefingInboxRepo defines the interface for fetching inbox notifications for agent briefing.
type BriefingInboxRepo interface {
	GetRecentUnreadForAgent(ctx context.Context, agentID string, limit int) ([]models.Notification, int, error)
}

// UpdateLastBriefingRepo defines the interface for updating the last briefing timestamp.
type UpdateLastBriefingRepo interface {
	UpdateLastBriefingAt(ctx context.Context, id string) error
}

// InboxSection represents the inbox portion of the agent /me response.
type InboxSection struct {
	UnreadCount int         `json:"unread_count"`
	Items       []InboxItem `json:"items"`
}

// InboxItem represents a single inbox notification item.
type InboxItem struct {
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	BodyPreview string    `json:"body_preview"`
	Link        string    `json:"link"`
	CreatedAt   time.Time `json:"created_at"`
}

// MeHandler handles the GET /v1/auth/me endpoint.
type MeHandler struct {
	config         *OAuthConfig
	userRepo       MeUserRepositoryInterface
	agentStatsRepo MeAgentStatsInterface
	authMethodRepo AuthMethodRepositoryInterface
	pool           PoolInterface
	inboxRepo      BriefingInboxRepo
	briefingRepo   UpdateLastBriefingRepo
}

// NewMeHandler creates a new MeHandler instance.
func NewMeHandler(config *OAuthConfig, userRepo MeUserRepositoryInterface, agentStatsRepo MeAgentStatsInterface, authMethodRepo AuthMethodRepositoryInterface, pool PoolInterface) *MeHandler {
	return &MeHandler{
		config:         config,
		userRepo:       userRepo,
		agentStatsRepo: agentStatsRepo,
		authMethodRepo: authMethodRepo,
		pool:           pool,
	}
}

// SetBriefingRepos sets the inbox and briefing repositories for agent /me enrichment.
func (h *MeHandler) SetBriefingRepos(inboxRepo BriefingInboxRepo, briefingRepo UpdateLastBriefingRepo) {
	h.inboxRepo = inboxRepo
	h.briefingRepo = briefingRepo
}

// MeResponse represents the response for GET /v1/me for humans (JWT auth).
// Per SPEC.md Part 5.2: GET /auth/me -> Current user info.
type MeResponse struct {
	ID          string           `json:"id"`
	Username    string           `json:"username"`
	DisplayName string           `json:"display_name"`
	Email       string           `json:"email"`
	AvatarURL   string           `json:"avatar_url,omitempty"`
	Bio         string           `json:"bio,omitempty"`
	Role        string           `json:"role"`
	Stats       models.UserStats `json:"stats"`
}

// AgentMeResponse represents the response for GET /v1/me for agents (API key auth).
// Per FIX-005: GET /v1/me with API key returns agent info.
// Per prd-v6-ipfs-expanded: includes AMCP identity and pinning quota fields.
type AgentMeResponse struct {
	ID                  string        `json:"id"`
	Type                string        `json:"type"` // Always "agent" to distinguish from user response
	DisplayName         string        `json:"display_name"`
	Bio                 string        `json:"bio,omitempty"`
	Specialties         []string      `json:"specialties,omitempty"`
	AvatarURL           string        `json:"avatar_url,omitempty"`
	Status              string        `json:"status"`
	Reputation          int           `json:"reputation"`
	HumanID             string        `json:"human_id,omitempty"`
	HasHumanBackedBadge bool          `json:"has_human_backed_badge"`
	AMCPEnabled         bool          `json:"amcp_enabled"`
	PinningQuotaBytes   int64         `json:"pinning_quota_bytes"`
	Inbox               *InboxSection `json:"inbox"`
}

// Me handles GET /v1/me
// Supports both JWT (humans) and API key (agents) authentication.
// Per SPEC.md Part 5.2: GET /auth/me -> Current user info.
// Per FIX-005: Must work with CombinedAuthMiddleware.
func (h *MeHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for agent authentication first (API key)
	// Per FIX-005: API key auth should return agent info
	agent := auth.AgentFromContext(ctx)
	if agent != nil {
		h.handleAgentMe(w, ctx, agent)
		return
	}

	// Check for user authentication (JWT)
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeMeUnauthorized(w, "UNAUTHORIZED", "Authentication required")
		return
	}

	h.handleUserMe(w, ctx, claims)
}

// handleAgentMe returns agent info for API key authenticated requests.
func (h *MeHandler) handleAgentMe(w http.ResponseWriter, ctx context.Context, agent *models.Agent) {
	response := AgentMeResponse{
		ID:                  agent.ID,
		Type:                "agent",
		DisplayName:         agent.DisplayName,
		Bio:                 agent.Bio,
		Specialties:         agent.Specialties,
		AvatarURL:           agent.AvatarURL,
		Status:              agent.Status,
		Reputation:          agent.Reputation,
		HasHumanBackedBadge: agent.HasHumanBackedBadge,
		AMCPEnabled:         agent.HasAMCPIdentity,
		PinningQuotaBytes:   agent.PinningQuotaBytes,
	}

	// Override with computed reputation from stats if available
	if h.agentStatsRepo != nil {
		if stats, err := h.agentStatsRepo.GetAgentStats(ctx, agent.ID); err == nil {
			response.Reputation = stats.Reputation
		}
	}

	// Include human_id if claimed
	if agent.HumanID != nil {
		response.HumanID = *agent.HumanID
	}

	// Populate inbox section with graceful degradation
	if h.inboxRepo != nil {
		const inboxLimit = 10
		notifications, totalUnread, err := h.inboxRepo.GetRecentUnreadForAgent(ctx, agent.ID, inboxLimit)
		if err == nil {
			items := make([]InboxItem, len(notifications))
			for i, n := range notifications {
				items[i] = InboxItem{
					Type:        n.Type,
					Title:       n.Title,
					BodyPreview: truncateString(n.Body, 100),
					Link:        n.Link,
					CreatedAt:   n.CreatedAt,
				}
			}
			response.Inbox = &InboxSection{
				UnreadCount: totalUnread,
				Items:       items,
			}
		}
		// If err != nil, Inbox remains nil (graceful degradation)
	}

	// Update last_briefing_at timestamp
	if h.briefingRepo != nil {
		// Fire and forget - don't fail the response if this fails
		_ = h.briefingRepo.UpdateLastBriefingAt(ctx, agent.ID)
	}

	writeMeJSON(w, http.StatusOK, response)
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// handleUserMe returns user info for JWT authenticated requests.
func (h *MeHandler) handleUserMe(w http.ResponseWriter, ctx context.Context, claims *auth.Claims) {
	// Look up user by ID from claims
	user, err := h.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		writeMeInternalError(w, "Failed to fetch user")
		return
	}

	// User not found
	if user == nil {
		writeMeNotFound(w, "User not found")
		return
	}

	// Get user stats
	stats, err := h.userRepo.GetUserStats(ctx, claims.UserID)
	if err != nil {
		// Log error but continue with empty stats
		stats = &models.UserStats{}
	}

	// Build response
	response := MeResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		AvatarURL:   user.AvatarURL,
		Bio:         user.Bio,
		Role:        user.Role,
		Stats:       *stats,
	}

	writeMeJSON(w, http.StatusOK, response)
}

// AuthMethodResponse represents a single auth method in the response.
type AuthMethodResponse struct {
	Provider   string `json:"provider"`      // "google", "github", "email"
	LinkedAt   string `json:"linked_at"`     // ISO8601 timestamp
	LastUsedAt string `json:"last_used_at"`  // ISO8601 timestamp
}

// AuthMethodsListResponse is the response for GET /v1/me/auth-methods
type AuthMethodsListResponse struct {
	AuthMethods []AuthMethodResponse `json:"auth_methods"`
}

// GetMyAuthMethods handles GET /v1/me/auth-methods
// Returns list of authentication methods linked to the current user's account.
// Requires JWT authentication (users only, not agents).
func (h *MeHandler) GetMyAuthMethods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Require user authentication (JWT only)
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeMeUnauthorized(w, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Fetch auth methods from repository
	methods, err := h.authMethodRepo.FindByUserID(ctx, claims.UserID)
	if err != nil {
		writeMeInternalError(w, "Failed to fetch authentication methods")
		return
	}

	// Transform to response format (exclude sensitive fields)
	response := AuthMethodsListResponse{
		AuthMethods: make([]AuthMethodResponse, len(methods)),
	}

	for i, method := range methods {
		response.AuthMethods[i] = AuthMethodResponse{
			Provider:   method.AuthProvider,
			LinkedAt:   method.CreatedAt.Format("2006-01-02T15:04:05.999999Z07:00"),
			LastUsedAt: method.LastUsedAt.Format("2006-01-02T15:04:05.999999Z07:00"),
		}
	}

	writeMeJSON(w, http.StatusOK, response)
}

// DeleteMe handles DELETE /v1/me
// Soft-deletes the authenticated user's account.
// Per PRD-v5 Task 12: User self-deletion.
//
// This endpoint requires JWT authentication. Agents cannot use this endpoint.
//
// Effects of deletion:
// - User is soft-deleted (deleted_at set to NOW())
// - User's agents are unclaimed (human_id set to NULL)
// - User's posts/contributions remain visible
// - User cannot log in after deletion
//
// Returns:
// - 200 OK: Account deleted successfully
// - 401 Unauthorized: No JWT token provided
// - 403 Forbidden: Agent tried to delete user account
// - 404 Not Found: User already deleted
// - 500 Internal Server Error: Database error
func (h *MeHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for agent authentication (API key) - agents cannot delete user accounts
	agent := auth.AgentFromContext(ctx)
	if agent != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "FORBIDDEN",
				"message": "agents cannot delete user accounts",
			},
		})
		return
	}

	// Require user authentication (JWT)
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeMeUnauthorized(w, "UNAUTHORIZED", "authentication required")
		return
	}

	userID := claims.UserID

	// Unclaim all agents owned by this user
	if h.pool != nil {
		if err := h.unclaimAgents(ctx, userID); err != nil {
			writeMeInternalError(w, "Failed to unclaim agents")
			return
		}
	}

	// Soft-delete the user
	err := h.userRepo.Delete(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) || err.Error() == "record not found" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"code":    "NOT_FOUND",
					"message": "user not found",
				},
			})
			return
		}

		writeMeInternalError(w, "Failed to delete account")
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": map[string]string{
			"message": "Account deleted successfully",
		},
	})
}

// unclaimAgents sets human_id to NULL for all agents owned by the given user.
// This allows agents to remain active but unclaimed after user deletion.
func (h *MeHandler) unclaimAgents(ctx context.Context, userID string) error {
	query := `UPDATE agents SET human_id = NULL WHERE human_id = $1`
	_, err := h.pool.Exec(ctx, query, userID)
	return err
}

// Helper functions for writing responses

func writeMeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func writeMeUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeMeNotFound(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "NOT_FOUND",
			"message": message,
		},
	})
}

func writeMeInternalError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": message,
		},
	})
}
