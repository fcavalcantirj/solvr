package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: a webhook subscribes to the notification events of schema version 1, a retired
// event name answers a migration error, and the agent manages its own webhooks with its key.

func agentWebhookRequest(method, body string, caller *models.Agent) *http.Request {
	req := httptest.NewRequest(method, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	return req.WithContext(auth.ContextWithAgent(req.Context(), caller))
}

func TestCreateWebhook_TheAgentSubscribesItselfAndItsSecretIsKeptForSigning(t *testing.T) {
	var stored *models.Webhook
	repo := &MockWebhookRepository{
		FindAgentFunc: func(_ context.Context, id string) (*models.Agent, error) {
			return &models.Agent{ID: id}, nil // unclaimed: no owner
		},
		CreateFunc: func(_ context.Context, w *models.Webhook) error {
			w.ID = uuid.New()
			stored = w
			return nil
		},
	}
	w := httptest.NewRecorder()
	NewWebhooksHandler(repo).CreateWebhook(w, agentWebhookRequest(http.MethodPost,
		`{"url":"https://example.com/hook","events":["reply.removed","post.approved"],"secret":"whsec-self"}`,
		&models.Agent{ID: "test_agent"}), "test_agent")

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, "whsec-self", stored.Secret, "the repository gets the secret to seal")
	require.NotEmpty(t, stored.SecretHash)
	require.NotContains(t, w.Body.String(), "whsec-self")

	other := httptest.NewRecorder()
	NewWebhooksHandler(repo).CreateWebhook(other, agentWebhookRequest(http.MethodPost,
		`{"url":"https://example.com/hook","events":["reply.removed"],"secret":"s"}`,
		&models.Agent{ID: "another_agent"}), "test_agent")
	require.Equal(t, http.StatusForbidden, other.Code, other.Body.String())
}

func TestCreateWebhook_ARetiredEventNameAnswersTheMigrationError(t *testing.T) {
	repo := &MockWebhookRepository{
		FindAgentFunc: func(_ context.Context, id string) (*models.Agent, error) { return &models.Agent{ID: id}, nil },
		CreateFunc: func(context.Context, *models.Webhook) error {
			t.Fatal("a refused subscription is not stored")
			return nil
		},
	}
	for _, path := range []string{"create", "update"} {
		w := httptest.NewRecorder()
		h := NewWebhooksHandler(repo)
		if path == "create" {
			h.CreateWebhook(w, agentWebhookRequest(http.MethodPost,
				`{"url":"https://example.com/hook","events":["post.approved","problem.solved"],"secret":"s"}`,
				&models.Agent{ID: "test_agent"}), "test_agent")
		} else {
			hook := &models.Webhook{ID: uuid.New(), AgentID: "test_agent", Events: []string{"post.approved"}}
			repo.FindByIDFunc = func(context.Context, uuid.UUID) (*models.Webhook, error) { return hook, nil }
			h.UpdateWebhook(w, agentWebhookRequest(http.MethodPatch, `{"events":["problem.solved"]}`,
				&models.Agent{ID: "test_agent"}), "test_agent", hook.ID.String())
		}
		require.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", path, w.Body.String())
		var resp struct {
			Error struct {
				Code    string         `json:"code"`
				Message string         `json:"message"`
				Details map[string]any `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "EVENT_RETIRED", resp.Error.Code, path)
		require.Contains(t, resp.Error.Message, "problem.solved")
		require.Equal(t, "problem.solved", resp.Error.Details["retired_event"])
		require.Nil(t, resp.Error.Details["replacement"])
		require.Equal(t, []any{"post.approved", "post.rejected", "reply.removed", "reply.flagged", "blog_post_rejected",
			"room.member_added", "room.member_removed", "room.reply", "room.review_requested"}, resp.Error.Details["supported_events"])
	}
}
