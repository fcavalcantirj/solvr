package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// mockServiceCheckReader implements ServiceCheckReader for testing.
type mockServiceCheckReader struct {
	latestByService []models.ServiceCheck
	dailyAggregates []models.DailyAggregate
	uptimePct       float64
	avgRT           float64
	err             error
}

func (m *mockServiceCheckReader) GetLatestByService(ctx context.Context) ([]models.ServiceCheck, error) {
	return m.latestByService, m.err
}

func (m *mockServiceCheckReader) GetDailyAggregates(ctx context.Context, days int) ([]models.DailyAggregate, error) {
	return m.dailyAggregates, m.err
}

func (m *mockServiceCheckReader) GetUptimePercentage(ctx context.Context, days int) (float64, error) {
	return m.uptimePct, m.err
}

func (m *mockServiceCheckReader) GetAvgResponseTime(ctx context.Context, days int) (float64, error) {
	return m.avgRT, m.err
}

// mockIncidentReader implements IncidentReader for testing.
type mockIncidentReader struct {
	incidents []models.IncidentWithUpdates
	err       error
}

func (m *mockIncidentReader) ListRecent(ctx context.Context, limit int) ([]models.IncidentWithUpdates, error) {
	return m.incidents, m.err
}

func TestStatusHandler_GetStatus_AllOperational(t *testing.T) {
	rt1 := 45
	rt2 := 8
	rt3 := 65
	now := time.Now()

	checks := &mockServiceCheckReader{
		latestByService: []models.ServiceCheck{
			{ID: 1, ServiceName: "api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt1, CheckedAt: now},
			{ID: 2, ServiceName: "database", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt2, CheckedAt: now},
			{ID: 3, ServiceName: "ipfs", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt3, CheckedAt: now},
		},
		dailyAggregates: []models.DailyAggregate{
			{Date: "2026-02-27", Status: "operational"},
		},
		uptimePct: 99.97,
		avgRT:     39.33,
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

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'data' key in response")
	}

	if data["overall_status"] != "operational" {
		t.Errorf("expected overall_status 'operational', got '%v'", data["overall_status"])
	}

	services, ok := data["services"].([]interface{})
	if !ok {
		t.Fatal("expected 'services' array")
	}

	if len(services) != 2 {
		t.Errorf("expected 2 categories, got %d", len(services))
	}

	summary, ok := data["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'summary' object")
	}

	if summary["service_count"].(float64) != 3 {
		t.Errorf("expected service_count 3, got %v", summary["service_count"])
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
		uptimePct:       95.0,
		avgRT:           272.5,
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
			{ID: 2, ServiceName: "ipfs", Status: models.ServiceStatusOutage, CheckedAt: now},
		},
		dailyAggregates: []models.DailyAggregate{},
		uptimePct:       50.0,
		avgRT:           45.0,
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
					ID:       "INC-2026-0001",
					Title:    "API Latency",
					Status:   models.IncidentStatusResolved,
					Severity: models.IncidentSeverityMinor,
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
