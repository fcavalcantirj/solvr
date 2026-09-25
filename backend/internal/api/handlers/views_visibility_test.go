package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// mockViewsRepository counts views in memory for the handler tests only.
type mockViewsRepository struct {
	hidden        bool
	visibleErr    error
	visibleCaller *string
	recorded      []string
	count         int
}

func (m *mockViewsRepository) RecordView(ctx context.Context, postID, viewerType, viewerID string) (int, error) {
	m.recorded = append(m.recorded, viewerType+":"+viewerID)
	m.count++
	return m.count, nil
}

func (m *mockViewsRepository) RecordAnonymousView(ctx context.Context, postID, sessionID string) (int, error) {
	return m.RecordView(ctx, postID, "anonymous", sessionID)
}

func (m *mockViewsRepository) GetViewCount(ctx context.Context, postID string) (int, error) {
	return m.count, nil
}

func (m *mockViewsRepository) PostVisibleTo(ctx context.Context, postID, callerHuman string) (bool, error) {
	m.visibleCaller = &callerHuman
	if m.visibleErr != nil {
		return false, m.visibleErr
	}
	return !m.hidden, nil
}

func viewsReq(method, target string, ctxFn func(context.Context) context.Context) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if ctxFn != nil {
		ctx = ctxFn(ctx)
	}
	return req.WithContext(ctx)
}

// A post the caller may not read has no view count and takes no view: both routes answer
// 404 like GET /v1/posts/{id}, and nothing is recorded.
func TestViews_HiddenPostIsNotFound(t *testing.T) {
	mock := &mockViewsRepository{hidden: true, count: 7}
	h := NewViewsHandler(mock)
	for name, call := range map[string]func(rec *httptest.ResponseRecorder){
		"count": func(rec *httptest.ResponseRecorder) {
			h.GetViewCount(rec, viewsReq(http.MethodGet, "/v1/posts/p1/views", nil))
		},
		"record": func(rec *httptest.ResponseRecorder) {
			h.RecordView(rec, viewsReq(http.MethodPost, "/v1/posts/p1/view", nil))
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			call(rec)
			if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "post not found") {
				t.Fatalf("status = %d, want 404 post not found; body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "view_count") {
				t.Fatalf("a hidden post's count leaked: %s", rec.Body.String())
			}
		})
	}
	if len(mock.recorded) != 0 {
		t.Fatalf("views recorded behind a hidden post: %v", mock.recorded)
	}
}

// The check asks about the caller's family, and an authenticated caller's view is counted
// under its own identity.
func TestViews_VisibilityIsCheckedForTheCallersFamily(t *testing.T) {
	human := "human-7"
	cases := map[string]struct {
		ctx          func(context.Context) context.Context
		family, seen string
	}{
		"anonymous": {nil, "", "anonymous:anonymous"},
		"claimed agent": {func(c context.Context) context.Context {
			return auth.ContextWithAgent(c, &models.Agent{ID: "agent-1", HumanID: &human})
		}, human, "agent:agent-1"},
		"human JWT": {func(c context.Context) context.Context {
			return auth.ContextWithClaims(c, &auth.Claims{UserID: "user-9"})
		}, "user-9", "human:user-9"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := &mockViewsRepository{}
			rec := httptest.NewRecorder()
			NewViewsHandler(mock).RecordView(rec, viewsReq(http.MethodPost, "/v1/posts/p1/view", tc.ctx))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if mock.visibleCaller == nil || *mock.visibleCaller != tc.family {
				t.Fatalf("visibility checked for %v, want %q", mock.visibleCaller, tc.family)
			}
			if len(mock.recorded) != 1 || mock.recorded[0] != tc.seen {
				t.Fatalf("recorded %v, want [%s]", mock.recorded, tc.seen)
			}
			mock.visibleCaller = nil
			rec = httptest.NewRecorder()
			NewViewsHandler(mock).GetViewCount(rec, viewsReq(http.MethodGet, "/v1/posts/p1/views", tc.ctx))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"view_count":1`) {
				t.Fatalf("count: status = %d; body=%s", rec.Code, rec.Body.String())
			}
			if mock.visibleCaller == nil || *mock.visibleCaller != tc.family {
				t.Fatalf("count visibility checked for %v, want %q", mock.visibleCaller, tc.family)
			}
		})
	}
}

// A failed visibility check is a server failure, not a hidden post.
func TestViews_VisibilityCheckFailureIsAServerError(t *testing.T) {
	mock := &mockViewsRepository{visibleErr: context.DeadlineExceeded}
	h := NewViewsHandler(mock)
	rec := httptest.NewRecorder()
	h.GetViewCount(rec, viewsReq(http.MethodGet, "/v1/posts/p1/views", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("count status = %d, want 500", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.RecordView(rec, viewsReq(http.MethodPost, "/v1/posts/p1/view", nil))
	if rec.Code != http.StatusInternalServerError || len(mock.recorded) != 0 {
		t.Fatalf("record status = %d, recorded %v; want 500 and nothing", rec.Code, mock.recorded)
	}
}
