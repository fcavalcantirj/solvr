package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Every reply surface that reaches a post answers 404 when the caller may not read that
// post, before any list, read or write happens. The sentinel errors prove the gate short-
// circuits: were the repository reached, the handler would surface them as a 500.
func TestReplies_HiddenPostIsNotFoundOnEverySurface(t *testing.T) {
	reached := errors.New("the repository must not be reached behind a hidden post")
	other := &models.ReplyWithAuthor{Reply: models.Reply{ID: "r1", PostID: "p1", AuthorType: models.AuthorTypeHuman, AuthorID: "someone-else", Body: "hidden body"}}
	cases := []struct {
		name, message string
		call          func(h *RepliesHandler, rec *httptest.ResponseRecorder)
	}{
		{"list", "post not found", func(h *RepliesHandler, rec *httptest.ResponseRecorder) {
			h.List(rec, agentReq(http.MethodGet, "/v1/posts/p1/replies", nil, map[string]string{"id": "p1"}))
		}},
		{"create", "post not found", func(h *RepliesHandler, rec *httptest.ResponseRecorder) {
			h.Create(rec, agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "a reply"}, map[string]string{"id": "p1"}))
		}},
		{"get", "reply not found", func(h *RepliesHandler, rec *httptest.ResponseRecorder) {
			h.Get(rec, agentReq(http.MethodGet, "/v1/replies/r1", nil, map[string]string{"id": "r1"}))
		}},
		{"vote", "reply not found", func(h *RepliesHandler, rec *httptest.ResponseRecorder) {
			h.Vote(rec, agentReq(http.MethodPost, "/v1/replies/r1/vote", map[string]any{"direction": "up"}, map[string]string{"id": "r1"}))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &MockRepliesRepository{postHidden: true, getResult: other, createErr: reached, voteErr: reached,
				pageResult: []models.ReplyWithAuthor{*other}, pageTotal: 1}
			rec := httptest.NewRecorder()
			tc.call(NewRepliesHandler(mock), rec)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"NOT_FOUND"`) || !strings.Contains(rec.Body.String(), tc.message) {
				t.Fatalf("body = %s, want NOT_FOUND %q", rec.Body.String(), tc.message)
			}
			if strings.Contains(rec.Body.String(), "hidden body") {
				t.Fatalf("a hidden reply leaked: %s", rec.Body.String())
			}
			if mock.lastPageParams != nil || mock.lastCreated != nil {
				t.Fatalf("the repository was reached behind a hidden post")
			}
		})
	}
}

// The check asks about the caller's family: an agent claimed by a human is scoped to that
// human, an unclaimed agent is public-only.
func TestReplies_VisibilityIsCheckedForTheCallersFamily(t *testing.T) {
	human := "human-7"
	for name, agent := range map[string]*models.Agent{
		"claimed agent":   {ID: "agent-1", HumanID: &human},
		"unclaimed agent": {ID: "agent-1"},
	} {
		t.Run(name, func(t *testing.T) {
			mock := &MockRepliesRepository{}
			req := agentReq(http.MethodGet, "/v1/posts/p1/replies", nil, map[string]string{"id": "p1"})
			req = req.WithContext(auth.ContextWithAgent(req.Context(), agent))
			rec := httptest.NewRecorder()
			NewRepliesHandler(mock).List(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			want := ""
			if agent.HumanID != nil {
				want = *agent.HumanID
			}
			if mock.visibleCaller == nil || *mock.visibleCaller != want {
				t.Fatalf("visibility checked for %v, want %q", mock.visibleCaller, want)
			}
		})
	}
}

// A failed visibility check is a server failure, not a hidden post.
func TestReplies_VisibilityCheckFailureIsAServerError(t *testing.T) {
	mock := &MockRepliesRepository{postVisibleErr: context.DeadlineExceeded}
	rec := httptest.NewRecorder()
	NewRepliesHandler(mock).List(rec, agentReq(http.MethodGet, "/v1/posts/p1/replies", nil, map[string]string{"id": "p1"}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}
