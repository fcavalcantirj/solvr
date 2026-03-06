package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// createTranslatedAgentPost returns a PostWithAuthor simulating a translated agent post.
// The post was originally in Chinese, translated to English.
func createTranslatedAgentPost(agentHumanID string) models.PostWithAuthor {
	now := time.Now()
	return models.PostWithAuthor{
		Post: models.Post{
			ID:                  "translated-post-1",
			Type:                models.PostTypeProblem,
			Title:               "English Title",
			Description:         "English description of the problem that is long enough for validation.",
			Tags:                []string{"docker", "linux"},
			PostedByType:        models.AuthorTypeAgent,
			PostedByID:          "bot-1",
			Status:              models.PostStatusOpen,
			CreatedAt:           now,
			UpdatedAt:           now,
			OriginalLanguage:    "Chinese",
			OriginalTitle:       "中文标题",
			OriginalDescription: "中文描述，这是一个关于Docker的问题。",
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeAgent,
			ID:          "bot-1",
			DisplayName: "bot-1",
		},
		AgentHumanID: agentHumanID,
	}
}

// TestGetPost_TranslationSwap_DirectAuthor verifies the agent author sees original language.
func TestGetPost_TranslationSwap_DirectAuthor(t *testing.T) {
	mockRepo := NewMockPostsRepository()
	post := createTranslatedAgentPost("human-123")
	mockRepo.SetPost(&post)

	handler := NewPostsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/translated-post-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "translated-post-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Authenticate as the agent author
	agent := &models.Agent{ID: "bot-1", DisplayName: "bot-1"}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})

	// Agent author should see Chinese (swapped)
	if data["title"] != "中文标题" {
		t.Errorf("expected title '中文标题' (swapped), got %v", data["title"])
	}
	// original_title should now contain the English version
	if data["original_title"] != "English Title" {
		t.Errorf("expected original_title 'English Title', got %v", data["original_title"])
	}
}

// TestGetPost_TranslationSwap_AgentOwner verifies the human who claimed the agent sees original language.
func TestGetPost_TranslationSwap_AgentOwner(t *testing.T) {
	mockRepo := NewMockPostsRepository()
	post := createTranslatedAgentPost("human-123")
	mockRepo.SetPost(&post)

	handler := NewPostsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/translated-post-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "translated-post-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Authenticate as the human owner of the agent
	req = addAuthContext(req, "human-123", "user")

	w := httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})

	// Agent owner (human) should see Chinese (swapped)
	if data["title"] != "中文标题" {
		t.Errorf("expected title '中文标题' (swapped for agent owner), got %v", data["title"])
	}
	if data["original_title"] != "English Title" {
		t.Errorf("expected original_title 'English Title', got %v", data["original_title"])
	}

	// AgentHumanID must NOT be in the JSON response (json:"-")
	if _, exists := data["agent_human_id"]; exists {
		t.Error("agent_human_id should not be in JSON response (json:\"-\")")
	}
}

// TestGetPost_TranslationSwap_NonOwnerHuman verifies a random human sees English (no swap).
func TestGetPost_TranslationSwap_NonOwnerHuman(t *testing.T) {
	mockRepo := NewMockPostsRepository()
	post := createTranslatedAgentPost("human-123")
	mockRepo.SetPost(&post)

	handler := NewPostsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/translated-post-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "translated-post-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Authenticate as a different human (NOT the agent's owner)
	req = addAuthContext(req, "different-human-uuid", "user")

	w := httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})

	// Non-owner human should see English (NOT swapped)
	if data["title"] != "English Title" {
		t.Errorf("expected title 'English Title' (no swap), got %v", data["title"])
	}
	if data["original_title"] != "中文标题" {
		t.Errorf("expected original_title '中文标题', got %v", data["original_title"])
	}
}

// TestGetPost_TranslationSwap_Anonymous verifies anonymous viewers see English (no swap).
func TestGetPost_TranslationSwap_Anonymous(t *testing.T) {
	mockRepo := NewMockPostsRepository()
	post := createTranslatedAgentPost("human-123")
	mockRepo.SetPost(&post)

	handler := NewPostsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/translated-post-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "translated-post-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// No auth context — anonymous viewer

	w := httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})

	// Anonymous should see English (NOT swapped)
	if data["title"] != "English Title" {
		t.Errorf("expected title 'English Title' (no swap), got %v", data["title"])
	}
	if data["original_title"] != "中文标题" {
		t.Errorf("expected original_title '中文标题', got %v", data["original_title"])
	}
}
