// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BadgeRepoInterface defines the interface for badge repository operations
// needed by the MeHandler for badges in /me and public badge endpoints.
type BadgeRepoInterface interface {
	ListForOwner(ctx context.Context, ownerType, ownerID string) ([]models.Badge, error)
}

// SetBadgeRepo sets the badge repository on the MeHandler.
func (h *MeHandler) SetBadgeRepo(repo BadgeRepoInterface) {
	h.badgeRepo = repo
}

// BadgesResponse is the response format for badge list endpoints.
type BadgesResponse struct {
	Badges []models.Badge `json:"badges"`
}

// GetAgentBadges handles GET /v1/agents/{id}/badges.
// Returns all badges for the specified agent. No auth required.
func (h *MeHandler) GetAgentBadges(w http.ResponseWriter, r *http.Request, agentID string) {
	ctx := r.Context()

	if h.badgeRepo == nil {
		writeMeJSON(w, http.StatusOK, BadgesResponse{Badges: []models.Badge{}})
		return
	}

	badges, err := h.badgeRepo.ListForOwner(ctx, "agent", agentID)
	if err != nil {
		writeMeInternalError(w, "Failed to fetch badges")
		return
	}

	writeMeJSON(w, http.StatusOK, BadgesResponse{Badges: badges})
}

// GetUserBadges handles GET /v1/users/{id}/badges.
// Returns all badges for the specified user. No auth required.
func (h *MeHandler) GetUserBadges(w http.ResponseWriter, r *http.Request, userID string) {
	ctx := r.Context()

	if h.badgeRepo == nil {
		writeMeJSON(w, http.StatusOK, BadgesResponse{Badges: []models.Badge{}})
		return
	}

	badges, err := h.badgeRepo.ListForOwner(ctx, "human", userID)
	if err != nil {
		writeMeInternalError(w, "Failed to fetch badges")
		return
	}

	writeMeJSON(w, http.StatusOK, BadgesResponse{Badges: badges})
}
