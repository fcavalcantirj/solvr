package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockUnsubscribeRepo tracks UnsubscribeByEmail calls.
type mockUnsubscribeRepo struct {
	calledWith string
	err        error
}

func (m *mockUnsubscribeRepo) UnsubscribeByEmail(ctx context.Context, email string) error {
	m.calledWith = email
	return m.err
}

func TestUnsubscribe_Success(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	handler := NewUnsubscribeHandler(repo, "test-secret-key")

	email := "alice@example.com"
	token := GenerateUnsubscribeToken(email, "test-secret-key")

	req := httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe?email="+email+"&token="+token, nil)
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "unsubscribed" {
		t.Errorf("expected status=unsubscribed, got %s", resp["status"])
	}
	if repo.calledWith != email {
		t.Errorf("expected repo called with %s, got %s", email, repo.calledWith)
	}
}

func TestUnsubscribe_InvalidToken(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	handler := NewUnsubscribeHandler(repo, "test-secret-key")

	req := httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe?email=alice@example.com&token=bad-token", nil)
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
	if repo.calledWith != "" {
		t.Error("repo should NOT be called with invalid token")
	}
}

func TestUnsubscribe_MissingParams(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	handler := NewUnsubscribeHandler(repo, "test-secret-key")

	// Missing both
	req := httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe", nil)
	rr := httptest.NewRecorder()
	handler.Unsubscribe(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing params, got %d", rr.Code)
	}

	// Missing token
	req = httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe?email=a@b.com", nil)
	rr = httptest.NewRecorder()
	handler.Unsubscribe(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing token, got %d", rr.Code)
	}
}

func TestUnsubscribe_RepoError(t *testing.T) {
	repo := &mockUnsubscribeRepo{err: fmt.Errorf("db error")}
	handler := NewUnsubscribeHandler(repo, "test-secret-key")

	email := "alice@example.com"
	token := GenerateUnsubscribeToken(email, "test-secret-key")

	req := httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe?email="+email+"&token="+token, nil)
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on repo error, got %d", rr.Code)
	}
}

func TestUnsubscribe_DifferentKeyRejectsToken(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	handler := NewUnsubscribeHandler(repo, "production-key")

	email := "alice@example.com"
	token := GenerateUnsubscribeToken(email, "different-key") // signed with wrong key

	req := httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe?email="+email+"&token="+token, nil)
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for token signed with different key, got %d", rr.Code)
	}
}

func TestGenerateUnsubscribeToken_Deterministic(t *testing.T) {
	token1 := GenerateUnsubscribeToken("alice@example.com", "key")
	token2 := GenerateUnsubscribeToken("alice@example.com", "key")
	if token1 != token2 {
		t.Error("same email+key should produce same token")
	}

	token3 := GenerateUnsubscribeToken("bob@example.com", "key")
	if token1 == token3 {
		t.Error("different emails should produce different tokens")
	}
}
