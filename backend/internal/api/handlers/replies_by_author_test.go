package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task idx 73 step 3: GET /v1/replies?author_type=&author_id= is the canonical list of one
// author's replies (it replaces GET /v1/users/{id}/contributions and GET /v1/me/contributions).
// The author is required, the page contract is the reply list's (limit default 50, at most 100,
// opaque cursor), and the viewer's family scopes which posts' replies are listed.

func authorListReq(query string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/v1/replies?"+query, nil)
}

func decodeAuthorList(t *testing.T, rec *httptest.ResponseRecorder) (data []map[string]any, meta map[string]any) {
	t.Helper()
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return resp.Data, resp.Meta
}

func TestRepliesByAuthor_RequiresTheAuthor(t *testing.T) {
	for _, query := range []string{
		"",
		"author_id=u1",
		"author_type=human",
		"author_type=robot&author_id=u1",
		"author_type=system&author_id=u1",
		"author_type=human&author_id=",
	} {
		mock := &MockRepliesRepository{}
		rec := httptest.NewRecorder()
		NewRepliesHandler(mock).ListByAuthor(rec, authorListReq(query))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%q: status = %d, want 400: %s", query, rec.Code, rec.Body.String())
		}
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body.Error.Code != "VALIDATION_ERROR" {
			t.Errorf("%q: code = %q, want VALIDATION_ERROR", query, body.Error.Code)
		}
		if mock.lastAuthorParams != nil {
			t.Errorf("%q: the repository was read for an invalid request", query)
		}
	}
}

func TestRepliesByAuthor_RejectsAMalformedPage(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=-1", "limit=x", "cursor=not-a-cursor"} {
		mock := &MockRepliesRepository{}
		rec := httptest.NewRecorder()
		NewRepliesHandler(mock).ListByAuthor(rec, authorListReq("author_type=human&author_id=u1&"+query))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestRepliesByAuthor_PassesAuthorViewerAndLookAhead(t *testing.T) {
	cases := []struct {
		name, query string
		ctx         func(context.Context) context.Context
		wantType    models.AuthorType
		wantViewer  string
		wantLimit   int
	}{
		{"anonymous, default page", "author_type=human&author_id=u1", nil, models.AuthorTypeHuman, "", defaultReplyPageLimit + 1},
		{"clamped limit", "author_type=agent&author_id=agent_x&limit=500", nil, models.AuthorTypeAgent, "", maxReplyPageLimit + 1},
		{"human viewer", "author_type=agent&author_id=agent_x&limit=3", func(c context.Context) context.Context {
			return auth.ContextWithClaims(c, &auth.Claims{UserID: "viewer-human"})
		}, models.AuthorTypeAgent, "viewer-human", 4},
		{"claimed agent viewer", "author_type=human&author_id=u1", func(c context.Context) context.Context {
			owner := "owning-human"
			return auth.ContextWithAgent(c, &models.Agent{ID: "agent-1", HumanID: &owner})
		}, models.AuthorTypeHuman, "owning-human", defaultReplyPageLimit + 1},
	}
	for _, c := range cases {
		mock := &MockRepliesRepository{authorPage: []models.ReplyWithPost{}}
		req := authorListReq(c.query)
		if c.ctx != nil {
			req = req.WithContext(c.ctx(req.Context()))
		}
		rec := httptest.NewRecorder()
		NewRepliesHandler(mock).ListByAuthor(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200: %s", c.name, rec.Code, rec.Body.String())
		}
		p := mock.lastAuthorParams
		if p == nil {
			t.Fatalf("%s: repository not read", c.name)
		}
		if p.AuthorType != c.wantType || p.ViewerHuman != c.wantViewer || p.Limit != c.wantLimit {
			t.Errorf("%s: params = %+v, want type %s viewer %q limit %d", c.name, *p, c.wantType, c.wantViewer, c.wantLimit)
		}
		if p.BeforeCreatedAt != nil {
			t.Errorf("%s: a first page starts at the newest reply, got keyset %v", c.name, p.BeforeCreatedAt)
		}
		data, meta := decodeAuthorList(t, rec)
		if data == nil || len(data) != 0 {
			t.Errorf("%s: data = %v, want an empty array", c.name, data)
		}
		if meta["has_more"] != false || meta["total"] != float64(0) {
			t.Errorf("%s: meta = %v, want total 0 and has_more false", c.name, meta)
		}
	}
}

func TestRepliesByAuthor_PagesWithTheCursor(t *testing.T) {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	rows := make([]models.ReplyWithPost, 3)
	for i := range rows {
		rows[i] = models.ReplyWithPost{
			ReplyWithAuthor: models.ReplyWithAuthor{Reply: models.Reply{
				ID: "r" + strconv.Itoa(i), PostID: "p1", Body: "b", CreatedAt: base.Add(-time.Duration(i) * time.Minute),
			}},
			Post: models.ReplyPost{ID: "p1", Type: "post", Title: "The post"},
		}
	}
	mock := &MockRepliesRepository{authorPage: rows, authorTotal: 7}
	rec := httptest.NewRecorder()
	NewRepliesHandler(mock).ListByAuthor(rec, authorListReq("author_type=human&author_id=u1&limit=2"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	data, meta := decodeAuthorList(t, rec)
	if len(data) != 2 || data[0]["id"] != "r0" || data[1]["id"] != "r1" {
		t.Fatalf("data = %v, want r0, r1 (the look-ahead row trimmed)", data)
	}
	post, _ := data[0]["post"].(map[string]any)
	if post["id"] != "p1" || post["type"] != "post" || post["title"] != "The post" {
		t.Errorf("data[0].post = %v, want the reply's post", data[0]["post"])
	}
	if meta["total"] != float64(7) || meta["has_more"] != true {
		t.Fatalf("meta = %v, want total 7 and has_more true", meta)
	}
	cursor, _ := meta["next_cursor"].(string)
	if cursor == "" {
		t.Fatalf("meta.next_cursor missing: %v", meta)
	}

	next := &MockRepliesRepository{authorPage: []models.ReplyWithPost{}}
	rec = httptest.NewRecorder()
	NewRepliesHandler(next).ListByAuthor(rec, authorListReq("author_type=human&author_id=u1&limit=2&cursor="+cursor))
	if rec.Code != http.StatusOK {
		t.Fatalf("second page status = %d", rec.Code)
	}
	p := next.lastAuthorParams
	if p == nil || p.BeforeCreatedAt == nil || !p.BeforeCreatedAt.Equal(rows[1].CreatedAt) || p.BeforeID != "r1" {
		t.Fatalf("second page keyset = %+v, want before (%v, r1)", p, rows[1].CreatedAt)
	}
	_, meta = decodeAuthorList(t, rec)
	if _, present := meta["next_cursor"]; present || meta["has_more"] != false {
		t.Errorf("last page meta = %v, want has_more false and no next_cursor", meta)
	}
}

func TestRepliesByAuthor_RepositoryFailureIs500(t *testing.T) {
	mock := &MockRepliesRepository{authorErr: errors.New("boom")}
	rec := httptest.NewRecorder()
	NewRepliesHandler(mock).ListByAuthor(rec, authorListReq("author_type=human&author_id=u1"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
