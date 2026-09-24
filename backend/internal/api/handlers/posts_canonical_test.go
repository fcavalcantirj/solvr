package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// canonicalBody builds a valid create-post request body without a type.
func canonicalBody(extra map[string]interface{}) []byte {
	body := map[string]interface{}{
		"title":       "A Canonical Post Title Long Enough",
		"description": "This canonical post body is at least fifty characters long so it passes validation.",
		"tags":        []string{"solvr", "canonical"},
	}
	for k, v := range extra {
		body[k] = v
	}
	b, _ := json.Marshal(body)
	return b
}

// TestCreatePost_NoTypeDefaultsToCanonical verifies POST /v1/posts succeeds without a
// type and produces a canonical untyped post (BART-583, step 3).
func TestCreatePost_NoTypeDefaultsToCanonical(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(nil)))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating a post without a type, got %d: %s", w.Code, w.Body.String())
	}
	if repo.createdPost.Type != models.PostTypePost {
		t.Errorf("expected canonical type %q, got %q", models.PostTypePost, repo.createdPost.Type)
	}
}

// TestCreatePost_UnknownTypeStillRejected verifies a provided but invalid type is still
// rejected even though type is optional.
func TestCreatePost_UnknownTypeStillRejected(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(map[string]interface{}{"type": "not-a-type"})))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown type, got %d", w.Code)
	}
}

// TestCreatePost_PublicStartsPendingModeration verifies a public post is created
// pending moderation and is not yet publicly eligible (BART-583, step 2).
func TestCreatePost_PublicStartsPendingModeration(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(map[string]interface{}{"visibility": "public"})))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if repo.createdPost.ModerationState != models.ModerationPending {
		t.Errorf("public post moderation_state = %q, want pending", repo.createdPost.ModerationState)
	}
	if repo.createdPost.PublicEligible() {
		t.Error("a public post pending moderation must not be publicly eligible")
	}
}

// TestCreatePost_ModerationStateNotSettableByAuthor verifies an author cannot approve
// their own post by putting moderation_state in the request (BART-583, step 2).
func TestCreatePost_ModerationStateNotSettableByAuthor(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(map[string]interface{}{
		"visibility":       "public",
		"moderation_state": "approved",
		"publication_state": "published",
	})))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if repo.createdPost.ModerationState == models.ModerationApproved {
		t.Error("author must not be able to set moderation_state=approved via the request")
	}
	if repo.createdPost.PublicEligible() {
		t.Error("author-supplied publication/moderation must not make a pending post publicly eligible")
	}
}

// TestCreatePost_SourceRoomIDPassedThrough verifies optional room provenance is carried
// to storage (BART-583, step 1).
func TestCreatePost_SourceRoomIDPassedThrough(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	roomID := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(map[string]interface{}{"source_room_id": roomID})))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if repo.createdPost.SourceRoomID == nil || *repo.createdPost.SourceRoomID != roomID {
		t.Errorf("source_room_id not carried through: got %v", repo.createdPost.SourceRoomID)
	}
}

// TestCreatePost_ResponseExposesCanonicalFields verifies the create response serializes
// the canonical publication_state and moderation_state fields (BART-583, step 1).
func TestCreatePost_ResponseExposesCanonicalFields(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(nil)))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("no data object in response: %s", w.Body.String())
	}
	for _, field := range []string{"publication_state", "moderation_state"} {
		if _, present := data[field]; !present {
			t.Errorf("create response missing canonical field %q", field)
		}
	}
}

// TestCreatePost_AgentWriterNeedsNoHumanAccount verifies an authenticated agent can
// create a public canonical post without a human account (BART-583, step 4).
func TestCreatePost_AgentWriterNeedsNoHumanAccount(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(canonicalBody(map[string]interface{}{"visibility": "public"})))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentContext(req, "agent-777")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for an agent writer without a human account, got %d: %s", w.Code, w.Body.String())
	}
	if repo.createdPost.PostedByType != models.AuthorTypeAgent {
		t.Errorf("expected agent author, got %q", repo.createdPost.PostedByType)
	}
}
