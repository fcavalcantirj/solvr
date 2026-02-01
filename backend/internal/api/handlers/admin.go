// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// AdminRepositoryInterface defines the data access methods for admin operations.
type AdminRepositoryInterface interface {
	ListFlags(ctx context.Context, opts *models.FlagListOptions) ([]models.Flag, int, error)
	GetFlagByID(ctx context.Context, id string) (*models.Flag, error)
	UpdateFlag(ctx context.Context, flag *models.Flag) error
	CreateAuditLog(ctx context.Context, entry *models.AuditLog) error
	ListUsers(ctx context.Context, opts *models.UserListOptions) ([]models.User, int, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	ListAgents(ctx context.Context, opts *models.AgentListOptions) ([]models.Agent, int, error)
	GetAgentByID(ctx context.Context, id string) (*models.Agent, error)
	UpdateAgent(ctx context.Context, agent *models.Agent) error
	ListAuditLog(ctx context.Context, opts *models.AuditListOptions) ([]models.AuditLog, int, error)
	GetStats(ctx context.Context) (*models.AdminStats, error)
	HardDeletePost(ctx context.Context, id string) error
	RestorePost(ctx context.Context, id string) error
}

// AdminHandler handles admin-related HTTP requests.
type AdminHandler struct {
	repo AdminRepositoryInterface
}

// NewAdminHandler creates a new AdminHandler with the given repository.
func NewAdminHandler(repo AdminRepositoryInterface) *AdminHandler {
	return &AdminHandler{repo: repo}
}

// ListFlags handles GET /v1/admin/flags
func (h *AdminHandler) ListFlags(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	opts := &models.FlagListOptions{
		Status:     r.URL.Query().Get("status"),
		TargetType: r.URL.Query().Get("target_type"),
		Page:       parseIntDefault(r.URL.Query().Get("page"), 1),
		PerPage:    parseIntDefault(r.URL.Query().Get("per_page"), 20),
	}

	flags, total, err := h.repo.ListFlags(r.Context(), opts)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list flags")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data":  flags,
		"total": total,
		"page":  opts.Page,
	})
}

// DismissFlag handles POST /v1/admin/flags/{id}/dismiss
func (h *AdminHandler) DismissFlag(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	flagID := chi.URLParam(r, "id")
	flag, err := h.repo.GetFlagByID(r.Context(), flagID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "flag not found")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	flag.Status = "dismissed"
	flag.ReviewedBy = &adminUUID
	flag.ReviewedAt = &now

	if err := h.repo.UpdateFlag(r.Context(), flag); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update flag")
		return
	}

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "dismiss_flag",
		TargetType: "flag",
		TargetID:   &flag.ID,
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": flag,
	})
}

// ActionOnFlag handles POST /v1/admin/flags/{id}/action
func (h *AdminHandler) ActionOnFlag(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var req models.FlagAction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if !models.IsValidFlagAction(req.Action) {
		writeAdminError(w, http.StatusBadRequest, "INVALID_ACTION", "invalid action")
		return
	}

	flagID := chi.URLParam(r, "id")
	flag, err := h.repo.GetFlagByID(r.Context(), flagID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "flag not found")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	flag.Status = "actioned"
	flag.ReviewedBy = &adminUUID
	flag.ReviewedAt = &now

	if err := h.repo.UpdateFlag(r.Context(), flag); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update flag")
		return
	}

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "action_flag",
		TargetType: "flag",
		TargetID:   &flag.ID,
		Details:    map[string]interface{}{"action": req.Action},
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": flag,
	})
}

// ListUsers handles GET /v1/admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	opts := &models.UserListOptions{
		Query:   r.URL.Query().Get("q"),
		Status:  r.URL.Query().Get("status"),
		Page:    parseIntDefault(r.URL.Query().Get("page"), 1),
		PerPage: parseIntDefault(r.URL.Query().Get("per_page"), 20),
	}

	users, total, err := h.repo.ListUsers(r.Context(), opts)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list users")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data":  users,
		"total": total,
		"page":  opts.Page,
	})
}

// GetUserDetail handles GET /v1/admin/users/{id}
func (h *AdminHandler) GetUserDetail(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	userID := chi.URLParam(r, "id")
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": user,
	})
}

// WarnUser handles POST /v1/admin/users/{id}/warn
func (h *AdminHandler) WarnUser(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID := chi.URLParam(r, "id")
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	targetUUID, _ := uuid.Parse(user.ID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "warn_user",
		TargetType: "user",
		TargetID:   &targetUUID,
		Details:    map[string]interface{}{"message": req.Message},
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"user_id": user.ID,
			"warned":  true,
		},
	})
}

// SuspendUser handles POST /v1/admin/users/{id}/suspend
func (h *AdminHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var req struct {
		Duration string `json:"duration"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID := chi.URLParam(r, "id")
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	user.Status = string(models.UserStatusSuspended)
	if err := h.repo.UpdateUser(r.Context(), user); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to suspend user")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	targetUUID, _ := uuid.Parse(user.ID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "suspend_user",
		TargetType: "user",
		TargetID:   &targetUUID,
		Details:    map[string]interface{}{"duration": req.Duration, "reason": req.Reason},
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": user,
	})
}

// BanUser handles POST /v1/admin/users/{id}/ban
func (h *AdminHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID := chi.URLParam(r, "id")
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	user.Status = string(models.UserStatusBanned)
	if err := h.repo.UpdateUser(r.Context(), user); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to ban user")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	targetUUID, _ := uuid.Parse(user.ID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "ban_user",
		TargetType: "user",
		TargetID:   &targetUUID,
		Details:    map[string]interface{}{"reason": req.Reason},
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": user,
	})
}

// ListAgents handles GET /v1/admin/agents
func (h *AdminHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	opts := &models.AgentListOptions{
		Query:   r.URL.Query().Get("q"),
		Status:  r.URL.Query().Get("status"),
		Page:    parseIntDefault(r.URL.Query().Get("page"), 1),
		PerPage: parseIntDefault(r.URL.Query().Get("per_page"), 20),
	}

	agents, total, err := h.repo.ListAgents(r.Context(), opts)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list agents")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data":  agents,
		"total": total,
		"page":  opts.Page,
	})
}

// RevokeAgentKey handles POST /v1/admin/agents/{id}/revoke-key
func (h *AdminHandler) RevokeAgentKey(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	agentID := chi.URLParam(r, "id")
	agent, err := h.repo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
		return
	}

	// Clear the API key hash to revoke access
	agent.APIKeyHash = ""
	if err := h.repo.UpdateAgent(r.Context(), agent); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke key")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "revoke_agent_key",
		TargetType: "agent",
		Details:    map[string]interface{}{"agent_id": agent.ID},
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"agent_id": agent.ID,
			"revoked":  true,
		},
	})
}

// SuspendAgent handles POST /v1/admin/agents/{id}/suspend
func (h *AdminHandler) SuspendAgent(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	agentID := chi.URLParam(r, "id")
	agent, err := h.repo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
		return
	}

	agent.Status = string(models.AgentStatusSuspended)
	if err := h.repo.UpdateAgent(r.Context(), agent); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to suspend agent")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "suspend_agent",
		TargetType: "agent",
		Details:    map[string]interface{}{"agent_id": agent.ID, "reason": req.Reason},
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": agent,
	})
}

// ListAuditLog handles GET /v1/admin/audit
func (h *AdminHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	opts := &models.AuditListOptions{
		Action:  r.URL.Query().Get("action"),
		Page:    parseIntDefault(r.URL.Query().Get("page"), 1),
		PerPage: parseIntDefault(r.URL.Query().Get("per_page"), 20),
	}

	// Parse from_date if provided
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		if t, err := time.Parse("2006-01-02", fromDate); err == nil {
			opts.FromDate = &t
		}
	}

	// Parse to_date if provided
	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		if t, err := time.Parse("2006-01-02", toDate); err == nil {
			opts.ToDate = &t
		}
	}

	entries, total, err := h.repo.ListAuditLog(r.Context(), opts)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list audit log")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data":  entries,
		"total": total,
		"page":  opts.Page,
	})
}

// GetStats handles GET /v1/admin/stats
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	stats, err := h.repo.GetStats(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get stats")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": stats,
	})
}

// HardDeletePost handles DELETE /v1/admin/posts/{id}
func (h *AdminHandler) HardDeletePost(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	postID := chi.URLParam(r, "id")
	if err := h.repo.HardDeletePost(r.Context(), postID); err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	postUUID, _ := uuid.Parse(postID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "hard_delete_post",
		TargetType: "post",
		TargetID:   &postUUID,
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	w.WriteHeader(http.StatusNoContent)
}

// RestorePost handles POST /v1/admin/posts/{id}/restore
func (h *AdminHandler) RestorePost(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if !isAdminOrAbove(claims.Role) {
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	postID := chi.URLParam(r, "id")
	if err := h.repo.RestorePost(r.Context(), postID); err != nil {
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		return
	}

	now := time.Now()
	adminUUID, _ := uuid.Parse(claims.UserID)
	postUUID, _ := uuid.Parse(postID)

	// Create audit log entry
	auditEntry := &models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminUUID,
		Action:     "restore_post",
		TargetType: "post",
		TargetID:   &postUUID,
		CreatedAt:  now,
	}
	h.repo.CreateAuditLog(r.Context(), auditEntry)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"post_id":  postID,
			"restored": true,
		},
	})
}

// Helper functions

func writeAdminJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func isAdminOrAbove(role string) bool {
	return role == "admin" || role == "super_admin"
}

func parseIntDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultVal
}
