package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4, room events: a room membership event names its canonical room in subject.room_id,
// under schema version 2 (SPEC.md Part 5.6: new event types and subject fields come under a
// new version; the version 1 events keep version 1). The room is an enforced relation, and
// the event is queued for the agent's webhooks subscribed to it like any other event. Each
// test runs on its own scratch database.

type roomEventSeed struct {
	agentID, roomID, slug string
}

func seedRoomEvent(t *testing.T, pool *Pool) roomEventSeed {
	t.Helper()
	ctx := context.Background()
	s := roomEventSeed{agentID: "agent_roomev_" + uuid.NewString()[:8], slug: "room-ev-" + uuid.NewString()[:8]}
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')`, s.agentID)
	require.NoError(t, err)
	room, err := NewRoomRepository(pool).Create(ctx, models.CreateRoomParams{Slug: s.slug, DisplayName: "Room " + s.slug, IsPrivate: true})
	require.NoError(t, err)
	s.roomID = room.ID.String()
	return s
}

func memberAddedEvent(s roomEventSeed, roomID *string) *models.Notification {
	return &models.Notification{
		AgentID: &s.agentID, Type: models.NotificationRoomMemberAdded, Title: "You were added to a room",
		Body: "You were added as a member", Link: "/rooms/" + s.slug,
		SchemaVersion: models.NotificationRoomSchemaVersion,
		Subject:       models.NotificationSubject{RoomID: roomID},
	}
}

func TestNotificationRoomEvents_ARoomEventIsVersionTwoAndNamesItsRoomOnEveryRead(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedRoomEvent(t, pool)
	repo := NewNotificationsRepository(pool)

	require.Equal(t, 2, models.NotificationRoomSchemaVersion)
	require.Equal(t, []int{1, 2}, models.NotificationSchemaVersions)

	created, err := repo.Create(ctx, memberAddedEvent(s, &s.roomID))
	require.NoError(t, err)
	check := func(where string, n models.Notification) {
		t.Helper()
		require.Equal(t, models.NotificationRoomMemberAdded, n.Type, where)
		require.Equal(t, 2, n.SchemaVersion, where)
		require.NotNil(t, n.Subject.RoomID, where)
		require.Equal(t, s.roomID, *n.Subject.RoomID, where)
		require.Nil(t, n.Subject.PostID, where)
		require.Nil(t, n.Subject.ReplyID, where)
	}
	check("create", *created)

	listed, total, err := repo.GetNotificationsForAgent(ctx, s.agentID, 1, 20, models.NotificationFilters{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	check("list", listed[0])
	recent, _, err := repo.GetRecentUnreadForAgent(ctx, s.agentID, 10)
	require.NoError(t, err)
	check("inbox", recent[0])
	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	check("find", *found)
	read, err := repo.MarkRead(ctx, created.ID)
	require.NoError(t, err)
	check("mark read", *read)

	raw, err := json.Marshal(created.Subject)
	require.NoError(t, err)
	require.JSONEq(t, `{"room_id":"`+s.roomID+`"}`, string(raw), "the subject names the room only")

	// A post event keeps version 1 and has no room.
	post := seedNotificationSubjects(t, pool)
	v1, err := repo.Create(ctx, replyRemovedEvent(post, &post.postID, &post.replyID))
	require.NoError(t, err)
	require.Equal(t, 1, v1.SchemaVersion)
	require.Nil(t, v1.Subject.RoomID)
}

func TestNotificationRoomEvents_TheRoomSubjectIsEnforced(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedRoomEvent(t, pool)
	repo := NewNotificationsRepository(pool)

	unknown := uuid.NewString()
	_, err := repo.Create(ctx, memberAddedEvent(s, &unknown))
	require.Error(t, err, "a room that does not exist is refused")

	underVersionOne := memberAddedEvent(s, &s.roomID)
	underVersionOne.SchemaVersion = models.NotificationSchemaVersion
	_, err = repo.Create(ctx, underVersionOne)
	require.Error(t, err, "subject.room_id exists under schema version 2 only")

	unknownVersion := memberAddedEvent(s, &s.roomID)
	unknownVersion.SchemaVersion = 3
	_, err = repo.Create(ctx, unknownVersion)
	require.Error(t, err, "no schema version 3 exists")

	var stored int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE agent_id = $1`, s.agentID).Scan(&stored))
	require.Zero(t, stored, "nothing refused was stored")
}

func TestNotificationRoomEvents_DeletingTheRoomKeepsTheNotification(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedRoomEvent(t, pool)
	repo := NewNotificationsRepository(pool)

	created, err := repo.Create(ctx, memberAddedEvent(s, &s.roomID))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, s.roomID)
	require.NoError(t, err)

	kept, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err, "the notification outlives its room")
	require.Nil(t, kept.Subject.RoomID, "a deleted room leaves the subject empty")
	require.Equal(t, 2, kept.SchemaVersion)
	require.Equal(t, "/rooms/"+s.slug, kept.Link)
}

func TestNotificationRoomEvents_ARoomEventIsQueuedForTheWebhooksSubscribedToIt(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedRoomEvent(t, pool)
	hooks := newWebhookTestRepo(pool)
	notifications := NewNotificationsRepository(pool)

	added := createTestWebhook(t, hooks, s.agentID, models.WebhookStatusActive,
		models.NotificationRoomMemberAdded, models.NotificationRoomMemberRemoved)
	removedOnly := createTestWebhook(t, hooks, s.agentID, models.WebhookStatusActive, models.NotificationRoomMemberRemoved)
	createTestWebhook(t, hooks, s.agentID, models.WebhookStatusActive, models.NotificationPostApproved)

	event, err := notifications.Create(ctx, memberAddedEvent(s, &s.roomID))
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]string{added.ID: models.NotificationRoomMemberAdded}, deliveriesOf(t, pool, event.ID),
		"only the webhook subscribed to room.member_added")

	var data string
	var version int
	require.NoError(t, pool.QueryRow(ctx, `SELECT data::text, schema_version FROM webhook_deliveries
		WHERE notification_id = $1 AND webhook_id = $2`, event.ID, added.ID).Scan(&data, &version))
	require.Equal(t, 2, version, "the delivery carries the event's schema version")
	require.JSONEq(t, `{"notification_id":"`+event.ID+`","agent_id":"`+s.agentID+`",
		"subject":{"room_id":"`+s.roomID+`"},
		"title":"You were added to a room","body":"You were added as a member","link":"/rooms/`+s.slug+`"}`, data)

	removed := memberAddedEvent(s, &s.roomID)
	removed.Type, removed.Title = models.NotificationRoomMemberRemoved, "You were removed from a room"
	gone, err := notifications.Create(ctx, removed)
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]string{added.ID: models.NotificationRoomMemberRemoved, removedOnly.ID: models.NotificationRoomMemberRemoved},
		deliveriesOf(t, pool, gone.ID))

	claimed, err := hooks.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 3)
	for _, d := range claimed {
		require.Equal(t, 2, d.SchemaVersion, "a claimed room delivery keeps version 2")
	}
}

func TestRoomMemberRepository_AddReportsWhenTheMembershipBecameActive(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedRoomEvent(t, pool)
	room, err := uuid.Parse(s.roomID)
	require.NoError(t, err)
	members := NewRoomMemberRepository(pool)
	owner := "agent_roomev_owner"
	_, err = pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')`, owner)
	require.NoError(t, err)
	_, err = members.Add(ctx, models.AddRoomMemberParams{RoomID: room, AgentID: owner, Role: models.RoleOwner, AddedBy: "system"})
	require.NoError(t, err)

	add := func(role string) *models.RoomMember {
		t.Helper()
		m, err := members.Add(ctx, models.AddRoomMemberParams{RoomID: room, AgentID: s.agentID, Role: role, AddedBy: owner})
		require.NoError(t, err)
		return m
	}
	require.True(t, add("").Admitted, "a new membership is an admission")
	require.False(t, add("").Admitted, "a re-add of an active member is not")
	require.False(t, add(models.RoleOwner).Admitted, "a role change of an active member is not")
	require.False(t, add(models.RoleMember).Admitted, "nor is a demotion")
	require.NoError(t, members.Remove(ctx, room, s.agentID))
	readmitted := add("")
	require.True(t, readmitted.Admitted, "a readmission after removal is an admission")
	require.Equal(t, models.RoleMember, readmitted.Role)

	raw, err := json.Marshal(readmitted)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "admitted", "the admission flag is never answered")
}
