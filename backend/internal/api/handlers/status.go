package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ServiceCheckReader reads health check data for the status page.
type ServiceCheckReader interface {
	GetLatestByService(ctx context.Context) ([]models.ServiceCheck, error)
	GetDailyAggregates(ctx context.Context, days int) ([]models.DailyAggregate, error)
	GetUptimePercentage(ctx context.Context, days int) (float64, error)
	GetAvgResponseTime(ctx context.Context, days int) (float64, error)
}

// IncidentReader reads incidents for the status page.
type IncidentReader interface {
	ListRecent(ctx context.Context, limit int) ([]models.IncidentWithUpdates, error)
}

// StatusHandler handles GET /v1/status.
type StatusHandler struct {
	checks    ServiceCheckReader
	incidents IncidentReader
}

// NewStatusHandler creates a new StatusHandler.
func NewStatusHandler(checks ServiceCheckReader, incidents IncidentReader) *StatusHandler {
	return &StatusHandler{checks: checks, incidents: incidents}
}

// statusServiceItem represents a single service in the JSON response.
type statusServiceItem struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Uptime      string  `json:"uptime"`
	LatencyMs   *int    `json:"latency_ms"`
	LastChecked *string `json:"last_checked"`
}

// statusCategory groups services by category.
type statusCategory struct {
	Category string              `json:"category"`
	Items    []statusServiceItem `json:"items"`
}

type statusSummary struct {
	Uptime30d         *float64 `json:"uptime_30d"`
	AvgResponseTimeMs *float64 `json:"avg_response_time_ms"`
	ServiceCount      int      `json:"service_count"`
	LastChecked       *string  `json:"last_checked"`
}

type statusIncidentUpdate struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type statusIncident struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Status    string                 `json:"status"`
	Severity  string                 `json:"severity"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
	Updates   []statusIncidentUpdate `json:"updates"`
}

type statusResponse struct {
	OverallStatus string               `json:"overall_status"`
	Services      []statusCategory     `json:"services"`
	Summary       statusSummary        `json:"summary"`
	UptimeHistory []models.DailyAggregate `json:"uptime_history"`
	Incidents     []statusIncident     `json:"incidents"`
}

// serviceDescriptions maps service names to human-readable descriptions.
var serviceDescriptions = map[string]string{
	"api":      "Primary API endpoints for all operations",
	"database": "PostgreSQL data store",
	"ipfs":     "Decentralized content storage (Kubo)",
}

// serviceCategoryMap maps service names to their category.
var serviceCategoryMap = map[string]string{
	"api":      "Core Services",
	"database": "Core Services",
	"ipfs":     "Storage",
}

// GetStatus handles GET /v1/status.
func (h *StatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch all data concurrently would be better, but sequential is fine for 4 queries
	latestChecks, err := h.checks.GetLatestByService(ctx)
	if err != nil {
		writeStatusError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get service checks")
		return
	}

	history, err := h.checks.GetDailyAggregates(ctx, 30)
	if err != nil {
		writeStatusError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get uptime history")
		return
	}

	uptimePct, err := h.checks.GetUptimePercentage(ctx, 30)
	if err != nil {
		writeStatusError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get uptime percentage")
		return
	}

	avgRT, err := h.checks.GetAvgResponseTime(ctx, 30)
	if err != nil {
		writeStatusError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get avg response time")
		return
	}

	recentIncidents, err := h.incidents.ListRecent(ctx, 10)
	if err != nil {
		writeStatusError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get incidents")
		return
	}

	// Build service categories from latest checks
	categoryMap := map[string]*statusCategory{}
	overallStatus := "operational"
	var lastCheckedTime *time.Time

	for _, check := range latestChecks {
		catName := serviceCategoryMap[check.ServiceName]
		if catName == "" {
			catName = "Other"
		}

		cat, exists := categoryMap[catName]
		if !exists {
			cat = &statusCategory{Category: catName}
			categoryMap[catName] = cat
		}

		item := statusServiceItem{
			Name:        serviceDisplayName(check.ServiceName),
			Description: serviceDescriptions[check.ServiceName],
			Status:      string(check.Status),
			Uptime:      fmt.Sprintf("%.2f%%", uptimePct),
		}

		if check.ResponseTimeMs != nil {
			rt := *check.ResponseTimeMs
			item.LatencyMs = &rt
		}

		checkedStr := check.CheckedAt.UTC().Format(time.RFC3339)
		item.LastChecked = &checkedStr

		if lastCheckedTime == nil || check.CheckedAt.After(*lastCheckedTime) {
			t := check.CheckedAt
			lastCheckedTime = &t
		}

		if check.Status == models.ServiceStatusOutage {
			overallStatus = "outage"
		} else if check.Status == models.ServiceStatusDegraded && overallStatus != "outage" {
			overallStatus = "degraded"
		}

		cat.Items = append(cat.Items, item)
	}

	// Order categories: Core Services first, then Storage
	categories := []statusCategory{}
	for _, name := range []string{"Core Services", "Storage"} {
		if cat, ok := categoryMap[name]; ok {
			categories = append(categories, *cat)
		}
	}

	// Build summary
	summary := statusSummary{
		ServiceCount: len(latestChecks),
	}
	if uptimePct > 0 {
		rounded := math.Round(uptimePct*100) / 100
		summary.Uptime30d = &rounded
	}
	if avgRT > 0 {
		rounded := math.Round(avgRT*100) / 100
		summary.AvgResponseTimeMs = &rounded
	}
	if lastCheckedTime != nil {
		s := lastCheckedTime.UTC().Format(time.RFC3339)
		summary.LastChecked = &s
	}

	// Build incidents
	incidentList := make([]statusIncident, 0, len(recentIncidents))
	for _, inc := range recentIncidents {
		si := statusIncident{
			ID:        inc.ID,
			Title:     inc.Title,
			Status:    string(inc.Status),
			Severity:  string(inc.Severity),
			CreatedAt: inc.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: inc.UpdatedAt.UTC().Format(time.RFC3339),
			Updates:   make([]statusIncidentUpdate, 0, len(inc.Updates)),
		}
		for _, u := range inc.Updates {
			si.Updates = append(si.Updates, statusIncidentUpdate{
				Time:    u.CreatedAt.UTC().Format("15:04 UTC"),
				Message: u.Message,
				Status:  string(u.Status),
			})
		}
		incidentList = append(incidentList, si)
	}

	resp := statusResponse{
		OverallStatus: overallStatus,
		Services:      categories,
		Summary:       summary,
		UptimeHistory: history,
		Incidents:     incidentList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": resp,
	})
}

// serviceDisplayName maps slug to display name.
func serviceDisplayName(slug string) string {
	names := map[string]string{
		"api":      "REST API",
		"database": "PostgreSQL",
		"ipfs":     "IPFS Node",
	}
	if name, ok := names[slug]; ok {
		return name
	}
	return slug
}

func writeStatusError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
