package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The share-attribution report (idx 88 steps 5-6) reads ONLY the connection funnel.
// Every figure below is seeded by hand in a historical window no other test or real
// traffic reaches, so each count is exact.

type shareSeed struct {
	t    *testing.T
	ctx  context.Context
	pool *db.Pool
	tag  string
}

func (s shareSeed) row(event, channel, actorType, actorRef, flow string, room *uuid.UUID, srcKind string, srcID *uuid.UUID, at time.Time) {
	s.t.Helper()
	nz := func(v string) any {
		if v == "" {
			return nil
		}
		return v
	}
	var kind any
	if srcKind != "" {
		kind = srcKind
	}
	_, err := s.pool.Exec(s.ctx, `
		INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, actor_ref,
			room_id, source_kind, source_id, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		nz(flow), event, channel, actorType, nz(actorRef), room, kind, srcID, at)
	require.NoError(s.t, err)
}

func (s shareSeed) ref(name string) string { return "shr_" + s.tag + "_" + name }

func newShareFixture(t *testing.T) (shareSeed, time.Time, time.Time) {
	t.Helper()
	pool, ctx := newFunnelTestPool(t)
	to := time.Now().Add(-1000 * 24 * time.Hour).Truncate(time.Second)
	from := to.Add(-30 * 24 * time.Hour)
	_, err := pool.Exec(ctx, `DELETE FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2`,
		from.Add(-60*24*time.Hour), to.Add(90*24*time.Hour))
	require.NoError(t, err)
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)
	return shareSeed{t: t, ctx: ctx, pool: pool, tag: tag}, from, to
}

// seedShareLoop builds: origin room O (activated before the window), origin post P,
// origin room Q (never activated); share visits, copies and Try-this flows from them;
// attributed rooms R1 (activated, anonymous flow), R2 (activated, identified human h2),
// R3 (not activated); and later returns of a2 (day 3), a1 (day 20) and h2 (day 2).
func seedShareLoop(s shareSeed, from time.Time) (r1Activated, r2Activated time.Time) {
	o, p, q := uuid.New(), uuid.New(), uuid.New()
	r1, r2, r3, r4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	base := from.Add(24 * time.Hour)
	at := func(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }

	// O reached its own two-way exchange before the window: an activated origin.
	s.row("first_two_way_exchange", "server", "agent", "", "", &o, "", nil, from.Add(-5*24*time.Hour))
	// a4 already collaborated elsewhere before the window: not a new agent.
	s.row("participant_joined", "server", "agent", s.ref("a4"), "", ptrUUID(uuid.New()), "", nil, from.Add(-10*24*time.Hour))

	// Share visits (4) and copies (3).
	s.row("share_visit", "browser", "anonymous", "", "", nil, "room", &o, at(1))
	s.row("share_visit", "browser", "anonymous", "", "", nil, "room", &o, at(2))
	s.row("share_visit", "browser", "human", s.ref("h1"), "", nil, "room", &o, at(3))
	s.row("share_visit", "browser", "anonymous", "", "", nil, "room", &q, at(4))
	s.row("share_link_copied", "browser", "anonymous", "", "", nil, "room", &o, at(5))
	s.row("share_link_copied", "browser", "anonymous", "", "", nil, "room", &o, at(6))
	s.row("share_link_copied", "browser", "human", s.ref("h1"), "", nil, "post", &p, at(7))

	// Try-this arrivals (3 sourced flows) and one ordinary flow.
	f1, f2, f3, f4 := s.ref("F1"), s.ref("F2"), s.ref("F3"), s.ref("F4")
	s.row("connection_started", "browser", "anonymous", "", f1, nil, "room", &o, at(10))
	s.row("connection_started", "browser", "human", s.ref("h2"), f2, nil, "post", &p, at(11))
	s.row("connection_started", "browser", "anonymous", "", f3, nil, "room", &o, at(12))
	s.row("connection_started", "browser", "anonymous", "", f4, nil, "", nil, at(13))

	// R1: from O, anonymous flow, agents a1 + a2, activated.
	s.row("room_created", "server", "agent", s.ref("a1"), f1, &r1, "room", &o, at(20))
	s.row("participant_joined", "server", "agent", s.ref("a1"), f1, &r1, "room", &o, at(20))
	s.row("participant_joined", "server", "agent", s.ref("a2"), f1, &r1, "room", &o, at(21))
	r1Activated = at(22)
	s.row("first_two_way_exchange", "server", "agent", "", f1, &r1, "room", &o, r1Activated)

	// R2: from P, human h2's flow, agents a3 + a4, activated.
	s.row("starter_prompt_copied", "browser", "human", s.ref("h2"), f2, nil, "", nil, at(23))
	s.row("room_created", "server", "agent", s.ref("a3"), f2, &r2, "post", &p, at(24))
	s.row("participant_joined", "server", "agent", s.ref("a3"), f2, &r2, "post", &p, at(24))
	s.row("participant_joined", "server", "agent", s.ref("a4"), f2, &r2, "post", &p, at(25))
	r2Activated = at(26)
	s.row("first_two_way_exchange", "server", "agent", "", f2, &r2, "post", &p, r2Activated)

	// R3: from O, never activated. R4: unsourced, activated — not in this report.
	s.row("room_created", "server", "agent", s.ref("a5"), f3, &r3, "room", &o, at(30))
	s.row("participant_joined", "server", "agent", s.ref("a5"), f3, &r3, "room", &o, at(30))
	s.row("room_created", "server", "agent", s.ref("a6"), f4, &r4, "", nil, at(31))
	s.row("first_two_way_exchange", "server", "agent", "", f4, &r4, "", nil, at(32))

	// Returns: a2 in another room on day 3, a1 on day 20, h2 views a room on day 2.
	s.row("participant_joined", "server", "agent", s.ref("a2"), "", ptrUUID(uuid.New()), "", nil, r1Activated.Add(3*24*time.Hour))
	s.row("participant_joined", "server", "agent", s.ref("a1"), "", ptrUUID(uuid.New()), "", nil, r1Activated.Add(20*24*time.Hour))
	s.row("room_viewed", "browser", "human", s.ref("h2"), "", nil, "", nil, r2Activated.Add(2*24*time.Hour))
	return r1Activated, r2Activated
}

func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }

func rateOf(t *testing.T, c db.ActivationConversion) float64 {
	t.Helper()
	require.NotNil(t, c.Rate)
	return *c.Rate
}

func TestShareAttribution_MeasuresTheShareLoopExactly(t *testing.T) {
	s, from, to := newShareFixture(t)
	seedShareLoop(s, from)
	repo := db.NewShareAttributionRepository(s.pool)

	rep, err := repo.Measure(s.ctx, from, to, to.Add(60*24*time.Hour))
	require.NoError(t, err)

	require.Equal(t, 4, rep.ShareVisits.Total)
	require.Equal(t, 3, rep.ShareVisits.Anonymous)
	require.Equal(t, 1, rep.ShareVisits.Human)
	require.Equal(t, 0, rep.ShareVisits.Agent)
	require.Equal(t, 3, rep.Invitations)
	require.Equal(t, 7, rep.ReferredVisits)
	require.Equal(t, 3, rep.AttributedFlows)
	require.Equal(t, 3, rep.AttributedRoomsCreated)
	require.Equal(t, 2, rep.AttributedRoomsActivated)

	require.Equal(t, 3, rep.Origins)
	require.Equal(t, 2, rep.ActivatedOrigins)
	require.InDelta(t, 1.5, *rep.InvitationsPerActivatedOrigin, 1e-9)
	require.InDelta(t, 3.0, *rep.ReferredVisitsPerActivatedOrigin, 1e-9)
	require.InDelta(t, 2.0/3.0, rateOf(t, rep.InviteToActivation), 1e-9)
	require.InDelta(t, 2.0/7.0, rateOf(t, rep.ReferredVisitActivation), 1e-9)
	require.InDelta(t, 3.0*2.0/7.0, *rep.K.Value, 1e-9)
	require.True(t, rep.K.ExperimentMetric)

	// a1, a2, a3 are new; a4 had collaborated before. Humans come only from human rows:
	// h2 is new; R1's anonymous flow is unresolved, never guessed.
	require.Equal(t, 3, rep.NewAgentActivations)
	require.Equal(t, 1, rep.NewHumanActivations)
	require.Equal(t, 1, rep.UnresolvedHumanActivations)

	require.Equal(t, db.ShareReturns{Eligible: 3, Returned: 1}, rep.AgentReturns7d)
	require.Equal(t, db.ShareReturns{Eligible: 3, Returned: 2}, rep.AgentReturns28d)
	require.Equal(t, db.ShareReturns{Eligible: 1, Returned: 1}, rep.HumanReturns7d)
	require.Equal(t, db.ShareReturns{Eligible: 1, Returned: 1}, rep.HumanReturns28d)
	require.NotEmpty(t, rep.Definitions)
}

func TestShareAttribution_ReturnsAreEligibleOnlyOnceTheirWindowHasElapsed(t *testing.T) {
	s, from, to := newShareFixture(t)
	r1, _ := seedShareLoop(s, from)
	repo := db.NewShareAttributionRepository(s.pool)

	// Ten days after R1's activation: the 7-day returns are measurable, the 28-day ones
	// are not yet, and a return that has not happened yet is not counted.
	rep, err := repo.Measure(s.ctx, from, to, r1.Add(10*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, db.ShareReturns{Eligible: 3, Returned: 1}, rep.AgentReturns7d)
	require.Equal(t, db.ShareReturns{Eligible: 0, Returned: 0}, rep.AgentReturns28d)
	require.Equal(t, db.ShareReturns{Eligible: 0, Returned: 0}, rep.HumanReturns28d)
}

func TestShareAttribution_AnEmptyWindowHasNoRatesRatherThanZeroes(t *testing.T) {
	s, from, to := newShareFixture(t)
	repo := db.NewShareAttributionRepository(s.pool)

	rep, err := repo.Measure(s.ctx, from, to, to)
	require.NoError(t, err)
	require.Zero(t, rep.ReferredVisits)
	require.Nil(t, rep.InvitationsPerActivatedOrigin)
	require.Nil(t, rep.ReferredVisitsPerActivatedOrigin)
	require.Nil(t, rep.InviteToActivation.Rate)
	require.Nil(t, rep.K.Value)
}
