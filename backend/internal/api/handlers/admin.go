// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// TranslationJobRunner is an interface for triggering the translation job.
// Using an interface here avoids an import cycle: services/oauth_adapter.go imports handlers,
// so handlers cannot import services or jobs.
type TranslationJobRunner interface {
	RunOnce(ctx context.Context) (translated, failed int)
}

// EmailSender sends a single email. Implemented by services.ResendClient.
type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string, headers ...map[string]string) error
}

// EmailBroadcastRepo persists broadcast log entries.
type EmailBroadcastRepo interface {
	CreateLog(ctx context.Context, broadcast *models.EmailBroadcast) (*models.EmailBroadcast, error)
	UpdateStatusAndCounts(ctx context.Context, id string, status string, sentCount, failedCount int, completedAt *time.Time) error
}

// UserEmailRepo provides user email listing for broadcasts.
type UserEmailRepo interface {
	ListActiveEmails(ctx context.Context) ([]models.EmailRecipient, error)
}

// AdminHandler handles admin operations like raw SQL queries.
type AdminHandler struct {
	pool                 *db.Pool
	translationJobRunner TranslationJobRunner
	emailSender          EmailSender
	emailBroadcastRepo   EmailBroadcastRepo
	userEmailRepo        UserEmailRepo
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(pool *db.Pool) *AdminHandler {
	return &AdminHandler{pool: pool}
}

// SetTranslationJobRunner injects the translation job runner dependency.
func (h *AdminHandler) SetTranslationJobRunner(runner TranslationJobRunner) {
	h.translationJobRunner = runner
}

// SetEmailSender injects the email sender dependency.
func (h *AdminHandler) SetEmailSender(sender EmailSender) {
	h.emailSender = sender
}

// SetEmailBroadcastRepo injects the email broadcast repository dependency.
func (h *AdminHandler) SetEmailBroadcastRepo(repo EmailBroadcastRepo) {
	h.emailBroadcastRepo = repo
}

// SetUserEmailRepo injects the user email repository dependency.
func (h *AdminHandler) SetUserEmailRepo(repo UserEmailRepo) {
	h.userEmailRepo = repo
}

// broadcastRequest is the JSON body for POST /admin/email/broadcast.
type broadcastRequest struct {
	Subject  string `json:"subject"`
	BodyHTML string `json:"body_html"`
	BodyText string `json:"body_text"`
	DryRun   bool   `json:"dry_run"`
}

// BroadcastEmail handles POST /admin/email/broadcast
// Sends email to all active users, or previews recipients in dry-run mode.
// Requires X-Admin-API-Key header.
// Returns 400 if subject or body_html is missing.
// Returns 503 if email service is not configured.
// With dry_run=true, returns recipient list without sending.
// With dry_run=false, sends emails synchronously and returns counts.
func (h *AdminHandler) BroadcastEmail(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	// Parse request body
	var req broadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Subject) == "" {
		writeAdminError(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELD", "subject is required")
		return
	}
	if strings.TrimSpace(req.BodyHTML) == "" {
		writeAdminError(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELD", "body_html is required")
		return
	}

	// Check email service configured
	if h.emailSender == nil {
		writeAdminJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "EMAIL_NOT_CONFIGURED",
		})
		return
	}

	// Get recipients
	recipients, err := h.userEmailRepo.ListActiveEmails(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list recipients")
		return
	}

	// Dry-run mode: return recipient list, no sends
	if req.DryRun {
		writeAdminJSON(w, http.StatusOK, map[string]interface{}{
			"would_send": len(recipients),
			"recipients": recipients,
		})
		return
	}

	// Live broadcast: create log, send emails, update log
	// Per-request 5-minute context deadline
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Create broadcast log entry
	broadcast := &models.EmailBroadcast{
		Subject:         req.Subject,
		BodyHTML:        req.BodyHTML,
		BodyText:        req.BodyText,
		TotalRecipients: len(recipients),
		Status:          "sending",
	}

	logEntry, err := h.emailBroadcastRepo.CreateLog(ctx, broadcast)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create broadcast log")
		return
	}

	startTime := time.Now()
	sentCount := 0
	failedCount := 0

	// List-Unsubscribe header for Gmail 2024 bulk sender requirements
	unsubHeaders := map[string]string{
		"List-Unsubscribe": "<mailto:unsubscribe@solvr.dev>",
	}

	// Send emails synchronously and sequentially
	for _, recipient := range recipients {
		err := h.emailSender.Send(ctx, recipient.Email, req.Subject, req.BodyHTML, req.BodyText, unsubHeaders)
		if err != nil {
			failedCount++
			// Log error but continue with next recipient
			continue
		}
		sentCount++
	}

	// Update broadcast log with final counts
	completedAt := time.Now()
	durationMs := completedAt.Sub(startTime).Milliseconds()

	_ = h.emailBroadcastRepo.UpdateStatusAndCounts(ctx, logEntry.ID, "completed", sentCount, failedCount, &completedAt)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"broadcast_id": logEntry.ID,
		"sent":         sentCount,
		"failed":       failedCount,
		"total":        len(recipients),
		"duration_ms":  durationMs,
	})
}

// QueryRequest represents a raw SQL query request.
type QueryRequest struct {
	Query string `json:"query"`
}

// QueryResponse represents the response from a raw SQL query.
type QueryResponse struct {
	Columns []string                 `json:"columns,omitempty"`
	Rows    []map[string]interface{} `json:"rows,omitempty"`
	Message string                   `json:"message,omitempty"`
	Error   string                   `json:"error,omitempty"`
}

// destructivePattern matches potentially destructive SQL operations
var destructivePattern = regexp.MustCompile(`(?i)^\s*(DROP|DELETE|TRUNCATE|ALTER|UPDATE|INSERT|CREATE|GRANT|REVOKE)`)

// ExecuteQuery handles POST /admin/query
// Requires X-Admin-API-Key header matching ADMIN_API_KEY env var.
// Respects DESTRUCTIVE_QUERIES env var (default: false) to allow/deny destructive operations.
func (h *AdminHandler) ExecuteQuery(w http.ResponseWriter, r *http.Request) {
	// Check admin API key
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		writeAdminError(w, http.StatusServiceUnavailable, "ADMIN_NOT_CONFIGURED", "admin API key not configured")
		return
	}

	providedKey := r.Header.Get("X-Admin-API-Key")
	if providedKey == "" {
		writeAdminError(w, http.StatusUnauthorized, "MISSING_API_KEY", "X-Admin-API-Key header required")
		return
	}

	if providedKey != adminKey {
		writeAdminError(w, http.StatusForbidden, "INVALID_API_KEY", "invalid admin API key")
		return
	}

	// Check database connection
	if h.pool == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database not connected")
		return
	}

	// Parse request body
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		writeAdminError(w, http.StatusBadRequest, "EMPTY_QUERY", "query cannot be empty")
		return
	}

	// Check for destructive queries
	destructiveAllowed := strings.ToLower(os.Getenv("DESTRUCTIVE_QUERIES")) == "true"
	if !destructiveAllowed && destructivePattern.MatchString(req.Query) {
		writeAdminError(w, http.StatusForbidden, "DESTRUCTIVE_QUERY_BLOCKED",
			"destructive queries not allowed (set DESTRUCTIVE_QUERIES=true to enable)")
		return
	}

	// Execute query
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Check if this looks like a SELECT query (should return rows)
	trimmedQuery := strings.TrimSpace(strings.ToUpper(req.Query))
	isSelect := strings.HasPrefix(trimmedQuery, "SELECT")

	if isSelect {
		// Use Query for SELECT statements
		rows, err := h.pool.Query(ctx, req.Query)
		if err != nil {
			writeAdminJSON(w, http.StatusOK, QueryResponse{
				Error: err.Error(),
			})
			return
		}
		defer rows.Close()

		// Get column names
		fieldDescs := rows.FieldDescriptions()
		columns := make([]string, len(fieldDescs))
		for i, fd := range fieldDescs {
			columns[i] = string(fd.Name)
		}

		// Collect rows
		var results []map[string]interface{}
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				writeAdminJSON(w, http.StatusOK, QueryResponse{
					Error: err.Error(),
				})
				return
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				row[col] = values[i]
			}
			results = append(results, row)
		}

		if err := rows.Err(); err != nil {
			writeAdminJSON(w, http.StatusOK, QueryResponse{
				Error: err.Error(),
			})
			return
		}

		writeAdminJSON(w, http.StatusOK, QueryResponse{
			Columns: columns,
			Rows:    results,
		})
		return
	}

	// For non-SELECT queries, use Exec which supports multiple statements
	_, err := h.pool.Exec(ctx, req.Query)
	if err != nil {
		writeAdminJSON(w, http.StatusOK, QueryResponse{
			Error: err.Error(),
		})
		return
	}

	writeAdminJSON(w, http.StatusOK, QueryResponse{
		Message: "query executed successfully",
	})
}

func writeAdminJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// RunTranslationJob handles POST /admin/jobs/translation/run
// Manually triggers one batch of the translation job. Returns translated/failed counts.
// The runner is injected via SetTranslationJobRunner (wired in router.go).
// Returns 503 if not configured (no GROQ key or no database).
func (h *AdminHandler) RunTranslationJob(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	if h.translationJobRunner == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "TRANSLATION_NOT_CONFIGURED", "translation job not configured (GROQ_API_KEY may be missing)")
		return
	}

	translated, failed := h.translationJobRunner.RunOnce(r.Context())

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"translated": translated,
		"failed":     failed,
	})
}

// checkAdminAuth validates the X-Admin-API-Key header.
// Returns true if authorized, false otherwise (and writes error response).
func (h *AdminHandler) checkAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		writeAdminError(w, http.StatusServiceUnavailable, "ADMIN_NOT_CONFIGURED", "admin API key not configured")
		return false
	}

	providedKey := r.Header.Get("X-Admin-API-Key")
	if providedKey == "" {
		writeAdminError(w, http.StatusUnauthorized, "MISSING_API_KEY", "X-Admin-API-Key header required")
		return false
	}

	if providedKey != adminKey {
		writeAdminError(w, http.StatusForbidden, "INVALID_API_KEY", "invalid admin API key")
		return false
	}

	return true
}

// HardDeleteUser permanently deletes a user (admin-only).
// Per PRD-v5 Task 17: Admin hard-delete endpoints.
// DELETE /admin/users/{id}
func (h *AdminHandler) HardDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	// Parse user ID from path
	userID := chi.URLParam(r, "id")
	if userID == "" {
		writeAdminError(w, http.StatusBadRequest, "MISSING_ID", "user ID required")
		return
	}

	// Execute hard delete
	userRepo := db.NewUserRepository(h.pool)
	err := userRepo.HardDelete(r.Context(), userID)
	if err != nil {
		if err == db.ErrNotFound {
			writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete user")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"message": "User permanently deleted",
		"id":      userID,
	})
}

// HardDeleteAgent permanently deletes an agent (admin-only).
// Per PRD-v5 Task 17: Admin hard-delete endpoints.
// DELETE /admin/agents/{id}
func (h *AdminHandler) HardDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	// Parse agent ID from path
	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeAdminError(w, http.StatusBadRequest, "MISSING_ID", "agent ID required")
		return
	}

	// Execute hard delete
	agentRepo := db.NewAgentRepository(h.pool)
	err := agentRepo.HardDelete(r.Context(), agentID)
	if err != nil {
		if err == db.ErrAgentNotFound {
			writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete agent")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Agent permanently deleted",
		"id":      agentID,
	})
}

// ListDeletedUsers returns soft-deleted users for admin review.
// Per PRD-v5 Task 17: List deleted accounts before permanent deletion.
// GET /admin/users/deleted?page=1&per_page=20
func (h *AdminHandler) ListDeletedUsers(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	// Parse pagination params
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 20
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	// Fetch deleted users
	userRepo := db.NewUserRepository(h.pool)
	users, total, err := userRepo.ListDeleted(r.Context(), page, perPage)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list deleted users")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"meta": map[string]interface{}{
			"total":    total,
			"page":     page,
			"per_page": perPage,
		},
	})
}

// ListDeletedAgents returns soft-deleted agents for admin review.
// Per PRD-v5 Task 17: List deleted accounts before permanent deletion.
// GET /admin/agents/deleted?page=1&per_page=20
func (h *AdminHandler) ListDeletedAgents(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	// Parse pagination params
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 20
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	// Fetch deleted agents
	agentRepo := db.NewAgentRepository(h.pool)
	agents, total, err := agentRepo.ListDeleted(r.Context(), page, perPage)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list deleted agents")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"agents": agents,
		"meta": map[string]interface{}{
			"total":    total,
			"page":     page,
			"per_page": perPage,
		},
	})
}
