package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The funnel ingest endpoint accepts only browser steps and stores no secrets;
// the contract endpoint publishes the one shared event vocabulary.

func TestFunnelIngest_RecordsBrowserEvent(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	repo := db.NewFunnelEventRepository(pool)
	h := NewFunnelHandler(repo)
	flow := "f_hdl_" + time.Now().Format("150405.000000")

	body := `{"flow_id":"` + flow + `","event":"connection_started","preset":"plan-and-build","entry_surface":"connect_page","instruction_version":"1.0"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/analytics/funnel", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.IngestBrowserEvent(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

	events, err := repo.ListByFlow(context.Background(), flow)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, models.FunnelConnectionStarted, events[0].EventName)
	require.Equal(t, models.FunnelSourceBrowser, events[0].SourceChannel)
	// Anonymous visitor: no actor reference is stored.
	require.Equal(t, models.FunnelActorAnonymous, events[0].ActorType)
	require.Empty(t, events[0].ActorRef)
}

func TestFunnelIngest_RejectsServerEvent(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	h := NewFunnelHandler(db.NewFunnelEventRepository(pool))

	// A client must not be able to self-report a server-only step.
	body := `{"event":"room_created","flow_id":"f_fake"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/analytics/funnel", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.IngestBrowserEvent(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "server events must not be accepted from a browser")
}

func TestFunnelIngest_RejectsUnknownEvent(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	h := NewFunnelHandler(db.NewFunnelEventRepository(pool))

	body := `{"event":"totally_made_up"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/analytics/funnel", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.IngestBrowserEvent(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFunnelContract_PublishesVocabulary(t *testing.T) {
	h := NewFunnelHandler(nil) // contract needs no store
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/funnel/contract", nil)
	rec := httptest.NewRecorder()
	h.GetContract(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var out struct {
		Data struct {
			InstructionVersion string                   `json:"instruction_version"`
			Events             []models.FunnelEventSpec `json:"events"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, ConnectInstructionVersion, out.Data.InstructionVersion)
	// 7 connection steps + the 2 share steps of idx 88 (share_visit, share_link_copied)
	// + the web server's skill_fetched.
	require.Len(t, out.Data.Events, 10)

	byName := map[string]models.FunnelEventSpec{}
	for _, e := range out.Data.Events {
		byName[e.Name] = e
	}
	require.Equal(t, models.FunnelSourceBrowser, byName[models.FunnelConnectionStarted].SourceChannel)
	require.Equal(t, models.FunnelSourceServer, byName[models.FunnelRoomCreated].SourceChannel)
	require.Equal(t, models.FunnelSourceServer, byName[models.FunnelFirstTwoWayExchange].SourceChannel)
	require.Equal(t, models.FunnelSourceBrowser, byName[models.FunnelShareVisit].SourceChannel)
	require.Equal(t, models.FunnelSourceBrowser, byName[models.FunnelShareLinkCopied].SourceChannel)
	require.Equal(t, "web_server", byName[models.FunnelSkillFetched].SourceChannel)
	require.Equal(t, []string{"flow_id", "entry_surface"}, byName[models.FunnelSkillFetched].Attributes)
	require.Contains(t, byName[models.FunnelSkillFetched].Description, "request_mode")
}
