package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The return-usage report (idx 92 steps 2, 4, 6) reads the funnel, room timelines,
// notifications and search queries. Every figure is seeded by hand in a historical window no
// other test or real traffic reaches, so each count is exact.

type returnSeed struct {
	t    *testing.T
	ctx  context.Context
	pool *db.Pool
	tag  string
	from time.Time
}

func (s returnSeed) funnel(event, actorType, actorRef, flow string, room uuid.UUID, at time.Time) {
	s.t.Helper()
	nz := func(v string) any {
		if v == "" {
			return nil
		}
		return v
	}
	var roomID any
	if room != uuid.Nil {
		roomID = room
	}
	_, err := s.pool.Exec(s.ctx, `INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, actor_ref, room_id, occurred_at)
		VALUES ($1, $2, CASE WHEN $2 IN ('connection_started','starter_prompt_copied','room_viewed') THEN 'browser' ELSE 'server' END, $3, $4, $5, $6)`,
		nz(flow), event, actorType, nz(actorRef), roomID, at)
	require.NoError(s.t, err)
}

func (s returnSeed) ref(n string) string { return "ret_" + s.tag + "_" + n }

func (s returnSeed) day(d float64) time.Time {
	return s.from.Add(time.Duration(d * float64(24*time.Hour)))
}

func (s returnSeed) room(name string, archived bool) *models.Room {
	s.t.Helper()
	slug := "ret-" + s.tag + "-" + name
	room, err := db.NewRoomRepository(s.pool).Create(s.ctx, models.CreateRoomParams{Slug: slug, DisplayName: slug})
	require.NoError(s.t, err)
	if archived {
		_, err = s.pool.Exec(s.ctx, `UPDATE rooms SET archived_at = $2 WHERE id = $1`, room.ID, s.day(20))
		require.NoError(s.t, err)
	}
	s.t.Cleanup(func() {
		s.pool.Exec(context.Background(), `DELETE FROM notifications WHERE room_id = $1`, room.ID) //nolint:errcheck
		s.pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = $1`, room.ID)              //nolint:errcheck
	})
	return room
}

func (s returnSeed) entry(room *models.Room, authorType, authorID string, at time.Time) int64 {
	s.t.Helper()
	m, err := db.NewMessageRepository(s.pool).Create(s.ctx, models.CreateMessageParams{
		RoomID: room.ID, AuthorType: authorType, AuthorID: &authorID, AgentName: authorID, Content: "work", ContentType: "text",
	})
	require.NoError(s.t, err)
	_, err = s.pool.Exec(s.ctx, `UPDATE room_entries SET created_at = $2 WHERE id = $1`, m.ID, at)
	require.NoError(s.t, err)
	return m.ID
}

func TestReturnUsage_MeasuresReturnsOutcomesNotificationsAndLanguageExactly(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	to := time.Now().Add(-1400 * 24 * time.Hour).Truncate(time.Second)
	from := to.Add(-30 * 24 * time.Hour)
	for _, q := range []string{
		`DELETE FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2`,
		`DELETE FROM search_queries WHERE searched_at >= $1 AND searched_at < $2`,
		`DELETE FROM notifications WHERE created_at >= $1 AND created_at < $2`,
	} {
		_, err := pool.Exec(ctx, q, from.Add(-60*24*time.Hour), to.Add(90*24*time.Hour))
		require.NoError(t, err)
	}
	s := returnSeed{t: t, ctx: ctx, pool: pool, tag: fmt.Sprint(time.Now().UnixNano() % 1000000000), from: from}

	// Collaborations: agent X activates A1 (before the window), A2 and A3; Y only A2; Z
	// activates A4 and, six hours later, A5. Human H1 started the flows of A2 and A3.
	a1, a2, a3, a4, a5, a6 := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	activate := func(room uuid.UUID, at time.Time, agents ...string) {
		for _, a := range agents {
			s.funnel("participant_joined", "agent", s.ref(a), "", room, at.Add(-time.Hour))
		}
		s.funnel("first_two_way_exchange", "agent", "", "", room, at)
	}
	activate(a1, s.day(-5), "X")
	s.funnel("room_created", "agent", s.ref("X"), s.ref("f2"), a2, s.day(1))
	s.funnel("connection_started", "human", s.ref("H1"), s.ref("f2"), uuid.Nil, s.day(0.9))
	activate(a2, s.day(2), "X", "Y")
	s.funnel("room_created", "agent", s.ref("X"), s.ref("f3"), a3, s.day(11))
	s.funnel("connection_started", "human", s.ref("H1"), s.ref("f3"), uuid.Nil, s.day(10.9))
	activate(a3, s.day(12), "X")
	s.funnel("room_created", "agent", s.ref("Z"), "", a4, s.day(2))
	activate(a4, s.day(3), "Z")
	activate(a5, s.day(3.25), "Z")
	s.funnel("room_created", "agent", s.ref("W"), "", a6, s.day(4)) // never activated

	// Outcomes of real rooms: R is archived and its creator Q collaborates again later; R2 has
	// a DONE event and its creator V never returns.
	r := s.room("archived", true)
	s.funnel("room_created", "agent", s.ref("Q"), "", r.ID, s.day(5))
	activate(r.ID, s.day(6), "Q")
	s.funnel("participant_joined", "agent", s.ref("Q"), "", uuid.New(), s.day(9))
	r2 := s.room("done", false)
	s.funnel("room_created", "agent", s.ref("V"), "", r2.ID, s.day(5))
	activate(r2.ID, s.day(6), "V")
	v := s.ref("V")
	_, _, err := db.NewRoomEntryRepository(pool).Create(ctx, models.CreateRoomEntryParams{RoomID: r2.ID, Kind: models.RoomEntryKindEvent,
		AuthorType: strPtrRet("agent"), AuthorID: &v, ActorLabel: v, EventType: strPtrRet("DONE")})
	require.NoError(t, err)

	// Resumed rooms: R3 resumes twice in the window (after 46 h, then 7 days); R4 once (after a
	// gap that started before the window). A two-hour pause is not a resumption.
	r3 := s.room("resume", false)
	s.entry(r3, "agent", s.ref("X"), s.day(1))
	m2 := s.entry(r3, "agent", s.ref("X"), s.day(1).Add(2*time.Hour))
	s.entry(r3, "agent", s.ref("X"), s.day(3))
	s.entry(r3, "agent", s.ref("X"), s.day(10))
	r4 := s.room("resume2", false)
	s.entry(r4, "agent", s.ref("Y"), s.day(-2))
	s.entry(r4, "agent", s.ref("Y"), s.day(1))

	// Notifications: U is told about a reply and posts in that room within 24 h; agent G is
	// asked for a review and never acts on it.
	u := insertAuthorityUser(ctx, t, pool, "ret"+s.tag[len(s.tag)-4:])
	g := insertRecentAgent(ctx, t, pool, "ret", nil)
	_, err = pool.Exec(ctx, `INSERT INTO notifications (user_id, type, title, schema_version, room_id, entry_id, created_at)
		VALUES ($1, 'room.reply', 'New reply', 3, $2, $3, $4)`, u, r3.ID, m2, s.day(1).Add(3*time.Hour))
	require.NoError(t, err)
	s.entry(r3, "human", u.String(), s.day(1).Add(5*time.Hour))
	_, err = pool.Exec(ctx, `INSERT INTO notifications (agent_id, type, title, schema_version, room_id, entry_id, created_at)
		VALUES ($1, 'room.review_requested', 'Review requested', 3, $2, $3, $4)`, g, r3.ID, m2, s.day(5))
	require.NoError(t, err)

	// Language demand from what people typed.
	for _, q := range []struct {
		text    string
		results int
	}{{"如何修复错误", 0}, {"erro de conexão no deploy", 2}, {"como resolver erro", 0}, {"ошибка подключения", 3},
		{"kubernetes ingress", 4}, {"kubernetes ingress tls", 0}} {
		_, err := pool.Exec(ctx, `INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms, searcher_type, searched_at)
			VALUES ($1, $1, $2, 'hybrid', 5, 'anonymous', $3)`, q.text, q.results, s.day(7))
		require.NoError(t, err)
	}

	rep, err := db.NewReturnUsageRepository(pool).Measure(ctx, from, to, to.Add(60*24*time.Hour))
	require.NoError(t, err)

	agents := rep.Collaborations.Agents
	require.Equal(t, 5, agents.Actors, "X, Y, Z, Q, V collaborated in the window")
	require.Equal(t, 2, agents.ActorsWithRepeat, "X and Z")
	require.Equal(t, 3, agents.Gaps, "X: 7 d and 10 d; Z: 6 h")
	require.InDelta(t, 168.0, *agents.MedianGapHours, 0.01)
	require.InDelta(t, 240.0, *agents.P90GapHours, 0.01)
	require.Equal(t, db.ReturnGapBuckets{Under1d: 1, From7To28d: 2}, agents.Buckets)
	humans := rep.Collaborations.Humans
	require.Equal(t, 1, humans.Actors)
	require.Equal(t, 1, humans.ActorsWithRepeat)
	require.Equal(t, 1, humans.Gaps)

	require.Equal(t, db.RoomOutcomes{Created: 6, OnboardingFailure: 1, StalledAfterActivation: 3,
		CompletedOneOff: 1, CompletedThenReturned: 1}, rep.Outcomes)
	require.Equal(t, db.ResumedRooms{Rooms: 2, Resumptions: 3}, rep.Resumed)

	require.Equal(t, map[string]int{"room.reply": 1, "room.review_requested": 1}, rep.Notifications.SentByType)
	require.Equal(t, 1, rep.Notifications.ResumedWithin24h)
	require.True(t, rep.Notifications.NotAGrowthMetric)

	byBucket := map[string]db.LanguageBucket{}
	for _, b := range rep.LanguageDemand {
		byBucket[b.Bucket] = b
	}
	require.Equal(t, 1, byBucket["han"].Queries)
	require.Equal(t, 1, byBucket["han"].ZeroResults)
	require.Equal(t, 2, byBucket["portuguese_marked"].Queries)
	require.Equal(t, 1, byBucket["portuguese_marked"].ZeroResults)
	require.Equal(t, 1, byBucket["other_non_latin"].Queries)
	require.Equal(t, 2, byBucket["latin_other"].Queries)
	require.Equal(t, 0, byBucket["japanese_korean"].Queries, "every bucket is reported, empty ones too")
	require.NotEmpty(t, rep.Definitions)
}

func strPtrRet(s string) *string { return &s }
