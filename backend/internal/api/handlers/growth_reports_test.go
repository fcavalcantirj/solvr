package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// fakeParticipantReader records the window end it was asked for and returns canned measures,
// so the growth report handler is tested without a database.
type fakeParticipantReader struct {
	gotEnd   time.Time
	measures growth.ParticipantMeasures
	err      error
}

func (f *fakeParticipantReader) Measure(_ context.Context, end time.Time) (growth.ParticipantMeasures, error) {
	f.gotEnd = end
	return f.measures, f.err
}

func growthRequest(t *testing.T, handler http.HandlerFunc, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestGrowthParticipants_RefusesWithoutTheOperatorKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewGrowthReportsHandler(GrowthReaders{Participants: &fakeParticipantReader{}})

	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = growthRequest(t, h.GetParticipants, "/admin/growth/participants",
		map[string]string{OperatorAccessHeader: "wrong"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
}

func TestGrowthParticipants_ServesTheReportForTheRequestedEnd(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeParticipantReader{measures: growth.ParticipantMeasures{Humans: 3, Agents: 5, KnownOverlap: 1}}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: reader})

	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants?end=2026-10-01T00:00:00Z",
		map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), reader.gotEnd)

	var body struct {
		Data growth.ParticipantReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 3, body.Data.Humans.Active)
	assert.Equal(t, 5, body.Data.AgentIdentities.Active)
	assert.Equal(t, 8, body.Data.Target.Measured)
	assert.Equal(t, growth.MonthlyActiveParticipantGoal, body.Data.Target.Goal)
	assert.Equal(t, growth.StatusUnmet, body.Data.Target.Status)
}

func TestGrowthParticipants_DefaultsToNowAndRejectsABadEnd(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeParticipantReader{}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: reader})

	before := time.Now().UTC()
	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants", map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, reader.gotEnd.Before(before.Truncate(time.Second)), "the window ends now by default")

	rec = growthRequest(t, h.GetParticipants, "/admin/growth/participants?end=yesterday", map[string]string{OperatorAccessHeader: "op-key"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGrowthParticipants_AReadFailureIsAServerError(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewGrowthReportsHandler(GrowthReaders{Participants: &fakeParticipantReader{err: errors.New("boom")}})
	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants", map[string]string{OperatorAccessHeader: "op-key"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "boom")
}

// fakeStageReader returns canned stage measures for the window ending at end.
type fakeStageReader struct {
	gotEnd   time.Time
	measures growth.StageMeasures
	err      error
}

func (f *fakeStageReader) Measure(_ context.Context, end time.Time) (growth.StageMeasures, error) {
	f.gotEnd = end
	return f.measures, f.err
}

func TestGrowthStages_ServesTheSequencedStagesWithParticipantGates(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	participants := &fakeParticipantReader{measures: growth.ParticipantMeasures{
		Humans: 6000, Agents: 5000, PreviousHumans: 5500, PreviousAgents: 4800,
	}}
	stages := &fakeStageReader{measures: growth.StageMeasures{WeeklyActivatedRooms: 12, WeeklyOwners: 4}}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: participants, Stages: stages})

	rec := growthRequest(t, h.GetStages, "/admin/growth/stages?end=2026-10-01T00:00:00Z",
		map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	want := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, want, participants.gotEnd)
	assert.Equal(t, want, stages.gotEnd)

	var body struct {
		Data growth.StageReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Stages, 4)
	assert.Equal(t, growth.StatusUnmet, body.Data.Stages[0].Status, "12 weekly activated rooms is short of 100")
	for _, g := range body.Data.Stages[1].Gates {
		if g.Key == "monthly_active_participants" {
			require.NotNil(t, g.Measured)
			assert.InDelta(t, 11000, *g.Measured, 1e-9)
			assert.Equal(t, growth.StatusMet, g.Status, "10,000 reached in two consecutive windows")
		}
	}
	assert.Equal(t, growth.StatusBlockedByPreviousStage, body.Data.Stages[1].Status)
}

func TestGrowthStages_RefusesWithoutTheOperatorKeyAndHidesFailures(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewGrowthReportsHandler(GrowthReaders{
		Participants: &fakeParticipantReader{},
		Stages:       &fakeStageReader{err: errors.New("stage boom")},
	})
	assert.Equal(t, http.StatusUnauthorized, growthRequest(t, h.GetStages, "/admin/growth/stages", nil).Code)
	rec := growthRequest(t, h.GetStages, "/admin/growth/stages", map[string]string{OperatorAccessHeader: "op-key"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "stage boom")
}

// fakeModelReader returns canned monthly flows for the month starting at monthStart.
type fakeModelReader struct {
	gotMonth time.Time
	flows    growth.MonthlyFlows
	err      error
}

func (f *fakeModelReader) MonthlyFlows(_ context.Context, monthStart time.Time) (growth.MonthlyFlows, error) {
	f.gotMonth = monthStart
	f.flows.Start = monthStart
	f.flows.End = monthStart.AddDate(0, 1, 0)
	return f.flows, f.err
}

func TestGrowthModel_ServesTheMonthlyModelWithScenariosAndBottleneck(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	model := &fakeModelReader{flows: growth.MonthlyFlows{Month: "2026-08",
		Humans: growth.PopulationFlows{Active: 300, Retained: 120, New: 150, Reactivated: 30, PreviousActive: 200},
		Agents: growth.PopulationFlows{Active: 80, Retained: 40, New: 40, PreviousActive: 50},
	}}
	stages := &fakeStageReader{measures: growth.StageMeasures{CoreChecks: 100, CoreOperational: 100, GateAEligible: 10, GateAConverted: 9}}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: &fakeParticipantReader{}, Stages: stages, Model: model})

	rec := growthRequest(t, h.GetModel, "/admin/growth/model?month=2026-08", map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), model.gotMonth)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), stages.gotEnd, "the bottleneck reads the gates at the month's end")

	var body struct {
		Data growth.ModelReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Data.Scenarios, 6, "three scenarios for humans and three for agents")
	assert.InDelta(t, 2_000_000, body.Data.Arithmetic.QualifiedVisitsToStayFlat, 1e-6)
	assert.Equal(t, growth.StatusNotYetMeasurable, body.Data.Bottleneck.Bottleneck, "gate A has 10 of 100 rooms")
	assert.False(t, body.Data.PaidAcquisition.Ready)
	assert.Equal(t, "monthly", body.Data.Review.Cadence)
}

func TestGrowthModel_DefaultsToTheLastCompleteMonthAndRejectsABadMonth(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	model := &fakeModelReader{}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: &fakeParticipantReader{}, Stages: &fakeStageReader{}, Model: model})

	rec := growthRequest(t, h.GetModel, "/admin/growth/model", map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code)
	now := time.Now().UTC()
	want := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	assert.Equal(t, want, model.gotMonth)

	for _, bad := range []string{"2026-13", "August", "2026-8-1"} {
		rec = growthRequest(t, h.GetModel, "/admin/growth/model?month="+bad, map[string]string{OperatorAccessHeader: "op-key"})
		assert.Equal(t, http.StatusBadRequest, rec.Code, bad)
	}
	assert.Equal(t, http.StatusUnauthorized, growthRequest(t, h.GetModel, "/admin/growth/model", nil).Code)
}
