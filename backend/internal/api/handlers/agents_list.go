package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ActivityResponse is the response structure for the activity endpoint.
type ActivityResponse struct {
	Data []models.ActivityItem `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// GetActivity handles GET /v1/agents/:id/activity - activity history.
// Per SPEC.md Part 4.9 and Part 5.6.
func (h *AgentsHandler) GetActivity(w http.ResponseWriter, r *http.Request, agentID string) {
	// Parse pagination parameters with defaults per SPEC.md Part 5.6
	page := 1
	perPage := 20
	maxPerPage := 50

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 {
			perPage = parsed
		}
	}

	// Cap per_page at max per SPEC.md Part 5.6
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	// Get activity from repository
	activities, total, err := h.repo.GetActivity(r.Context(), agentID, page, perPage)
	if err != nil {
		// FIX-026: Check for both local ErrAgentNotFound (mock) and db.ErrAgentNotFound (real DB)
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get activity")
		return
	}

	// Calculate has_more
	hasMore := page*perPage < total

	// Build response
	resp := ActivityResponse{}
	resp.Data = activities
	if resp.Data == nil {
		resp.Data = []models.ActivityItem{} // Ensure empty array, not null
	}
	resp.Meta.Total = total
	resp.Meta.Page = page
	resp.Meta.PerPage = perPage
	resp.Meta.HasMore = hasMore

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// AgentsListResponse is the response for listing agents.
// Per API-001: GET /v1/agents.
type AgentsListResponse struct {
	Data []models.AgentWithPostCount `json:"data"`
	Meta AgentsListMeta              `json:"meta"`
}

// AgentsListMeta contains metadata for agent list responses.
type AgentsListMeta struct {
	Total            int  `json:"total"`
	Page             int  `json:"page"`
	PerPage          int  `json:"per_page"`
	HasMore          bool `json:"has_more"`
	ActiveCount      int  `json:"active_count"`
	HumanBackedCount int  `json:"human_backed_count"`
}

// ListAgents handles GET /v1/agents - list registered agents.
// Per API-001: Returns paginated list of agents with post counts.
// Supports sorting by: newest, oldest, reputation, posts.
// Supports filtering by status: active, pending, or all.
func (h *AgentsHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	page := 1
	perPage := 20
	maxPerPage := 100

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 {
			perPage = parsed
		}
	}

	// Also support "limit" and "offset" as per API-001 spec
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if parsed, err := strconv.Atoi(limit); err == nil && parsed > 0 {
			perPage = parsed
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if parsed, err := strconv.Atoi(offset); err == nil && parsed >= 0 {
			// Convert offset to page number
			page = (parsed / perPage) + 1
		}
	}

	// Cap per_page at max
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	// Parse filters
	opts := models.AgentListOptions{
		Page:    page,
		PerPage: perPage,
		Status:  r.URL.Query().Get("status"),
		Sort:    r.URL.Query().Get("sort"),
	}

	// Validate sort option
	validSorts := map[string]bool{"newest": true, "oldest": true, "reputation": true, "posts": true, "": true}
	if !validSorts[opts.Sort] {
		writeAgentValidationError(w, "sort must be one of: newest, oldest, reputation, posts")
		return
	}

	// Validate status option
	validStatuses := map[string]bool{"active": true, "pending": true, "all": true, "": true}
	if !validStatuses[opts.Status] {
		writeAgentValidationError(w, "status must be one of: active, pending, all")
		return
	}

	// Fetch agents
	agents, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list agents")
		return
	}

	// Fetch stats for meta
	activeCount, err := h.repo.CountActive(r.Context())
	if err != nil {
		// Log error but continue with 0
		activeCount = 0
	}
	humanBackedCount, err := h.repo.CountHumanBacked(r.Context())
	if err != nil {
		// Log error but continue with 0
		humanBackedCount = 0
	}

	// Calculate has_more
	hasMore := page*perPage < total

	// Build response
	resp := AgentsListResponse{
		Data: agents,
		Meta: AgentsListMeta{
			Total:            total,
			Page:             page,
			PerPage:          perPage,
			HasMore:          hasMore,
			ActiveCount:      activeCount,
			HumanBackedCount: humanBackedCount,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
