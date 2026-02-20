package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock Repository ---

type mockFollowsRepo struct {
	follows        []models.Follow
	followResult   *models.Follow
	isFollowing    bool
	followersCount int
	followingCount int
	createErr      error
	deleteErr      error
	listErr        error
	isFollowingErr error
	countErr       error
}

func (m *mockFollowsRepo) Create(_ context.Context, followerType, followerID, followedType, followedID string) (*models.Follow, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.followResult != nil {
		return m.followResult, nil
	}
	return &models.Follow{
		ID:           "follow-1",
		FollowerType: followerType,
		FollowerID:   followerID,
		FollowedType: followedType,
		FollowedID:   followedID,
	}, nil
}

func (m *mockFollowsRepo) Delete(_ context.Context, _, _, _, _ string) error {
	return m.deleteErr
}

func (m *mockFollowsRepo) ListFollowing(_ context.Context, _, _ string, _, _ int) ([]models.Follow, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.follows, nil
}

func (m *mockFollowsRepo) ListFollowers(_ context.Context, _, _ string, _, _ int) ([]models.Follow, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.follows, nil
}

func (m *mockFollowsRepo) IsFollowing(_ context.Context, _, _, _, _ string) (bool, error) {
	return m.isFollowing, m.isFollowingErr
}

func (m *mockFollowsRepo) CountFollowers(_ context.Context, _, _ string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.followersCount, nil
}

func (m *mockFollowsRepo) CountFollowing(_ context.Context, _, _ string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.followingCount, nil
}

// --- Auth helpers for tests ---

func addFollowsAgentAuth(r *http.Request, agentID string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

func addFollowsHumanAuth(r *http.Request, userID string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// --- Tests ---

func TestFollow_AgentFollowsAgent(t *testing.T) {
	repo := &mockFollowsRepo{}
	handler := NewFollowsHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"target_type": "agent",
		"target_id":   "other-agent",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/follow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addFollowsAgentAuth(req, "my-agent")

	rec := httptest.NewRecorder()
	handler.Follow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in response")
	}
	if data["followed_type"] != "agent" {
		t.Errorf("expected followed_type=agent, got %v", data["followed_type"])
	}
	if data["followed_id"] != "other-agent" {
		t.Errorf("expected followed_id=other-agent, got %v", data["followed_id"])
	}
}

func TestFollow_AgentFollowsUser(t *testing.T) {
	repo := &mockFollowsRepo{}
	handler := NewFollowsHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"target_type": "human",
		"target_id":   "user-123",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/follow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addFollowsAgentAuth(req, "my-agent")

	rec := httptest.NewRecorder()
	handler.Follow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in response")
	}
	if data["followed_type"] != "human" {
		t.Errorf("expected followed_type=human, got %v", data["followed_type"])
	}
}

func TestFollow_Unfollow(t *testing.T) {
	repo := &mockFollowsRepo{}
	handler := NewFollowsHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"target_type": "agent",
		"target_id":   "other-agent",
	})

	req := httptest.NewRequest(http.MethodDelete, "/v1/follow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addFollowsAgentAuth(req, "my-agent")

	rec := httptest.NewRecorder()
	handler.Unfollow(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFollow_SelfFollow(t *testing.T) {
	repo := &mockFollowsRepo{}
	handler := NewFollowsHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"target_type": "agent",
		"target_id":   "my-agent",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/follow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addFollowsAgentAuth(req, "my-agent")

	rec := httptest.NewRecorder()
	handler.Follow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error in response")
	}
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR code, got %v", errObj["code"])
	}
}

func TestFollow_ListFollowing(t *testing.T) {
	repo := &mockFollowsRepo{
		follows: []models.Follow{
			{ID: "f1", FollowerType: "agent", FollowerID: "my-agent", FollowedType: "agent", FollowedID: "agent-a"},
			{ID: "f2", FollowerType: "agent", FollowerID: "my-agent", FollowedType: "human", FollowedID: "user-b"},
		},
		followingCount: 2,
	}
	handler := NewFollowsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/following?limit=20&offset=0", nil)
	req = addFollowsAgentAuth(req, "my-agent")

	rec := httptest.NewRecorder()
	handler.ListFollowing(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []models.Follow `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 follows, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Meta.Total)
	}
}

func TestFollow_Unauthenticated(t *testing.T) {
	repo := &mockFollowsRepo{}
	handler := NewFollowsHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"target_type": "agent",
		"target_id":   "other-agent",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/follow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No auth context set

	rec := httptest.NewRecorder()
	handler.Follow(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
