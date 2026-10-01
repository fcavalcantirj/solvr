package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// idx 78 step 1: PATCH /v1/replies/{id} answers the same Reply as GET /v1/replies/{id}, author
// included (the published ReplyResponse requires it). An edit never changes the author, so the
// author read before the write is the one answered.
func TestReplies_UpdateAnswersTheEditedReplyWithItsAuthor(t *testing.T) {
	read := replyAtVersion(time.Now())
	read.Author = models.ReplyAuthor{ID: "agent-1", Type: models.AuthorTypeAgent, DisplayName: "Agent One"}
	edited := time.Now().Add(time.Second)
	mock := &MockRepliesRepository{getResult: read, updated: &models.Reply{
		ID: "r1", PostID: "p1", Body: "edited", AuthorType: models.AuthorTypeAgent, AuthorID: "agent-1", UpdatedAt: edited,
	}}
	h := NewRepliesHandler(mock)
	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	req.Header.Set("If-Match", replyETag(read.UpdatedAt))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			ID     string              `json:"id"`
			Body   string              `json:"body"`
			Author *models.ReplyAuthor `json:"author"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Data.ID != "r1" || out.Data.Body != "edited" {
		t.Errorf("update answered %s, want the edited reply", rec.Body.String())
	}
	if out.Data.Author == nil || out.Data.Author.DisplayName != "Agent One" || out.Data.Author.Type != models.AuthorTypeAgent {
		t.Errorf("update answered %s, want the edited reply with its author", rec.Body.String())
	}
	if got := rec.Header().Get("ETag"); got != replyETag(edited) {
		t.Errorf("ETag = %q, want the edited version %q", got, replyETag(edited))
	}
}
