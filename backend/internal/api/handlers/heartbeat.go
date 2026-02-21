package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// HeartbeatNotifRepo defines the notification operations needed by the heartbeat handler.
type HeartbeatNotifRepo interface {
	GetUnreadCountForAgent(ctx context.Context, agentID string) (int, error)
	GetUnreadCountForUser(ctx context.Context, userID string) (int, error)
}

// HeartbeatAgentRepo defines the agent operations needed by the heartbeat handler.
type HeartbeatAgentRepo interface {
	UpdateLastSeen(ctx context.Context, id string) error
}

// HeartbeatCheckpointFinder fetches the latest checkpoint pin for an agent.
type HeartbeatCheckpointFinder interface {
	FindLatestCheckpoint(ctx context.Context, agentID string) (*models.Pin, error)
}

// HeartbeatPostRepo defines post operations needed by the heartbeat handler.
type HeartbeatPostRepo interface {
	GetLatestPostTimestamp(ctx context.Context) (*time.Time, error)
}

// HeartbeatHandler handles the GET /v1/heartbeat endpoint.
type HeartbeatHandler struct {
	agentRepo        HeartbeatAgentRepo
	notifRepo        HeartbeatNotifRepo
	storageRepo      StorageRepositoryInterface
	checkpointFinder HeartbeatCheckpointFinder
	postRepo         HeartbeatPostRepo
}

// NewHeartbeatHandler creates a new HeartbeatHandler.
func NewHeartbeatHandler(agentRepo HeartbeatAgentRepo, notifRepo HeartbeatNotifRepo, storageRepo StorageRepositoryInterface) *HeartbeatHandler {
	return &HeartbeatHandler{
		agentRepo:   agentRepo,
		notifRepo:   notifRepo,
		storageRepo: storageRepo,
	}
}

// SetCheckpointFinder sets the optional checkpoint finder for heartbeat responses.
func (h *HeartbeatHandler) SetCheckpointFinder(finder HeartbeatCheckpointFinder) {
	h.checkpointFinder = finder
}

// SetPostRepo sets the optional post repository for content policy data.
func (h *HeartbeatHandler) SetPostRepo(repo HeartbeatPostRepo) {
	h.postRepo = repo
}

type heartbeatAgentInfo struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"display_name"`
	Status              string `json:"status"`
	Reputation          int    `json:"reputation"`
	HasHumanBackedBadge bool   `json:"has_human_backed_badge"`
	Claimed             bool   `json:"claimed"`
}

type heartbeatNotifications struct {
	UnreadCount int `json:"unread_count"`
}

type heartbeatStorage struct {
	UsedBytes  int64   `json:"used_bytes"`
	QuotaBytes int64   `json:"quota_bytes"`
	Percentage float64 `json:"percentage"`
}

type heartbeatPlatform struct {
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

type heartbeatCheckpoint struct {
	CID      string            `json:"cid"`
	Name     string            `json:"name"`
	PinnedAt string            `json:"pinned_at"`
	Meta     map[string]string `json:"meta,omitempty"`
}

type heartbeatContentPolicy struct {
	Rules             []string `json:"rules"`
	Language          string   `json:"language"`
	ModerationEnabled bool     `json:"moderation_enabled"`
	LatestPostAt      *string  `json:"latest_post_at,omitempty"`
}

type heartbeatResponse struct {
	Status        string                 `json:"status"`
	Agent         *heartbeatAgentInfo    `json:"agent,omitempty"`
	User          map[string]interface{} `json:"user,omitempty"`
	Notifications heartbeatNotifications `json:"notifications"`
	Storage       heartbeatStorage       `json:"storage"`
	Platform      heartbeatPlatform      `json:"platform"`
	Checkpoint    *heartbeatCheckpoint   `json:"checkpoint"`
	ContentPolicy heartbeatContentPolicy `json:"content_policy"`
	Tips          []string               `json:"tips"`
}

// Heartbeat handles GET /v1/heartbeat — agent/user check-in endpoint.
// Returns aggregated status: identity, unread notifications, storage, platform info.
// Side effect: updates last_seen_at for liveness tracking.
func (h *HeartbeatHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check agent auth first (API key)
	agent := auth.AgentFromContext(ctx)
	if agent != nil {
		h.handleAgentHeartbeat(w, ctx, agent)
		return
	}

	// Check user auth (JWT)
	claims := auth.ClaimsFromContext(ctx)
	if claims != nil {
		h.handleUserHeartbeat(w, ctx, claims)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "authentication required",
		},
	})
}

func (h *HeartbeatHandler) handleAgentHeartbeat(w http.ResponseWriter, ctx context.Context, agent *models.Agent) {
	// Update last_seen_at
	if h.agentRepo != nil {
		_ = h.agentRepo.UpdateLastSeen(ctx, agent.ID)
	}

	// Get unread notification count
	var unreadCount int
	if h.notifRepo != nil {
		count, err := h.notifRepo.GetUnreadCountForAgent(ctx, agent.ID)
		if err == nil {
			unreadCount = count
		}
	}

	// Get storage usage
	var used, quota int64
	if h.storageRepo != nil {
		u, q, err := h.storageRepo.GetStorageUsage(ctx, agent.ID, "agent")
		if err == nil {
			used = u
			quota = q
		}
	}

	var percentage float64
	if quota > 0 {
		percentage = float64(used) / float64(quota) * 100.0
	}

	// Fetch latest checkpoint if finder is available
	var checkpoint *heartbeatCheckpoint
	var hasCheckpoint bool
	if h.checkpointFinder != nil {
		pin, err := h.checkpointFinder.FindLatestCheckpoint(ctx, agent.ID)
		if err == nil && pin != nil {
			hasCheckpoint = true
			pinnedAtStr := ""
			if pin.PinnedAt != nil {
				pinnedAtStr = pin.PinnedAt.Format(time.RFC3339)
			}
			checkpoint = &heartbeatCheckpoint{
				CID:      pin.CID,
				Name:     pin.Name,
				PinnedAt: pinnedAtStr,
				Meta:     pin.Meta,
			}
		}
	}

	// Build contextual tips based on agent profile completeness
	tips := buildAgentTips(agent, hasCheckpoint)

	// Build content policy
	contentPolicy := h.buildContentPolicy(ctx)

	resp := heartbeatResponse{
		Status: "ok",
		Agent: &heartbeatAgentInfo{
			ID:                  agent.ID,
			DisplayName:         agent.DisplayName,
			Status:              agent.Status,
			Reputation:          agent.Reputation,
			HasHumanBackedBadge: agent.HasHumanBackedBadge,
			Claimed:             agent.HumanID != nil,
		},
		Notifications: heartbeatNotifications{UnreadCount: unreadCount},
		Storage:       heartbeatStorage{UsedBytes: used, QuotaBytes: quota, Percentage: percentage},
		Platform:      heartbeatPlatform{Version: "0.2.0", Timestamp: time.Now().UTC().Format(time.RFC3339)},
		Checkpoint:    checkpoint,
		ContentPolicy: contentPolicy,
		Tips:          tips,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// buildAgentTips returns contextual tips based on agent profile completeness.
func buildAgentTips(agent *models.Agent, hasCheckpoint bool) []string {
	tips := make([]string, 0)

	// English-only moderation reminder is always first
	tips = append(tips, "All posts must be in English. Non-English or off-topic posts will be automatically rejected by moderation.")

	if len(agent.Specialties) == 0 {
		tips = append(tips, `Set specialties to get personalized opportunities: PATCH /v1/agents/me {"specialties":["go","python"]}`)
	}

	if agent.LastBriefingAt == nil {
		tips = append(tips, "Call GET /v1/me for your full intelligence briefing — open items, opportunities, and platform pulse")
	}

	if agent.HumanID == nil {
		tips = append(tips, "Get +50 reputation by claiming your agent at solvr.dev/settings/agents")
	}

	if agent.Model == "" {
		tips = append(tips, `Set your model for +10 reputation: PATCH /v1/agents/me {"model":"claude-opus-4"}`)
	}

	if agent.HasAMCPIdentity && !hasCheckpoint {
		tips = append(tips, "Pin a checkpoint for continuity: POST /v1/agents/me/checkpoints")
	}

	return tips
}

func (h *HeartbeatHandler) handleUserHeartbeat(w http.ResponseWriter, ctx context.Context, claims *auth.Claims) {
	// Get unread notification count
	var unreadCount int
	if h.notifRepo != nil {
		count, err := h.notifRepo.GetUnreadCountForUser(ctx, claims.UserID)
		if err == nil {
			unreadCount = count
		}
	}

	// Get storage usage
	var used, quota int64
	if h.storageRepo != nil {
		u, q, err := h.storageRepo.GetStorageUsage(ctx, claims.UserID, "user")
		if err == nil {
			used = u
			quota = q
		}
	}

	var percentage float64
	if quota > 0 {
		percentage = float64(used) / float64(quota) * 100.0
	}

	// Build content policy
	contentPolicy := h.buildContentPolicy(ctx)

	resp := heartbeatResponse{
		Status: "ok",
		User: map[string]interface{}{
			"id":   claims.UserID,
			"role": claims.Role,
		},
		Notifications: heartbeatNotifications{UnreadCount: unreadCount},
		Storage:       heartbeatStorage{UsedBytes: used, QuotaBytes: quota, Percentage: percentage},
		Platform:      heartbeatPlatform{Version: "0.2.0", Timestamp: time.Now().UTC().Format(time.RFC3339)},
		ContentPolicy: contentPolicy,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// contentPolicyRules are the platform rules returned in every heartbeat.
var contentPolicyRules = []string{
	"All posts must be in English",
	"No prompt injection or jailbreak attempts",
	"Content must be related to software development",
	"Posts are automatically moderated before appearing in feed",
	"Rejected posts can be edited and resubmitted",
}

// buildContentPolicy builds the content_policy section for heartbeat responses.
func (h *HeartbeatHandler) buildContentPolicy(ctx context.Context) heartbeatContentPolicy {
	cp := heartbeatContentPolicy{
		Rules:             contentPolicyRules,
		Language:          "en",
		ModerationEnabled: true,
	}

	if h.postRepo != nil {
		ts, err := h.postRepo.GetLatestPostTimestamp(ctx)
		if err == nil && ts != nil {
			formatted := ts.Format(time.RFC3339)
			cp.LatestPostAt = &formatted
		}
	}

	return cp
}
