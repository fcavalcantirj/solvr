package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/ops"
)

// ServiceCheckReader reads health check data for the status page, scoped to
// the services it is asked about.
type ServiceCheckReader interface {
	GetLatestByService(ctx context.Context, services []string) ([]models.ServiceCheck, error)
	GetDailyAggregates(ctx context.Context, days int, services []string) ([]models.DailyAggregate, error)
	GetServiceStats(ctx context.Context, days int, services []string) ([]models.ServiceStat, error)
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
	OverallStatus string                  `json:"overall_status"`
	Services      []statusCategory        `json:"services"`
	Summary       statusSummary           `json:"summary"`
	UptimeHistory []models.DailyAggregate `json:"uptime_history"`
	Incidents     []statusIncident        `json:"incidents"`
}

// serviceDescriptions maps service names to human-readable descriptions.
var serviceDescriptions = map[string]string{
	"api":      "Primary API endpoints for all operations",
	"database": "PostgreSQL data store",
}

// serviceCategoryMap maps service names to their category.
var serviceCategoryMap = map[string]string{
	"api":      "Core Services",
	"database": "Core Services",
}

// statusWindowDays is the window the uptime, latency and history cover.
const statusWindowDays = 30

// GetStatus handles GET /v1/status. It reports the core services only
// (ops.CoreServices, the services the HealthCheckJob checks): checks of any
// other service still in the table, such as the retired IPFS node, never count.
func (h *StatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services := ops.CoreServices

	// Graceful degradation: log errors, continue with defaults.
	// This avoids 500s when tables don't exist yet or DB is temporarily down.
	latestChecks, err := h.checks.GetLatestByService(ctx, services)
	if err != nil {
		slog.Error("status: failed to get service checks", "error", err)
		latestChecks = nil
	}

	history, err := h.checks.GetDailyAggregates(ctx, statusWindowDays, services)
	if err != nil {
		slog.Error("status: failed to get uptime history", "error", err)
		history = nil
	}

	stats, err := h.checks.GetServiceStats(ctx, statusWindowDays, services)
	if err != nil {
		slog.Error("status: failed to get service stats", "error", err)
		stats = nil
	}
	statsByService := make(map[string]models.ServiceStat, len(stats))
	for _, stat := range stats {
		statsByService[stat.ServiceName] = stat
	}

	recentIncidents, err := h.incidents.ListRecent(ctx, 10)
	if err != nil {
		slog.Error("status: failed to get incidents", "error", err)
		recentIncidents = nil
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
			Uptime:      "—",
		}

		// Each service's own uptime and average latency over the window.
		if stat, ok := statsByService[check.ServiceName]; ok && stat.Checks > 0 {
			item.Uptime = fmt.Sprintf("%.2f%%", float64(stat.Operational)/float64(stat.Checks)*100)
			if stat.AvgResponseMs != nil {
				rt := int(math.Round(*stat.AvgResponseMs))
				item.LatencyMs = &rt
			}
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

	categories := []statusCategory{}
	for _, name := range []string{"Core Services"} {
		if cat, ok := categoryMap[name]; ok {
			categories = append(categories, *cat)
		}
	}

	// Build summary
	summary := statusSummary{
		ServiceCount: len(latestChecks),
	}
	// The summary pools the core services: operational checks over all checks,
	// and their average latency weighted by how many checks measured one.
	var totalChecks, operational, samples int
	var latencySum float64
	for _, stat := range stats {
		totalChecks += stat.Checks
		operational += stat.Operational
		if stat.AvgResponseMs != nil {
			latencySum += *stat.AvgResponseMs * float64(stat.ResponseSamples)
			samples += stat.ResponseSamples
		}
	}
	if totalChecks > 0 {
		rounded := math.Round(float64(operational)/float64(totalChecks)*100*100) / 100
		summary.Uptime30d = &rounded
	}
	if samples > 0 {
		rounded := math.Round(latencySum/float64(samples)*100) / 100
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
	}
	if name, ok := names[slug]; ok {
		return name
	}
	return slug
}
