package handlers

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: a rejected reply's notification is the reply.removed event (reply.flagged when the
// reply could not be hidden) under schema version 1, naming its canonical post and reply. A
// retired legacy kind keeps its contribution_* notification outside the contract.

type eventModerator struct{ result *ModerationResult }

func (m eventModerator) ModerateContent(context.Context, ModerationInput) (*ModerationResult, error) {
	return m.result, nil
}

type eventStore struct{ hideErr error }

func (s eventStore) Hide(context.Context, string, string) error { return s.hideErr }
func (s eventStore) ParentPost(context.Context, string, string) (string, string, string, error) {
	return "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11", "post", "Parent title", nil
}
func (s eventStore) CreateFlag(_ context.Context, f *models.Flag) (*models.Flag, error) {
	return f, nil
}

func moderateForEvent(t *testing.T, kind string, hideErr error) *models.Notification {
	t.Helper()
	var got *models.Notification
	m := NewContributionModerator(eventModerator{&ModerationResult{Approved: false, Explanation: "spam"}}, eventStore{hideErr},
		func(_ context.Context, n *models.Notification) (*models.Notification, error) { got = n; return n, nil })
	m.moderate(kind, "0d4c3f0e-8a7b-4c1d-9e2f-3a4b5c6d7e8f", "body", "agent", "agent_x")
	require.NotNil(t, got, "the author is notified")
	return got
}

func TestContributionModeration_RejectedReplyIsTheReplyRemovedEvent(t *testing.T) {
	post, reply := "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11", "0d4c3f0e-8a7b-4c1d-9e2f-3a4b5c6d7e8f"

	n := moderateForEvent(t, "reply", nil)
	require.Equal(t, models.NotificationReplyRemoved, n.Type)
	require.Equal(t, models.NotificationSchemaVersion, n.SchemaVersion)
	require.Equal(t, models.NotificationSubject{PostID: &post, ReplyID: &reply}, n.Subject)
	require.Equal(t, "/posts/"+post, n.Link)

	n = moderateForEvent(t, "reply", db.ErrNotHideable)
	require.Equal(t, models.NotificationReplyFlagged, n.Type)
	require.Equal(t, models.NotificationSchemaVersion, n.SchemaVersion)
	require.Equal(t, models.NotificationSubject{PostID: &post, ReplyID: &reply}, n.Subject)
}

func TestContributionModeration_LegacyKindStaysOutsideTheContract(t *testing.T) {
	n := moderateForEvent(t, "answer", nil)
	require.Equal(t, "contribution_removed", n.Type)
	require.Equal(t, 0, n.SchemaVersion)
	require.Equal(t, models.NotificationSubject{}, n.Subject)

	n = moderateForEvent(t, "progress_note", db.ErrNotHideable)
	require.Equal(t, "contribution_flagged", n.Type)
	require.Equal(t, 0, n.SchemaVersion)
}
