package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/traffic"
)

// A client-reported funnel step stores the class and the reason it was given at ingest
// (SPEC.md 27.6). A step stored without one is unclassified, and so is every confirmed server
// step: those are never judged by a request's headers.
func TestFunnelEventRepository_StoresTheTrafficLabel(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	flow := "f_label_" + time.Now().Format("150405.000000")
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM funnel_events WHERE flow_id = $1`, flow) }) //nolint:errcheck

	require.NoError(t, repo.RecordBrowserEvent(ctx, db.BrowserFunnelEvent{
		FlowID: flow, EventName: models.FunnelConnectionStarted, ActorType: models.FunnelActorAnonymous,
		EntrySurface: "connect_page",
		Traffic:      db.TrafficLabel{Class: traffic.Automation, Reason: traffic.ReasonNoBrowserHeaders},
	}))
	require.NoError(t, repo.RecordBrowserEvent(ctx, db.BrowserFunnelEvent{
		FlowID: flow, EventName: models.FunnelRoomViewed, ActorType: models.FunnelActorAnonymous,
	}))
	require.NoError(t, repo.RecordSkillFetched(ctx, flow, models.FunnelActorAnonymous, "", models.FunnelSurfaceAgentFetch,
		db.TrafficLabel{Class: traffic.Agent, Reason: traffic.ReasonHTTPClient}))
	require.NoError(t, repo.RecordRoomCreated(ctx, uuid.New(), models.FunnelActorAgent, db.PseudonymizeActor("label-agent"), flow))

	events, err := repo.ListByFlow(ctx, flow)
	require.NoError(t, err)
	require.Len(t, events, 4)
	got := map[string][2]string{}
	for _, e := range events {
		got[e.EventName] = [2]string{e.TrafficClass, e.TrafficReason}
	}
	require.Equal(t, [2]string{"automation", "no_browser_headers"}, got[models.FunnelConnectionStarted])
	require.Equal(t, [2]string{"unclassified", ""}, got[models.FunnelRoomViewed])
	require.Equal(t, [2]string{"agent", "http_client"}, got[models.FunnelSkillFetched])
	require.Equal(t, [2]string{"unclassified", ""}, got[models.FunnelRoomCreated])
}
