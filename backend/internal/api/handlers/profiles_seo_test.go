package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProfileContent answers a profile's verdict and counts by id.
type fakeProfileContent struct {
	agents map[string]models.ProfileContent
	users  map[string]models.ProfileContent
	err    error
}

func (f *fakeProfileContent) AgentContent(_ context.Context, id string) (models.ProfileContent, error) {
	if f.err != nil {
		return models.ProfileContent{}, f.err
	}
	return f.agents[id], nil
}

func (f *fakeProfileContent) UserContent(_ context.Context, id string) (models.ProfileContent, error) {
	if f.err != nil {
		return models.ProfileContent{}, f.err
	}
	return f.users[id], nil
}

const profileUserID = "6bc3eb04-e6a9-4579-bcdf-000000000001"

func profileSEOServer(t *testing.T, content *fakeProfileContent) http.Handler {
	t.Helper()
	agents := NewMockAgentRepository()
	agents.agents["agent_one"] = &models.Agent{ID: "agent_one", DisplayName: "Dev Nine", Bio: "Plans **Go** services.", Email: "owner@example.com"}
	agents.agents["agent_mail"] = &models.Agent{ID: "agent_mail", DisplayName: "bot@example.com"}
	users := NewMockUsersUserRepository()
	users.users[profileUserID] = &models.User{ID: profileUserID, Username: "felipe", DisplayName: "felipe@example.com", Bio: "Reviews plans."}
	h := NewProfileSEOHandler(agents, users, content)
	r := chi.NewRouter()
	r.Get("/v1/agents/{id}/seo", h.GetAgentSEO)
	r.Get("/v1/users/{id}/seo", h.GetUserSEO)
	return r
}

func getProfileSEO(t *testing.T, srv http.Handler, path string) (int, ProfileSEO, string) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body struct {
		Data  ProfileSEO `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	if w.Code != http.StatusOK {
		return w.Code, ProfileSEO{}, body.Error.Code
	}
	return w.Code, body.Data, w.Body.String()
}

// SPEC.md 27.1: the agent profile's verdict, title and description come from the API.
func TestGetAgentSEO_AnswersTheVerdictTitleAndDescription(t *testing.T) {
	srv := profileSEOServer(t, &fakeProfileContent{agents: map[string]models.ProfileContent{
		"agent_one": {Indexable: true, Posts: 12, Replies: 30, Rooms: 3},
	}})
	status, seo, raw := getProfileSEO(t, srv, "/v1/agents/agent_one/seo")
	require.Equal(t, http.StatusOK, status, raw)
	assert.True(t, seo.Indexable)
	assert.Equal(t, "Dev Nine (AI agent)", seo.Title)
	assert.Equal(t, "Dev Nine, an AI agent on Solvr: 12 posts, 30 replies and 3 rooms. Plans Go services.", seo.Description)
	assert.NotContains(t, raw, "owner@example.com")
}

func TestGetAgentSEO_IsNotIndexableWithoutContent(t *testing.T) {
	srv := profileSEOServer(t, &fakeProfileContent{})
	status, seo, raw := getProfileSEO(t, srv, "/v1/agents/agent_one/seo")
	require.Equal(t, http.StatusOK, status, raw)
	assert.False(t, seo.Indexable)
	assert.Equal(t, "Dev Nine, an AI agent on Solvr. Plans Go services.", seo.Description)
}

// No e-mail address is a title: an agent named by one is named by its id.
func TestGetAgentSEO_NeverTitlesAnAgentWithAnEmail(t *testing.T) {
	srv := profileSEOServer(t, &fakeProfileContent{})
	status, seo, raw := getProfileSEO(t, srv, "/v1/agents/agent_mail/seo")
	require.Equal(t, http.StatusOK, status, raw)
	assert.Equal(t, "agent_mail (AI agent)", seo.Title)
	assert.NotContains(t, raw, "bot@example.com")
}

// The verdict answers 404 exactly when GET /v1/agents/{id} does, and a failure is a
// retryable 500, never a false "not indexable" (SPEC.md 27.4).
func TestGetAgentSEO_RefusesLikeTheProfileAndFailsRetryably(t *testing.T) {
	status, _, code := getProfileSEO(t, profileSEOServer(t, &fakeProfileContent{}), "/v1/agents/nobody/seo")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "NOT_FOUND", code)

	status, _, code = getProfileSEO(t, profileSEOServer(t, &fakeProfileContent{err: errors.New("db down")}), "/v1/agents/agent_one/seo")
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "INTERNAL_ERROR", code)
}

// A person whose stored name is an e-mail address is titled and described by their username.
func TestGetUserSEO_NamesThePersonWithoutTheirEmail(t *testing.T) {
	srv := profileSEOServer(t, &fakeProfileContent{users: map[string]models.ProfileContent{
		profileUserID: {Indexable: true, Posts: 1, Rooms: 2},
	}})
	status, seo, raw := getProfileSEO(t, srv, "/v1/users/"+profileUserID+"/seo")
	require.Equal(t, http.StatusOK, status, raw)
	assert.True(t, seo.Indexable)
	assert.Equal(t, "felipe", seo.Title)
	assert.Equal(t, "felipe on Solvr: 1 post and 2 rooms. Reviews plans.", seo.Description)
	assert.NotContains(t, raw, "felipe@example.com")
}

func TestGetUserSEO_RefusesLikeTheProfileAndFailsRetryably(t *testing.T) {
	srv := profileSEOServer(t, &fakeProfileContent{})
	status, _, code := getProfileSEO(t, srv, "/v1/users/not-a-uuid/seo")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "BAD_REQUEST", code)
	status, _, code = getProfileSEO(t, srv, "/v1/users/6bc3eb04-e6a9-4579-bcdf-00000000ffff/seo")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "NOT_FOUND", code)

	status, _, code = getProfileSEO(t, profileSEOServer(t, &fakeProfileContent{err: errors.New("db down")}), "/v1/users/"+profileUserID+"/seo")
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "INTERNAL_ERROR", code)
}
