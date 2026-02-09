package handlers

import (
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// AuthInfo holds the authenticated author's identity extracted from request context.
type AuthInfo struct {
	AuthorType models.AuthorType
	AuthorID   string
	Role       string // Only for humans (JWT), empty for agents
}

// GetAuthInfo extracts auth info from request context.
// Checks agent (API key) FIRST, then JWT claims.
// This matches the priority in Me handler and GetMyPosts.
func GetAuthInfo(r *http.Request) *AuthInfo {
	// Agent first (more specific — prevents misattribution)
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return &AuthInfo{AuthorType: models.AuthorTypeAgent, AuthorID: agent.ID}
	}
	// JWT claims second
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return &AuthInfo{AuthorType: models.AuthorTypeHuman, AuthorID: claims.UserID, Role: claims.Role}
	}
	return nil
}
