package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Aggregate API usage, measured at the API boundary.
//
// Everything asserted here is a CALL VOLUME. The suite shares one database and
// other packages serve real requests while these run, so every figure is
// checked as a delta around this fixture's own rows, or against a census taken
// in the same breath.

// apiUsageFixture is an isolated slice of recorded requests. Every row it
// writes carries a namespaced request id so cleanup removes exactly its own.
type apiUsageFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *db.Pool
	repo   *db.APIUsageRepository
	prefix string
	n      int
}

func newAPIUsageFixture(t *testing.T, ctx context.Context, pool *db.Pool) *apiUsageFixture {
	t.Helper()
	return &apiUsageFixture{
		t:      t,
		ctx:    ctx,
		pool:   pool,
		repo:   db.NewAPIUsageRepository(pool),
		prefix: fmt.Sprintf("auxfix%s", time.Now().Format("150405.000000")[:13]),
	}
}

// cleanup must be deferred by the test itself, AFTER the pool's own deferred
// close: t.Cleanup would run once the pool is already shut and delete nothing.
func (f *apiUsageFixture) cleanup() {
	f.pool.Exec(f.ctx, `DELETE FROM api_request_events WHERE request_id LIKE $1`, f.prefix+"%") //nolint:errcheck
}

func (f *apiUsageFixture) event(operation, family, kind, actor string, class int, ago time.Duration) db.APIRequestEvent {
	f.n++
	return db.APIRequestEvent{
		RequestID:       fmt.Sprintf("%s-%03d", f.prefix, f.n),
		RouteTemplate:   "/v1/" + operation,
		Method:          "GET",
		Operation:       operation,
		OperationFamily: family,
		OperationKind:   kind,
		ActorType:       actor,
		StatusClass:     class,
		OccurredAt:      time.Now().Add(-ago),
	}
}

func (f *apiUsageFixture) record(events ...db.APIRequestEvent) {
	f.t.Helper()
	require.NoError(f.t, f.repo.RecordRequests(f.ctx, events))
}

func (f *apiUsageFixture) rowCount() int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.pool.QueryRow(f.ctx,
		`SELECT COUNT(*) FROM api_request_events WHERE request_id LIKE $1`, f.prefix+"%").Scan(&n))
	return n
}

func TestAPIUsagePulse_CountsSuccessfulCallsAgentCallsAndDistinctOperations(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()

	before, err := repo.GetAPIUsagePulse(ctx, window)
	require.NoError(t, err)

	f.record(
		f.event("room.message.send", db.APIFamilyRoom, db.APIOperationWrite, db.APIActorAgent, 2, time.Minute),
		f.event("room.message.send", db.APIFamilyRoom, db.APIOperationWrite, db.APIActorAgent, 2, 2*time.Minute),
		f.event("room.message.read", db.APIFamilyRoom, db.APIOperationPoll, db.APIActorAgent, 2, 3*time.Minute),
		f.event("knowledge.search", db.APIFamilyKnowledge, db.APIOperationSearch, db.APIActorHuman, 2, 4*time.Minute),
		f.event("identity.agent.register", db.APIFamilyIdentity, db.APIOperationWrite, db.APIActorAnonymous, 2, 5*time.Minute),
		// A rejected call is recorded but is not a successful call.
		f.event("room.create", db.APIFamilyRoom, db.APIOperationWrite, db.APIActorAgent, 4, 6*time.Minute),
	)

	after, err := repo.GetAPIUsagePulse(ctx, window)
	require.NoError(t, err)

	assert.Equal(t, 5, after.SuccessfulCalls-before.SuccessfulCalls, "only 2xx calls are successful calls")
	assert.Equal(t, 3, after.AgentCalls-before.AgentCalls, "agent calls count the successful ones authenticated with an agent key")
	assert.Equal(t, 1, after.HumanCalls-before.HumanCalls)
	assert.Equal(t, 1, after.AnonymousCalls-before.AnonymousCalls)
	assert.Equal(t, 1, after.PassivePolls-before.PassivePolls,
		"a passive poll is a read, and a search is not one")
	assert.Equal(t, 4, after.WriteAndSearchCalls-before.WriteAndSearchCalls,
		"create/send and search are counted apart from passive polling")
	assert.Equal(t, after.SuccessfulCalls, after.PassivePolls+after.WriteAndSearchCalls,
		"passive polling and create/send/search must partition the successful calls exactly")
	assert.GreaterOrEqual(t, after.RoomKnowledgeOperations, 3,
		"send, read and search are three distinct room or knowledge operations; "+
			"the rejected create and the identity call are neither")
	assert.Equal(t, window.Value, after.Window.Value)
}

// A failed call is never counted as a successful one, and a rejected
// credential never inflates the agent figure.
func TestAPIUsagePulse_RejectedCallsAreNotSuccessfulCalls(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()

	before, err := repo.GetAPIUsagePulse(ctx, window)
	require.NoError(t, err)

	f.record(
		f.event("room.create", db.APIFamilyRoom, db.APIOperationWrite, db.APIActorAgent, 4, time.Minute),
		f.event("room.create", db.APIFamilyRoom, db.APIOperationWrite, db.APIActorAgent, 5, 2*time.Minute),
	)

	after, err := repo.GetAPIUsagePulse(ctx, window)
	require.NoError(t, err)

	assert.Equal(t, before.SuccessfulCalls, after.SuccessfulCalls)
	assert.Equal(t, before.AgentCalls, after.AgentCalls)
	assert.Equal(t, before.WriteAndSearchCalls, after.WriteAndSearchCalls)
}

// One INCOMING request is one count, however many times it is offered: a proxy
// retry and a route adapter re-dispatch both carry the same request id.
func TestRecordRequests_CountsOneIncomingRequestOnce(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()

	event := f.event("room.message.send", db.APIFamilyRoom, db.APIOperationWrite, db.APIActorAgent, 2, time.Minute)
	adapter := event
	adapter.RouteTemplate = "/r/{slug}/message"

	f.record(event)
	f.record(event)  // the proxy retried
	f.record(adapter) // the same request reached a second route adapter

	assert.Equal(t, 1, f.rowCount(), "one incoming request is counted once")
}

// A request recorded without an identifier is still counted: dedup applies
// where an identifier exists, and its absence must not silently drop volume.
func TestRecordRequests_CountsRequestsWithoutAnIdentifier(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()

	before, err := repo.GetAPIUsagePulse(ctx, window)
	require.NoError(t, err)

	anonymousEvent := f.event("knowledge.search", db.APIFamilyKnowledge, db.APIOperationSearch, db.APIActorAnonymous, 2, time.Minute)
	anonymousEvent.RequestID = ""
	second := anonymousEvent
	f.record(anonymousEvent)
	f.record(second)

	after, err := repo.GetAPIUsagePulse(ctx, window)
	require.NoError(t, err)
	assert.Equal(t, 2, after.SuccessfulCalls-before.SuccessfulCalls)

	// Those two rows carry no fixture prefix, so remove them by hand.
	_, err = pool.Exec(ctx, `DELETE FROM api_request_events WHERE request_id IS NULL AND operation = 'knowledge.search' AND occurred_at > NOW() - INTERVAL '5 minutes'`)
	require.NoError(t, err)
}

// The series has one bucket per step of the window, oldest first, with zeros
// for quiet steps rather than gaps.
func TestAPIUsagePulse_SeriesFillsEveryBucket(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)

	f.record(f.event("knowledge.search", db.APIFamilyKnowledge, db.APIOperationSearch, db.APIActorAgent, 2, time.Minute))

	for _, window := range db.RoomStatsWindows {
		pulse, err := repo.GetAPIUsagePulse(ctx, window)
		require.NoError(t, err)
		require.Len(t, pulse.Series, window.Buckets, "%s series length", window.Value)
		for i := 1; i < len(pulse.Series); i++ {
			assert.True(t, pulse.Series[i].BucketStart.After(pulse.Series[i-1].BucketStart),
				"%s series must run oldest first", window.Value)
		}
		total := 0
		for _, bucket := range pulse.Series {
			total += bucket.Count
		}
		assert.GreaterOrEqual(t, total, 1, "%s series must carry the call just recorded", window.Value)
	}
}

// The series starts when instrumentation started. Before any request was ever
// recorded there is no measurement at all, and a zero would be a claim.
func TestAPIUsagePulse_ReportsWhenInstrumentationBegan(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)

	f.record(f.event("knowledge.search", db.APIFamilyKnowledge, db.APIOperationSearch, db.APIActorAgent, 2, time.Minute))

	pulse, err := repo.GetAPIUsagePulse(ctx, db.DefaultRoomStatsWindow())
	require.NoError(t, err)

	require.NotNil(t, pulse.InstrumentedSince, "a recorded request means instrumentation has begun")
	assert.False(t, pulse.InstrumentedSince.After(time.Now()),
		"instrumentation cannot have begun in the future")
}

// A row keeps nothing that could identify anybody: no slug, no id, no body,
// no address, no credential. The route TEMPLATE is what is stored.
func TestRecordRequests_StoresTemplatesNotIdentifiers(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newAPIUsageFixture(t, ctx, pool)
	defer f.cleanup()

	columns := map[string]bool{}
	rows, err := pool.Query(ctx, `
		SELECT column_name FROM information_schema.columns
		 WHERE table_name = 'api_request_events'`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		columns[name] = true
	}
	require.NoError(t, rows.Err())

	for _, forbidden := range []string{
		"ip_address", "ip", "user_agent", "request_body", "body", "query_string",
		"path", "url", "room_id", "room_slug", "slug", "user_id", "agent_id",
		"account_id", "token", "api_key",
	} {
		assert.False(t, columns[forbidden],
			"api_request_events must not store %q: an aggregate call volume needs nothing that identifies anybody", forbidden)
	}
	assert.True(t, columns["route_template"], "the stable route template is what a row keeps")
}
