package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// commentsCallerReq builds a comments request for target_type/id as the given caller:
// "claimed agent" (scoped to human-7), "unclaimed agent", "human JWT" (user-9) or
// "anonymous".
func commentsCallerReq(t *testing.T, method, caller string) *http.Request {
	t.Helper()
	var body map[string]any
	if method == http.MethodPost {
		body = map[string]any{"content": "a comment"}
	}
	req := agentReq(method, "/v1/posts/p1/comments", body, map[string]string{"target_type": "post", "id": "p1"})
	human := "human-7"
	switch caller {
	case "claimed agent":
		return req.WithContext(auth.ContextWithAgent(req.Context(), &models.Agent{ID: "agent-1", HumanID: &human}))
	case "unclaimed agent":
		return req
	case "human JWT":
		ctx := auth.ContextWithAgent(req.Context(), nil)
		ctx = auth.ContextWithClaims(ctx, &auth.Claims{UserID: "user-9", Role: models.UserRoleUser})
		return req.WithContext(ctx)
	default:
		return req.WithContext(auth.ContextWithAgent(req.Context(), nil))
	}
}

// The comment gate asks about the caller's family (the claimed agent's human, the JWT's
// user, or "" for public-only), and the list is scoped to that same family, so the total
// counts what the caller may see.
func TestComments_VisibilityIsCheckedForTheCallersFamily(t *testing.T) {
	want := map[string]string{"claimed agent": "human-7", "unclaimed agent": "", "human JWT": "user-9", "anonymous": ""}
	for caller, human := range want {
		t.Run("list/"+caller, func(t *testing.T) {
			mock := &MockCommentsRepository{targetExists: true}
			rec := httptest.NewRecorder()
			NewCommentsHandler(mock).List(rec, commentsCallerReq(t, http.MethodGet, caller))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if mock.visibleCaller == nil || *mock.visibleCaller != human {
				t.Fatalf("visibility checked for %v, want %q", mock.visibleCaller, human)
			}
			if mock.lastListOpts == nil || mock.lastListOpts.CallerHuman != human {
				t.Fatalf("list scoped to %+v, want caller %q", mock.lastListOpts, human)
			}
		})
		if caller == "anonymous" {
			continue
		}
		t.Run("create/"+caller, func(t *testing.T) {
			mock := &MockCommentsRepository{targetExists: true}
			rec := httptest.NewRecorder()
			NewCommentsHandler(mock).Create(rec, commentsCallerReq(t, http.MethodPost, caller))
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
			}
			if mock.visibleCaller == nil || *mock.visibleCaller != human {
				t.Fatalf("visibility checked for %v, want %q", mock.visibleCaller, human)
			}
		})
	}
}

// A target the caller may not see is 404 before anything is listed or written.
func TestComments_HiddenTargetIsNotFoundBeforeTheRepository(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			mock := &MockCommentsRepository{targetExists: false, createErr: context.Canceled, listErr: context.Canceled}
			rec := httptest.NewRecorder()
			h := NewCommentsHandler(mock)
			if method == http.MethodGet {
				h.List(rec, commentsCallerReq(t, method, "claimed agent"))
			} else {
				h.Create(rec, commentsCallerReq(t, method, "claimed agent"))
			}
			if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"NOT_FOUND"`) {
				t.Fatalf("status = %d, want 404 NOT_FOUND; body=%s", rec.Code, rec.Body.String())
			}
			if mock.lastListOpts != nil {
				t.Fatalf("the list ran behind a hidden target")
			}
		})
	}
}
