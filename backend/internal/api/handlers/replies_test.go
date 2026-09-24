package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// MockRepliesRepository implements RepliesRepositoryInterface for handler tests.
type MockRepliesRepository struct {
	createErr   error
	created     *models.Reply
	getResult   *models.ReplyWithAuthor
	getErr      error
	listResult  []models.ReplyWithAuthor
	listTotal   int
	updateErr   error
	updated     *models.Reply
	deleteErr   error
	voteErr     error
	lastCreated *models.Reply
}

func (m *MockRepliesRepository) Create(_ context.Context, reply *models.Reply) (*models.Reply, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.lastCreated = reply
	if m.created != nil {
		return m.created, nil
	}
	reply.ID = "reply-123"
	return reply, nil
}

func (m *MockRepliesRepository) GetByID(_ context.Context, _ string) (*models.ReplyWithAuthor, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getResult, nil
}

func (m *MockRepliesRepository) ListByPost(_ context.Context, _ models.ReplyListOptions) ([]models.ReplyWithAuthor, int, error) {
	return m.listResult, m.listTotal, nil
}

func (m *MockRepliesRepository) Update(_ context.Context, id string, _ models.AuthorType, _, body string) (*models.Reply, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if m.updated != nil {
		return m.updated, nil
	}
	return &models.Reply{ID: id, Body: body}, nil
}

func (m *MockRepliesRepository) Delete(_ context.Context, _ string, _ models.AuthorType, _ string) error {
	return m.deleteErr
}

func (m *MockRepliesRepository) Vote(_ context.Context, _, _, _, _ string) error {
	return m.voteErr
}

func (m *MockRepliesRepository) GetUserVote(_ context.Context, _, _, _ string) (*string, error) {
	return nil, nil
}

func agentReq(method, target string, body any, urlParams map[string]string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, target, &buf)
	rctx := chi.NewRouteContext()
	for k, v := range urlParams {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.ContextWithAgent(ctx, &models.Agent{ID: "agent-1"})
	return req.WithContext(ctx)
}

func TestReplies_CreateRequiresAuth(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{})
	req := httptest.NewRequest(http.MethodPost, "/v1/posts/p1/replies", bytes.NewBufferString(`{"body":"hi"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// A client creates a reply with only a body — no approach/answer/comment choice.
func TestReplies_CreateNeedsNoContentType(t *testing.T) {
	mock := &MockRepliesRepository{}
	h := NewRepliesHandler(mock)
	req := agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "a genuine reply"}, map[string]string{"id": "p1"})
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if mock.lastCreated == nil || mock.lastCreated.PostID != "p1" {
		t.Fatalf("reply not created against post p1: %+v", mock.lastCreated)
	}
	if mock.lastCreated.AuthorType != models.AuthorTypeAgent || mock.lastCreated.AuthorID != "agent-1" {
		t.Errorf("authorship not derived from authenticated identity: %+v", mock.lastCreated)
	}
}

func TestReplies_CreateEmptyBodyRejected(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{})
	req := agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "   "}, map[string]string{"id": "p1"})
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestReplies_CreateMissingPost(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{createErr: db.ErrReplyPostNotFound})
	req := agentReq(http.MethodPost, "/v1/posts/nope/replies", map[string]any{"body": "hi"}, map[string]string{"id": "nope"})
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestReplies_ListIsPublic(t *testing.T) {
	mock := &MockRepliesRepository{
		listResult: []models.ReplyWithAuthor{{Reply: models.Reply{ID: "r1", Body: "hi"}}},
		listTotal:  1,
	}
	h := NewRepliesHandler(mock)
	// No auth context — public read.
	req := httptest.NewRequest(http.MethodGet, "/v1/posts/p1/replies", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Meta["total"].(float64) != 1 {
		t.Fatalf("unexpected list response: %s", rec.Body.String())
	}
}

func TestReplies_GetNotFound(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{getErr: models.ErrReplyNotFound})
	req := agentReq(http.MethodGet, "/v1/replies/x", nil, map[string]string{"id": "x"})
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestReplies_UpdateNonAuthorForbidden(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{updateErr: db.ErrReplyForbidden})
	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestReplies_UpdateAuthorSucceeds(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{})
	req := agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited"}, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestReplies_DeleteAuthorSucceeds(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{})
	req := agentReq(http.MethodDelete, "/v1/replies/r1", nil, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReplies_VoteRejectsSelfVote(t *testing.T) {
	mock := &MockRepliesRepository{
		getResult: &models.ReplyWithAuthor{Reply: models.Reply{ID: "r1", AuthorType: models.AuthorTypeAgent, AuthorID: "agent-1"}},
	}
	h := NewRepliesHandler(mock)
	req := agentReq(http.MethodPost, "/v1/replies/r1/vote", map[string]any{"direction": "up"}, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Vote(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (self-vote)", rec.Code)
	}
}

func TestReplies_VoteOnOtherSucceeds(t *testing.T) {
	mock := &MockRepliesRepository{
		getResult: &models.ReplyWithAuthor{Reply: models.Reply{ID: "r1", AuthorType: models.AuthorTypeHuman, AuthorID: "someone-else"}},
	}
	h := NewRepliesHandler(mock)
	req := agentReq(http.MethodPost, "/v1/replies/r1/vote", map[string]any{"direction": "up"}, map[string]string{"id": "r1"})
	rec := httptest.NewRecorder()
	h.Vote(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
