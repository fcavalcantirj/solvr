package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task idx 76 step 3 (feature:embedding-workers): the canonical reply API embeds the
// reply body on create and edit, like posts, so replies are semantically searchable
// through hybrid_search_replies. An embedding failure never fails the write, and an
// edit whose vector could not be computed clears the stale one.

// embeddingRecordingRepo records the vector the handler hands to Update.
type embeddingRecordingRepo struct {
	MockRepliesRepository
	updateCalls     int
	updateEmbedding *string
}

func (m *embeddingRecordingRepo) Update(ctx context.Context, id string, authorType models.AuthorType, authorID, body string, embedding *string) (*models.Reply, error) {
	m.updateCalls++
	m.updateEmbedding = embedding
	return m.MockRepliesRepository.Update(ctx, id, authorType, authorID, body, embedding)
}

func TestReplies_CreateEmbedsTheBody(t *testing.T) {
	mock := &MockRepliesRepository{}
	embed := &MockEmbeddingService{embedding: []float32{0.5, 0.25}}
	h := NewRepliesHandler(mock)
	h.SetEmbeddingService(embed)

	body := "tried pinning the HNSW ef_search and recall recovered"
	rec := httptest.NewRecorder()
	h.Create(rec, agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": body}, map[string]string{"id": "p1"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if embed.callCount != 1 || embed.lastInput != body {
		t.Fatalf("embedding calls = %d with %q, want 1 with the reply body", embed.callCount, embed.lastInput)
	}
	if mock.lastCreated == nil || mock.lastCreated.EmbeddingStr == nil || *mock.lastCreated.EmbeddingStr != "[0.5,0.25]" {
		t.Fatalf("reply handed to the repository carries embedding %v, want [0.5,0.25]", mock.lastCreated)
	}
}

func TestReplies_CreateStillSucceedsWhenEmbeddingFails(t *testing.T) {
	mock := &MockRepliesRepository{}
	h := NewRepliesHandler(mock)
	h.SetEmbeddingService(&MockEmbeddingService{err: errors.New("voyage unavailable")})

	rec := httptest.NewRecorder()
	h.Create(rec, agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "a reply"}, map[string]string{"id": "p1"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 even when embedding fails; body=%s", rec.Code, rec.Body.String())
	}
	if mock.lastCreated == nil || mock.lastCreated.EmbeddingStr != nil {
		t.Fatal("a failed embedding must store no vector")
	}
}

func TestReplies_CreateOnHiddenPostEmbedsNothing(t *testing.T) {
	embed := &MockEmbeddingService{embedding: []float32{1}}
	h := NewRepliesHandler(&MockRepliesRepository{postHidden: true})
	h.SetEmbeddingService(embed)

	rec := httptest.NewRecorder()
	h.Create(rec, agentReq(http.MethodPost, "/v1/posts/p1/replies", map[string]any{"body": "a reply"}, map[string]string{"id": "p1"}))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if embed.callCount != 0 {
		t.Fatalf("embedding calls = %d for a refused reply, want 0", embed.callCount)
	}
}

func TestReplies_UpdateReembedsTheEditedBody(t *testing.T) {
	mock := &embeddingRecordingRepo{}
	embed := &MockEmbeddingService{embedding: []float32{0.75}}
	h := NewRepliesHandler(mock)
	h.SetEmbeddingService(embed)

	rec := httptest.NewRecorder()
	h.Update(rec, agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited body"}, map[string]string{"id": "r1"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if embed.callCount != 1 || embed.lastInput != "edited body" {
		t.Fatalf("embedding calls = %d with %q, want 1 with the edited body", embed.callCount, embed.lastInput)
	}
	if mock.updateEmbedding == nil || *mock.updateEmbedding != "[0.75]" {
		t.Fatalf("Update received embedding %v, want [0.75]", mock.updateEmbedding)
	}
}

func TestReplies_UpdateClearsTheVectorWhenEmbeddingFails(t *testing.T) {
	mock := &embeddingRecordingRepo{}
	h := NewRepliesHandler(mock)
	h.SetEmbeddingService(&MockEmbeddingService{err: errors.New("voyage unavailable")})

	rec := httptest.NewRecorder()
	h.Update(rec, agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "edited body"}, map[string]string{"id": "r1"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even when embedding fails; body=%s", rec.Code, rec.Body.String())
	}
	if mock.updateCalls != 1 || mock.updateEmbedding != nil {
		t.Fatalf("Update calls = %d with embedding %v, want 1 with nil (clear the stale vector)", mock.updateCalls, mock.updateEmbedding)
	}
}

func TestReplies_UpdateByNonAuthorEmbedsNothing(t *testing.T) {
	mock := &embeddingRecordingRepo{MockRepliesRepository: MockRepliesRepository{
		getResult: &models.ReplyWithAuthor{Reply: models.Reply{ID: "r1", PostID: "p1", AuthorType: models.AuthorTypeAgent, AuthorID: "agent-2"}},
	}}
	embed := &MockEmbeddingService{embedding: []float32{1}}
	h := NewRepliesHandler(mock)
	h.SetEmbeddingService(embed)

	rec := httptest.NewRecorder()
	h.Update(rec, agentReq(http.MethodPatch, "/v1/replies/r1", map[string]any{"body": "hijack"}, map[string]string{"id": "r1"}))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if embed.callCount != 0 || mock.updateCalls != 0 {
		t.Fatalf("embedding calls = %d, Update calls = %d for a non-author edit, want 0 and 0", embed.callCount, mock.updateCalls)
	}
}
