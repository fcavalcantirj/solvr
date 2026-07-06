package handlers

import (
	"context"
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

// callerHumanID returns the caller's family human UUID for BART-151 visibility scoping:
// a claimed agent's human_id, or a human user's id. Returns "" for anonymous callers,
// unclaimed agents, and the auth-less MCP path — all of which map to public-only.
// Note: this is NOT GetAuthInfo().AuthorID for agents (that returns the agent id, not the human).
func callerHumanID(r *http.Request) string {
	return callerHumanFromCtx(r.Context())
}

// callerHumanFromCtx is the context-based form of callerHumanID, so shared helpers that
// only carry a context (e.g. findQuestion/findIdea/findProblem) can scope by family too.
func callerHumanFromCtx(ctx context.Context) string {
	if agent := auth.AgentFromContext(ctx); agent != nil {
		if agent.HumanID != nil {
			return *agent.HumanID
		}
		return ""
	}
	if claims := auth.ClaimsFromContext(ctx); claims != nil {
		return claims.UserID
	}
	return ""
}
