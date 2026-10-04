package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/ops"
)

// mockServiceCheckReader implements ServiceCheckReader for testing. askedFor
// records the services each read was scoped to.
type mockServiceCheckReader struct {
	latestByService    []models.ServiceCheck
	latestByServiceErr error
	dailyAggregates    []models.DailyAggregate
	dailyAggregatesErr error
	serviceStats       []models.ServiceStat
	serviceStatsErr    error
	askedFor           [][]string
}

func (m *mockServiceCheckReader) GetLatestByService(ctx context.Context, services []string) ([]models.ServiceCheck, error) {
	m.askedFor = append(m.askedFor, services)
	return m.latestByService, m.latestByServiceErr
}

func (m *mockServiceCheckReader) GetDailyAggregates(ctx context.Context, days int, services []string) ([]models.DailyAggregate, error) {
	m.askedFor = append(m.askedFor, services)
	return m.dailyAggregates, m.dailyAggregatesErr
}

func (m *mockServiceCheckReader) GetServiceStats(ctx context.Context, days int, services []string) ([]models.ServiceStat, error) {
	m.askedFor = append(m.askedFor, services)
	return m.serviceStats, m.serviceStatsErr
}

func avgMs(v float64) *float64 { return &v }

// mockIncidentReader implements IncidentReader for testing.
type mockIncidentReader struct {
	incidents []models.IncidentWithUpdates
	err       error
}

func (m *mockIncidentReader) ListRecent(ctx context.Context, limit int) ([]models.IncidentWithUpdates, error) {
	return m.incidents, m.err
}

func TestStatusHandler_GetStatus_AllOperational(t *testing.T) {
	rt1 := 1
	rt2 := 3
	now := time.Now()

	checks := &mockServiceCheckReader{
		latestByService: []models.ServiceCheck{
			{ID: 1, ServiceName: "api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt1, CheckedAt: now},
			{ID: 2, ServiceName: "database", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt2, CheckedAt: now},
		},
		dailyAggregates: []models.DailyAggregate{
			{Date: "2026-02-27", Status: "operational"},
		},
		serviceStats: []models.ServiceStat{
			{ServiceName: "api", Checks: 100, Operational: 100, AvgResponseMs: avgMs(1), ResponseSamples: 100},
			{ServiceName: "database", Checks: 200, Operational: 197, AvgResponseMs: avgMs(4), ResponseSamples: 50},
		},
	}

	incidents := &mockIncidentReader{
		incidents: []models.IncidentWithUpdates{},
	}

	handler := NewStatusHandler(checks, incidents)

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Data struct {
			OverallStatus string `json:"overall_status"`
			Services      []struct {
				Category string `json:"category"`
				Items    []struct {
					Name      string `json:"name"`
					Uptime    string `json:"uptime"`
					LatencyMs *int   `json:"latency_ms"`
				} `json:"items"`
			} `json:"services"`
			Summary struct {
				Uptime30d         *float64 `json:"uptime_30d"`
				AvgResponseTimeMs *float64 `json:"avg_response_time_ms"`
				ServiceCount      int      `json:"service_count"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	data := resp.Data

	// Every read is scoped to the core services; nothing else is measured.
	if len(checks.askedFor) != 3 {
		t.Fatalf("expected 3 scoped reads, got %d", len(checks.askedFor))
	}
	for _, asked := range checks.askedFor {
		if fmt.Sprint(asked) != fmt.Sprint(ops.CoreServices) {
			t.Errorf("expected a read scoped to %v, got %v", ops.CoreServices, asked)
		}
	}

	if data.OverallStatus != "operational" {
		t.Errorf("expected overall_status 'operational', got '%v'", data.OverallStatus)
	}

	// One category: the core services. There is no storage service.
	if len(data.Services) != 1 || data.Services[0].Category != "Core Services" {
		t.Fatalf("expected only the Core Services category, got %+v", data.Services)
	}

	// Each row carries its own uptime and its own average latency.
	want := map[string]struct {
		uptime  string
		latency int
	}{
		"REST API":   {"100.00%", 1},
		"PostgreSQL": {"98.50%", 4},
	}
	items := data.Services[0].Items
	if len(items) != 2 {
		t.Fatalf("expected 2 services, got %+v", items)
	}
	for _, item := range items {
		w, ok := want[item.Name]
		if !ok {
			t.Errorf("unexpected service %q", item.Name)
			continue
		}
		if item.Uptime != w.uptime {
			t.Errorf("%s: expected uptime %s, got %s", item.Name, w.uptime, item.Uptime)
		}
		if item.LatencyMs == nil || *item.LatencyMs != w.latency {
			t.Errorf("%s: expected latency %dms, got %v", item.Name, w.latency, item.LatencyMs)
		}
	}

	// The summary pools the core services: 297 of 300 checks, (1*100 + 4*50) / 150 ms.
	if data.Summary.ServiceCount != 2 {
		t.Errorf("expected service_count 2, got %v", data.Summary.ServiceCount)
	}
	if data.Summary.Uptime30d == nil || *data.Summary.Uptime30d != 99 {
		t.Errorf("expected uptime_30d 99, got %v", data.Summary.Uptime30d)
	}
	if data.Summary.AvgResponseTimeMs == nil || *data.Summary.AvgResponseTimeMs != 2 {
		t.Errorf("expected avg_response_time_ms 2, got %v", data.Summary.AvgResponseTimeMs)
	}
}

func TestStatusHandler_GetStatus_WithDegradedService(t *testing.T) {
	rt1 := 45
	rt2 := 500
	now := time.Now()

	checks := &mockServiceCheckReader{
		latestByService: []models.ServiceCheck{
			{ID: 1, ServiceName: "api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt1, CheckedAt: now},
			{ID: 2, ServiceName: "database", Status: models.ServiceStatusDegraded, ResponseTimeMs: &rt2, CheckedAt: now},
		},
		dailyAggregates: []models.DailyAggregate{},
	}

	incidents := &mockIncidentReader{incidents: []models.IncidentWithUpdates{}}

	handler := NewStatusHandler(checks, incidents)
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	if data["overall_status"] != "degraded" {
		t.Errorf("expected overall_status 'degraded', got '%v'", data["overall_status"])
	}
}

func TestStatusHandler_GetStatus_WithOutage(t *testing.T) {
	rt1 := 45
	now := time.Now()

	checks := &mockServiceCheckReader{
		latestByService: []models.ServiceCheck{
			{ID: 1, ServiceName: "api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt1, CheckedAt: now},
			{ID: 2, ServiceName: "database", Status: models.ServiceStatusOutage, CheckedAt: now},
		},
		dailyAggregates: []models.DailyAggregate{},
	}

	incidents := &mockIncidentReader{incidents: []models.IncidentWithUpdates{}}

	handler := NewStatusHandler(checks, incidents)
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	if data["overall_status"] != "outage" {
		t.Errorf("expected overall_status 'outage', got '%v'", data["overall_status"])
	}
}

func TestStatusHandler_GetStatus_WithIncidents(t *testing.T) {
	now := time.Now()
	checks := &mockServiceCheckReader{
		latestByService: []models.ServiceCheck{},
		dailyAggregates: []models.DailyAggregate{},
	}

	incidents := &mockIncidentReader{
		incidents: []models.IncidentWithUpdates{
			{
				Incident: models.Incident{
					ID:        "INC-2026-0001",
					Title:     "API Latency",
					Status:    models.IncidentStatusResolved,
					Severity:  models.IncidentSeverityMinor,
					CreatedAt: now.Add(-2 * time.Hour),
					UpdatedAt: now.Add(-1 * time.Hour),
				},
				Updates: []models.IncidentUpdate{
					{ID: 1, IncidentID: "INC-2026-0001", Status: models.IncidentStatusResolved, Message: "Fixed", CreatedAt: now.Add(-1 * time.Hour)},
					{ID: 2, IncidentID: "INC-2026-0001", Status: models.IncidentStatusInvestigating, Message: "Looking", CreatedAt: now.Add(-2 * time.Hour)},
				},
			},
		},
	}

	handler := NewStatusHandler(checks, incidents)
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	incidentsList := data["incidents"].([]interface{})
	if len(incidentsList) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidentsList))
	}

	inc := incidentsList[0].(map[string]interface{})
	if inc["id"] != "INC-2026-0001" {
		t.Errorf("expected incident id 'INC-2026-0001', got '%v'", inc["id"])
	}
	if inc["title"] != "API Latency" {
		t.Errorf("expected title 'API Latency', got '%v'", inc["title"])
	}

	updates := inc["updates"].([]interface{})
	if len(updates) != 2 {
		t.Errorf("expected 2 updates, got %d", len(updates))
	}
}

func TestStatusHandler_GetStatus_RepoErrors(t *testing.T) {
	// When all repos return errors (e.g., tables don't exist yet),
	// handler should return 200 with empty/default data, not 500.
	repoErr := fmt.Errorf("relation \"service_checks\" does not exist")

	checks := &mockServiceCheckReader{
		latestByServiceErr: repoErr,
		dailyAggregatesErr: repoErr,
		serviceStatsErr:    repoErr,
	}
	incidents := &mockIncidentReader{err: repoErr}

	handler := NewStatusHandler(checks, incidents)
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for repo errors (graceful degradation), got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'data' key in response")
	}

	if data["overall_status"] != "operational" {
		t.Errorf("expected 'operational' default, got '%v'", data["overall_status"])
	}

	summary := data["summary"].(map[string]interface{})
	if summary["service_count"].(float64) != 0 {
		t.Errorf("expected service_count 0, got %v", summary["service_count"])
	}
}

func TestStatusHandler_GetStatus_EmptyData(t *testing.T) {
	checks := &mockServiceCheckReader{
		latestByService: []models.ServiceCheck{},
		dailyAggregates: []models.DailyAggregate{},
	}
	incidents := &mockIncidentReader{incidents: []models.IncidentWithUpdates{}}

	handler := NewStatusHandler(checks, incidents)
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for empty data, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	// Should be operational with no services (cold start)
	if data["overall_status"] != "operational" {
		t.Errorf("expected 'operational' for cold start, got '%v'", data["overall_status"])
	}

	summary := data["summary"].(map[string]interface{})
	if summary["service_count"].(float64) != 0 {
		t.Errorf("expected service_count 0, got %v", summary["service_count"])
	}
}
