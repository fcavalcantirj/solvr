package services

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: the post moderation verdict is a post.approved / post.rejected event under schema
// version 1 that names its canonical post.
func TestNotifyOnModerationResult_NamesThePostUnderTheEventContract(t *testing.T) {
	for _, approved := range []bool{true, false} {
		var captured *NotificationInput
		repo := &MockNotificationRepository{
			createFunc: func(_ context.Context, n *NotificationInput) (*NotificationRecord, error) {
				captured = n
				return &NotificationRecord{ID: "n1", Type: n.Type}, nil
			},
		}
		svc := NewNotificationService(repo, nil, nil, nil, nil)
		postID := "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11"
		require.NoError(t, svc.NotifyOnModerationResult(context.Background(), postID, "Title", "post", "agent", "agent_x", approved, "why"))

		require.NotNil(t, captured)
		require.Equal(t, models.NotificationSchemaVersion, captured.SchemaVersion, "approved=%v", approved)
		require.Equal(t, models.NotificationSubject{PostID: &postID}, captured.Subject, "approved=%v", approved)
		require.Equal(t, "/posts/"+postID, captured.Link)
	}
}

// The legacy notify paths are not under the contract: their events stay at version 0.
func TestCreateNotification_WithoutTheContractStaysVersionZero(t *testing.T) {
	var captured *NotificationInput
	repo := &MockNotificationRepository{
		createFunc: func(_ context.Context, n *NotificationInput) (*NotificationRecord, error) {
			captured = n
			return &NotificationRecord{ID: "n1"}, nil
		},
	}
	agent := "agent_x"
	_, err := NewNotificationService(repo, nil, nil, nil, nil).CreateNotification(context.Background(),
		&CreateNotificationParams{AgentID: &agent, Type: NotificationTypeMention, Title: "t"})
	require.NoError(t, err)
	require.Equal(t, 0, captured.SchemaVersion)
	require.Equal(t, models.NotificationSubject{}, captured.Subject)
}
