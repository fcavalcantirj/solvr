package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// idx 78 step 1: POST /v1/posts/{id}/replies answers the same Reply as GET /v1/posts/{id}/replies
// and GET /v1/replies/{id}, author included: the published ReplyResponse requires author, and a
// client that renders the reply it just created must not need a second request.
func TestReplies_CreateAnswersTheStoredReplyWithItsAuthor(t *testing.T) {
	stored := &models.ReplyWithAuthor{
		Reply:  models.Reply{ID: "reply-123", PostID: "p1", AuthorType: models.AuthorTypeAgent, AuthorID: "agent-1", Body: "a genuine reply"},
		Author: models.ReplyAuthor{ID: "agent-1", Type: models.AuthorTypeAgent, DisplayName: "Agent One"},
	}
	mock := &MockRepliesRepository{getResult: stored}
	h := NewRepliesHandler(mock)
	rec := httptest.NewRecorder()
	h.Create(rec, agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "a genuine reply"}, map[string]string{"id": "p1"}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if mock.lastGetID != "reply-123" {
		t.Errorf("read back %q, want the created reply reply-123", mock.lastGetID)
	}
	var out struct {
		Data struct {
			ID     string             `json:"id"`
			Author models.ReplyAuthor `json:"author"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Data.ID != "reply-123" || out.Data.Author.DisplayName != "Agent One" || out.Data.Author.Type != models.AuthorTypeAgent {
		t.Errorf("create answered %s, want the stored reply with its author", rec.Body.String())
	}
}

// The reply is already stored when the read-back fails: the create still answers 201 with
// the stored reply, so a client does not retry and post it twice.
func TestReplies_CreateStillAnswersTheReplyWhenTheReadBackFails(t *testing.T) {
	mock := &MockRepliesRepository{getErr: errors.New("connection reset")}
	h := NewRepliesHandler(mock)
	rec := httptest.NewRecorder()
	h.Create(rec, agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "a genuine reply"}, map[string]string{"id": "p1"}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Data.ID != "reply-123" {
		t.Errorf("create answered %s (%v), want the stored reply", rec.Body.String(), err)
	}
}
