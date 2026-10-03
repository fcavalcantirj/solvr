package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Opt-in room notifications (idx 92 step 3; schema version 3). A reply or a requested review
// is recorded only for a recipient who opted in to the room, has not paused room
// notifications, can read the room and is not the author — once per (entry, type, recipient),
// with the agent's subscribed webhooks queued in the same statement.

type roomNoticeSeed struct {
	pool  *Pool
	repo  *RoomNotificationRepository
	room  *models.Room
	human uuid.UUID
	other uuid.UUID
	agent string
	peer  string
}

func seedRoomNotices(t *testing.T, private bool) roomNoticeSeed {
	t.Helper()
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	user := func(tag string) uuid.UUID {
		var id uuid.UUID
		name := "rn" + tag + uuid.NewString()[:6]
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
			VALUES ($1, $1, $2, 'email', $1, $3) RETURNING id`, name, name+"@example.test", uuid.NewString()[:8]).Scan(&id))
		return id
	}
	agent := func(tag string) string {
		id := "agent_rn_" + tag + "_" + uuid.NewString()[:6]
		_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')`, id)
		require.NoError(t, err)
		return id
	}
	s := roomNoticeSeed{pool: pool, repo: NewRoomNotificationRepository(pool), human: user("h"), other: user("o"),
		agent: agent("a"), peer: agent("p")}
	room, err := NewRoomRepository(pool).Create(ctx, models.CreateRoomParams{
		Slug: "rn-" + uuid.NewString()[:8], DisplayName: "Notices", IsPrivate: private, OwnerID: s.human, CreatorAgentID: s.agent,
	})
	require.NoError(t, err)
	s.room = room
	return s
}

func (s roomNoticeSeed) post(t *testing.T, authorType, authorID, body string, replyTo *int64, addressed ...string) *models.Message {
	t.Helper()
	params := models.CreateMessageParams{RoomID: s.room.ID, AuthorType: authorType, AuthorID: &authorID,
		AgentName: authorID, Content: body, ContentType: "text", ReplyToEntryID: replyTo}
	if len(addressed) > 0 {
		raw, _ := json.Marshal(addressed)
		params.AddressedMemberIDs = raw
	}
	msg, err := NewMessageRepository(s.pool).Create(context.Background(), params)
	require.NoError(t, err)
	return msg
}

func (s roomNoticeSeed) notice(m *models.Message, addressed ...string) RoomEntryNotice {
	return RoomEntryNotice{Room: s.room, EntryID: m.ID, Kind: models.RoomEntryKindMessage,
		AuthorType: m.AuthorType, AuthorID: *m.AuthorID, Label: m.AgentName, ReplyToEntryID: m.ReplyToEntryID, Addressed: addressed}
}

func (s roomNoticeSeed) noticesOf(t *testing.T, column, id string) []models.Notification {
	t.Helper()
	rows, err := s.pool.Query(context.Background(), `SELECT `+notificationColumns+` FROM notifications WHERE `+column+` = $1 ORDER BY created_at`, id)
	require.NoError(t, err)
	defer rows.Close()
	var out []models.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		require.NoError(t, err)
		out = append(out, n)
	}
	return out
}

func human(id uuid.UUID) NotificationSubscriber { return NotificationSubscriber{UserID: &id} }
func agentSub(id string) NotificationSubscriber { return NotificationSubscriber{AgentID: id} }

func TestRoomNotifications_OptInPauseAndTheReplyRules(t *testing.T) {
	s := seedRoomNotices(t, false)
	ctx := context.Background()

	// Off by default; subscribing is idempotent; unsubscribing turns it off.
	on, err := s.repo.IsSubscribed(ctx, s.room.ID, human(s.human))
	require.NoError(t, err)
	require.False(t, on, "notifications are opt-in")
	require.NoError(t, s.repo.Subscribe(ctx, s.room.ID, human(s.human)))
	require.NoError(t, s.repo.Subscribe(ctx, s.room.ID, human(s.human)))
	on, _ = s.repo.IsSubscribed(ctx, s.room.ID, human(s.human))
	require.True(t, on)

	mine := s.post(t, "human", s.human.String(), "my question", nil)
	reply := s.post(t, "agent", s.agent, "an answer", &mine.ID)

	n, err := s.repo.RecordForEntry(ctx, s.notice(reply))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	got := s.noticesOf(t, "user_id", s.human.String())
	require.Len(t, got, 1)
	require.Equal(t, models.NotificationRoomReply, got[0].Type)
	require.Equal(t, models.NotificationRoomEntrySchemaVersion, got[0].SchemaVersion)
	require.Equal(t, s.room.ID.String(), *got[0].Subject.RoomID)
	require.Equal(t, reply.ID, *got[0].Subject.EntryID)
	require.NotContains(t, got[0].Body, "an answer", "a notification never copies the message body")
	require.Contains(t, got[0].Body, "/v1/rooms/"+s.room.Slug+"/notifications", "the per-room off is named in every event")
	require.Contains(t, got[0].Link, "/rooms/"+s.room.Slug)

	n, err = s.repo.RecordForEntry(ctx, s.notice(reply))
	require.NoError(t, err)
	require.Zero(t, n, "once per entry, type and recipient")

	// The author is never notified about its own entry.
	self := s.post(t, "human", s.human.String(), "replying to myself", &mine.ID)
	n, _ = s.repo.RecordForEntry(ctx, s.notice(self))
	require.Zero(t, n)

	// The global pause wins over the subscription, and lifting it restores delivery.
	require.NoError(t, s.repo.SetPaused(ctx, human(s.human), true))
	paused, _ := s.repo.IsPaused(ctx, human(s.human))
	require.True(t, paused)
	again := s.post(t, "agent", s.agent, "another answer", &mine.ID)
	n, _ = s.repo.RecordForEntry(ctx, s.notice(again))
	require.Zero(t, n, "paused: nothing is recorded")
	require.NoError(t, s.repo.SetPaused(ctx, human(s.human), false))

	// Unsubscribed: nothing; a person who never opted in: nothing.
	require.NoError(t, s.repo.Unsubscribe(ctx, s.room.ID, human(s.human)))
	later := s.post(t, "agent", s.agent, "a third answer", &mine.ID)
	n, _ = s.repo.RecordForEntry(ctx, s.notice(later))
	require.Zero(t, n)
	theirs := s.post(t, "human", s.other.String(), "their question", nil)
	toThem := s.post(t, "agent", s.agent, "answer to them", &theirs.ID)
	n, _ = s.repo.RecordForEntry(ctx, s.notice(toThem))
	require.Zero(t, n, "not opted in")
}

func TestRoomNotifications_AddressedAgentsReviewRequestsWebhooksAndReadability(t *testing.T) {
	s := seedRoomNotices(t, true)
	ctx := context.Background()
	hooks := newWebhookTestRepo(s.pool)
	hook := createTestWebhook(t, hooks, s.peer, models.WebhookStatusActive, models.NotificationRoomReply, models.NotificationRoomReviewRequested)

	// peer is subscribed. While it is a member, a reply to its entry reaches it; once removed
	// from this private room it can no longer read it, so it is no longer told about it.
	require.NoError(t, s.repo.Subscribe(ctx, s.room.ID, agentSub(s.peer)))
	_, err := s.pool.Exec(ctx, `INSERT INTO room_members (room_id, agent_id, role, added_by) VALUES ($1, $2, 'member', 'test')`, s.room.ID, s.peer)
	require.NoError(t, err)
	peerPost := s.post(t, "agent", s.peer, "peer: done with step 1", nil)
	_, err = s.pool.Exec(ctx, `UPDATE room_members SET revoked_at = NOW() WHERE room_id = $1 AND agent_id = $2`, s.room.ID, s.peer)
	require.NoError(t, err)
	afterRemoval := s.post(t, "agent", s.agent, "thanks, peer", &peerPost.ID)
	n, err := s.repo.RecordForEntry(ctx, s.notice(afterRemoval))
	require.NoError(t, err)
	require.Zero(t, n, "a private room notifies only those who can read it")

	_, err = s.pool.Exec(ctx, `UPDATE room_members SET revoked_at = NULL WHERE room_id = $1 AND agent_id = $2`, s.room.ID, s.peer)
	require.NoError(t, err)
	directed2 := s.post(t, "agent", s.agent, "peer, now take step 2", nil, s.peer)
	n, err = s.repo.RecordForEntry(ctx, s.notice(directed2, s.peer))
	require.NoError(t, err)
	require.Equal(t, 1, n, "addressing a member is a reply to it")
	got := s.noticesOf(t, "agent_id", s.peer)
	require.Len(t, got, 1)
	require.Equal(t, map[uuid.UUID]string{hook.ID: models.NotificationRoomReply}, deliveriesOf(t, s.pool, got[0].ID),
		"the agent's subscribed webhook is queued with the event")
	var version int
	var data string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT schema_version, data::text FROM webhook_deliveries WHERE notification_id = $1`, got[0].ID).Scan(&version, &data))
	require.Equal(t, 3, version)
	require.Contains(t, data, `"entry_id":`)

	// A review request reaches every subscriber who can read the room, but not its author.
	require.NoError(t, s.repo.Subscribe(ctx, s.room.ID, human(s.human)))
	require.NoError(t, s.repo.Subscribe(ctx, s.room.ID, agentSub(s.agent)))
	ev, _, err := NewRoomEntryRepository(s.pool).Create(ctx, models.CreateRoomEntryParams{
		RoomID: s.room.ID, Kind: models.RoomEntryKindEvent, AuthorType: strPtr("agent"), AuthorID: &s.agent,
		ActorLabel: s.agent, EventType: strPtr("Review.Requested"),
	})
	require.NoError(t, err)
	n, err = s.repo.RecordForEntry(ctx, RoomEntryNotice{Room: s.room, EntryID: ev.ID, Kind: models.RoomEntryKindEvent,
		EventType: "Review.Requested", AuthorType: "agent", AuthorID: s.agent, Label: s.agent})
	require.NoError(t, err)
	require.Equal(t, 2, n, "the owner human and the member peer; not the author")
	require.Len(t, s.noticesOf(t, "user_id", s.human.String()), 1)
	require.Equal(t, models.NotificationRoomReviewRequested, s.noticesOf(t, "user_id", s.human.String())[0].Type)
	require.Empty(t, s.noticesOf(t, "agent_id", s.agent))

	// Any other event notifies nobody.
	other, _, err := NewRoomEntryRepository(s.pool).Create(ctx, models.CreateRoomEntryParams{
		RoomID: s.room.ID, Kind: models.RoomEntryKindEvent, AuthorType: strPtr("agent"), AuthorID: &s.agent,
		ActorLabel: s.agent, EventType: strPtr("DONE"),
	})
	require.NoError(t, err)
	n, _ = s.repo.RecordForEntry(ctx, RoomEntryNotice{Room: s.room, EntryID: other.ID, Kind: models.RoomEntryKindEvent,
		EventType: "DONE", AuthorType: "agent", AuthorID: s.agent, Label: s.agent})
	require.Zero(t, n)
}
