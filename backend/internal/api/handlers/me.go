// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MeUserRepositoryInterface defines the interface for user repository operations
// needed by the Me handler.
type MeUserRepositoryInterface interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
	GetUserStats(ctx context.Context, userID string) (*models.UserStats, error)
}

// MeAgentStatsInterface defines the interface for fetching computed agent stats.
type MeAgentStatsInterface interface {
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
}

// AuthMethodRepositoryInterface defines the interface for auth method repository operations.
type AuthMethodRepositoryInterface interface {
	FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error)
}

// MeHandler handles the GET /v1/auth/me endpoint.
type MeHandler struct {
	config         *OAuthConfig
	userRepo       MeUserRepositoryInterface
	agentStatsRepo MeAgentStatsInterface
	authMethodRepo AuthMethodRepositoryInterface
}

// NewMeHandler creates a new MeHandler instance.
func NewMeHandler(config *OAuthConfig, userRepo MeUserRepositoryInterface, agentStatsRepo MeAgentStatsInterface, authMethodRepo AuthMethodRepositoryInterface) *MeHandler {
	return &MeHandler{
		config:         config,
		userRepo:       userRepo,
		agentStatsRepo: agentStatsRepo,
		authMethodRepo: authMethodRepo,
	}
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
type AgentMeResponse struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"` // Always "agent" to distinguish from user response
	DisplayName         string   `json:"display_name"`
	Bio                 string   `json:"bio,omitempty"`
	Specialties         []string `json:"specialties,omitempty"`
	AvatarURL           string   `json:"avatar_url,omitempty"`
	Status              string   `json:"status"`
	Reputation          int      `json:"reputation"`
	HumanID             string   `json:"human_id,omitempty"`
	HasHumanBackedBadge bool     `json:"has_human_backed_badge"`
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

	writeMeJSON(w, http.StatusOK, response)
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
