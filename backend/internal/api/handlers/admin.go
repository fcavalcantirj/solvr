// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// AdminHandler handles admin operations like raw SQL queries.
type AdminHandler struct {
	pool *db.Pool
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(pool *db.Pool) *AdminHandler {
	return &AdminHandler{pool: pool}
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
