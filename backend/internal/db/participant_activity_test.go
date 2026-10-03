package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The monthly-active-participant counter (spec.json idx 86) runs on its own scratch database,
// so every count below is exact: no other test's rows can move it. The legacy contribution
// tables are dropped first, the way schema cleanup will drop them, so the counter is proven to
// read only canonical records.
//
// One scenario carries every case the definition names: an agent posting from two sessions is
// one identity; an invited or referred human who never acts is not a participant; registration,
// heartbeat, presence and monitoring are not activity; tombstoned, banned and suspended
// identities are excluded; anonymous activity is counted as events beside the identities, and
// an anonymous flow joins an identity only through the server-issued flow_id.

type participantFixture struct {
	t    *testing.T
	ctx  context.Context
	pool *Pool
	end  time.Time
	n    int64
}

func (f *participantFixture) ago(days float64) time.Time {
	return f.end.Add(-time.Duration(days * float64(24*time.Hour)))
}

func (f *participantFixture) user(label string) string {
	f.t.Helper()
	u, err := NewUserRepository(f.pool).Create(f.ctx, &models.User{
		Username: fmt.Sprintf("pa%s%d", label, f.n), DisplayName: "Participant " + label,
		Email: fmt.Sprintf("pa%s%d@example.com", label, f.n), AuthProvider: models.AuthProviderGitHub,
		AuthProviderID: fmt.Sprintf("gh_pa%s%d", label, f.n), Role: models.UserRoleUser,
	})
	require.NoError(f.t, err)
	return u.ID
}

func (f *participantFixture) agent(label string) string {
	f.t.Helper()
	id := fmt.Sprintf("agent_pa_%s_%d", label, f.n)
	_, err := f.pool.Exec(f.ctx, `INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $1, $2, 'active')`, id, "hash_"+id)
	require.NoError(f.t, err)
	return id
}

func (f *participantFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *participantFixture) post(kind, id string, at time.Time) string {
	f.t.Helper()
	var postID string
	require.NoError(f.t, f.pool.QueryRow(f.ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, created_at)
		VALUES ('question', $1, 'A post written by a participant fixture.', $2, $3, 'open', $4)
		RETURNING id`, "Participant post "+id, kind, id, at).Scan(&postID))
	return postID
}

func (f *participantFixture) room(slug string) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	require.NoError(f.t, f.pool.QueryRow(f.ctx, `
		INSERT INTO rooms (slug, display_name, is_private) VALUES ($1, $2, false) RETURNING id`,
		fmt.Sprintf("%s-%d", slug, f.n), "Participant room "+slug).Scan(&id))
	return id
}

func (f *participantFixture) message(room uuid.UUID, seq int, kind, id, label string, at time.Time) {
	f.exec(`INSERT INTO room_entries (room_id, sequence, kind, author_type, author_id, actor_label, body, created_at)
		VALUES ($1, $2, 'message', $3, $4, $5, 'a real message', $6)`, room, seq, kind, id, label, at)
}

func (f *participantFixture) search(kind, id, userAgent string, at time.Time) {
	var searcher any
	if id != "" {
		searcher = id
	}
	f.exec(`INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms,
			searcher_type, searcher_id, user_agent, searched_at)
		VALUES ('postgres deadlock', 'postgres deadlock', 3, 'hybrid', 12, $1, $2, $3, $4)`,
		kind, searcher, userAgent, at)
}

func (f *participantFixture) funnel(flow any, event, channel, actorType string, actorRef any, room any, at time.Time) {
	f.exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, actor_ref, room_id, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, flow, event, channel, actorType, actorRef, room, at)
}

func TestParticipantActivity_CountsVerifiedIdentitiesOnce(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	dropLegacy()
	ctx := context.Background()
	f := &participantFixture{t: t, ctx: ctx, pool: pool,
		end: time.Now().UTC().Add(time.Minute).Truncate(time.Second), n: time.Now().UnixNano() % 1_000_000_000}
	// The window ends a minute from now, so accounts the fixture creates NOW fall inside it.

	// --- Humans -----------------------------------------------------------------------------
	hReader, hSearcher, hAuthor, hRoom := f.user("reader"), f.user("searcher"), f.user("author"), f.user("room")
	hInvited, hRegistered, hDeleted, hOld := f.user("invited"), f.user("registered"), f.user("deleted"), f.user("old")

	readPost := f.post("human", hAuthor, f.ago(3)) // hAuthor creates: active
	f.exec(`INSERT INTO post_views (post_id, viewer_type, viewer_id, viewed_at) VALUES ($1, 'human', $2, $3)`,
		readPost, hReader, f.ago(5)) // hReader reads: active
	f.search("human", hSearcher, "Mozilla/5.0", f.ago(2))  // active
	f.search("human", hSearcher, "Mozilla/5.0", f.ago(40)) // and in the previous window: returning
	f.post("human", hDeleted, f.ago(2))
	f.exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, hDeleted) // tombstoned: excluded
	f.post("human", hOld, f.ago(31))                                      // previous window only

	roomA, roomB := f.room("pa-room-a"), f.room("pa-room-b")
	f.message(roomA, 1, "human", hRoom, "human:"+hRoom, f.ago(1)) // active

	// An invitation is not a participant: a referral and an owner-added membership, no action.
	f.exec(`INSERT INTO referrals (referrer_id, referred_id) VALUES ($1, $2)`, hAuthor, hInvited)
	f.exec(`INSERT INTO room_members (room_id, user_id, role, added_by, access_source) VALUES ($1, $2, 'member', $3, 'direct')`,
		roomA, hInvited, hAuthor)
	_ = hRegistered // registration only: not a participant

	// --- Agents -----------------------------------------------------------------------------
	aTwoSessions, aClaimed, aSearcher, aCreator, aVoter := f.agent("two"), f.agent("claimed"), f.agent("searcher"), f.agent("creator"), f.agent("voter")
	aMonitor, aHeartbeat, aSuspended, aBanned, aTombstoned := f.agent("monitor"), f.agent("heartbeat"), f.agent("suspended"), f.agent("banned"), f.agent("tomb")

	// Two CLI sessions of one agent: two display names, one authenticated identity.
	f.message(roomA, 2, "agent", aTwoSessions, "planner-session", f.ago(2))
	f.message(roomA, 3, "agent", aTwoSessions, "executor-session", f.ago(2))

	f.exec(`UPDATE agents SET human_id = $1, human_claimed_at = NOW() WHERE id = $2`, hAuthor, aClaimed)
	f.exec(`INSERT INTO replies (post_id, author_type, author_id, body, created_at) VALUES ($1, 'agent', $2, 'a real reply', $3)`,
		readPost, aClaimed, f.ago(4))

	f.search("agent", aSearcher, "solvr-cli/1.4", f.ago(3))
	f.search("agent", aMonitor, "UptimeRobot/2.0", f.ago(3)) // known monitoring: excluded

	creatorRef := PseudonymizeActor(aCreator)
	f.funnel("f_attributed", "room_created", "server", "agent", creatorRef, roomB, f.ago(1)) // room action: active

	f.exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed, created_at) VALUES ('post', $1, 'agent', $2, 'up', true, $3)`,
		readPost, aVoter, f.ago(6))

	// Heartbeat, briefing and presence only: a membership an owner granted, liveness, nothing done.
	f.exec(`UPDATE agents SET last_seen_at = NOW(), last_briefing_at = NOW() WHERE id = $1`, aHeartbeat)
	f.exec(`INSERT INTO room_members (room_id, agent_id, role, added_by, access_source) VALUES ($1, $2, 'member', $3, 'direct')`,
		roomA, aHeartbeat, hAuthor)
	f.exec(`INSERT INTO agent_presence (room_id, agent_id, agent_name, card_json, last_seen) VALUES ($1, $2, 'hb', '{}', NOW())`,
		roomA, aHeartbeat)

	f.post("agent", aSuspended, f.ago(2))
	f.exec(`UPDATE agents SET status = 'suspended' WHERE id = $1`, aSuspended)
	f.message(roomB, 1, "agent", aBanned, "banned", f.ago(2))
	f.exec(`INSERT INTO banned_identities (kind, value, provider, source_type, reason) VALUES ('agent_id', $1, '', 'test', 'spam')`, aBanned)
	f.exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed) VALUES ('post', $1, 'agent', $2, 'down', true)`,
		readPost, aTombstoned)
	f.exec(`UPDATE agents SET deleted_at = NOW() WHERE id = $1`, aTombstoned)

	// --- Anonymous --------------------------------------------------------------------------
	f.search("anonymous", "", "Mozilla/5.0", f.ago(1))
	f.search("anonymous", "", "Pingdom.com_bot", f.ago(1)) // monitoring
	f.exec(`INSERT INTO post_views (post_id, viewer_type, viewer_id, viewed_at) VALUES ($1, 'anonymous', NULL, $2)`, readPost, f.ago(1))
	f.funnel("f_attributed", "connection_started", "browser", "anonymous", nil, nil, f.ago(2))
	f.funnel("f_anonymous", "connection_started", "browser", "anonymous", nil, nil, f.ago(3))
	f.funnel(nil, "room_viewed", "browser", "anonymous", nil, nil, f.ago(3))

	// --- Traffic ----------------------------------------------------------------------------
	for _, r := range []struct {
		actor, kind string
		at          time.Time
	}{{"agent", "poll", f.ago(1)}, {"human", "write", f.ago(2)}, {"anonymous", "search", f.ago(3)}, {"agent", "write", f.ago(45)}} {
		f.exec(`INSERT INTO api_request_events (route_template, method, operation, operation_family, operation_kind, actor_type, status_class, occurred_at)
			VALUES ('/v1/search', 'GET', 'search', 'knowledge', $1, $2, 2, $3)`, r.kind, r.actor, r.at)
	}
	f.funnel("f_attributed", "first_two_way_exchange", "server", "agent", nil, roomB, f.ago(1))

	m, err := NewParticipantActivityRepository(pool).Measure(ctx, f.end)
	require.NoError(t, err)

	assert.Equal(t, f.end, m.WindowEnd)
	assert.Equal(t, f.ago(30), m.WindowStart)
	assert.Equal(t, f.ago(60), m.PreviousWindowStart)

	// hReader, hSearcher, hAuthor, hRoom. Not: invited, registered, deleted, the 31-day-old author.
	assert.Equal(t, 4, m.Humans, "active humans")
	// aTwoSessions (once), aClaimed, aSearcher, aCreator, aVoter. Not: monitor, heartbeat,
	// suspended, banned, tombstoned.
	assert.Equal(t, 5, m.Agents, "active agent identities")
	assert.Equal(t, 2, m.PreviousHumans, "hSearcher's older search and hOld's post")
	assert.Equal(t, 0, m.PreviousAgents)
	assert.Equal(t, 1, m.ReturningHumans, "hSearcher is active in both windows")
	assert.Equal(t, 0, m.ReturningAgents)

	assert.Equal(t, 1, m.KnownOverlap, "aClaimed belongs to hAuthor, who is also active")
	assert.Equal(t, 4, m.UnresolvedOverlap, "four active agents no human has claimed")

	actions := map[string]growth.ActionIdentities{}
	for _, a := range m.ByAction {
		actions[a.Action] = a
	}
	assert.Equal(t, growth.ActionIdentities{Action: "read", Humans: 1, Agents: 0}, actions["read"])
	assert.Equal(t, growth.ActionIdentities{Action: "search", Humans: 1, Agents: 1}, actions["search"])
	assert.Equal(t, growth.ActionIdentities{Action: "create", Humans: 1, Agents: 1}, actions["create"])
	assert.Equal(t, growth.ActionIdentities{Action: "participate", Humans: 0, Agents: 1}, actions["participate"])
	assert.Equal(t, growth.ActionIdentities{Action: "room", Humans: 1, Agents: 2}, actions["room"])

	assert.Equal(t, growth.AnonymousMeasures{
		Searches: 1, PostViews: 1, BrowserSteps: 3, Flows: 2, FlowsAttributedToIdentity: 1,
	}, m.Anonymous)

	tr := m.Traffic
	assert.Equal(t, 3, tr.APIRequests, "the 45-day-old request is outside the window")
	assert.Equal(t, map[string]int{"agent": 1, "human": 1, "anonymous": 1}, tr.APIRequestsByActor)
	assert.Equal(t, map[string]int{"poll": 1, "write": 1, "search": 1}, tr.APIRequestsByKind)
	assert.Equal(t, map[string]int{"human": 1, "agent": 1, "anonymous": 1}, tr.SearchesBySearcher)
	assert.Equal(t, 2, tr.MonitoringSearches)
	assert.Equal(t, 2, tr.PostViewsRecorded)
	assert.Equal(t, 2, tr.ActiveRooms)
	assert.Equal(t, 1, tr.Activations)
	assert.Equal(t, 7, tr.HumanRegistrations, "eight users created, one tombstoned")
	assert.Equal(t, 9, tr.AgentRegistrations, "ten agents created, one tombstoned")
}
