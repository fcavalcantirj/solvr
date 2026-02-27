package models

import "time"

// ServiceCheckStatus represents the health status of a monitored service.
type ServiceCheckStatus string

const (
	ServiceStatusOperational ServiceCheckStatus = "operational"
	ServiceStatusDegraded    ServiceCheckStatus = "degraded"
	ServiceStatusOutage      ServiceCheckStatus = "outage"
)

// ServiceCheck records a single health check result for a service.
type ServiceCheck struct {
	ID             int64              `json:"id"`
	ServiceName    string             `json:"service_name"`
	Status         ServiceCheckStatus `json:"status"`
	ResponseTimeMs *int               `json:"response_time_ms,omitempty"`
	ErrorMessage   *string            `json:"error_message,omitempty"`
	CheckedAt      time.Time          `json:"checked_at"`
}

// DailyAggregate represents the worst status for a service on a given day.
type DailyAggregate struct {
	Date   string `json:"date"`   // "2026-02-27"
	Status string `json:"status"` // "operational", "degraded", "outage"
}

// IncidentStatus represents the status of an incident.
type IncidentStatus string

const (
	IncidentStatusInvestigating IncidentStatus = "investigating"
	IncidentStatusIdentified    IncidentStatus = "identified"
	IncidentStatusMonitoring    IncidentStatus = "monitoring"
	IncidentStatusResolved      IncidentStatus = "resolved"
)

// IncidentSeverity represents the severity of an incident.
type IncidentSeverity string

const (
	IncidentSeverityMinor    IncidentSeverity = "minor"
	IncidentSeverityMajor    IncidentSeverity = "major"
	IncidentSeverityCritical IncidentSeverity = "critical"
)

// Incident represents a service incident.
type Incident struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Status           IncidentStatus   `json:"status"`
	Severity         IncidentSeverity `json:"severity"`
	AffectedServices []string         `json:"affected_services,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	ResolvedAt       *time.Time       `json:"resolved_at,omitempty"`
}

// IncidentUpdate represents a timeline entry for an incident.
type IncidentUpdate struct {
	ID         int64          `json:"id"`
	IncidentID string         `json:"incident_id"`
	Status     IncidentStatus `json:"status"`
	Message    string         `json:"message"`
	CreatedAt  time.Time      `json:"created_at"`
}

// IncidentWithUpdates is an incident together with its timeline updates.
type IncidentWithUpdates struct {
	Incident
	Updates []IncidentUpdate `json:"updates"`
}
