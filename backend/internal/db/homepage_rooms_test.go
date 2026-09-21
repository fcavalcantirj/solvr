package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The public room statistics behind the homepage.
//
// Every assertion here is about an HONEST number: presence is "now" and never
// windowed, historical counts move with the selected window, private rooms are
// invisible, expired rooms drop out of the live figures but keep their history,
// and an agent identity that was never authenticated is counted as unverified
// rather than silently mixed in with the ones that were.

// roomStatsFixture is one isolated slice of room data: every row it writes is
// namespaced by the same suffix so it can be removed again without touching
// anything else in the database.
type roomStatsFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *db.Pool
	suffix string
	rooms  []uuid.UUID
	agents []string
}

func newRoomStatsFixture(t *testing.T, ctx context.Context, pool *db.Pool) *roomStatsFixture {
	t.Helper()
	f := &roomStatsFixture{
		t:      t,
		ctx:    ctx,
		pool:   pool,
		suffix: fmt.Sprintf("hr%s", time.Now().Format("150405.000000")[:12]),
	}
	f.suffix = sanitizeSlugPart(f.suffix)
	t.Cleanup(f.cleanup)
	return f
}

// sanitizeSlugPart keeps only characters the rooms.slug CHECK constraint allows.
func sanitizeSlugPart(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		}
	}
	return string(out)
}

func (f *roomStatsFixture) cleanup() {
	for _, id := range f.rooms {
		f.pool.Exec(f.ctx, `DELETE FROM room_events WHERE room_id = $1`, id)       //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM agent_presence WHERE room_id = $1`, id)    //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM messages WHERE room_id = $1`, id)          //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM room_agent_tokens WHERE room_id = $1`, id) //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM rooms WHERE id = $1`, id)                  //nolint:errcheck
	}
	for _, id := range f.agents {
		f.pool.Exec(f.ctx, `DELETE FROM agents WHERE id = $1`, id) //nolint:errcheck
	}
}

// room inserts a room and returns its id. private/expired/deleted are the three
// ways a room can drop out of the public figures.
func (f *roomStatsFixture) room(name string, private bool, expiresAt *time.Time, deleted bool) uuid.UUID {
	f.t.Helper()
	slug := fmt.Sprintf("%s-%s", f.suffix, name)

	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}

	var id uuid.UUID
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO rooms (slug, display_name, token_hash, is_private, expires_at, deleted_at)
		VALUES ($1, $2, 'hash_homepage_rooms_test', $3, $4, $5)
		RETURNING id
	`, slug, "Room "+name, private, expiresAt, deletedAt).Scan(&id)
	require.NoError(f.t, err, "insert room %s", slug)

	f.rooms = append(f.rooms, id)
	return id
}

// message inserts one message at a chosen age. authorID nil means the message
// was posted with the shared room token: real, but not identity-verified.
func (f *roomStatsFixture) message(roomID uuid.UUID, authorType, agentName string, authorID *string, age time.Duration, deleted bool) {
	f.t.Helper()

	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}

	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO messages (room_id, author_type, author_id, agent_name, content, created_at, deleted_at)
		VALUES ($1, $2, $3, $4, 'homepage rooms fixture message', NOW() - $5::interval, $6)
	`, roomID, authorType, authorID, agentName, fmt.Sprintf("%d seconds", int(age.Seconds())), deletedAt)
	require.NoError(f.t, err, "insert message in %s", roomID)
}

// presence makes an agent live (or expired) in a room.
func (f *roomStatsFixture) presence(roomID uuid.UUID, agentName string, live bool) {
	f.t.Helper()

	lastSeen := "NOW()"
	if !live {
		lastSeen = "NOW() - INTERVAL '2 hours'"
	}
	_, err := f.pool.Exec(f.ctx, fmt.Sprintf(`
		INSERT INTO agent_presence (room_id, agent_name, card_json, last_seen, ttl_seconds)
		VALUES ($1, $2, '{}'::jsonb, %s, 900)
	`, lastSeen), roomID, agentName)
	require.NoError(f.t, err, "insert presence for %s", agentName)
}

// authenticate registers the agent and issues it a per-agent room token, which
// is what makes its identity verified rather than merely claimed by name.
func (f *roomStatsFixture) authenticate(roomID uuid.UUID, agentName string) {
	f.t.Helper()

	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO agents (id, display_name) VALUES ($1, $1)
		ON CONFLICT (id) DO NOTHING
	`, agentName)
	require.NoError(f.t, err, "insert agent %s", agentName)
	f.agents = append(f.agents, agentName)

	_, err = f.pool.Exec(f.ctx, `
		INSERT INTO room_agent_tokens (room_id, agent_id, token_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, agent_id) DO NOTHING
	`, roomID, agentName, "hash_"+f.suffix+"_"+agentName)
	require.NoError(f.t, err, "issue room token to %s", agentName)
}

// activate records the activation milestone for a room at a chosen age.
func (f *roomStatsFixture) activate(roomID uuid.UUID, age time.Duration) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO room_events (room_id, event_type, actor, created_at)
		VALUES ($1, $2, 'system', NOW() - $3::interval)
	`, roomID, db.RoomActivationEventType, fmt.Sprintf("%d seconds", int(age.Seconds())))
	require.NoError(f.t, err, "record activation for %s", roomID)
}

func newRoomStatsPool(t *testing.T) (*db.Pool, context.Context, func()) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	pool, err := db.NewPool(ctx, url)
	require.NoError(t, err)

	return pool, ctx, func() {
		pool.Close()
		cancel()
	}
}

// roomCensus is what the repository SHOULD have answered, recomputed from the
// stated definitions and written a different way (subqueries and make_interval
// rather than joins and string casts).
//
// The whole suite shares one database and other packages post room messages
// while these tests run, so a before/after delta on a global aggregate is not a
// stable measurement. Comparing the repository against a census taken in the
// same breath is: ambient rows land in BOTH numbers, and a wrong predicate --
// counting a private room, a deleted message, a system notice, an expired
// room's presence -- shows up as a disagreement no matter what else is going on.
type roomCensus struct {
	AgentsOnline          int
	VerifiedAgentsOnline  int
	RoomsWithAgentsOnline int
	RoomsWithConversation int
	AgentMessages         int
	UnverifiedAgentMsgs   int
	HumanMessages         int
	PublicRooms           int
}

func takeRoomCensus(t *testing.T, ctx context.Context, pool *db.Pool, window db.RoomStatsWindow) roomCensus {
	t.Helper()

	var c roomCensus
	err := pool.QueryRow(ctx, `
		WITH public_rooms AS (
			SELECT id, expires_at FROM rooms WHERE NOT is_private AND deleted_at IS NULL
		), live_rooms AS (
			SELECT id FROM public_rooms WHERE expires_at IS NULL OR expires_at > NOW()
		), online AS (
			SELECT ap.agent_name, ap.room_id
			  FROM agent_presence ap
			 WHERE ap.room_id IN (SELECT id FROM live_rooms)
			   AND ap.last_seen + make_interval(secs => ap.ttl_seconds) > NOW()
		), talk AS (
			SELECT m.room_id, m.author_type, m.author_id, m.created_at
			  FROM messages m
			 WHERE m.room_id IN (SELECT id FROM public_rooms)
			   AND m.deleted_at IS NULL
			   AND m.author_type IN ('agent', 'human')
		)
		SELECT
			(SELECT COUNT(DISTINCT agent_name) FROM online),
			(SELECT COUNT(DISTINCT o.agent_name) FROM online o
			  WHERE o.agent_name IN (
			        SELECT rt.agent_id FROM room_agent_tokens rt
			         WHERE rt.room_id = o.room_id
			           AND (rt.expires_at IS NULL OR rt.expires_at > NOW()))),
			(SELECT COUNT(DISTINCT room_id) FROM online),
			(SELECT COUNT(DISTINCT room_id) FROM talk WHERE created_at > NOW() - $1::interval),
			(SELECT COUNT(*) FROM talk WHERE author_type = 'agent' AND created_at > NOW() - $1::interval),
			(SELECT COUNT(*) FROM talk WHERE author_type = 'agent' AND author_id IS NULL
			   AND created_at > NOW() - $1::interval),
			(SELECT COUNT(*) FROM talk WHERE author_type = 'human' AND created_at > NOW() - $1::interval),
			(SELECT COUNT(*) FROM public_rooms)
	`, fmt.Sprintf("%d seconds", int(window.Duration.Seconds()))).Scan(
		&c.AgentsOnline, &c.VerifiedAgentsOnline, &c.RoomsWithAgentsOnline,
		&c.RoomsWithConversation, &c.AgentMessages, &c.UnverifiedAgentMsgs,
		&c.HumanMessages, &c.PublicRooms,
	)
	require.NoError(t, err)
	return c
}

// assertMatchesCensus holds the repository to the definitions it publishes.
func assertMatchesCensus(t *testing.T, pulse db.RoomPulse, c roomCensus) {
	t.Helper()
	assert.Equal(t, c.AgentsOnline, pulse.Presence.AgentsOnline, "agents online")
	assert.Equal(t, c.VerifiedAgentsOnline, pulse.Presence.VerifiedAgentsOnline, "verified agents online")
	assert.Equal(t, c.RoomsWithAgentsOnline, pulse.Presence.RoomsWithAgentsOnline, "rooms with agents online")
	assert.Equal(t, c.RoomsWithConversation, pulse.Stats.RoomsWithConversation, "rooms with conversation")
	assert.Equal(t, c.AgentMessages, pulse.Stats.AgentMessages, "agent messages")
	assert.Equal(t, c.UnverifiedAgentMsgs, pulse.Stats.UnverifiedAgentMessages, "unverified agent messages")
	assert.Equal(t, c.HumanMessages, pulse.Stats.HumanMessages, "human messages")
	assert.Equal(t, c.PublicRooms, pulse.PublicRooms, "public rooms")
}

func TestRoomStatsWindows_OfferTwentyFourHoursSevenDaysThirtyDays(t *testing.T) {
	values := make([]string, 0, len(db.RoomStatsWindows))
	for _, w := range db.RoomStatsWindows {
		values = append(values, w.Value)
		assert.NotEmpty(t, w.Label, "window %q needs a label", w.Value)
		assert.NotEmpty(t, w.WindowText, "window %q needs display text", w.Value)
		assert.Positive(t, w.Buckets, "window %q needs buckets", w.Value)
	}
	assert.Equal(t, []string{"24h", "7d", "30d"}, values)

	assert.Equal(t, "24h", db.DefaultRoomStatsWindow().Value,
		"the homepage defaults to 24 hours")

	w, ok := db.RoomStatsWindowByValue("7d")
	require.True(t, ok)
	assert.Equal(t, 7*24*time.Hour, w.Duration)

	_, ok = db.RoomStatsWindowByValue("90d")
	assert.False(t, ok, "an unknown window is rejected, not silently accepted")
}

func TestGetRoomPulse_PresenceIsNowAndIgnoresTheSelectedWindow(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	room := f.room("live", false, nil, false)
	f.presence(room, "planner"+f.suffix, true)
	f.presence(room, "executor"+f.suffix, true)
	f.presence(room, "ghost"+f.suffix, false) // heartbeat expired

	repo := db.NewHomepageRepository(pool)

	// Each reading is checked against a census of the same instant, so the
	// claim survives whatever else the suite is writing.
	for _, value := range []string{"24h", "7d", "30d"} {
		window := mustWindow(t, value)
		pulse, err := repo.GetRoomPulse(ctx, window)
		require.NoError(t, err)
		census := takeRoomCensus(t, ctx, pool, window)

		assert.Equal(t, census.AgentsOnline, pulse.Presence.AgentsOnline,
			"presence is measured now; window %q must not move it", value)
		assert.Equal(t, census.RoomsWithAgentsOnline, pulse.Presence.RoomsWithAgentsOnline,
			"window %q", value)
		assert.GreaterOrEqual(t, pulse.Presence.AgentsOnline, 2,
			"both live agents are online under window %q", value)
		assert.GreaterOrEqual(t, pulse.Presence.RoomsWithAgentsOnline, 1)
	}
}

func TestGetRoomPulse_OnlineCountsUnexpiredPresenceOnly(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	quiet := f.room("quiet", false, nil, false)
	f.presence(quiet, "stale"+f.suffix, false)
	f.presence(quiet, "fresh"+f.suffix, true)

	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()
	pulse, err := repo.GetRoomPulse(ctx, window)
	require.NoError(t, err)
	census := takeRoomCensus(t, ctx, pool, window)

	// The census expires a heartbeat with make_interval; the repository does it
	// with a string cast. A room holding one live and one dead heartbeat proves
	// they agree about which is which.
	assert.Equal(t, census.AgentsOnline, pulse.Presence.AgentsOnline,
		"only the agent with an unexpired heartbeat is online")
	assert.Equal(t, census.RoomsWithAgentsOnline, pulse.Presence.RoomsWithAgentsOnline)
	assert.GreaterOrEqual(t, pulse.Presence.AgentsOnline, 1)
}

func TestGetRoomPulse_SeparatesVerifiedIdentitiesFromNameOnlyPresence(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	room := f.room("identity", false, nil, false)

	verified := "verified" + f.suffix
	claimed := "claimed" + f.suffix
	f.presence(room, verified, true)
	f.presence(room, claimed, true)
	f.authenticate(room, verified)

	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()
	pulse, err := repo.GetRoomPulse(ctx, window)
	require.NoError(t, err)
	census := takeRoomCensus(t, ctx, pool, window)

	assert.Equal(t, census.VerifiedAgentsOnline, pulse.Presence.VerifiedAgentsOnline)
	assert.Equal(t,
		pulse.Presence.AgentsOnline,
		pulse.Presence.VerifiedAgentsOnline+pulse.Presence.UnverifiedAgentsOnline,
		"every online agent is either verified or explicitly unverified; nothing is mixed")
	assert.GreaterOrEqual(t, pulse.Presence.VerifiedAgentsOnline, 1,
		"the agent holding a per-agent room token is a verified identity")
	assert.GreaterOrEqual(t, pulse.Presence.UnverifiedAgentsOnline, 1,
		"the agent that only claimed a name is unverified, not verified")
}

func TestGetRoomPulse_PrivateAndDeletedRoomsAreInvisible(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)

	private := f.room("private", true, nil, false)
	f.presence(private, "secret"+f.suffix, true)
	f.message(private, "agent", "secret"+f.suffix, nil, time.Minute, false)

	deleted := f.room("deleted", false, nil, true)
	f.presence(deleted, "gone"+f.suffix, true)
	f.message(deleted, "agent", "gone"+f.suffix, nil, time.Minute, false)

	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()
	pulse, err := repo.GetRoomPulse(ctx, window)
	require.NoError(t, err)
	census := takeRoomCensus(t, ctx, pool, window)

	// A live private room and a live deleted room are sitting in the data. If
	// any predicate let one through, the repository would out-count the census.
	assertMatchesCensus(t, pulse, census)
}

func TestGetRoomPulse_ExpiredRoomsLeaveLiveFiguresButKeepTheirHistory(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)

	past := time.Now().Add(-time.Hour)
	archived := f.room("archived", false, &past, false)
	f.presence(archived, "lingering"+f.suffix, true)
	f.message(archived, "agent", "lingering"+f.suffix, nil, 30*time.Minute, false)

	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()
	pulse, err := repo.GetRoomPulse(ctx, window)
	require.NoError(t, err)
	census := takeRoomCensus(t, ctx, pool, window)

	// The census excludes expired rooms from presence and keeps their messages
	// in history. An archived room with both proves the repository does too.
	assertMatchesCensus(t, pulse, census)
	assert.GreaterOrEqual(t, pulse.Stats.AgentMessages, 1,
		"the transcript of a retained expired room still counts as history")
}

func TestGetRoomPulse_HistoricalMetricsFollowTheSelectedWindow(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	recent := f.room("recent", false, nil, false)
	older := f.room("older", false, nil, false)
	f.message(recent, "agent", "now"+f.suffix, nil, time.Hour, false)
	f.message(older, "agent", "then"+f.suffix, nil, 10*24*time.Hour, false)

	// Read the narrow window first: 30 days is a superset of 24 hours, and the
	// rest of the suite only ever writes messages at NOW(), which land in both.
	// The 10-day-old message is the only thing that can separate the two.
	day, err := repo.GetRoomPulse(ctx, mustWindow(t, "24h"))
	require.NoError(t, err)
	month, err := repo.GetRoomPulse(ctx, mustWindow(t, "30d"))
	require.NoError(t, err)

	assert.GreaterOrEqual(t, month.Stats.AgentMessages, day.Stats.AgentMessages+1,
		"30 days sees the 10-day-old message that 24 hours cannot")
	assert.GreaterOrEqual(t, month.Stats.RoomsWithConversation, day.Stats.RoomsWithConversation+1,
		"and it sees the room that message is in")
	assert.GreaterOrEqual(t, day.Stats.AgentMessages, 1,
		"the message posted an hour ago is inside 24 hours")
}

func TestGetRoomPulse_SeparatesAgentAndHumanMessagesAndExcludesSystemAndDeleted(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)

	room := f.room("mixed", false, nil, false)
	authorID := "agent" + f.suffix
	f.message(room, "agent", authorID, &authorID, time.Minute, false)
	f.message(room, "agent", "shared"+f.suffix, nil, time.Minute, false)
	f.message(room, "human", "person"+f.suffix, nil, time.Minute, false)
	f.message(room, "system", "system", nil, time.Minute, false)
	f.message(room, "agent", "erased"+f.suffix, nil, time.Minute, true)

	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()
	pulse, err := repo.GetRoomPulse(ctx, window)
	require.NoError(t, err)
	census := takeRoomCensus(t, ctx, pool, window)

	// The fixture holds one of everything the counts must tell apart: an
	// authored agent message, a shared-token one, a human one, a system notice
	// and a deleted message. The census counts them by the published
	// definitions; a repository that mixed any two would disagree.
	assertMatchesCensus(t, pulse, census)
	assert.GreaterOrEqual(t, pulse.Stats.AgentMessages, 2)
	assert.GreaterOrEqual(t, pulse.Stats.HumanMessages, 1)
	assert.GreaterOrEqual(t, pulse.Stats.UnverifiedAgentMessages, 1,
		"the shared-token message is agent work with an unverified identity")
}

func TestGetRoomPulse_RoomsWithConversationIgnoresSystemOnlyRooms(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)

	plumbing := f.room("plumbing", false, nil, false)
	f.message(plumbing, "system", "system", nil, time.Minute, false)

	repo := db.NewHomepageRepository(pool)
	window := db.DefaultRoomStatsWindow()
	pulse, err := repo.GetRoomPulse(ctx, window)
	require.NoError(t, err)
	census := takeRoomCensus(t, ctx, pool, window)

	// A public room whose only message is a system notice. Transport noise is
	// not a conversation, so it must not appear in either count.
	assert.Equal(t, census.RoomsWithConversation, pulse.Stats.RoomsWithConversation)
	assert.Equal(t, census.AgentMessages, pulse.Stats.AgentMessages)
}

func TestGetRoomPulse_TwoWayExchangesComeFromTheActivationMilestone(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	before, err := repo.GetRoomPulse(ctx, mustWindow(t, "24h"))
	require.NoError(t, err)

	activated := f.room("activated", false, nil, false)
	f.activate(activated, time.Hour)

	old := f.room("oldactivation", false, nil, false)
	f.activate(old, 10*24*time.Hour)

	after, err := repo.GetRoomPulse(ctx, mustWindow(t, "24h"))
	require.NoError(t, err)
	month, err := repo.GetRoomPulse(ctx, mustWindow(t, "30d"))
	require.NoError(t, err)

	assert.Equal(t, before.Stats.TwoWayExchangeRooms+1, after.Stats.TwoWayExchangeRooms,
		"only the milestone recorded inside the window counts")
	assert.GreaterOrEqual(t, month.Stats.TwoWayExchangeRooms, after.Stats.TwoWayExchangeRooms+1)

	require.NotNil(t, after.Stats.ActivationInstrumentedSince,
		"once a milestone exists the API can say since when it has been measured")
}

func TestRecordRoomActivation_FiresOnceWhenASecondAuthorSpeaks(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	events := db.NewRoomEventRepository(pool)
	room := f.room("milestone", false, nil, false)

	// One author talking to themselves is not an exchange.
	f.message(room, "agent", "solo"+f.suffix, nil, 2*time.Minute, false)
	recorded, err := events.RecordActivation(ctx, room)
	require.NoError(t, err)
	assert.False(t, recorded, "a monologue never activates a room")
	assert.Equal(t, 0, countActivations(t, ctx, pool, room))

	// A second distinct author makes it a two-way exchange.
	f.message(room, "agent", "second"+f.suffix, nil, time.Minute, false)
	recorded, err = events.RecordActivation(ctx, room)
	require.NoError(t, err)
	assert.True(t, recorded, "the second distinct author activates the room")
	assert.Equal(t, 1, countActivations(t, ctx, pool, room))

	// And it is recorded once, not once per message afterwards.
	f.message(room, "agent", "second"+f.suffix, nil, 30*time.Second, false)
	recorded, err = events.RecordActivation(ctx, room)
	require.NoError(t, err)
	assert.False(t, recorded, "activation is a milestone, not a counter")
	assert.Equal(t, 1, countActivations(t, ctx, pool, room))
}

func TestRecordRoomActivation_IgnoresSystemAndDeletedMessages(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	events := db.NewRoomEventRepository(pool)
	room := f.room("noiseonly", false, nil, false)

	f.message(room, "agent", "solo"+f.suffix, nil, 2*time.Minute, false)
	f.message(room, "system", "system", nil, time.Minute, false)
	f.message(room, "agent", "erased"+f.suffix, nil, time.Minute, true)

	recorded, err := events.RecordActivation(ctx, room)
	require.NoError(t, err)
	assert.False(t, recorded, "system notices and deleted messages are not a second participant")
	assert.Equal(t, 0, countActivations(t, ctx, pool, room))
}

func TestGetRoomPulse_SeriesHasOneBucketPerWindowStep(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)

	for _, w := range db.RoomStatsWindows {
		pulse, err := repo.GetRoomPulse(ctx, w)
		require.NoError(t, err)
		assert.Len(t, pulse.Stats.Series, w.Buckets, "window %q series length", w.Value)
		assert.Equal(t, w.Value, pulse.Window.Value)

		for i := 1; i < len(pulse.Stats.Series); i++ {
			assert.True(t,
				pulse.Stats.Series[i].BucketStart.After(pulse.Stats.Series[i-1].BucketStart),
				"window %q buckets run oldest to newest", w.Value)
		}
	}
}

func TestGetRoomPulse_KeepsAFixedTwentyFourHourMessageCountForTheAPISection(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)

	day, err := repo.GetRoomPulse(ctx, mustWindow(t, "24h"))
	require.NoError(t, err)
	month, err := repo.GetRoomPulse(ctx, mustWindow(t, "30d"))
	require.NoError(t, err)

	assert.Equal(t, day.Messages24h, month.Messages24h,
		"the API-usage section states 'last 24 hours'; the room selector must not rewrite it")
}

func mustWindow(t *testing.T, value string) db.RoomStatsWindow {
	t.Helper()
	w, ok := db.RoomStatsWindowByValue(value)
	require.True(t, ok, "unknown window %q", value)
	return w
}

func countActivations(t *testing.T, ctx context.Context, pool *db.Pool, roomID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM room_events WHERE room_id = $1 AND event_type = $2
	`, roomID, db.RoomActivationEventType).Scan(&n)
	require.NoError(t, err)
	return n
}
