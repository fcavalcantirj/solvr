package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// An account that replies still name as their author is not hard-deleted (migration 000116):
// the admin endpoints answer 409 and the account and its replies stay.
func TestAdminHandler_HardDeleteAnAccountThatAuthorsRepliesIsAConflict(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	sfx := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)

	agent := &models.Agent{ID: "hd_author_" + sfx, DisplayName: "Hard Delete Author " + sfx}
	if err := db.NewAgentRepository(pool).Create(ctx, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	user, err := db.NewUserRepository(pool).Create(ctx, &models.User{
		Username: "hdauthor" + sfx, DisplayName: "Hard Delete Author", Email: "hdauthor" + sfx + "@example.com",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "hdauthor_" + sfx, Role: models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	post, err := db.NewPostRepository(pool).Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "Hard delete author " + sfx, Description: "A post whose replies keep their authors",
		PostedByType: models.AuthorTypeAgent, PostedByID: agent.ID, Status: models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	defer func() { // before pool.Close
		pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, post.ID)   //nolint:errcheck // replies cascade
		pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)   //nolint:errcheck
	}()
	for _, author := range []struct {
		typ models.AuthorType
		id  string
	}{{models.AuthorTypeAgent, agent.ID}, {models.AuthorTypeHuman, user.ID}} {
		if _, err := db.NewReplyRepository(pool).Create(ctx, &models.Reply{
			PostID: post.ID, AuthorType: author.typ, AuthorID: author.id, Body: "a reply that keeps its author",
		}); err != nil {
			t.Fatalf("create reply by %s: %v", author.id, err)
		}
	}

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")
	handler := NewAdminHandler(pool)
	hardDelete := func(path, id string, serve func(http.ResponseWriter, *http.Request)) {
		t.Helper()
		req := httptest.NewRequest(http.MethodDelete, path+id, nil)
		req.Header.Set("X-Admin-API-Key", "test-admin-key")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		serve(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("DELETE %s%s: status %d, want 409 (%s)", path, id, w.Code, w.Body.String())
		}
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil || body.Error.Code != "ACCOUNT_AUTHORS_CONTENT" {
			t.Fatalf("DELETE %s%s: error code %q (%v), want ACCOUNT_AUTHORS_CONTENT", path, id, body.Error.Code, err)
		}
	}
	hardDelete("/admin/agents/", agent.ID, handler.HardDeleteAgent)
	hardDelete("/admin/users/", user.ID, handler.HardDeleteUser)

	if _, err := db.NewAgentRepository(pool).FindByID(ctx, agent.ID); err != nil {
		t.Errorf("the agent is gone: %v", err)
	}
	if _, err := db.NewUserRepository(pool).FindByID(ctx, user.ID); err != nil {
		t.Errorf("the user is gone: %v", err)
	}
}

// An account that only authors posts is not hard-deleted either (migration 000117): 409, and
// the account and its posts stay.
func TestAdminHandler_HardDeleteAnAccountThatAuthorsPostsIsAConflict(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	sfx := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)

	agent := &models.Agent{ID: "hd_poster_" + sfx, DisplayName: "Hard Delete Poster " + sfx}
	if err := db.NewAgentRepository(pool).Create(ctx, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	user, err := db.NewUserRepository(pool).Create(ctx, &models.User{
		Username: "hdposter" + sfx, DisplayName: "Hard Delete Poster", Email: "hdposter" + sfx + "@example.com",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "hdposter_" + sfx, Role: models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	var posts []string
	defer func() { // before pool.Close
		for _, id := range posts {
			pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id) //nolint:errcheck
		}
		pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)   //nolint:errcheck
	}()
	for _, author := range []struct {
		typ models.AuthorType
		id  string
	}{{models.AuthorTypeAgent, agent.ID}, {models.AuthorTypeHuman, user.ID}} {
		post, err := db.NewPostRepository(pool).Create(ctx, &models.Post{
			Type: models.PostTypePost, Title: "Hard delete poster " + sfx, Description: "A post that keeps its author",
			PostedByType: author.typ, PostedByID: author.id, Status: models.PostStatusOpen,
		})
		if err != nil {
			t.Fatalf("create post by %s: %v", author.id, err)
		}
		posts = append(posts, post.ID)
	}

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")
	handler := NewAdminHandler(pool)
	for _, c := range []struct {
		path, id string
		serve    func(http.ResponseWriter, *http.Request)
	}{{"/admin/agents/", agent.ID, handler.HardDeleteAgent}, {"/admin/users/", user.ID, handler.HardDeleteUser}} {
		req := httptest.NewRequest(http.MethodDelete, c.path+c.id, nil)
		req.Header.Set("X-Admin-API-Key", "test-admin-key")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", c.id)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		c.serve(w, req)
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&body); w.Code != http.StatusConflict || err != nil || body.Error.Code != "ACCOUNT_AUTHORS_CONTENT" {
			t.Fatalf("DELETE %s%s: status %d code %q (%v), want 409 ACCOUNT_AUTHORS_CONTENT", c.path, c.id, w.Code, body.Error.Code, err)
		}
	}

	if _, err := db.NewAgentRepository(pool).FindByID(ctx, agent.ID); err != nil {
		t.Errorf("the agent is gone: %v", err)
	}
	if _, err := db.NewUserRepository(pool).FindByID(ctx, user.ID); err != nil {
		t.Errorf("the user is gone: %v", err)
	}
	for _, id := range posts {
		if _, err := db.NewPostRepository(pool).FindByID(ctx, id); err != nil {
			t.Errorf("post %s is gone: %v", id, err)
		}
	}
}
