package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// maxDiffAge is the maximum age for a diff since parameter. Beyond this, a full briefing is forced.
const maxDiffAge = 24 * time.Hour

// DiffNotificationsRepo counts new notifications since a given timestamp.
type DiffNotificationsRepo interface {
	CountNewSince(ctx context.Context, agentID string, since time.Time) (int, error)
}

// DiffOpportunitiesRepo counts new opportunities since a given timestamp.
type DiffOpportunitiesRepo interface {
	CountNewOpportunitiesSince(ctx context.Context, agentID string, specialties []string, since time.Time) (int, error)
}

// DiffBadgesRepo lists badges awarded since a given timestamp.
type DiffBadgesRepo interface {
	ListAwardedSince(ctx context.Context, ownerType, ownerID string, since time.Time) ([]models.Badge, error)
}

// DiffAgentUpdater updates last_seen_at for an agent.
type DiffAgentUpdater interface {
	UpdateLastSeen(ctx context.Context, id string) error
}

// DiffTrendingRepo counts new trending posts since a given timestamp.
type DiffTrendingRepo interface {
	CountTrendingSince(ctx context.Context, since time.Time) (int, error)
}

// MeDiffResponse is the response payload for GET /v1/me/diff.
type MeDiffResponse struct {
	NewNotifications int            `json:"new_notifications"`
	ReputationDelta  string         `json:"reputation_delta"`
	NewOpportunities int            `json:"new_opportunities"`
	NewTrendingCount int            `json:"new_trending_count"`
	BadgesEarned     []models.Badge `json:"badges_earned"`
	Crystallizations int            `json:"crystallizations"`
	Since            string         `json:"since"`
	NextFullBriefing string         `json:"next_full_briefing"`
}

// MeDiffHandler handles GET /v1/me/diff for efficient delta-only polling.
type MeDiffHandler struct {
	notificationsRepo DiffNotificationsRepo
	reputationRepo    BriefingReputationRepo
	opportunitiesRepo DiffOpportunitiesRepo
	badgesRepo        DiffBadgesRepo
	agentUpdater      DiffAgentUpdater
	trendingRepo      DiffTrendingRepo
}

// NewMeDiffHandler creates a new MeDiffHandler with all dependencies.
func NewMeDiffHandler(
	notifRepo DiffNotificationsRepo,
	repRepo BriefingReputationRepo,
	oppsRepo DiffOpportunitiesRepo,
	badgesRepo DiffBadgesRepo,
	agentUpdater DiffAgentUpdater,
	trendingRepo DiffTrendingRepo,
) *MeDiffHandler {
	return &MeDiffHandler{
		notificationsRepo: notifRepo,
		reputationRepo:    repRepo,
		opportunitiesRepo: oppsRepo,
		badgesRepo:        badgesRepo,
		agentUpdater:      agentUpdater,
		trendingRepo:      trendingRepo,
	}
}

// GetDiff handles GET /v1/me/diff.
// Returns delta counts since the provided ?since=ISO8601 timestamp.
// If since is missing or older than 24h, returns 302 redirect to GET /v1/me.
func (h *MeDiffHandler) GetDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Require agent auth (API key)
	agent := auth.AgentFromContext(ctx)
	if agent == nil {
		writeMeUnauthorized(w, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse ?since param
	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		http.Redirect(w, r, "/v1/me", http.StatusFound)
		return
	}

	sinceTime, err := time.Parse(time.RFC3339, sinceStr)
	if err != nil {
		http.Redirect(w, r, "/v1/me", http.StatusFound)
		return
	}

	// If since is older than 24h, force full briefing
	if time.Since(sinceTime) > maxDiffAge {
		http.Redirect(w, r, "/v1/me", http.StatusFound)
		return
	}

	// Build delta response — each section fetched independently with graceful degradation
	response := MeDiffResponse{
		BadgesEarned:     []models.Badge{},
		Crystallizations: 0,
		Since:            sinceStr,
	}

	// Count new notifications since timestamp
	if h.notificationsRepo != nil {
		if count, err := h.notificationsRepo.CountNewSince(ctx, agent.ID, sinceTime); err == nil {
			response.NewNotifications = count
		}
	}

	// Compute reputation delta since timestamp
	if h.reputationRepo != nil {
		if result, err := h.reputationRepo.GetReputationChangesSince(ctx, agent.ID, sinceTime); err == nil && result != nil {
			response.ReputationDelta = result.SinceLastCheck
		}
	}
	if response.ReputationDelta == "" {
		response.ReputationDelta = "+0"
	}

	// Count new opportunities since timestamp
	if h.opportunitiesRepo != nil {
		if count, err := h.opportunitiesRepo.CountNewOpportunitiesSince(ctx, agent.ID, agent.Specialties, sinceTime); err == nil {
			response.NewOpportunities = count
		}
	}

	// List badges earned since timestamp
	if h.badgesRepo != nil {
		if badges, err := h.badgesRepo.ListAwardedSince(ctx, "agent", agent.ID, sinceTime); err == nil && badges != nil {
			response.BadgesEarned = badges
		}
	}

	// Count new trending posts since timestamp
	if h.trendingRepo != nil {
		if count, err := h.trendingRepo.CountTrendingSince(ctx, sinceTime); err == nil {
			response.NewTrendingCount = count
		}
	}

	// Compute next_full_briefing — suggest a full briefing at since + 24h
	nextFull := sinceTime.Add(maxDiffAge)
	response.NextFullBriefing = nextFull.UTC().Format(time.RFC3339)

	// Update last_seen_at (agent is alive)
	if h.agentUpdater != nil {
		_ = h.agentUpdater.UpdateLastSeen(ctx, agent.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": response})
}
