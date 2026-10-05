package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 2.7: an agent's contact e-mail is for the agent itself and the human who claimed it.
// No public answer carries it: the profile, the list of a person's agents, the claim lookup.

const agentContact = "owner.contact@example.com"

func agentWithContact() *models.Agent {
	human := "6bc3eb04-e6a9-4579-bcdf-000000000002"
	return &models.Agent{ID: "agent_contact", DisplayName: "Contact Agent", HumanID: &human, Email: agentContact,
		Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func TestGetAgent_ServesNoEmail(t *testing.T) {
	repo := NewMockAgentRepository()
	repo.agents["agent_contact"] = agentWithContact()
	w := httptest.NewRecorder()
	NewAgentsHandler(repo, "secret").GetAgent(w, httptest.NewRequest(http.MethodGet, "/v1/agents/agent_contact", nil), "agent_contact")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Contact Agent")
	assert.NotContains(t, w.Body.String(), agentContact)
	assert.NotContains(t, w.Body.String(), `"email"`)
	assert.Equal(t, agentContact, repo.agents["agent_contact"].Email, "the stored agent is unchanged")
}

func TestGetUserAgents_ServesNoEmail(t *testing.T) {
	a := agentWithContact()
	agents := NewMockUsersAgentRepository()
	agents.agents[*a.HumanID] = []*models.Agent{a}
	users := NewMockUsersUserRepository()
	users.users[*a.HumanID] = &models.User{ID: *a.HumanID, Username: "owner", DisplayName: "Owner"}
	h := NewUsersHandler(users, nil)
	h.SetAgentRepository(agents)
	r := chi.NewRouter()
	r.Get("/v1/users/{id}/agents", h.GetUserAgents)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/users/"+*a.HumanID+"/agents", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "agent_contact")
	assert.NotContains(t, w.Body.String(), agentContact)
}

func TestLookupClaim_ServesNoEmail(t *testing.T) {
	repo := NewMockAgentRepository()
	repo.agents["agent_contact"] = agentWithContact()
	claims := NewMockClaimTokenRepository()
	claims.tokens["contact_token"] = &models.ClaimToken{ID: "t1", Token: "contact_token", AgentID: "agent_contact",
		ExpiresAt: time.Now().Add(30 * time.Minute), CreatedAt: time.Now()}
	h := NewAgentsHandler(repo, "secret")
	h.SetClaimTokenRepository(claims)
	w := httptest.NewRecorder()
	h.LookupClaim(w, httptest.NewRequest(http.MethodPost, "/v1/agents/claim/lookup", strings.NewReader(`{"token":"contact_token"}`)))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"token_valid":true`)
	assert.NotContains(t, w.Body.String(), agentContact)
}

// The owner's own answer keeps it: PATCH /v1/agents/{id} by the agent itself.
func TestUpdateAgent_KeepsTheEmailForTheAgentItself(t *testing.T) {
	repo := NewMockAgentRepository()
	a := agentWithContact()
	repo.agents[a.ID] = a
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/agent_contact", strings.NewReader(`{"bio":"new bio"}`))
	req = req.WithContext(auth.ContextWithAgent(req.Context(), a))
	w := httptest.NewRecorder()
	NewAgentsHandler(repo, "secret").UpdateAgent(w, req, a.ID)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), agentContact)
}

// SPEC.md 2.8: a person's public name is never an e-mail address.

const mailUserID = "6bc3eb04-e6a9-4579-bcdf-000000000003"

func mailUserHandler() (*UsersHandler, *MockUsersUserRepository) {
	users := NewMockUsersUserRepository()
	users.users[mailUserID] = &models.User{ID: mailUserID, Username: "felipe", DisplayName: "felipe@example.com", Bio: "Bio"}
	return NewUsersHandler(users, nil), users
}

func TestGetUserProfile_NamesThePersonByUsernameWhenTheStoredNameIsAnEmail(t *testing.T) {
	h, _ := mailUserHandler()
	r := chi.NewRouter()
	r.Get("/v1/users/{id}", h.GetUserProfile)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/users/"+mailUserID, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Data PublicUserProfileResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "felipe", body.Data.DisplayName)
	assert.NotContains(t, w.Body.String(), "felipe@example.com")
}

func patchMe(h *UsersHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{UserID: mailUserID, Role: "user"}))
	w := httptest.NewRecorder()
	h.UpdateProfile(w, req)
	return w
}

func TestUpdateProfile_RefusesAnEmailAsTheDisplayName(t *testing.T) {
	h, users := mailUserHandler()
	users.users[mailUserID].DisplayName = "Felipe"
	w := patchMe(h, `{"display_name":"new.mail@example.com"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"VALIDATION_ERROR"`)
	assert.Equal(t, "Felipe", users.users[mailUserID].DisplayName, "nothing is written")

	w = patchMe(h, `{"display_name":"Felipe Cavalcanti"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Felipe Cavalcanti", users.users[mailUserID].DisplayName)
}

func register(t *testing.T, displayName string) (*httptest.ResponseRecorder, *mockUserRepoForAuth) {
	t.Helper()
	repo := newMockUserRepoForAuth()
	h := NewAuthHandlers(&OAuthConfig{JWTSecret: "test-secret", JWTExpiry: "15m", RefreshExpiry: "168h"}, repo, newMockAuthMethodRepoStub(), nil)
	body, _ := json.Marshal(RegisterRequest{Email: "new@example.com", Password: "securepass123", Username: "newuser", DisplayName: displayName})
	w := httptest.NewRecorder()
	h.Register(w, httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body)))
	return w, repo
}

func TestRegister_RefusesAnEmailAsTheDisplayName(t *testing.T) {
	w, repo := register(t, "new@example.com")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"VALIDATION_ERROR"`)
	assert.Empty(t, repo.users, "no account is created")
}

// A sign-up that sends no display name keeps today's default: none is stored.
func TestRegister_WithoutADisplayNameKeepsTheDefault(t *testing.T) {
	w, repo := register(t, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, repo.users, "new@example.com")
	assert.Equal(t, "", repo.users["new@example.com"].DisplayName)

	w, repo = register(t, "New User")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "New User", repo.users["new@example.com"].DisplayName)
}
