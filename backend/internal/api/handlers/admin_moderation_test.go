package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// POST /admin/posts/{id}/moderation (SEO audit 2026-11-08, Release A): the operator's own
// decision on a post the automatic moderation never finished. Eleven posts sat in
// pending_review for up to two months with no flag and no path out: the only route to
// `open` was Groq's verdict. The decision goes through PostRepository.UpdateStatus, the one
// path that derives the canonical states, and is recorded as a flag with the operator's note.

type fakeModeratedPosts struct {
	post     *models.PostWithAuthor
	findErr  error
	statuses []models.PostStatus
	updErr   error
	flags    []*models.Flag
}

func (f *fakeModeratedPosts) FindByID(_ context.Context, id string) (*models.PostWithAuthor, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.post == nil || f.post.ID != id {
		return nil, db.ErrPostNotFound
	}
	return f.post, nil
}

func (f *fakeModeratedPosts) UpdateStatus(_ context.Context, _ string, status models.PostStatus) error {
	if f.updErr != nil {
		return f.updErr
	}
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeModeratedPosts) CreateFlag(_ context.Context, flag *models.Flag) (*models.Flag, error) {
	f.flags = append(f.flags, flag)
	return flag, nil
}

func pending(id string, status models.PostStatus) *models.PostWithAuthor {
	return &models.PostWithAuthor{Post: models.Post{ID: id, Title: "Stuck post", Status: status}}
}

func decide(t *testing.T, h *PostModerationDecisionHandler, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/posts/"+id+"/moderation", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Decide(rec, req)
	return rec
}

func TestPostModerationDecision_ApprovesAndRejects(t *testing.T) {
	id := uuid.NewString()
	for _, c := range []struct {
		decision string
		status   models.PostStatus
	}{{"approve", models.PostStatusOpen}, {"reject", models.PostStatusRejected}} {
		t.Run(c.decision, func(t *testing.T) {
			posts := &fakeModeratedPosts{post: pending(id, models.PostStatusPendingReview)}
			rec := decide(t, NewPostModerationDecisionHandler(posts), id, `{"decision":"`+c.decision+`","note":"operator review 2026-11-08"}`)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, []models.PostStatus{c.status}, posts.statuses)
			require.Len(t, posts.flags, 1)
			flag := posts.flags[0]
			assert.Equal(t, "post", flag.TargetType)
			assert.Equal(t, id, flag.TargetID.String())
			assert.Equal(t, "system", flag.ReporterType)
			assert.Equal(t, "operator", flag.ReporterID)
			assert.Equal(t, "other", flag.Reason, "the flags table admits no new reason; the decision is in details")
			assert.Equal(t, "actioned", flag.Status)
			assert.Contains(t, flag.Details, "operator "+c.decision)
			assert.Contains(t, flag.Details, "operator review 2026-11-08")
			var out struct {
				Data struct {
					ID       string `json:"id"`
					Decision string `json:"decision"`
					From     string `json:"from_status"`
					To       string `json:"status"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			assert.Equal(t, id, out.Data.ID)
			assert.Equal(t, c.decision, out.Data.Decision)
			assert.Equal(t, "pending_review", out.Data.From)
			assert.Equal(t, string(c.status), out.Data.To)
		})
	}
}

func TestPostModerationDecision_Refusals(t *testing.T) {
	id := uuid.NewString()
	pendingPosts := func() *fakeModeratedPosts {
		return &fakeModeratedPosts{post: pending(id, models.PostStatusPendingReview)}
	}
	cases := []struct {
		posts  *fakeModeratedPosts
		name   string
		id     string
		body   string
		code   string
		status int
	}{
		{pendingPosts(), "a decision that is not approve or reject", id, `{"decision":"maybe"}`, "VALIDATION_ERROR", http.StatusBadRequest},
		{pendingPosts(), "a body that is not JSON", id, `{"decision":`, "VALIDATION_ERROR", http.StatusBadRequest},
		{pendingPosts(), "an id that is not a UUID", "not-a-uuid", `{"decision":"approve"}`, "VALIDATION_ERROR", http.StatusBadRequest},
		{&fakeModeratedPosts{}, "a post the API does not have", id, `{"decision":"approve"}`, "NOT_FOUND", http.StatusNotFound},
		{&fakeModeratedPosts{post: pending(id, models.PostStatusOpen)}, "a post already open", id, `{"decision":"approve"}`, "NOT_PENDING", http.StatusConflict},
		{&fakeModeratedPosts{post: pending(id, models.PostStatusPendingReview), updErr: errors.New("down")}, "a store that fails", id, `{"decision":"approve"}`, "INTERNAL_ERROR", http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := decide(t, NewPostModerationDecisionHandler(c.posts), c.id, c.body)
			assert.Equal(t, c.status, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), c.code)
			assert.Empty(t, c.posts.statuses, "nothing is written on a refusal")
		})
	}
	// A rejected post may be approved later (the operator overrules an automatic rejection),
	// and a draft may be too; only open and closed posts are not "pending".
	posts := &fakeModeratedPosts{post: pending(id, models.PostStatusRejected)}
	rec := decide(t, NewPostModerationDecisionHandler(posts), id, `{"decision":"approve"}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
