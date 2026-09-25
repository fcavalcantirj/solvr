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

	// The list answers what GET /v1/agents/{id} answers for an agent that does not exist.
	if h.agentFinderRepo != nil {
		if _, err := h.agentFinderRepo.FindByID(ctx, agentID); err != nil {
			if isAgentNotFound(err) {
				writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
				return
			}
			writeMeInternalError(w, "Failed to fetch agent")
			return
		}
	}

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

	// The list answers what GET /v1/users/{id} answers for an id that names no user.
	if h.userRepo != nil && findPublicUser(ctx, w, h.userRepo, userID) == nil {
		return
	}

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
