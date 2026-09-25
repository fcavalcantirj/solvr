package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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
	// pagePage/pageTotal drive ListPageByPost; lastPageParams records what the
	// handler passed so a test can assert cursor/limit clamping.
	pageResult     []models.ReplyWithAuthor
	pageTotal      int
	lastPageParams *models.ReplyPageParams
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

func (m *MockRepliesRepository) ListPageByPost(_ context.Context, params models.ReplyPageParams) ([]models.ReplyWithAuthor, int, error) {
	p := params
	m.lastPageParams = &p
	return m.pageResult, m.pageTotal, nil
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
	one := []models.ReplyWithAuthor{{Reply: models.Reply{ID: "r1", Body: "hi"}}}
	mock := &MockRepliesRepository{
		listResult: one, listTotal: 1,
		pageResult: one, pageTotal: 1,
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

// listReplyReq builds a public GET for a post's replies with the given raw query.
func listReplyReq(query string) *http.Request {
	target := "/v1/posts/p1/replies"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// Default page: opaque-cursor envelope carries has_more=false and no next_cursor
// when a single page holds every reply (idx 73 step 2).
func TestReplies_ListDefaultPageEnvelope(t *testing.T) {
	mock := &MockRepliesRepository{
		pageResult: []models.ReplyWithAuthor{{Reply: models.Reply{ID: "r1", Body: "hi"}}},
		pageTotal:  1,
	}
	h := NewRepliesHandler(mock)
	rec := httptest.NewRecorder()
	h.List(rec, listReplyReq(""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d, want 1", len(resp.Data))
	}
	if hm, ok := resp.Meta["has_more"].(bool); !ok || hm {
		t.Fatalf("meta.has_more = %v (ok=%v), want false present", resp.Meta["has_more"], ok)
	}
	if _, present := resp.Meta["next_cursor"]; present {
		t.Fatalf("next_cursor must be absent when has_more is false, got %v", resp.Meta["next_cursor"])
	}
	// Handler asks the repo for limit+1 (default 50 -> 51) to detect a next page.
	if mock.lastPageParams == nil || mock.lastPageParams.Limit != defaultReplyPageLimit+1 {
		t.Fatalf("repo Limit = %v, want %d (default+1 look-ahead)", mock.lastPageParams, defaultReplyPageLimit+1)
	}
}

// A full page yields has_more=true and an opaque next_cursor; the extra
// look-ahead row is trimmed from data.
func TestReplies_ListHasMoreAndNextCursor(t *testing.T) {
	// Return default+1 rows so the handler detects a following page.
	rows := make([]models.ReplyWithAuthor, defaultReplyPageLimit+1)
	for i := range rows {
		rows[i] = models.ReplyWithAuthor{Reply: models.Reply{
			ID: "r" + strconv.Itoa(i), Body: "b", CreatedAt: time.Unix(int64(1700000000+i), 0),
		}}
	}
	mock := &MockRepliesRepository{pageResult: rows, pageTotal: 999}
	h := NewRepliesHandler(mock)
	rec := httptest.NewRecorder()
	h.List(rec, listReplyReq(""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != defaultReplyPageLimit {
		t.Fatalf("data len = %d, want %d (look-ahead trimmed)", len(resp.Data), defaultReplyPageLimit)
	}
	if hm, _ := resp.Meta["has_more"].(bool); !hm {
		t.Fatalf("meta.has_more = %v, want true", resp.Meta["has_more"])
	}
	nc, ok := resp.Meta["next_cursor"].(string)
	if !ok || nc == "" {
		t.Fatalf("meta.next_cursor = %v, want non-empty opaque cursor", resp.Meta["next_cursor"])
	}
	// The cursor points at the last returned row's keyset, not the trimmed one.
	if _, id, valid := decodeReplyCursor(nc); !valid || id != "r"+strconv.Itoa(defaultReplyPageLimit-1) {
		t.Fatalf("next_cursor decodes to id=%q valid=%v, want last returned row", id, valid)
	}
}

// A malformed cursor is a client error, not a silently-ignored parameter.
func TestReplies_ListRejectsBadCursor(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{})
	rec := httptest.NewRecorder()
	h.List(rec, listReplyReq("cursor=%21%21not-base64%21%21"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for malformed cursor", rec.Code)
	}
}

// A non-positive limit is rejected rather than passed to the repo.
func TestReplies_ListRejectsBadLimit(t *testing.T) {
	h := NewRepliesHandler(&MockRepliesRepository{})
	rec := httptest.NewRecorder()
	h.List(rec, listReplyReq("limit=0"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for limit=0", rec.Code)
	}
}

// limit is clamped to the API-wide maximum of 100 before the +1 look-ahead.
func TestReplies_ListClampsLimit(t *testing.T) {
	mock := &MockRepliesRepository{}
	h := NewRepliesHandler(mock)
	rec := httptest.NewRecorder()
	h.List(rec, listReplyReq("limit=500"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if mock.lastPageParams == nil || mock.lastPageParams.Limit != maxReplyPageLimit+1 {
		t.Fatalf("repo Limit = %v, want %d (max+1)", mock.lastPageParams, maxReplyPageLimit+1)
	}
}

// A valid cursor is decoded into the keyset position handed to the repo.
func TestReplies_ListDecodesCursor(t *testing.T) {
	mock := &MockRepliesRepository{}
	h := NewRepliesHandler(mock)
	ts := time.Unix(1700000123, 456000) // microsecond-aligned
	cursor := encodeReplyCursor(ts, "reply-cursor-target")
	rec := httptest.NewRecorder()
	h.List(rec, listReplyReq("cursor="+cursor))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if mock.lastPageParams == nil {
		t.Fatal("repo not called")
	}
	if mock.lastPageParams.AfterID != "reply-cursor-target" {
		t.Fatalf("AfterID = %q, want reply-cursor-target", mock.lastPageParams.AfterID)
	}
	if mock.lastPageParams.AfterCreatedAt == nil || !mock.lastPageParams.AfterCreatedAt.Equal(ts) {
		t.Fatalf("AfterCreatedAt = %v, want %v", mock.lastPageParams.AfterCreatedAt, ts)
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
