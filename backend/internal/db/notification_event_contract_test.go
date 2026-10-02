package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: notification events name the canonical post and reply they are about, under a
// documented schema version. A notification written under the contract (schema_version 1)
// stores its subject as typed, enforced post_id/reply_id columns; a row written outside it
// (schema_version 0: retired producers, rows recorded before the contract) has no subject.
// Each test runs on its own scratch database.

type notificationSubjectSeed struct {
	agentID, postID, replyID, otherPostID string
}

func seedNotificationSubjects(t *testing.T, pool *Pool) notificationSubjectSeed {
	t.Helper()
	ctx := context.Background()
	s := notificationSubjectSeed{agentID: "agent_notif_" + uuid.NewString()[:8]}
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')`, s.agentID)
	require.NoError(t, err)
	post := func() string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state)
			VALUES ('post', $1, 'A post a notification event is about.', 'agent', $2, 'open', 'published', 'approved')
			RETURNING id::text`, "Notification subject "+uuid.NewString(), s.agentID).Scan(&id))
		return id
	}
	s.postID, s.otherPostID = post(), post()
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'agent', $2, 'a reply a notification event is about') RETURNING id::text`, s.postID, s.agentID).Scan(&s.replyID))
	return s
}

func replyRemovedEvent(s notificationSubjectSeed, postID, replyID *string) *models.Notification {
	return &models.Notification{
		AgentID: &s.agentID, Type: models.NotificationReplyRemoved, Title: "Your reply was removed",
		Body: "Your reply did not pass moderation", Link: "/posts/" + s.postID,
		SchemaVersion: models.NotificationSchemaVersion,
		Subject:       models.NotificationSubject{PostID: postID, ReplyID: replyID},
	}
}

func TestNotificationEventContract_EveryReadReturnsTheSchemaVersionAndSubject(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := NewNotificationsRepository(pool)

	created, err := repo.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)

	want := models.NotificationSubject{PostID: &s.postID, ReplyID: &s.replyID}
	requireEvent := func(where string, n models.Notification) {
		t.Helper()
		require.Equal(t, models.NotificationReplyRemoved, n.Type, where)
		require.Equal(t, 1, n.SchemaVersion, where)
		require.Equal(t, want, n.Subject, where)
	}
	requireEvent("Create", *created)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	requireEvent("FindByID", *found)

	list, total, err := repo.GetNotificationsForAgent(ctx, s.agentID, 1, 20, models.NotificationFilters{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	requireEvent("GetNotificationsForAgent", list[0])

	recent, unread, err := repo.GetRecentUnreadForAgent(ctx, s.agentID, 10)
	require.NoError(t, err)
	require.Equal(t, 1, unread)
	requireEvent("GetRecentUnreadForAgent", recent[0])

	read, err := repo.MarkRead(ctx, created.ID)
	require.NoError(t, err)
	requireEvent("MarkRead", *read)

	// A post-only event names the post and no reply.
	postOnly, err := repo.Create(ctx, &models.Notification{
		AgentID: &s.agentID, Type: models.NotificationPostRejected, Title: "Post needs changes",
		Link: "/posts/" + s.postID, SchemaVersion: models.NotificationSchemaVersion,
		Subject: models.NotificationSubject{PostID: &s.postID},
	})
	require.NoError(t, err)
	found, err = repo.FindByID(ctx, postOnly.ID)
	require.NoError(t, err)
	require.Equal(t, 1, found.SchemaVersion)
	require.Equal(t, models.NotificationSubject{PostID: &s.postID}, found.Subject)
}

func TestNotificationEventContract_RowsOutsideTheContractAreVersionZero(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := NewNotificationsRepository(pool)

	// A row recorded the way every notification was before the contract existed.
	var rawID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO notifications (agent_id, type, title, link)
		VALUES ($1, 'auto_solve_warning', 'legacy', '/problems/x') RETURNING id::text`, s.agentID).Scan(&rawID))
	raw, err := repo.FindByID(ctx, rawID)
	require.NoError(t, err)
	require.Equal(t, 0, raw.SchemaVersion)
	require.Equal(t, models.NotificationSubject{}, raw.Subject)

	// A producer that does not write the contract (a retired job) stays at version 0.
	legacy, err := repo.Create(ctx, &models.Notification{AgentID: &s.agentID, Type: "approach_abandonment_warning", Title: "legacy"})
	require.NoError(t, err)
	require.Equal(t, 0, legacy.SchemaVersion)
	require.Equal(t, models.NotificationSubject{}, legacy.Subject)
}

func TestNotificationEventContract_TheSubjectMustExistAndBelongTogether(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := NewNotificationsRepository(pool)
	missing := uuid.NewString()

	_, err := repo.Create(ctx, replyRemovedEvent(s, &missing, nil))
	require.Error(t, err, "a post that does not exist")
	_, err = repo.Create(ctx, replyRemovedEvent(s, &s.postID, &missing))
	require.Error(t, err, "a reply that does not exist")
	_, err = repo.Create(ctx, replyRemovedEvent(s, &s.otherPostID, &s.replyID))
	require.Error(t, err, "a reply named under a post it does not belong to")
	_, err = repo.Create(ctx, replyRemovedEvent(s, nil, &s.replyID))
	require.Error(t, err, "a reply without its post")

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE agent_id = $1`, s.agentID).Scan(&count))
	require.Equal(t, 0, count, "no refused event was stored")
}

func TestNotificationEventContract_DeletingTheSubjectKeepsTheNotification(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := NewNotificationsRepository(pool)

	created, err := repo.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM replies WHERE id = $1::uuid`, s.replyID)
	require.NoError(t, err)
	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, models.NotificationSubject{PostID: &s.postID}, found.Subject, "a hard-deleted reply leaves the post")

	var replyID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'agent', $2, 'another reply') RETURNING id::text`, s.postID, s.agentID).Scan(&replyID))
	second, err := repo.Create(ctx, replyRemovedEvent(s, &s.postID, &replyID))
	require.NoError(t, err)

	// A hard-deleted post takes its replies with it; both notifications stay, with no subject.
	_, err = pool.Exec(ctx, `DELETE FROM posts WHERE id = $1::uuid`, s.postID)
	require.NoError(t, err)
	for _, id := range []string{created.ID, second.ID} {
		found, err := repo.FindByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, models.NotificationSubject{}, found.Subject, id)
		require.Equal(t, 1, found.SchemaVersion, id)
	}
}
