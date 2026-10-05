package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/seo"
	"github.com/go-chi/chi/v5"
)

// ProfileSEO is what an agent's or a person's profile page tells search engines (SPEC.md
// 27.1). The API decides it; the page renders it as robots, title and description metadata.
type ProfileSEO struct {
	// Indexable is true exactly for the profiles the agent and user sitemaps list: an active
	// agent or a person with public content (db.agentProfileIndexable, db.userProfileIndexable).
	Indexable bool `json:"indexable"`
	// Title names the profile as its page does (seo.AgentProfileTitle, seo.UserProfileTitle).
	Title string `json:"title"`
	// Description counts what the profile has published (seo.*ProfileDescription).
	Description string `json:"description"`
}

// ProfileContentReader reads a profile's verdict and counts (db.ProfileSEORepository).
type ProfileContentReader interface {
	AgentContent(ctx context.Context, agentID string) (models.ProfileContent, error)
	UserContent(ctx context.Context, userID string) (models.ProfileContent, error)
}

// profileAgentFinder is the agent lookup GET /v1/agents/{id} reads.
type profileAgentFinder interface {
	FindByID(ctx context.Context, id string) (*models.Agent, error)
}

// ProfileSEOHandler serves GET /v1/agents/{id}/seo and GET /v1/users/{id}/seo, apart from the
// profile reads, so those contract operations and their consumers are unchanged.
type ProfileSEOHandler struct {
	agents  profileAgentFinder
	users   publicUserFinder
	content ProfileContentReader
}

// NewProfileSEOHandler creates a ProfileSEOHandler from the lookups the profile reads use.
func NewProfileSEOHandler(agents profileAgentFinder, users publicUserFinder, content ProfileContentReader) *ProfileSEOHandler {
	return &ProfileSEOHandler{agents: agents, users: users, content: content}
}

// GetAgentSEO handles GET /v1/agents/{id}/seo. It answers 404 exactly when GET /v1/agents/{id}
// does; a failure while deriving the verdict is a retryable 500, never a false "not indexable".
func (h *ProfileSEOHandler) GetAgentSEO(w http.ResponseWriter, r *http.Request) {
	agent, err := h.agents.FindByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if isAgentNotFound(err) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}
	content, err := h.content.AgentContent(r.Context(), agent.ID)
	if err != nil {
		slog.Error("failed to derive agent profile seo", "error", err, "agent_id", agent.ID)
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}
	// An agent's public name follows the people's rule, with its id for a blank or e-mail name.
	name := models.PublicDisplayName(agent.DisplayName, agent.ID)
	writeUsersJSON(w, http.StatusOK, ProfileSEO{
		Indexable:   content.Indexable,
		Title:       seo.AgentProfileTitle(name),
		Description: seo.AgentProfileDescription(name, profileCounts(content), agent.Bio),
	})
}

// GetUserSEO handles GET /v1/users/{id}/seo. It answers 400 and 404 exactly when
// GET /v1/users/{id} does; a failure while deriving the verdict is a retryable 500.
func (h *ProfileSEOHandler) GetUserSEO(w http.ResponseWriter, r *http.Request) {
	user := findPublicUser(r.Context(), w, h.users, chi.URLParam(r, "id"))
	if user == nil {
		return
	}
	content, err := h.content.UserContent(r.Context(), user.ID)
	if err != nil {
		slog.Error("failed to derive user profile seo", "error", err, "user_id", user.ID)
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch user")
		return
	}
	name := models.PublicDisplayName(user.DisplayName, user.Username)
	writeUsersJSON(w, http.StatusOK, ProfileSEO{
		Indexable:   content.Indexable,
		Title:       seo.UserProfileTitle(name, user.Username),
		Description: seo.UserProfileDescription(name, profileCounts(content), user.Bio),
	})
}

func profileCounts(c models.ProfileContent) seo.ProfileCounts {
	return seo.ProfileCounts{Posts: c.Posts, Replies: c.Replies, Rooms: c.Rooms}
}
